/*
Copyright 2026 The Kubernetes-CSI-Addons Authors.

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

package util

import (
	"flag"
	"fmt"
	"strconv"
)

// klogVerbosityFlag adapts the klog-style -v verbosity flag onto
// controller-runtime's --zap-log-level flag.
//
// klog uses -v=0 as the default(info) level and larger numbers for more
// verbose output. zap's level flag, however, rejects 0 and any non-positive
// integer(accepts positive integers or the strings debug/info/error).
//
// This wrapper maps -v=0(and lower) to the info level and delegates positive values to
// the underlying zap level flag.
type klogVerbosityFlag struct {
	zapLevel flag.Value
	value    string
}

var _ flag.Value = &klogVerbosityFlag{}

func (k *klogVerbosityFlag) String() string {
	if k == nil {
		return ""
	}
	return k.value
}

func (k *klogVerbosityFlag) Set(v string) error {
	level, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("invalid verbosity %q: must be an integer", v)
	}

	k.value = v
	if level <= 0 {
		// klog -v=0(and lower) corresponds to the default info level.
		return k.zapLevel.Set("info")
	}

	return k.zapLevel.Set(v)
}

// AddKlogVerbosityFlag adds a klog-compatible verbosity flag on the
// given FlagSet that maps to zap's "--zap-log-level" flag.
func AddKlogVerbosityFlag(fs *flag.FlagSet) {
	if f := fs.Lookup("zap-log-level"); f != nil {
		fs.Var(&klogVerbosityFlag{zapLevel: f.Value}, "v", "Alias for --zap-log-level, accepts klog-style verbosity (e.g. -v=0, -v=3)")
	}
}
