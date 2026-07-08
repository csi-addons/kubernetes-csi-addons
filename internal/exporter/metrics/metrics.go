// Package metrics provides Prometheus metrics for the CSI volume device exporter.
package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/csi-addons/kubernetes-csi-addons/internal/exporter/discovery"
)

// Metrics holds all Prometheus metrics for the exporter.
// All fields are unexported; mutation must go through the provided methods
// to keep internal bookkeeping (e.g. stale-series tracking) consistent.
type Metrics struct {
	volumeDeviceInfo  *prometheus.GaugeVec
	discoveryErrors   *prometheus.CounterVec
	volumesDiscovered *prometheus.GaugeVec
	lastSuccessful    prometheus.Gauge

	registry *prometheus.Registry

	mu              sync.Mutex
	previous        map[string]prometheus.Labels
	previousDrivers map[string]struct{}
}

// New creates a new Metrics instance with a custom registry.
func New() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		volumeDeviceInfo: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "csi_volume_node_device_info",
			Help: "Maps CSI volumes to node block devices. Value is always 1.",
		}, []string{"node", "volume_handle", "driver", "device"}),

		discoveryErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "csi_volume_device_exporter_discovery_errors_total",
			Help: "Total number of discovery errors by discoverer.",
		}, []string{"discoverer"}),

		volumesDiscovered: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "csi_volume_device_exporter_volumes_discovered",
			Help: "Number of volumes discovered per driver.",
		}, []string{"driver"}),

		lastSuccessful: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "csi_volume_device_exporter_last_successful_discovery_timestamp_seconds",
			Help: "Unix timestamp of last successful discovery cycle.",
		}),

		registry:        reg,
		previous:        make(map[string]prometheus.Labels),
		previousDrivers: make(map[string]struct{}),
	}

	reg.MustRegister(m.volumeDeviceInfo)
	reg.MustRegister(m.discoveryErrors)
	reg.MustRegister(m.volumesDiscovered)
	reg.MustRegister(m.lastSuccessful)

	return m
}

// Registry returns the custom Prometheus registry.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

// IncDiscoveryErrors increments the error counter for the named discoverer.
func (m *Metrics) IncDiscoveryErrors(discoverer string) {
	m.discoveryErrors.WithLabelValues(discoverer).Inc()
}

// SetLastSuccessfulNow records the current time as the last successful discovery timestamp.
func (m *Metrics) SetLastSuccessfulNow() {
	m.lastSuccessful.SetToCurrentTime()
}

// Reconcile updates the volume device info metrics to match the given set of
// discovered volumes. It only deletes series that are no longer present, avoiding
// a full Reset() which would cause scrape gaps.
func (m *Metrics) Reconcile(volumes map[string]discovery.VolumeDevice) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current := make(map[string]prometheus.Labels, len(volumes))

	for key, v := range volumes {
		labels := prometheus.Labels{
			"node":          v.Node,
			"volume_handle": v.VolumeHandle,
			"driver":        v.Driver,
			"device":        v.Device,
		}
		m.volumeDeviceInfo.With(labels).Set(1)
		current[key] = labels
	}

	for key, labels := range m.previous {
		if _, exists := current[key]; !exists {
			m.volumeDeviceInfo.Delete(labels)
		}
	}

	m.previous = current

	driverCounts := make(map[string]float64)
	for _, v := range volumes {
		driverCounts[v.Driver]++
	}

	for driver := range m.previousDrivers {
		if _, exists := driverCounts[driver]; !exists {
			m.volumesDiscovered.Delete(prometheus.Labels{"driver": driver})
		}
	}

	currentDrivers := make(map[string]struct{}, len(driverCounts))
	for driver, count := range driverCounts {
		m.volumesDiscovered.With(prometheus.Labels{"driver": driver}).Set(count)
		currentDrivers[driver] = struct{}{}
	}
	m.previousDrivers = currentDrivers
}
