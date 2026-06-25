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

package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	volumeHealthAnnotationKeyPrefix = "csiaddons.openshift.io/volumehealth."
)

type volumeHealthAnnotation struct {
	State       string `json:"state"`
	LastChecked string `json:"lastChecked"`
	Since       string `json:"since"`
}

type VolumeHealthCleanupWorker struct {
	client.Client
	CleanupInterval time.Duration
	StaleThreshold  time.Duration
}

func (w *VolumeHealthCleanupWorker) Start(ctx context.Context) error {
	if w.CleanupInterval <= 0 {
		return fmt.Errorf("volume health cleanup interval must be > 0")
	}

	if w.StaleThreshold <= 0 {
		return fmt.Errorf("volume health stale threshold must be > 0")
	}

	logger := ctrllog.FromContext(ctx).WithName("volume-health-cleanup-worker")
	// Run an initial cleanup, until the first ticker interval runs.
	if err := w.runCleanup(ctx, logger); err != nil {
		logger.Error(err, "Initial cleanup failed")
	}

	ticker := time.NewTicker(w.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping volume health cleanup worker")

			return nil
		case <-ticker.C:
			if err := w.runCleanup(ctx, logger); err != nil {
				logger.Error(err, "Cleanup failed")
			}
		}
	}
}

func (w *VolumeHealthCleanupWorker) runCleanup(ctx context.Context, logger logr.Logger) error {
	var pvcList corev1.PersistentVolumeClaimList
	if err := w.List(ctx, &pvcList); err != nil {
		return fmt.Errorf("failed to list PVCs for stale volume health cleanup: %w", err)
	}

	now := time.Now()
	pvcWithVolumeHealth := 0
	cleanedPVCs := 0
	skippedMalformed := 0
	cleanupErrors := 0

	for i := range pvcList.Items {
		pvc := &pvcList.Items[i]
		if pvc.DeletionTimestamp != nil || len(pvc.Annotations) == 0 {
			continue
		}

		staleKeys, malformedCount, hasVolumeHealth := staleVolumeHealthAnnotationKeys(logger, pvc.Namespace, pvc.Name, pvc.Annotations, now, w.StaleThreshold)
		if hasVolumeHealth {
			pvcWithVolumeHealth++
		}
		skippedMalformed += malformedCount
		if len(staleKeys) == 0 {
			continue
		}

		patch, err := staleAnnotationDeletePatch(staleKeys)
		if err != nil {
			cleanupErrors++
			logger.Error(err, "Failed to build stale annotation cleanup patch", "pvc", pvc.Name, "namespace", pvc.Namespace)

			continue
		}

		if err := w.Patch(ctx, pvc, client.RawPatch(types.StrategicMergePatchType, patch)); err != nil {
			cleanupErrors++
			logger.Error(err, "Failed to patch stale volume health annotations", "pvc", pvc.Name, "namespace", pvc.Namespace)

			continue
		}

		cleanedPVCs++
	}

	logger.Info("Completed volume health cleanup",
		"pvcWithVolumeHealth", pvcWithVolumeHealth,
		"pvcCleaned", cleanedPVCs,
		"malformedSkipped", skippedMalformed,
		"errors", cleanupErrors)

	return nil
}

func staleVolumeHealthAnnotationKeys(
	logger logr.Logger,
	namespace string,
	pvcName string,
	annotations map[string]string,
	now time.Time,
	staleThreshold time.Duration,
) ([]string, int, bool) {
	staleKeys := make([]string, 0)
	malformedCount := 0
	hasVolumeHealth := false

	for key, val := range annotations {
		if !strings.HasPrefix(key, volumeHealthAnnotationKeyPrefix) {
			continue
		}
		hasVolumeHealth = true

		lastChecked, err := annotationLastChecked(val)
		if err != nil {
			malformedCount++
			logger.Error(err, "Skipping malformed volume health annotation",
				"namespace", namespace,
				"pvc", pvcName,
				"annotationKey", key)
			continue
		}

		if now.Sub(lastChecked) > staleThreshold {
			staleKeys = append(staleKeys, key)
		}
	}

	return staleKeys, malformedCount, hasVolumeHealth
}

func staleAnnotationDeletePatch(keys []string) ([]byte, error) {
	annotationPatch := make(map[string]interface{}, len(keys))
	for _, key := range keys {
		annotationPatch[key] = nil
	}

	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": annotationPatch,
		},
	}

	return json.Marshal(patch)
}

func annotationLastChecked(payload string) (time.Time, error) {
	var state volumeHealthAnnotation
	if err := json.Unmarshal([]byte(payload), &state); err != nil {
		return time.Time{}, err
	}

	if state.LastChecked == "" {
		return time.Time{}, fmt.Errorf("missing lastChecked")
	}

	parsed, err := time.Parse(time.RFC3339, state.LastChecked)
	if err != nil {
		return time.Time{}, err
	}

	return parsed, nil
}
