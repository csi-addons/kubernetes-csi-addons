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
			name:      "config file does not exist",
			dataMap:   nil,
			newConfig: expectedConfig(nil),
			wantErr:   false,
		},
		{
			name:      "config file does exist but empty configuration",
			dataMap:   make(map[string]string),
			newConfig: expectedConfig(nil),
			wantErr:   false,
		},
		{
			name: "config file modifies volume-health-cleanup-interval",
			dataMap: map[string]string{
				"volume-health-cleanup-interval": "45m",
			},
			newConfig: expectedConfig(func(cfg *Config) {
				cfg.VolumeHealthCleanupInterval = 45 * time.Minute
			}),
			wantErr: false,
		},
		{
			name: "config file has invalid volume-health-cleanup-interval",
			dataMap: map[string]string{
				"volume-health-cleanup-interval": "invalid",
			},
			newConfig: expectedConfig(nil),
			wantErr:   true,
		},
		{
			name: "config file has non positive volume-health-cleanup-interval",
			dataMap: map[string]string{
				"volume-health-cleanup-interval": "0s",
			},
			newConfig: expectedConfig(nil),
			wantErr:   true,
		},
		{
			name: "config file modifies volume-health-stale-threshold",
			dataMap: map[string]string{
				"volume-health-stale-threshold": "3h",
			},
			newConfig: expectedConfig(func(cfg *Config) {
				cfg.VolumeHealthStaleThreshold = 3 * time.Hour
			}),
			wantErr: false,
		},
		{
			name: "config file has invalid volume-health-stale-threshold",
			dataMap: map[string]string{
				"volume-health-stale-threshold": "hours",
			},
			newConfig: expectedConfig(nil),
			wantErr:   true,
		},
		{
			name: "config file modifies reclaim-space-timeout",
			dataMap: map[string]string{
				"reclaim-space-timeout": "10m",
			},
			newConfig: expectedConfig(func(cfg *Config) {
				cfg.ReclaimSpaceTimeout = 10 * time.Minute
			}),
			wantErr: false,
		},
		{
			name: "config file modifies reclaim-space-timeout but invalid",
			dataMap: map[string]string{
				"reclaim-space-timeout": "hours",
			},
			newConfig: expectedConfig(nil),
			wantErr:   true,
		},
		{
			name: "config file modifies max-concurrent-reconciles",
			dataMap: map[string]string{
				"max-concurrent-reconciles": "1",
			},
			newConfig: expectedConfig(func(cfg *Config) {
				cfg.MaxConcurrentReconciles = 1
			}),
			wantErr: false,
		},
		{
			name: "config file modifies max-concurrent-reconcilesbut invalid",
			dataMap: map[string]string{
				"max-concurrent-reconciles": "invalid",
			},
			newConfig: expectedConfig(nil),
			wantErr:   true,
		},
		{
			name: "config file modifies both reclaim-space-timeout and max-concurrent-reconciles",
			dataMap: map[string]string{
				"reclaim-space-timeout":     "10m",
				"max-concurrent-reconciles": "5",
			},
			newConfig: expectedConfig(func(cfg *Config) {
				cfg.ReclaimSpaceTimeout = 10 * time.Minute
				cfg.MaxConcurrentReconciles = 5
			}),
			wantErr: false,
		},
		{
			name: "config file contains invalid option",
			dataMap: map[string]string{
				"network-fence-duration": "3m",
			},
			newConfig: expectedConfig(nil),
			wantErr:   true,
		},
		{
			name: "config file modifies schedule-precedence to pvc",
			dataMap: map[string]string{
				"schedule-precedence": "pvc",
			},
			newConfig: expectedConfig(nil),
			wantErr:   false,
		},
		{
			name: "config file modifies schedule-precedence to storageclass",
			dataMap: map[string]string{
				"schedule-precedence": "storageclass",
			},
			newConfig: expectedConfig(func(cfg *Config) {
				cfg.SchedulePrecedence = ScheduleSC
			}),
			wantErr: false,
		},
		{
			name: "config file modifies schedule-precedence to a once valid value of sc-only",
			dataMap: map[string]string{
				"schedule-precedence": "sc-only",
			},
			newConfig: expectedConfig(func(cfg *Config) {
				cfg.SchedulePrecedence = ScheduleSC
			}),
			wantErr: false,
		},
		{
			name: "config file has invalid schedule-precedence",
			dataMap: map[string]string{
				"schedule-precedence": "invalid-precedence",
			},
			newConfig: expectedConfig(nil),
			wantErr:   true,
		},
		{
			name: "config file has empty schedule-precedence which was once valid and inferred as pvc",
			dataMap: map[string]string{
				"schedule-precedence": "",
			},
			newConfig: expectedConfig(nil),
			wantErr:   false,
		},
		{
			name: "config file has empty max-group-pvcs",
			dataMap: map[string]string{
				"max-group-pvcs": "",
			},
			newConfig: expectedConfig(nil),
			wantErr:   true,
		},
		{
			name: "config file modifies max-group-pvcs",
			dataMap: map[string]string{
				"max-group-pvcs": "25",
			},
			newConfig: expectedConfig(func(cfg *Config) {
				cfg.MaxGroupPVC = 25
			}),
			wantErr: false,
		},
		{
			name: "config file has invalid max-group-pvcs",
			dataMap: map[string]string{
				"max-group-pvcs": "200",
			},
			newConfig: expectedConfig(nil),
			wantErr:   true,
		},
		{
			name: "config file has empty csi-addons-node-retry-delay",
			dataMap: map[string]string{
				"csi-addons-node-retry-delay": "",
			},
			newConfig: expectedConfig(nil),
			wantErr:   true,
		},
		{
			name: "config file modifies csi-addons-node-retry-delay",
			dataMap: map[string]string{
				"csi-addons-node-retry-delay": "30",
			},
			newConfig: expectedConfig(func(cfg *Config) {
				cfg.CSIAddonsNodeRetryDelay = 30
			}),
			wantErr: false,
		},
		{
			name: "config file has invalid csi-addons-node-retry-delay",
			dataMap: map[string]string{
				"csi-addons-node-retry-delay": "0",
			},
			newConfig: expectedConfig(nil),
			wantErr:   true,
		},
		{
			name: "config file modifies cronjob-stagger-window",
			dataMap: map[string]string{
				"cronjob-stagger-window": "5",
			},
			newConfig: expectedConfig(func(cfg *Config) {
				cfg.CronJobStaggerWindow = 5
			}),
			wantErr: false,
		},
		{
			name: "config file has empty cronjob-stagger-window",
			dataMap: map[string]string{
				"cronjob-stagger-window": "",
			},
			newConfig: expectedConfig(nil),
			wantErr:   true,
		},
		{
			name: "config file sets cronjob-stagger-window to zero to disable",
			dataMap: map[string]string{
				"cronjob-stagger-window": "0",
			},
			newConfig: expectedConfig(func(cfg *Config) {
				cfg.CronJobStaggerWindow = 0
			}),
			wantErr: false,
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

func expectedConfig(override func(*Config)) Config {
	cfg := NewConfig()

	if override != nil {
		override(&cfg)
	}

	return cfg
}
