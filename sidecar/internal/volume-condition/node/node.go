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

package node

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/volume-condition/volume"
)

// Node can handle requests for the local worker node.
type Node interface {
	// ListCSIVolumes returns a slice of CSI-volumes that are attached to the
	// local worker node.
	ListCSIVolumes(ctx context.Context) ([]volume.CSIVolume, error)
}

type node struct {
	clientset *kubernetes.Clientset

	// nodename is used to filter the attachments by node
	nodename string
}

// assert that node implements the Node interface.
var _ Node = &node{}

// NewNode creates a new Node that represents the local worker node.
func NewNode(ctx context.Context, clientset *kubernetes.Clientset, nodename string) (Node, error) {
	// verify that my local Node exists, will be fetched again in .ListCSIVolumes()
	_, err := clientset.CoreV1().Nodes().Get(ctx, nodename, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get my local Node %q: %v", nodename, err)
	}

	return &node{
		clientset: clientset,
		nodename:  nodename,
	}, nil
}

func (n *node) ListCSIVolumes(ctx context.Context) ([]volume.CSIVolume, error) {
	localNode, err := n.clientset.CoreV1().Nodes().Get(ctx, n.nodename, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not get Node %q: %v", n.nodename, err)
	}

	vols := []volume.CSIVolume{}
	for _, attached := range localNode.Status.VolumesAttached {
		var vol volume.CSIVolume
		vol, err = newAttachedCSIVolume(attached.Name)
		if err != nil {
			klog.Infof("skipping non-CSI volume: %v", err)
			continue
		}

		vols = append(vols, vol)
	}

	return vols, nil
}

// newAttachedCSIVolume parses a UniqueVolumeName that can be obtained from the list of attached
// volumes on a node.
// UniqueVolumeName is a string like "kubernetes.io/csi/ebs.csi.aws.com^vol-0c577c9c55718d592"
// "vol-0c577c9c55718d592" is the volumeHandle (as in PV.Spec.CSI.volumeHandle)
func newAttachedCSIVolume(name corev1.UniqueVolumeName) (volume.CSIVolume, error) {
	const (
		csiDriverPrefix       = "kubernetes.io/csi/"
		driverVolumeSeparator = "^"
	)

	driverVolume, found := strings.CutPrefix(string(name), csiDriverPrefix)
	if !found {
		return nil, fmt.Errorf("attached volume %q is not a CSI volume", name)
	}

	parts := strings.Split(driverVolume, driverVolumeSeparator)
	if len(parts) != 2 {
		return nil, fmt.Errorf("format of attached volume %q is not supported: %s", name, driverVolume)
	}

	return volume.NewCSIVolume(parts[0], parts[1]), nil
}
