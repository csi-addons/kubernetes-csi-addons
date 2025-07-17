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
	"k8s.io/klog/v2"

	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/volume-condition/volume"
)

type logRecorder struct{}

// assert on conditionRecorder interface
var _ conditionRecorder = &logRecorder{}

// WithLogRecorder can be passed to the VolumeConditionReporter so that it
// will report the volume condition in the logs of the CSI-Addons sidecar.
func WithLogRecorder() RecorderOption {
	return RecorderOption{
		newRecorder: func(client *kubernetes.Clientset, hostname string) (conditionRecorder, error) {
			return &logRecorder{}, nil
		},
	}
}

func (lr *logRecorder) record(
	ctx context.Context,
	pv *corev1.PersistentVolume,
	vc volume.VolumeCondition,
) error {
	if vc.IsHealthy() {
		msg := vc.GetMessage()
		if msg != "" {
			msg = ": " + msg
		}
		klog.Infof("persistent volume %q is healthy"+msg, pv.GetName())
	} else {
		klog.Warningf(
			"persistent volume %q is not healthy: %v",
			pv.GetName(),
			vc.GetMessage(),
		)
	}

	return nil
}
