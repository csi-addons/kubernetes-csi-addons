// Package rules assembles all Prometheus alerting and recording rules for the
// CSI volume device exporter and can render them as a Kubernetes PrometheusRule
// manifest or as a plain Prometheus groups file suitable for promtool.
package rules

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/csi-addons/kubernetes-csi-addons/internal/exporter/rules/alerts"
)

const (
	ruleGroupName = "csi-volume-path-health"
)

// prometheusRuleGroup is the wire format consumed by promtool and embedded in
// the PrometheusRule CRD spec.
type prometheusRuleGroup struct {
	Name  string         `yaml:"name"`
	Rules []prometheusRule `yaml:"rules"`
}

type prometheusRule struct {
	Alert       string            `yaml:"alert,omitempty"`
	Record      string            `yaml:"record,omitempty"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

type prometheusRuleGroups struct {
	Groups []prometheusRuleGroup `yaml:"groups"`
}

// prometheusRuleManifest is the full Kubernetes PrometheusRule CRD.
type prometheusRuleManifest struct {
	APIVersion string                 `yaml:"apiVersion"`
	Kind       string                 `yaml:"kind"`
	Metadata   prometheusRuleMetadata `yaml:"metadata"`
	Spec       prometheusRuleGroups   `yaml:"spec"`
}

type prometheusRuleMetadata struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

func buildGroups() []prometheusRuleGroup {
	var rules []prometheusRule
	for _, a := range alerts.All() {
		rules = append(rules, prometheusRule{
			Alert:       a.Name,
			Expr:        a.Expr,
			For:         a.For,
			Labels:      a.Labels,
			Annotations: a.Annotations,
		})
	}
	return []prometheusRuleGroup{{Name: ruleGroupName, Rules: rules}}
}

// WritePrometheusRulesFile writes a plain Prometheus groups YAML file (no
// Kubernetes CRD wrapper) to path. This is the format consumed by promtool.
func WritePrometheusRulesFile(path string) error {
	groups := prometheusRuleGroups{Groups: buildGroups()}
	data, err := marshalYAML(groups)
	if err != nil {
		return fmt.Errorf("marshal rules: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write rules file %s: %w", path, err)
	}
	return nil
}

// WritePrometheusRuleManifest writes a Kubernetes PrometheusRule CRD YAML to
// path. The namespace is embedded in the metadata; labels are added for the
// Prometheus Operator to pick up the rule.
func WritePrometheusRuleManifest(path, namespace string) error {
	manifest := prometheusRuleManifest{
		APIVersion: "monitoring.coreos.com/v1",
		Kind:       "PrometheusRule",
		Metadata: prometheusRuleMetadata{
			Name:      "csi-volume-path-health",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name": "csi-volume-device-exporter",
			},
		},
		Spec: prometheusRuleGroups{Groups: buildGroups()},
	}
	data, err := marshalYAML(manifest)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest %s: %w", path, err)
	}
	return nil
}

// RulesAsString returns the plain Prometheus groups YAML as a string, useful
// for tests and the generate target.
func RulesAsString() (string, error) {
	groups := prometheusRuleGroups{Groups: buildGroups()}
	data, err := marshalYAML(groups)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func marshalYAML(v any) ([]byte, error) {
	return yaml.Marshal(v)
}
