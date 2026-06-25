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
	"crypto/tls"
	"fmt"
	"strings"
)

type TLSOptions struct {
	MinVersion       string
	CipherSuites     string
	CurvePreferences string
}

var tlsVersions = map[string]uint16{
	"VersionTLS12": tls.VersionTLS12,
	"VersionTLS13": tls.VersionTLS13,
}

var curveIDs = func() map[string]tls.CurveID {
	ids := []tls.CurveID{
		tls.CurveP256,
		tls.CurveP384,
		tls.CurveP521,
		tls.X25519,
		tls.X25519MLKEM768,
		tls.SecP256r1MLKEM768,
		tls.SecP384r1MLKEM1024,
	}
	m := make(map[string]tls.CurveID, len(ids))
	for _, id := range ids {
		m[id.String()] = id
	}
	return m
}()

// BuildTLSConfig validates the given TLSOptions and returns a *tls.Config.
// Empty option values result in Go defaults being used.
func BuildTLSConfig(opts TLSOptions) (*tls.Config, error) {
	config := &tls.Config{}

	if opts.MinVersion != "" {
		v, ok := tlsVersions[opts.MinVersion]
		if !ok {
			return nil, fmt.Errorf(
				"unsupported TLS version %q, supported values: VersionTLS12, VersionTLS13",
				opts.MinVersion,
			)
		}
		config.MinVersion = v
	}

	if opts.CipherSuites != "" {
		supported := make(map[string]uint16)
		for _, cs := range tls.CipherSuites() {
			supported[cs.Name] = cs.ID
		}

		var suiteIDs []uint16
		for name := range strings.SplitSeq(opts.CipherSuites, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			id, ok := supported[name]
			if !ok {
				return nil, fmt.Errorf("unsupported cipher suite %q", name)
			}
			suiteIDs = append(suiteIDs, id)
		}
		if len(suiteIDs) == 0 {
			return nil, fmt.Errorf("no valid cipher suites specified")
		}
		config.CipherSuites = suiteIDs
	}

	if opts.CurvePreferences != "" {
		var curves []tls.CurveID
		for name := range strings.SplitSeq(opts.CurvePreferences, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			id, ok := curveIDs[name]
			if !ok {
				supported := make([]string, 0, len(curveIDs))
				for k := range curveIDs {
					supported = append(supported, k)
				}
				return nil, fmt.Errorf(
					"unsupported curve %q, supported values: %s",
					name, strings.Join(supported, ", "),
				)
			}
			curves = append(curves, id)
		}
		if len(curves) == 0 {
			return nil, fmt.Errorf("no valid curves specified")
		}
		config.CurvePreferences = curves
	}

	return config, nil
}
