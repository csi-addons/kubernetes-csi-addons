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
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestStaleVolumeHealthAnnotationKeys(t *testing.T) {
	now := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	annotations := map[string]string{
		"csiaddons.openshift.io/volumehealth.node-a": `{"state":"unhealthy","lastChecked":"2026-06-24T07:30:00Z","since":"2026-06-24T07:00:00Z"}`,
		"csiaddons.openshift.io/volumehealth.node-b": `{"state":"healthy","lastChecked":"2026-06-24T09:50:00Z"}`,
		"csiaddons.openshift.io/volumehealth.node-c": `{"state":"unhealthy","lastchecked":"2026-06-24T07:45:00Z"}`,
		"csiaddons.openshift.io/volumehealth.node-d": `{"state":"healthy"}`,
		"example.com/other":                          "value",
	}

	keys, malformed, hasVolumeHealth := staleVolumeHealthAnnotationKeys(logr.Discard(), "default", "test-pvc", annotations, now, time.Hour)

	assert.ElementsMatch(t, []string{
		"csiaddons.openshift.io/volumehealth.node-a",
		"csiaddons.openshift.io/volumehealth.node-c",
	}, keys)
	assert.Equal(t, 1, malformed)
	assert.True(t, hasVolumeHealth)
}

func TestCleanupOnceRemovesOnlyStaleVolumeHealthAnnotations(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))

	now := time.Now().UTC()
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: "default",
			Annotations: map[string]string{
				"csiaddons.openshift.io/volumehealth.node-a": `{"state":"unhealthy","lastChecked":"` + now.Add(-3*time.Hour).Format(time.RFC3339) + `"}`,
				"csiaddons.openshift.io/volumehealth.node-b": `{"state":"healthy","lastChecked":"` + now.Add(-5*time.Minute).Format(time.RFC3339) + `"}`,
				"example.com/keep":                           "true",
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pvc).
		Build()

	worker := &VolumeHealthCleanupWorker{
		Client:          cl,
		CleanupInterval: 30 * time.Minute,
		StaleThreshold:  time.Hour,
	}

	assert.NoError(t, worker.runCleanup(context.Background(), logr.Discard()))

	got := &corev1.PersistentVolumeClaim{}
	assert.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, got))
	assert.NotContains(t, got.Annotations, "csiaddons.openshift.io/volumehealth.node-a")
	assert.Contains(t, got.Annotations, "csiaddons.openshift.io/volumehealth.node-b")
	assert.Equal(t, "true", got.Annotations["example.com/keep"])
}

func TestCleanupOnceSkipsMalformedPayload(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))

	now := time.Now().UTC()
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc-malformed",
			Namespace: "default",
			Annotations: map[string]string{
				"csiaddons.openshift.io/volumehealth.node-a": `{"state":"unhealthy","lastChecked":"not-a-time"}`,
				"csiaddons.openshift.io/volumehealth.node-b": `{"state":"healthy","lastChecked":"` + now.Add(-2*time.Minute).Format(time.RFC3339) + `"}`,
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pvc).
		Build()

	worker := &VolumeHealthCleanupWorker{
		Client:          cl,
		CleanupInterval: 30 * time.Minute,
		StaleThreshold:  time.Hour,
	}

	assert.NoError(t, worker.runCleanup(context.Background(), logr.Discard()))

	got := &corev1.PersistentVolumeClaim{}
	assert.NoError(t, cl.Get(context.Background(), types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, got))
	assert.Contains(t, got.Annotations, "csiaddons.openshift.io/volumehealth.node-a")
	assert.Contains(t, got.Annotations, "csiaddons.openshift.io/volumehealth.node-b")
}
