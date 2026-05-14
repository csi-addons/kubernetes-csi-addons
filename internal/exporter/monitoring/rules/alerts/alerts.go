/*
Copyright 2025 The Kubernetes-CSI-Addons Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package alerts defines Prometheus alert rules for the CSI volume device exporter.
package alerts

import (
	"fmt"
	"os"
	"strings"
)

const (
	runbookAnnotationKey = "runbook_url"

	RunbookURLTemplateEnv = "RUNBOOK_URL_TEMPLATE"

	defaultRunbookURLTemplate = "https://github.com/csi-addons/kubernetes-csi-addons/blob/main/docs/csi-volume-device-exporter/runbooks/%s.md"
)

func runbookURL(alertName string) string {
	tpl := os.Getenv(RunbookURLTemplateEnv)
	if tpl == "" || strings.Count(tpl, "%s") != 1 {
		tpl = defaultRunbookURLTemplate
	}
	return fmt.Sprintf(tpl, alertName)
}

type Alert struct {
	Name        string
	Expr        string
	For         string
	Labels      map[string]string
	Annotations map[string]string
}

func All() []Alert {
	var all []Alert
	all = append(all, multipathAlerts()...)
	all = append(all, nvmeSubsystemAlerts()...)
	all = append(all, exporterAlerts()...)
	return all
}

func exporterAlerts() []Alert {
	return []Alert{
		{
			Name: "CSIVolumeDeviceExporterDown",
			Expr: `absent(up{job="csi-volume-device-exporter"}) == 1`,
			For:  "5m",
			Labels: map[string]string{
				"severity":              "warning",
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

func multipathAlerts() []Alert {
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
