package rules

import (
	"os"
	"strings"
	"testing"

	"github.com/csi-addons/kubernetes-csi-addons/internal/exporter/rules/alerts"
)

func TestAllAlerts_HaveRequiredFields(t *testing.T) {
	for _, a := range alerts.All() {
		t.Run(a.Name, func(t *testing.T) {
			if a.Name == "" {
				t.Error("alert name must not be empty")
			}
			if a.Expr == "" {
				t.Errorf("alert %s: expr must not be empty", a.Name)
			}
			if a.For == "" {
				t.Errorf("alert %s: for duration must not be empty", a.Name)
			}
			if a.Labels["severity"] == "" {
				t.Errorf("alert %s: severity label must be set", a.Name)
			}
			if a.Annotations["summary"] == "" {
				t.Errorf("alert %s: summary annotation must be set", a.Name)
			}
			if a.Annotations["description"] == "" {
				t.Errorf("alert %s: description annotation must be set", a.Name)
			}
			if a.Annotations["runbook_url"] == "" {
				t.Errorf("alert %s: runbook_url annotation must be set", a.Name)
			}
		})
	}
}

func TestRulesAsString_ProducesValidYAML(t *testing.T) {
	s, err := RulesAsString()
	if err != nil {
		t.Fatalf("RulesAsString: %v", err)
	}
	if !strings.Contains(s, "groups:") {
		t.Error("expected 'groups:' key in output")
	}
	if !strings.Contains(s, "CSIVolumeMultipathDegraded") {
		t.Error("expected CSIVolumeMultipathDegraded in output")
	}
	if !strings.Contains(s, "CSIVolumeDeviceExporterDown") {
		t.Error("expected CSIVolumeDeviceExporterDown in output")
	}
}

func TestWritePrometheusRulesFile(t *testing.T) {
	path := t.TempDir() + "/rules.yaml"
	if err := WritePrometheusRulesFile(path); err != nil {
		t.Fatalf("WritePrometheusRulesFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read rules file: %v", err)
	}
	content := string(data)
	wantCount := len(alerts.All())
	if wantCount == 0 {
		t.Fatal("alerts.All() returned no alerts")
	}
	got := strings.Count(content, "alert:")
	if got != wantCount {
		t.Errorf("expected %d alert entries in rules file, got %d", wantCount, got)
	}
	if !strings.Contains(content, "groups:") {
		t.Error("rules file missing 'groups:' key")
	}
}

func TestWritePrometheusRuleManifest(t *testing.T) {
	path := t.TempDir() + "/manifest.yaml"
	const ns = "monitoring"
	if err := WritePrometheusRuleManifest(path, ns); err != nil {
		t.Fatalf("WritePrometheusRuleManifest: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "kind: PrometheusRule") {
		t.Error("manifest missing 'kind: PrometheusRule'")
	}
	if !strings.Contains(content, "namespace: "+ns) {
		t.Errorf("manifest missing 'namespace: %s'", ns)
	}
	wantCount := len(alerts.All())
	got := strings.Count(content, "alert:")
	if got != wantCount {
		t.Errorf("expected %d alert entries in manifest, got %d", wantCount, got)
	}
}
