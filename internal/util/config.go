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
}

const (
	csiAddonsConfigMapName         = "csi-addons-config"
	ReclaimSpaceTimeoutKey         = "reclaim-space-timeout"
	MaxConcurrentReconcilesKey     = "max-concurrent-reconciles"
	defaultNamespace               = "csi-addons-system"
	defaultMaxConcurrentReconciles = 100
	defaultReclaimSpaceTimeout     = time.Minute * 3
)

// NewConfig returns a new Config object with default values.
func NewConfig() Config {
	return Config{
		Namespace:               defaultNamespace,
		ReclaimSpaceTimeout:     defaultReclaimSpaceTimeout,
		MaxConcurrentReconciles: defaultMaxConcurrentReconciles,
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

		default:
			return fmt.Errorf("unknown config key %q", key)
		}
	}

	return nil
}
