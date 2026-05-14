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

// generate-exporter-rules writes the Prometheus rule YAML files derived from
// the typed alert definitions in internal/exporter/monitoring/rules/alerts.
//
// Flags:
//
//	-namespace <ns>   Kubernetes namespace embedded in the PrometheusRule manifest.
//	                  Falls back to the NAMESPACE environment variable, then
//	                  defaults to "NAMESPACE" as a placeholder when neither is set.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/csi-addons/kubernetes-csi-addons/internal/exporter/monitoring/rules"
)

func main() {
	namespace := flag.String("namespace", "", "Kubernetes namespace for the PrometheusRule manifest (overrides NAMESPACE env var)")
	flag.Parse()

	if *namespace == "" {
		*namespace = os.Getenv("NAMESPACE")
	}
	if *namespace == "" {
		*namespace = "NAMESPACE"
		fmt.Fprintln(os.Stderr, "generate-exporter-rules: namespace not set via -namespace flag or NAMESPACE env var; using placeholder \"NAMESPACE\"")
	}

	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	rulesDir := filepath.Join(repoRoot, "internal", "exporter", "monitoring", "rules")

	if err := rules.WritePrometheusRulesFile(filepath.Join(rulesDir, "alerts-rules.yaml")); err != nil {
		fmt.Fprintf(os.Stderr, "generate-exporter-rules: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("wrote internal/exporter/monitoring/rules/alerts-rules.yaml")

	if err := rules.WritePrometheusRuleManifest(filepath.Join(rulesDir, "alerts.yaml"), *namespace); err != nil {
		fmt.Fprintf(os.Stderr, "generate-exporter-rules: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("wrote internal/exporter/monitoring/rules/alerts.yaml (namespace: %s)\n", *namespace)
}
