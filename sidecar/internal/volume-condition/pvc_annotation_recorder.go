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

package condition

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/volume-condition/volume"
)

const (
	pvcVolumeHealthAnnotationKeyPrefix = "csiaddons.openshift.io/volumehealth."

	pvcVolumeHealthAnnotationStateHealthy   = "healthy"
	pvcVolumeHealthAnnotationStateUnhealthy = "unhealthy"
)

type pvcAnnotationRecorder struct {
	client   *kubernetes.Clientset
	hostName string
	nodeUID  string
}

type pvcVolumeHealthAnnotation struct {
	State       string `json:"state"`
	LastChecked string `json:"lastChecked"`
	Since       string `json:"since"`
	Node        string `json:"node"`
}

// assert on conditionRecorder interface
var _ conditionRecorder = &pvcAnnotationRecorder{}

// WithPVCAnnotationRecorder can be passed to the VolumeConditionReporter so
// that it annotates a PersistentVolumeClaim with the local node volume-health
// state and timestamps.
func WithPVCAnnotationRecorder() RecorderOption {
	return RecorderOption{
		newRecorder: func(client *kubernetes.Clientset, hostname, nodeUID string) (conditionRecorder, error) {
			return &pvcAnnotationRecorder{
				client:   client,
				hostName: hostname,
				nodeUID:  nodeUID,
			}, nil
		},
	}
}

func (par *pvcAnnotationRecorder) record(
	ctx context.Context,
	pv *corev1.PersistentVolume,
	pvc *corev1.PersistentVolumeClaim,
	vc volume.VolumeCondition,
) error {
	if pvc.GetDeletionTimestamp() != nil {
		return nil
	}

	patch, err := buildPVCVolumeHealthAnnotationPatch(
		pvc.GetAnnotations(),
		par.nodeUID,
		par.hostName,
		vc.IsHealthy(),
	)
	if err != nil {
		return fmt.Errorf("failed to build volume health annotation patch for persistent volume claim %q in namespace %q: %w",
			pvc.Name, pvc.Namespace, err)
	}

	_, err = par.client.CoreV1().PersistentVolumeClaims(pvc.Namespace).Patch(
		ctx,
		pvc.Name,
		types.MergePatchType,
		patch,
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf(
			"failed to patch volume health annotation for persistent volume claim %q in namespace %q: %w",
			pvc.Name,
			pvc.Namespace,
			err,
		)
	}

	return nil
}

func (par *pvcAnnotationRecorder) needsPVC() bool {
	return true
}

func (par *pvcAnnotationRecorder) recordUnchangedVolumes() bool {
	// "true" so the sidecar keeps refreshing the PVC health annotation
	// every reporting tick, even when health stays the same.
	// Otherwise, "lastChecked" timestamp would become stale.
	return true
}

func buildPVCVolumeHealthAnnotationPatch(
	annotations map[string]string,
	nodeUID string,
	nodeName string,
	healthy bool,
) ([]byte, error) {
	if nodeUID == "" {
		return nil, fmt.Errorf("node UID cannot be empty")
	}

	state := pvcVolumeHealthAnnotationStateHealthy
	if !healthy {
		state = pvcVolumeHealthAnnotationStateUnhealthy
	}

	annotationKey := getPVCVolumeHealthAnnotationKey(nodeUID)
	lastCheckedTimestamp := time.Now().Format(time.RFC3339)
	since := getSinceTimestamp(annotations[annotationKey], state)
	if since == "" {
		since = lastCheckedTimestamp
	}

	annotationValue, err := json.Marshal(pvcVolumeHealthAnnotation{
		State:       state,
		LastChecked: lastCheckedTimestamp,
		Since:       since,
		Node:        nodeName,
	})
	if err != nil {
		return nil, err
	}
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				annotationKey: string(annotationValue),
			},
		},
	}

	data, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func getPVCVolumeHealthAnnotationKey(nodeUID string) string {
	return pvcVolumeHealthAnnotationKeyPrefix + nodeUID
}

func getSinceTimestamp(encodedAnnotation string, state string) string {
	if encodedAnnotation == "" {
		return ""
	}

	annotation := pvcVolumeHealthAnnotation{}
	if err := json.Unmarshal([]byte(encodedAnnotation), &annotation); err != nil {
		return ""
	}

	if annotation.State != state {
		return ""
	}

	return annotation.Since
}
