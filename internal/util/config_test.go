/*
Copyright 2023 The Kubernetes-CSI-Addons Authors.

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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigReadConfigFile(t *testing.T) {
	tests := []struct {
		name      string
		dataMap   map[string]string
		newConfig Config
		wantErr   bool
	}{
		{
			name:    "config file does not exist",
			dataMap: nil,
			newConfig: Config{
				Namespace:               defaultNamespace,
				ReclaimSpaceTimeout:     defaultReclaimSpaceTimeout,
				MaxConcurrentReconciles: defaultMaxConcurrentReconciles,
				SchedulePrecedence:      "",
			},
			wantErr: false,
		},
		{
			name:    "config file does exist but empty configuration",
			dataMap: make(map[string]string),
			newConfig: Config{
				Namespace:               defaultNamespace,
				ReclaimSpaceTimeout:     defaultReclaimSpaceTimeout,
				MaxConcurrentReconciles: defaultMaxConcurrentReconciles,
				SchedulePrecedence:      "",
			},
			wantErr: false,
		},
		{
			name: "config file modifies reclaim-space-timeout",
			dataMap: map[string]string{
				"reclaim-space-timeout": "10m",
			},
			newConfig: Config{
				Namespace:               defaultNamespace,
				ReclaimSpaceTimeout:     time.Minute * 10,
				MaxConcurrentReconciles: defaultMaxConcurrentReconciles,
				SchedulePrecedence:      "",
			},
			wantErr: false,
		},
		{
			name: "config file modifies reclaim-space-timeout but invalid",
			dataMap: map[string]string{
				"reclaim-space-timeout": "hours",
			},
			newConfig: Config{
				Namespace:               defaultNamespace,
				ReclaimSpaceTimeout:     defaultReclaimSpaceTimeout,
				MaxConcurrentReconciles: defaultMaxConcurrentReconciles,
				SchedulePrecedence:      "",
			},
			wantErr: true,
		},
		{
			name: "config file modifies max-concurrent-reconciles",
			dataMap: map[string]string{
				"max-concurrent-reconciles": "1",
			},
			newConfig: Config{
				Namespace:               defaultNamespace,
				ReclaimSpaceTimeout:     defaultReclaimSpaceTimeout,
				MaxConcurrentReconciles: 1,
				SchedulePrecedence:      "",
			},
			wantErr: false,
		},
		{
			name: "config file modifies max-concurrent-reconcilesbut invalid",
			dataMap: map[string]string{
				"max-concurrent-reconciles": "invalid",
			},
			newConfig: Config{
				Namespace:               defaultNamespace,
				ReclaimSpaceTimeout:     defaultReclaimSpaceTimeout,
				MaxConcurrentReconciles: defaultMaxConcurrentReconciles,
				SchedulePrecedence:      "",
			},
			wantErr: true,
		},
		{
			name: "config file modifies both reclaim-space-timeout and max-concurrent-reconciles",
			dataMap: map[string]string{
				"reclaim-space-timeout":     "10m",
				"max-concurrent-reconciles": "5",
			},
			newConfig: Config{
				Namespace:               defaultNamespace,
				ReclaimSpaceTimeout:     time.Minute * 10,
				MaxConcurrentReconciles: 5,
				SchedulePrecedence:      "",
			},
			wantErr: false,
		},
		{
			name: "config file contains invalid option",
			dataMap: map[string]string{
				"network-fence-duration": "3m",
			},
			newConfig: Config{
				Namespace:               defaultNamespace,
				ReclaimSpaceTimeout:     defaultReclaimSpaceTimeout,
				MaxConcurrentReconciles: defaultMaxConcurrentReconciles,
				SchedulePrecedence:      "",
			},
			wantErr: true,
		},
		{
			name: "config file modifies schedule-precedence",
			dataMap: map[string]string{
				"schedule-precedence": "sc-only",
			},
			newConfig: Config{
				Namespace:               defaultNamespace,
				ReclaimSpaceTimeout:     defaultReclaimSpaceTimeout,
				MaxConcurrentReconciles: defaultMaxConcurrentReconciles,
				SchedulePrecedence:      ScheduleSCOnly,
			},
			wantErr: false,
		},
		{
			name: "config file has invalid schedule-precedence",
			dataMap: map[string]string{
				"schedule-precedence": "invalid-precedence",
			},
			newConfig: Config{
				Namespace:               defaultNamespace,
				ReclaimSpaceTimeout:     defaultReclaimSpaceTimeout,
				MaxConcurrentReconciles: defaultMaxConcurrentReconciles,
				SchedulePrecedence:      "",
			},
			wantErr: true,
		},
		{
			name: "config file has empty schedule-precedence",
			dataMap: map[string]string{
				"schedule-precedence": "",
			},
			newConfig: Config{
				Namespace:               defaultNamespace,
				ReclaimSpaceTimeout:     defaultReclaimSpaceTimeout,
				MaxConcurrentReconciles: defaultMaxConcurrentReconciles,
				SchedulePrecedence:      "",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig()
			err := cfg.readConfig(tt.dataMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("config.readConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.newConfig, cfg)
		})
	}
}
