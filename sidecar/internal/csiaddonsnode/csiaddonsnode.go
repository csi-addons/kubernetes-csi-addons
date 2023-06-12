/*
Copyright 2021 The Kubernetes-CSI-Addons Authors.

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

package csiaddonsnode

import (
	"context"
	"errors"
	"fmt"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/apis/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	// nodeCreationRetry is the delay for calling newCSIAddonsNode after a
	// failure.
	nodeCreationRetry = time.Minute * 5
)

var (
	// errInvalidConfig is returned when an invalid configuration setting
	// is detected.
	errInvalidConfig = errors.New("invalid configuration")
)

// Manager is a helper that creates the CSIAddonsNode for the running sidecar.
type Manager struct {
	// Client contains the gRPC connection to the CSI-driver that supports
	// the CSI-Addons operations. This is used to get the identity of the
	// CSI-driver that is included in the CSIAddonsNode object.
	Client client.Client

	// Config is a ReST Config for the Kubernets API.
	Config *rest.Config

	// Node is the hostname of the system where the sidecar is running.
	Node string

	// Endpoint is the location where the sidecar receives connections on
	// from the CSI-Addons Controller.
	Endpoint string

	// PodName is the (unique) name of the Pod that contains this sidecar.
	PodName string

	// PodNamespace is the Kubernetes Namespace where the Pod with this
	// sidecar is running.
	PodNamespace string

	// PodUID is the UID of the Pod that contains this sidecar.
	PodUID string
}

// Deploy creates CSIAddonsNode custom resource with all required information.
// When information to create the CSIAddonsNode is missing, an error will be
// returned immediately. If creating the CSIAddonsNode in the Kubernetes
// cluster fails (missing CRD, RBAC limitations, ...), an error will be logged,
// and creation will be retried.
func (mgr *Manager) Deploy() error {
	object, err := mgr.getCSIAddonsNode()
	if err != nil {
		return fmt.Errorf("failed to get csiaddonsNode object: %w", err)
	}

	// loop until the CSIAddonsNode has been created
	return wait.PollImmediateInfinite(nodeCreationRetry, func() (bool, error) {
		err := mgr.newCSIAddonsNode(object)
		if err != nil {
			klog.Errorf("failed to create CSIAddonsNode %s/%s: %v",
				object.Namespace, object.Name, err)

			// return false to retry, discard the error
			return false, nil
		}

		// no error, so the CSIAddonsNode has been created
		return true, nil
	})
}

// newCSIAddonsNode initializes the CRD and creates the CSIAddonsNode object in
// the Kubernetes cluster.
// If the CSIAddonsNode object already exists, it will not be re-created or
// modified, and the existing object is kept as-is.
func (mgr *Manager) newCSIAddonsNode(node *csiaddonsv1alpha1.CSIAddonsNode) error {
	scheme, err := csiaddonsv1alpha1.SchemeBuilder.Build()
	if err != nil {
		return fmt.Errorf("failed to add scheme: %w", err)
	}

	crdConfig := *mgr.Config
	crdConfig.GroupVersion = &csiaddonsv1alpha1.GroupVersion
	crdConfig.APIPath = "/apis"
	crdConfig.NegotiatedSerializer = serializer.NewCodecFactory(scheme)
	crdConfig.UserAgent = rest.DefaultKubernetesUserAgent()

	c, err := rest.UnversionedRESTClientFor(&crdConfig)
	if err != nil {
		return fmt.Errorf("failed to get REST Client: %w", err)
	}

	err = c.Post().
		Resource("csiaddonsnodes").
		Namespace(node.Namespace).
		Name(node.Name).
		Body(node).
		Do(context.TODO()).
		Error()

	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create csiaddonsnode object: %w", err)
	}

	return nil
}

// getCSIAddonsNode fills required information and return CSIAddonsNode object.
func (mgr *Manager) getCSIAddonsNode() (*csiaddonsv1alpha1.CSIAddonsNode, error) {
	if mgr.PodName == "" {
		return nil, fmt.Errorf("%w: missing Pod name", errInvalidConfig)
	}
	if mgr.PodNamespace == "" {
		return nil, fmt.Errorf("%w: missing Pod namespace", errInvalidConfig)
	}
	if mgr.PodUID == "" {
		return nil, fmt.Errorf("%w: missing Pod UID", errInvalidConfig)
	}
	if mgr.Endpoint == "" {
		return nil, fmt.Errorf("%w: missing endpoint", errInvalidConfig)
	}
	if mgr.Node == "" {
		return nil, fmt.Errorf("%w: missing node", errInvalidConfig)
	}

	driver, err := mgr.Client.GetDriverName()
	if err != nil {
		return nil, fmt.Errorf("failed to get driver name: %w", err)
	}
	if driver == "" {
		return nil, fmt.Errorf("%w: CSI-driver returned an empty driver name",
			errInvalidConfig)
	}

	return &csiaddonsv1alpha1.CSIAddonsNode{
		ObjectMeta: v1.ObjectMeta{
			Name:      mgr.PodName,
			Namespace: mgr.PodNamespace,
			OwnerReferences: []v1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       mgr.PodName,
					UID:        types.UID(mgr.PodUID),
				},
			},
		},
		Spec: csiaddonsv1alpha1.CSIAddonsNodeSpec{
			Driver: csiaddonsv1alpha1.CSIAddonsNodeDriver{
				Name:     driver,
				EndPoint: mgr.Endpoint,
				NodeID:   mgr.Node,
			},
		},
	}, nil
}
