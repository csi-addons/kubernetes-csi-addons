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

// Package metrics provides Prometheus metrics for the CSI volume device exporter.
package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/csi-addons/kubernetes-csi-addons/internal/exporter/discovery"
)

type Metrics struct {
	volumeDeviceInfo  *prometheus.GaugeVec
	discoveryDuration *prometheus.HistogramVec
	discoveryErrors   *prometheus.CounterVec
	volumesDiscovered *prometheus.GaugeVec
	lastSuccessful    prometheus.Gauge

	registry *prometheus.Registry

	mu              sync.Mutex
	previous        map[string]prometheus.Labels
	previousDrivers map[string]struct{}
}

func New() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		volumeDeviceInfo: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "csi_volume_node_device_info",
			Help: "Maps CSI volumes to node block devices. Value is always 1.",
		}, []string{"node", "volume_handle", "driver", "device"}),

		discoveryDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "csi_volume_device_exporter_discovery_duration_seconds",
			Help:    "Duration of volume discovery by discoverer.",
			Buckets: prometheus.DefBuckets,
		}, []string{"discoverer"}),

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
	reg.MustRegister(m.discoveryDuration)
	reg.MustRegister(m.discoveryErrors)
	reg.MustRegister(m.volumesDiscovered)
	reg.MustRegister(m.lastSuccessful)

	return m
}

func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

func (m *Metrics) ObserveDiscoveryDuration(discoverer string, seconds float64) {
	m.discoveryDuration.WithLabelValues(discoverer).Observe(seconds)
}

func (m *Metrics) IncDiscoveryErrors(discoverer string) {
	m.discoveryErrors.WithLabelValues(discoverer).Inc()
}

func (m *Metrics) SetLastSuccessfulNow() {
	m.lastSuccessful.SetToCurrentTime()
}

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
