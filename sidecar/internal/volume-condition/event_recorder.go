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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/volume-condition/volume"
)

type eventRecorder struct {
	client   *kubernetes.Clientset
	recorder record.EventRecorder
}

// assert on conditionRecorder interface
var _ conditionRecorder = &eventRecorder{}

// WithEventRecorder can be passed to the VolumeConditionRecorder so that it
// will create an Event for the PersistentVolumeClaim with the volume
// condition.
func WithEventRecorder() RecorderOption {
	return RecorderOption{
		newRecorder: newEventRecorder,
	}
}

func newEventRecorder(client *kubernetes.Clientset, hostname string) (conditionRecorder, error) {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{
			Interface: client.CoreV1().Events(""),
		},
	)
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		corev1.EventSource{
			Component: "CSI-Addons",
			Host:      hostname,
		},
	)

	return &eventRecorder{
		client:   client,
		recorder: recorder,
	}, nil
}

func (er *eventRecorder) record(
	ctx context.Context,
	pv *corev1.PersistentVolume,
	vc volume.VolumeCondition,
) error {
	claim := pv.Spec.ClaimRef
	if claim == nil {
		return fmt.Errorf("persistent volume %q does not have a claim (unused?)", pv.Name)
	}

	pvc, err := er.client.CoreV1().PersistentVolumeClaims(claim.Namespace).Get(
		ctx,
		claim.Name,
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to get persistent volume claim %q in namespace %q: %w",
			claim.Name,
			claim.Namespace,
			err,
		)
	}

	severity := corev1.EventTypeNormal
	if !vc.IsHealthy() {
		severity = corev1.EventTypeWarning
	}

	er.recorder.Event(pvc, severity, vc.GetReason(), vc.GetMessage())

	return nil
}
