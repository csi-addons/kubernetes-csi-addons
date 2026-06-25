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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildTLSConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		opts        TLSOptions
		wantErr     string
		checkConfig func(t *testing.T, cfg *tls.Config)
	}{
		{
			name: "empty options use defaults",
			opts: TLSOptions{},
			checkConfig: func(t *testing.T, cfg *tls.Config) {
				assert.Equal(t, uint16(0), cfg.MinVersion)
				assert.Nil(t, cfg.CipherSuites)
				assert.Nil(t, cfg.CurvePreferences)
			},
		},
		{
			name: "valid TLS 1.2",
			opts: TLSOptions{MinVersion: "VersionTLS12"},
			checkConfig: func(t *testing.T, cfg *tls.Config) {
				assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
			},
		},
		{
			name: "valid TLS 1.3",
			opts: TLSOptions{MinVersion: "VersionTLS13"},
			checkConfig: func(t *testing.T, cfg *tls.Config) {
				assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
			},
		},
		{
			name:    "invalid TLS version",
			opts:    TLSOptions{MinVersion: "VersionTLS11"},
			wantErr: "unsupported TLS version",
		},
		{
			name: "valid cipher suite",
			opts: TLSOptions{CipherSuites: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
			checkConfig: func(t *testing.T, cfg *tls.Config) {
				assert.Len(t, cfg.CipherSuites, 1)
			},
		},
		{
			name: "multiple cipher suites",
			opts: TLSOptions{CipherSuites: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
			checkConfig: func(t *testing.T, cfg *tls.Config) {
				assert.Len(t, cfg.CipherSuites, 2)
			},
		},
		{
			name:    "invalid cipher suite",
			opts:    TLSOptions{CipherSuites: "TLS_INVALID_CIPHER"},
			wantErr: "unsupported cipher suite",
		},
		{
			name: "valid curve X25519",
			opts: TLSOptions{CurvePreferences: "X25519"},
			checkConfig: func(t *testing.T, cfg *tls.Config) {
				if assert.Len(t, cfg.CurvePreferences, 1) {
					assert.Equal(t, tls.X25519, cfg.CurvePreferences[0])
				}
			},
		},
		{
			name: "multiple curves",
			opts: TLSOptions{CurvePreferences: "X25519,CurveP256,CurveP384"},
			checkConfig: func(t *testing.T, cfg *tls.Config) {
				if assert.Len(t, cfg.CurvePreferences, 3) {
					assert.Equal(t, tls.X25519, cfg.CurvePreferences[0])
					assert.Equal(t, tls.CurveP256, cfg.CurvePreferences[1])
					assert.Equal(t, tls.CurveP384, cfg.CurvePreferences[2])
				}
			},
		},
		{
			name:    "invalid curve",
			opts:    TLSOptions{CurvePreferences: "InvalidCurve"},
			wantErr: "unsupported curve",
		},
		{
			name: "all options set",
			opts: TLSOptions{
				MinVersion:       "VersionTLS13",
				CipherSuites:     "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				CurvePreferences: "X25519,CurveP256",
			},
			checkConfig: func(t *testing.T, cfg *tls.Config) {
				assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
				assert.Len(t, cfg.CipherSuites, 1)
				assert.Len(t, cfg.CurvePreferences, 2)
			},
		},
		{
			name: "whitespace in comma-separated values is trimmed",
			opts: TLSOptions{CurvePreferences: "X25519 , CurveP256"},
			checkConfig: func(t *testing.T, cfg *tls.Config) {
				if assert.Len(t, cfg.CurvePreferences, 2) {
					assert.Equal(t, tls.X25519, cfg.CurvePreferences[0])
					assert.Equal(t, tls.CurveP256, cfg.CurvePreferences[1])
				}
			},
		},
		{
			name: "trailing comma is ignored",
			opts: TLSOptions{CurvePreferences: "X25519,"},
			checkConfig: func(t *testing.T, cfg *tls.Config) {
				if assert.Len(t, cfg.CurvePreferences, 1) {
					assert.Equal(t, tls.X25519, cfg.CurvePreferences[0])
				}
			},
		},
		{
			name:    "only commas in cipher suites yields error",
			opts:    TLSOptions{CipherSuites: ",,"},
			wantErr: "no valid cipher suites specified",
		},
		{
			name:    "only whitespace in curves yields error",
			opts:    TLSOptions{CurvePreferences: " , , "},
			wantErr: "no valid curves specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := BuildTLSConfig(tt.opts)
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			if assert.NoError(t, err) && assert.NotNil(t, cfg) {
				if tt.checkConfig != nil {
					tt.checkConfig(t, cfg)
				}
			}
		})
	}
}
