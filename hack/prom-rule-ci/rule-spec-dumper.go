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

// rule-spec-dumper writes the Prometheus rule groups to a file so that
// verify-rules.sh can pass them to promtool for linting and unit testing.
//
// Usage: rule-spec-dumper <output-file>
package main

import (
	"fmt"
	"os"

	"github.com/csi-addons/kubernetes-csi-addons/internal/exporter/monitoring/rules"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: rule-spec-dumper <output-file>\n")
		os.Exit(1)
	}
	if err := rules.WritePrometheusRulesFile(os.Args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "rule-spec-dumper: %v\n", err)
		os.Exit(1)
	}
}
