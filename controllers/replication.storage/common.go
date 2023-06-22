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

package controllers

import (
	"testing"

	volumegroupv1 "github.com/IBM/csi-volume-group-operator/apis/volumegroup.storage/v1"
	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/apis/replication.storage/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	mockNamespace    = "test-ns"
	mockVolumeHandle = "test-volume-handle"
	vrName           = "test-vr"
)

func getMockVolumeReplicationObject(sourceName string) *replicationv1alpha1.VolumeReplication {
	return &replicationv1alpha1.VolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "volume-replication",
			Namespace: mockNamespace,
		},
		Spec: replicationv1alpha1.VolumeReplicationSpec{
			VolumeReplicationClass: "volume-replication-class",
			DataSource: corev1.TypedLocalObjectReference{
				Name: sourceName,
			},
		},
	}
}

func createFakeScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme, err := replicationv1alpha1.SchemeBuilder.Build()
	if err != nil {
		assert.Fail(t, "unable to build scheme")
	}
	err = corev1.AddToScheme(scheme)
	if err != nil {
		assert.Fail(t, "failed to add corev1 scheme")
	}
	err = volumegroupv1.AddToScheme(scheme)
	if err != nil {
		assert.Fail(t, "failed to add volumegroupv1 scheme")
	}
	err = replicationv1alpha1.AddToScheme(scheme)
	if err != nil {
		assert.Fail(t, "failed to add replicationv1alpha1 scheme")
	}

	return scheme
}

func createFakeVolumeReplicationReconciler(t *testing.T, obj ...runtime.Object) VolumeReplicationReconciler {
	t.Helper()
	scheme := createFakeScheme(t)
	client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(obj...).Build()

	return VolumeReplicationReconciler{
		Client: client,
		Scheme: scheme,
	}
}
