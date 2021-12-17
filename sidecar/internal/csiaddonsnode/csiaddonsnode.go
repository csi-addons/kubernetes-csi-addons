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
	"strings"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/v1alpha1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

const (
	podNameEnvKey      = "POD_NAME"
	podNamespaceEnvKey = "POD_NAMESPACE"
	podUIDEnvKey       = "POD_UID"
	podIPEnvKey        = "POD_IP"
)

// Deploy creates CSIAddonsNode custom resource with all required information.
func Deploy(config *rest.Config, driverName, nodeID, endpoint string) error {
	object, err := getCSIAddonsNode(driverName, endpoint, nodeID)
	if err != nil {
		return fmt.Errorf("failed to get csiaddonsNode object: %w", err)
	}

	err = csiaddonsv1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return fmt.Errorf("failed to add scheme: %w", err)
	}

	crdConfig := *config
	crdConfig.ContentConfig.GroupVersion = &csiaddonsv1alpha1.GroupVersion
	crdConfig.APIPath = "/apis"
	crdConfig.NegotiatedSerializer = serializer.NewCodecFactory(scheme.Scheme)
	crdConfig.UserAgent = rest.DefaultKubernetesUserAgent()

	c, err := rest.UnversionedRESTClientFor(&crdConfig)
	if err != nil {
		return fmt.Errorf("failed to get REST Client: %w", err)
	}

	err = c.Post().
		Resource("csiaddonsnodes").
		Namespace(object.Namespace).
		Name(object.Name).
		Body(object).
		Do(context.TODO()).
		Into(nil)

	if err != nil && !strings.Contains(err.Error(), "already exists") {
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
	podIP, err := lookupEnv(podIPEnvKey)
	if err != nil {
		return nil, err
	}

	return &csiaddonsv1alpha1.CSIAddonsNode{
		ObjectMeta: v1.ObjectMeta{
			Name:      podUID,
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
				EndPoint: podIP + ":" + endpoint,
				NodeID:   nodeID,
			},
		},
	}, nil
}
