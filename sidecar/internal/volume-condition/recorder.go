/*
Copyright 2025 The Kubernetes-CSI-Addons Authors.

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

package condition

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/volume-condition/volume"
)

// conditionRecorder provides the interface for recorders that will record the
// volume condition.
type conditionRecorder interface {
	record(ctx context.Context, pv *corev1.PersistentVolume, vc volume.VolumeCondition) error
}

// RecorderOption is the interface for creating new recorders. The
// VolumeConditionReporter will use this interface to configure a recorder
// and uses it to report the volume condition.
type RecorderOption struct {
	newRecorder func(client *kubernetes.Clientset, hostname string) (conditionRecorder, error)
}
