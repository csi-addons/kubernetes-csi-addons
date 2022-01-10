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
	"fmt"
	"os"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	podNameEnvKey      = "POD_NAME"
	podNamespaceEnvKey = "POD_NAMESPACE"
	podUIDEnvKey       = "POD_UID"

	// nodeCreationRetry is the delay for calling newCSIAddonsNode after a
	// failure.
	nodeCreationRetry = time.Minute * 5
)

// Deploy creates CSIAddonsNode custom resource with all required information.
// When information to create the CSIAddonsNode is missing, an error will be
// returned immediately. If creating the CSIAddonsNode in the Kubernetes
// cluster fails (missing CRD, RBAC limitations, ...), an error will be logged,
// and creation will be retried.
func Deploy(config *rest.Config, driverName, nodeID, endpoint string) error {
	object, err := getCSIAddonsNode(driverName, endpoint, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get csiaddonsNode object: %w", err)
	}

	// loop until the CSIAddonsNode has been created
	wait.PollImmediateInfinite(nodeCreationRetry, func() (bool, error) {
		err := newCSIAddonsNode(config, object)
		if err != nil {
			klog.Errorf("failed to create CSIAddonsNode %s/%s: %v",
				object.Namespace, object.Name, err)

			// return false to retry, discard the error
			return false, nil
		}

		// no error, so the CSIAddonsNode has been created
		return true, nil
	})

	return nil
}

// newCSIAddonsNode initializes the CRD and creates the CSIAddonsNode object in
// the Kubernetes cluster.
// If the CSIAddonsNode object already exists, it will not be re-created or
// modified, and the existing object is kept as-is.
func newCSIAddonsNode(config *rest.Config, node *csiaddonsv1alpha1.CSIAddonsNode) error {
	scheme, err := csiaddonsv1alpha1.SchemeBuilder.Build()
	if err != nil {
		return fmt.Errorf("failed to add scheme: %w", err)
	}

	crdConfig := *config
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

	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create csiaddonsnode object: %w", err)
	}

	return nil
}

// lookupEnc returns environmental variable value given the name.
func lookupEnv(name string) (string, error) {
	val, ok := os.LookupEnv(name)
	if !ok {
		return val, fmt.Errorf("required environmental variable %q not found", name)
	}

	return val, nil
}

// getCSIAddonsNode fills required information and return CSIAddonsNode object.
func getCSIAddonsNode(driverName, endpoint, nodeID string) (*csiaddonsv1alpha1.CSIAddonsNode, error) {
	podName, err := lookupEnv(podNameEnvKey)
	if err != nil {
		return nil, err
	}
	podNamespace, err := lookupEnv(podNamespaceEnvKey)
	if err != nil {
		return nil, err
	}
	podUID, err := lookupEnv(podUIDEnvKey)
	if err != nil {
		return nil, err
	}

	return &csiaddonsv1alpha1.CSIAddonsNode{
		ObjectMeta: v1.ObjectMeta{
			Name:      podName,
			Namespace: podNamespace,
			OwnerReferences: []v1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       podName,
					UID:        types.UID(podUID),
				},
			},
		},
		Spec: csiaddonsv1alpha1.CSIAddonsNodeSpec{
			Driver: csiaddonsv1alpha1.CSIAddonsNodeDriver{
				Name:     driverName,
				EndPoint: endpoint,
				NodeID:   nodeID,
			},
		},
	}, nil
}
