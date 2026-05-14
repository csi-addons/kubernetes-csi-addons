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

//go:build e2e

package exporter

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestExporterE2E validates the exporter against a real kubelet filesystem.
// Run with: go test -tags=e2e -v ./test/e2e/exporter/ -kubelet-root=/var/lib/kubelet
//
// Prerequisites:
// - Run on a node with CSI volumes staged (or mount the host paths into the test environment)
// - The binary must be built: go build -o bin/csi-volume-device-exporter ./cmd/csi-volume-device-exporter
func TestExporterE2E(t *testing.T) {
	binary := os.Getenv("EXPORTER_BINARY")
	if binary == "" {
		binary = "../../../bin/csi-volume-device-exporter"
	}

	if _, err := os.Stat(binary); os.IsNotExist(err) {
		t.Skipf("exporter binary not found at %s; build with: go build -o bin/csi-volume-device-exporter ./cmd/csi-volume-device-exporter", binary)
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		nodeName = "e2e-test-node"
	}

	listenAddr := ":19091"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hostProc := os.Getenv("HOST_PROC")
	if hostProc == "" {
		hostProc = "/proc"
	}
	hostSys := os.Getenv("HOST_SYS")
	if hostSys == "" {
		hostSys = "/sys"
	}
	hostKubelet := os.Getenv("HOST_KUBELET")
	if hostKubelet == "" {
		hostKubelet = "/var/lib/kubelet"
	}
	kubeletRoot := os.Getenv("KUBELET_ROOT")
	if kubeletRoot == "" {
		kubeletRoot = "/var/lib/kubelet"
	}
	hostTrident := os.Getenv("HOST_TRIDENT_TRACKING")
	if hostTrident == "" {
		hostTrident = "/var/lib/trident/tracking"
	}

	cmd := exec.CommandContext(ctx, binary,
		"--listen-address="+listenAddr,
		"--poll-interval=5s",
		"--log-level=debug",
		"--host-proc="+hostProc,
		"--host-sys="+hostSys,
		"--host-kubelet="+hostKubelet,
		"--kubelet-root="+kubeletRoot,
		"--host-trident-tracking="+hostTrident,
	)
	cmd.Env = append(os.Environ(),
		"NODE_NAME="+nodeName,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start exporter: %v", err)
	}
	defer func() {
		cancel()
		_ = cmd.Wait()
	}()

	if err := waitForReady(listenAddr, 10*time.Second); err != nil {
		t.Fatalf("exporter did not become ready: %v", err)
	}

	t.Run("healthz returns 200", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://localhost%s/healthz", listenAddr))
		if err != nil {
			t.Fatalf("healthz request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(body))
		}
	})

	t.Run("metrics endpoint serves prometheus format", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://localhost%s/metrics", listenAddr))
		if err != nil {
			t.Fatalf("metrics request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		content := string(body)

		if !strings.Contains(content, "csi_volume_device_exporter_last_successful_discovery_timestamp_seconds") {
			t.Error("expected csi_volume_device_exporter_last_successful_discovery_timestamp_seconds in output")
		}
		if !strings.Contains(content, "csi_volume_device_exporter_discovery_duration_seconds") {
			t.Error("expected csi_volume_device_exporter_discovery_duration_seconds in output")
		}
	})

	t.Run("discovers volumes if present", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("http://localhost%s/metrics", listenAddr))
		if err != nil {
			t.Fatalf("metrics request failed: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		content := string(body)

		if strings.Contains(content, "csi_volume_node_device_info") {
			t.Logf("SUCCESS: Found csi_volume_node_device_info metrics:")
			for _, line := range strings.Split(content, "\n") {
				if strings.HasPrefix(line, "csi_volume_node_device_info{") {
					t.Logf("  %s", line)
				}
			}
		} else {
			t.Logf("NOTE: No csi_volume_node_device_info metrics found. " +
				"This is expected if no CSI volumes are staged on this node. " +
				"Deploy a CSI PVC and re-run.")
		}

		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "csi_volume_device_exporter_volumes_discovered{") {
				t.Logf("  %s", line)
			}
		}
	})
}

func waitForReady(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("http://localhost%s/healthz", addr))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for exporter at %s", addr)
}
