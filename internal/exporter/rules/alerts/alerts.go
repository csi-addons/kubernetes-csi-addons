// Package alerts defines Prometheus alert rules for the CSI volume device exporter.
package alerts

import (
	"fmt"
	"os"
	"strings"
)

const (
	runbookAnnotationKey = "runbook_url"

	// RunbookURLTemplateEnv is the environment variable that overrides the
	// runbook URL template at runtime. The single %s verb is replaced with
	// the alert name.
	//
	// Example:
	//   RUNBOOK_URL_TEMPLATE=https://example.com/runbooks/%s.md
	RunbookURLTemplateEnv = "RUNBOOK_URL_TEMPLATE"

	defaultRunbookURLTemplate = "https://github.com/csi-addons/kubernetes-csi-addons/blob/main/docs/volume-device-exporter/runbooks/%s.md"
)

// runbookURL returns the runbook URL for the named alert.
// The RUNBOOK_URL_TEMPLATE env var takes precedence over the compiled-in default.
// If the template does not contain exactly one %s verb, the default is used.
func runbookURL(alertName string) string {
	tpl := os.Getenv(RunbookURLTemplateEnv)
	if tpl == "" || strings.Count(tpl, "%s") != 1 {
		tpl = defaultRunbookURLTemplate
	}
	return fmt.Sprintf(tpl, alertName)
}

// Alert is a typed Prometheus alerting rule definition.
type Alert struct {
	// Name is the alert name used in the `alert:` field.
	Name string
	// Expr is the PromQL expression that triggers the alert.
	Expr string
	// For is the minimum duration the condition must be true before firing.
	For string
	// Labels are static labels added to every firing alert instance.
	Labels map[string]string
	// Annotations are human-readable metadata attached to firing alerts.
	Annotations map[string]string
}

// All returns every alert rule defined in this package.
// New alert groups should be added here.
func All() []Alert {
	var all []Alert
	all = append(all, multipathAlerts()...)
	all = append(all, nvmeSubsystemAlerts()...)
	all = append(all, exporterAlerts()...)
	return all
}

// exporterAlerts covers the operational health of the exporter itself.
// These alert on infrastructure gaps that prevent CSIVolumeMultipathDegraded
// from firing when it should, so they have operator_health_impact: warning.
func exporterAlerts() []Alert {
	return []Alert{
		{
			Name: "CSIVolumeDeviceExporterDown",
			// absent() fires only when the job has completely disappeared from
			// Prometheus (no targets at all), unlike up==0 which fires on every
			// failed scrape including rolling updates and transient restarts.
			Expr: `absent(up{job="csi-volume-device-exporter"}) == 1`,
			For:  "5m",
			Labels: map[string]string{
				"severity":               "warning",
				"operator_health_impact": "warning",
			},
			Annotations: map[string]string{
				"summary":            "CSI volume device exporter is not running",
				"description":        "No csi-volume-device-exporter targets have been scraped for 5 minutes. Storage path health for CSI volumes cannot be reported.",
				runbookAnnotationKey: runbookURL("CSIVolumeDeviceExporterDown"),
			},
		},
	}
}

// multipathAlerts covers DM-multipath storage path health for CSI volumes.
func multipathAlerts() []Alert {
	// pvToMultipathExpr is the common three-way join: PV → CSI device → multipath device.
	// It produces a result set with labels: persistentvolume, device (mpathX), node, driver.
	const pvToMultipathExpr = `label_replace(
  label_replace(
    kube_persistentvolume_info,
    "volume_handle", "$1", "csi_volume_handle", "(.+)"
  )
  * on(volume_handle) group_left(device, node, driver)
  csi_volume_node_device_info,
  "sysfs_name", "$1", "device", "(.*)"
)
* on(sysfs_name, node) group_left(device)
node_dmmultipath_device_info`

	return []Alert{
		{
			Name: "CSIVolumeMultipathDegraded",
			Expr: `(` + pvToMultipathExpr + `)
* on(device, node) group_left()
(
  count by(device, node) (node_dmmultipath_path_state{state!~"running|live"} == 1) > 0
)`,
			For: "5m",
			Labels: map[string]string{
				"severity": "warning",
			},
			Annotations: map[string]string{
				"summary":            "PV {{ $labels.persistentvolume }} on {{ $labels.node }} has degraded multipath (device {{ $labels.device }})",
				"description":        "At least one path to the multipath device has failed. Storage is still accessible via remaining paths but redundancy is lost.",
				runbookAnnotationKey: runbookURL("CSIVolumeMultipathDegraded"),
			},
		},
		{
			Name: "CSIVolumeMultipathLost",
			Expr: `(` + pvToMultipathExpr + `)
* on(device, node) group_left()
(
  node_dmmultipath_device_paths_active == 0
)`,
			For: "1m",
			Labels: map[string]string{
				"severity": "critical",
			},
			Annotations: map[string]string{
				"summary":            "PV {{ $labels.persistentvolume }} on {{ $labels.node }} has lost all multipath paths (device {{ $labels.device }})",
				"description":        "All paths to the multipath device are down. I/O is likely failing. Immediate investigation of storage connectivity on node {{ $labels.node }} is required.",
				runbookAnnotationKey: runbookURL("CSIVolumeMultipathLost"),
			},
		},
	}
}

// nvmeSubsystemAlerts covers NVMe-oF subsystem path health for CSI volumes.
// The join uses node_nvmesubsystem_namespace_info to map the NVMe namespace
// device (e.g. nvme0n1) from csi_volume_node_device_info to its subsystem.
func nvmeSubsystemAlerts() []Alert {
	const pvToNVMeExpr = `label_replace(
  kube_persistentvolume_info,
  "volume_handle", "$1", "csi_volume_handle", "(.+)"
)
* on(volume_handle) group_left(device, node, driver)
csi_volume_node_device_info{device=~"nvme.*"}
* on(device, node) group_left(subsystem)
node_nvmesubsystem_namespace_info`

	return []Alert{
		{
			Name: "CSIVolumeNVMeSubsystemDegraded",
			Expr: `(` + pvToNVMeExpr + `)
* on(subsystem) group_left()
(
  node_nvmesubsystem_paths - node_nvmesubsystem_paths_live > 0
)`,
			For: "5m",
			Labels: map[string]string{
				"severity": "warning",
			},
			Annotations: map[string]string{
				"summary":            "PV {{ $labels.persistentvolume }} on {{ $labels.node }} has degraded NVMe-oF subsystem ({{ $labels.subsystem }})",
				"description":        "At least one NVMe-oF controller path is not live. Storage is still accessible via remaining controllers but redundancy is lost.",
				runbookAnnotationKey: runbookURL("CSIVolumeNVMeSubsystemDegraded"),
			},
		},
		{
			Name: "CSIVolumeNVMeSubsystemLost",
			Expr: `(` + pvToNVMeExpr + `)
* on(subsystem) group_left()
(
  node_nvmesubsystem_paths_live == 0
)`,
			For: "1m",
			Labels: map[string]string{
				"severity": "critical",
			},
			Annotations: map[string]string{
				"summary":            "PV {{ $labels.persistentvolume }} on {{ $labels.node }} has lost all NVMe-oF controller paths ({{ $labels.subsystem }})",
				"description":        "All NVMe-oF controller paths are dead. I/O is likely failing. Immediate investigation of storage connectivity on node {{ $labels.node }} is required.",
				runbookAnnotationKey: runbookURL("CSIVolumeNVMeSubsystemLost"),
			},
		},
	}
}
