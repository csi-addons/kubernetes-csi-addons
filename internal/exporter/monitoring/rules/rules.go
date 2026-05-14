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

// Package rules assembles all Prometheus alerting and recording rules for the
// CSI volume device exporter and can render them as a Kubernetes PrometheusRule
// manifest or as a plain Prometheus groups file suitable for promtool.
package rules

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/csi-addons/kubernetes-csi-addons/internal/exporter/monitoring/rules/alerts"
)

const (
	ruleGroupName = "csi-volume-path-health"
)

type prometheusRuleGroup struct {
	Name  string           `yaml:"name"`
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

type prometheusRuleManifest struct {
	APIVersion string                 `yaml:"apiVersion"`
	Kind       string                 `yaml:"kind"`
	Metadata   map[string]interface{} `yaml:"metadata"`
	Spec       prometheusRuleGroups   `yaml:"spec"`
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

func WritePrometheusRuleManifest(path, namespace string) error {
	manifest := prometheusRuleManifest{
		APIVersion: "monitoring.coreos.com/v1",
		Kind:       "PrometheusRule",
		Metadata: map[string]interface{}{
			"name":      "csi-volume-path-health",
			"namespace": namespace,
			"labels": map[string]string{
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

func RulesAsString() (string, error) {
	groups := prometheusRuleGroups{Groups: buildGroups()}
	data, err := marshalYAML(groups)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func marshalYAML(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}
