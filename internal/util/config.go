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
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Config holds the configuration options that
// can be overrrided via a config file.
type Config struct {
	Namespace               string
	ReclaimSpaceTimeout     time.Duration
	MaxConcurrentReconciles int
	SchedulePrecedence      string
	MaxGroupPVC             int
	CSIAddonsNodeRetryDelay int
}

const (
	csiAddonsConfigMapName         = "csi-addons-config"
	ReclaimSpaceTimeoutKey         = "reclaim-space-timeout"
	MaxConcurrentReconcilesKey     = "max-concurrent-reconciles"
	defaultNamespace               = "csi-addons-system"
	defaultMaxConcurrentReconciles = 100
	defaultReclaimSpaceTimeout     = time.Minute * 3
	SchedulePrecedenceKey          = "schedule-precedence"
	ScheduleSC                     = "storageclass"
	SchedulePVC                    = "pvc"
	MaxGroupPVCKey                 = "max-group-pvcs"
	defaultMaxGroupPVC             = 100 // based on ceph's support/testing
	defaultCSIAddonsNodeRetryDelay = 5   //seconds
	CsiaddonsNodeRetryDelayKey     = "csi-addons-node-retry-delay"
)

// NewConfig returns a new Config object with default values.
func NewConfig() Config {
	return Config{
		Namespace:               defaultNamespace,
		ReclaimSpaceTimeout:     defaultReclaimSpaceTimeout,
		MaxConcurrentReconciles: defaultMaxConcurrentReconciles,
		SchedulePrecedence:      SchedulePVC,
		MaxGroupPVC:             defaultMaxGroupPVC,
		CSIAddonsNodeRetryDelay: defaultCSIAddonsNodeRetryDelay,
	}
}

// ReadConfigFile fetches the config file and updates the
// config options with values read from the config map.
func (cfg *Config) ReadConfigMap(ctx context.Context, kubeClient *kubernetes.Clientset) error {
	cm, err := kubeClient.CoreV1().ConfigMaps(cfg.Namespace).
		Get(ctx, csiAddonsConfigMapName, metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get configmap %q: %w", csiAddonsConfigMapName, err)
	}

	return cfg.readConfig(cm.Data)
}

// readConfigFile reads the config dataMap and updates the
// config options.
func (cfg *Config) readConfig(dataMap map[string]string) error {
	for key, val := range dataMap {
		switch key {
		case ReclaimSpaceTimeoutKey:
			timeout, err := time.ParseDuration(val)
			if err != nil {
				return fmt.Errorf("failed to parse key %q value %q as duration: %w",
					ReclaimSpaceTimeoutKey, val, err)
			}
			cfg.ReclaimSpaceTimeout = timeout

		case MaxConcurrentReconcilesKey:
			maxConcurrentReconciles, err := strconv.Atoi(val)
			if err != nil {
				return fmt.Errorf("failed to parse key %q value %q as int: %w",
					MaxConcurrentReconcilesKey, val, err)
			}
			cfg.MaxConcurrentReconciles = maxConcurrentReconciles

		case SchedulePrecedenceKey:
			if err := cfg.validateAndSetSchedulePrecedence(val); err != nil {
				return err
			}

		case MaxGroupPVCKey:
			maxGroupPVCs, err := strconv.Atoi(val)
			if err != nil {
				return fmt.Errorf("failed to parse key %q value %q as int: %w",
					MaxGroupPVCKey, val, err)
			}
			if maxGroupPVCs <= 0 || maxGroupPVCs > 100 {
				return fmt.Errorf("invalid value %q for key %q", val, MaxGroupPVCKey)
			}
			cfg.MaxGroupPVC = maxGroupPVCs

		case CsiaddonsNodeRetryDelayKey:
			if err := cfg.validateAndSetCSIAddonsNodeRetryDelay(val); err != nil {
				return err
			}

		default:
			return fmt.Errorf("unknown config key %q", key)
		}
	}

	return nil
}

// validateAndSetSchedulePrecedence is a helper function to check for
// valid values for schedule-precedence ConfigMap key.
// It also preserves backwards compatibility in cases where users might be using
// once valid values for it i.e. "sc-only" and "".
func (cfg *Config) validateAndSetSchedulePrecedence(val string) error {
	validVals := []string{
		SchedulePVC,
		ScheduleSC,
		"sc-only", // This is kept to avoid breaking changes and should be removed in later releases
		"",        // Same as above
	}

	if !slices.Contains(validVals, val) {
		return fmt.Errorf("invalid value %q for key %q", val, SchedulePrecedenceKey)
	}
	cfg.SchedulePrecedence = val

	// FIXME: Remove in later releases
	switch val {
	case "sc-only":
		cfg.SchedulePrecedence = ScheduleSC
	case "":
		cfg.SchedulePrecedence = SchedulePVC
	}

	return nil
}

func (cfg *Config) validateAndSetCSIAddonsNodeRetryDelay(val string) error {
	delay, err := strconv.Atoi(val)
	if err != nil {
		return fmt.Errorf("failed to parse value: %q for key: %q as int, %w", val, CsiaddonsNodeRetryDelayKey, err)
	}

	if delay < 1 {
		return fmt.Errorf("got an invalid value: %q for key: %q", delay, CsiaddonsNodeRetryDelayKey)
	}
	cfg.CSIAddonsNodeRetryDelay = delay

	return nil
}
