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

package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/csi-addons/kubernetes-csi-addons/internal/exporter/discovery"
)

func TestReconcile_AddsNewMetrics(t *testing.T) {
	m := New()

	volumes := map[string]discovery.VolumeDevice{
		"vol-1": {VolumeHandle: "vol-1", Driver: "csi.trident.netapp.io", Device: "dm-0", Node: "node1"},
		"vol-2": {VolumeHandle: "vol-2", Driver: "csi.hpe.com", Device: "dm-1", Node: "node1"},
	}

	m.Reconcile(volumes)

	count := testutil.CollectAndCount(m.volumeDeviceInfo)
	if count != 2 {
		t.Errorf("expected 2 metrics, got %d", count)
	}
}

func TestReconcile_RemovesStaleSeries(t *testing.T) {
	m := New()

	volumes1 := map[string]discovery.VolumeDevice{
		"vol-1": {VolumeHandle: "vol-1", Driver: "driver-a", Device: "sda", Node: "node1"},
		"vol-2": {VolumeHandle: "vol-2", Driver: "driver-b", Device: "sdb", Node: "node1"},
	}
	m.Reconcile(volumes1)

	volumes2 := map[string]discovery.VolumeDevice{
		"vol-1": {VolumeHandle: "vol-1", Driver: "driver-a", Device: "sda", Node: "node1"},
	}
	m.Reconcile(volumes2)

	count := testutil.CollectAndCount(m.volumeDeviceInfo)
	if count != 1 {
		t.Errorf("expected 1 metric after stale removal, got %d", count)
	}
}

func TestReconcile_HandlesEmptyVolumes(t *testing.T) {
	m := New()

	volumes := map[string]discovery.VolumeDevice{
		"vol-1": {VolumeHandle: "vol-1", Driver: "driver-a", Device: "sda", Node: "node1"},
	}
	m.Reconcile(volumes)

	m.Reconcile(map[string]discovery.VolumeDevice{})

	count := testutil.CollectAndCount(m.volumeDeviceInfo)
	if count != 0 {
		t.Errorf("expected 0 metrics after reconciling empty, got %d", count)
	}
}

func TestReconcile_UpdatesVolumesDiscovered(t *testing.T) {
	m := New()

	volumes := map[string]discovery.VolumeDevice{
		"vol-1": {VolumeHandle: "vol-1", Driver: "csi.trident.netapp.io", Device: "dm-0", Node: "node1"},
		"vol-2": {VolumeHandle: "vol-2", Driver: "csi.trident.netapp.io", Device: "dm-1", Node: "node1"},
		"vol-3": {VolumeHandle: "vol-3", Driver: "csi.hpe.com", Device: "dm-2", Node: "node1"},
	}
	m.Reconcile(volumes)

	expected := `
# HELP csi_volume_device_exporter_volumes_discovered Number of volumes discovered per driver.
# TYPE csi_volume_device_exporter_volumes_discovered gauge
csi_volume_device_exporter_volumes_discovered{driver="csi.hpe.com"} 1
csi_volume_device_exporter_volumes_discovered{driver="csi.trident.netapp.io"} 2
`
	if err := testutil.CollectAndCompare(m.volumesDiscovered, strings.NewReader(expected)); err != nil {
		t.Errorf("unexpected metric output: %v", err)
	}
}

func TestMetrics_RegistryNotNil(t *testing.T) {
	m := New()
	if m.Registry() == nil {
		t.Error("registry should not be nil")
	}
}

func TestSetLastSuccessfulNow_UpdatesGauge(t *testing.T) {
	m := New()

	before := time.Now().Unix()
	m.SetLastSuccessfulNow()
	after := time.Now().Unix()

	gathered, err := m.registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var found bool
	for _, mf := range gathered {
		if mf.GetName() == "csi_volume_device_exporter_last_successful_discovery_timestamp_seconds" {
			found = true
			v := mf.GetMetric()[0].GetGauge().GetValue()
			if int64(v) < before || int64(v) > after {
				t.Errorf("last_successful gauge value %v is outside expected window [%d, %d]", v, before, after)
			}
		}
	}
	if !found {
		t.Error("last_successful gauge not found in registry")
	}
}

func TestObserveDiscoveryDuration_RecordsHistogram(t *testing.T) {
	m := New()
	m.ObserveDiscoveryDuration("kubelet", 0.5)
	m.ObserveDiscoveryDuration("kubelet", 1.2)

	gathered, err := m.registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var found bool
	for _, mf := range gathered {
		if mf.GetName() == "csi_volume_device_exporter_discovery_duration_seconds" {
			found = true
			if len(mf.GetMetric()) == 0 {
				t.Error("expected at least one histogram metric")
			}
			count := mf.GetMetric()[0].GetHistogram().GetSampleCount()
			if count != 2 {
				t.Errorf("expected sample count 2, got %d", count)
			}
		}
	}
	if !found {
		t.Error("discovery_duration histogram not found in registry")
	}
}

func TestIncDiscoveryErrors_IncrementsCounter(t *testing.T) {
	m := New()
	m.IncDiscoveryErrors("trident")
	m.IncDiscoveryErrors("trident")
	m.IncDiscoveryErrors("hpe")

	expected := `
# HELP csi_volume_device_exporter_discovery_errors_total Total number of discovery errors by discoverer.
# TYPE csi_volume_device_exporter_discovery_errors_total counter
csi_volume_device_exporter_discovery_errors_total{discoverer="hpe"} 1
csi_volume_device_exporter_discovery_errors_total{discoverer="trident"} 2
`
	if err := testutil.CollectAndCompare(m.discoveryErrors, strings.NewReader(expected)); err != nil {
		t.Errorf("unexpected metric output: %v", err)
	}
}
