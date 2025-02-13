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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/sidecar/internal/client"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// nodeCreationRetry is the delay for calling newCSIAddonsNode after a
	// failure.
	nodeCreationRetry = time.Minute * 5
	// nodeCreationTimeout is the time after which the context for node creation request is cancelled.
	nodeCreationTimeout = time.Minute * 3

	// watcherRetryCount is the number of times a watcher creation would be retried for.
	watcherRetryCount = 10
	// watcherRetryDelay is the amount of time to wait before trying recreation of a watcher.
	watcherRetryDelay = time.Second * 5
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

	// Config is a ReST Config for the Kubernetes API.
	Config *rest.Config

	// kubernetes client to interact with the Kubernetes API.
	KubeClient kubernetes.Interface

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

// deploy creates CSIAddonsNode custom resource with all required information.
// If creating the CSIAddonsNode in the Kubernetes cluster fails (missing CRD, RBAC limitations, ...)
// an error will be logged and creation will be retried.
func (mgr *Manager) deploy(object *csiaddonsv1alpha1.CSIAddonsNode) error {

	// loop until the CSIAddonsNode has been created
	return wait.PollUntilContextTimeout(context.TODO(), nodeCreationRetry, nodeCreationTimeout, true, func(ctx context.Context) (bool, error) {
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
	s := runtime.NewScheme()
	if err := csiaddonsv1alpha1.AddToScheme(s); err != nil {
		return fmt.Errorf("failed to register scheme: %w", err)
	}

	cli, err := ctrlClient.New(mgr.Config, ctrlClient.Options{Scheme: s})
	if err != nil {
		return fmt.Errorf("failed to create controller-runtime client: %w", err)
	}
	ctx := context.TODO()
	csiaddonNode := &csiaddonsv1alpha1.CSIAddonsNode{
		ObjectMeta: v1.ObjectMeta{
			Name:      node.Name,
			Namespace: node.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, cli, csiaddonNode, func() error {
		if !csiaddonNode.DeletionTimestamp.IsZero() {
			return errors.New("csiaddonnode is being deleted")
		}

		// update the resourceVersion
		resourceVersion := csiaddonNode.ResourceVersion
		if resourceVersion != "" {
			csiaddonNode.ResourceVersion = resourceVersion
		}
		node.Spec.DeepCopyInto(&csiaddonNode.Spec)
		// set the ownerReferences
		csiaddonNode.ObjectMeta.OwnerReferences = node.ObjectMeta.OwnerReferences
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create/update csiaddonsnode object: %w", err)
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

	// Get the owner of the pod as we want to set the owner of the CSIAddonsNode
	// so that it won't get deleted when the pod is deleted rather it will be deleted
	// when the owner is deleted.

	pod, err := mgr.KubeClient.CoreV1().Pods(mgr.PodNamespace).Get(context.TODO(), mgr.PodName, v1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	if len(pod.OwnerReferences) == 0 {
		return nil, fmt.Errorf("%w: pod has no owner", errInvalidConfig)
	}

	ownerKindForCSIAddonsName := ""
	ownerNameForCSIAddonsName := ""

	ownerReferences := make([]v1.OwnerReference, 1)
	if pod.OwnerReferences[0].Kind == "ReplicaSet" {
		// If the pod is owned by a ReplicaSet, we need to get the owner of the ReplicaSet i.e. Deployment
		rs, err := mgr.KubeClient.AppsV1().ReplicaSets(mgr.PodNamespace).Get(context.TODO(), pod.OwnerReferences[0].Name, v1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get replicaset: %w", err)
		}
		if len(rs.OwnerReferences) == 0 {
			return nil, fmt.Errorf("%w: replicaset has no owner", errInvalidConfig)
		}
		ownerKindForCSIAddonsName = rs.OwnerReferences[0].Kind
		ownerNameForCSIAddonsName = rs.OwnerReferences[0].Name

		// The pod (created using deployment) might move to new nodes and this might create the
		// stale CSIAddonsNode object.
		// So, we need to set the pod as the owner for the CSIAddonsNode as we dont store any details
		// that are required later on for any other operations like Fencing etc.
		ownerReferences[0] = v1.OwnerReference{
			APIVersion: "v1",
			Kind:       "Pod",
			Name:       pod.Name,
			UID:        types.UID(mgr.PodUID),
		}
	} else {
		ownerKindForCSIAddonsName = pod.OwnerReferences[0].Kind
		ownerNameForCSIAddonsName = pod.OwnerReferences[0].Name
		// If the pod is owned by DeamonSet or StatefulSet get the owner of the pod.
		ownerReferences[0] = pod.OwnerReferences[0]
	}

	// we need to have the constant name for the CSIAddonsNode object.
	// We will use the nodeID and the ownerName for the CSIAddonsNode object name.
	name, err := generateName(mgr.Node, mgr.PodNamespace, ownerKindForCSIAddonsName, ownerNameForCSIAddonsName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate name: %w", err)
	}

	return &csiaddonsv1alpha1.CSIAddonsNode{
		ObjectMeta: v1.ObjectMeta{
			Name:            name,
			Namespace:       mgr.PodNamespace,
			OwnerReferences: ownerReferences,
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

func generateName(nodeID, namespace, ownerKind, ownerName string) (string, error) {
	if nodeID == "" {
		return "", fmt.Errorf("nodeID is required")
	}
	if ownerKind == "" {
		return "", fmt.Errorf("ownerKind is required")
	}
	if ownerName == "" {
		return "", fmt.Errorf("ownerName is required")
	}

	if namespace == "" {
		return "", fmt.Errorf("namespace is required")
	}
	// convert ownerKind to lowercase as the name should be case-insensitive
	ownerKind = strings.ToLower(ownerKind)
	base := fmt.Sprintf("%s-%s-%s-%s", nodeID, namespace, ownerKind, ownerName)
	if len(base) > 253 {
		// Generate a UUID based on nodeID, ownerKind, and ownerName
		data := nodeID + namespace + ownerKind + ownerName
		hash := sha256.Sum256([]byte(data))
		uuid := hex.EncodeToString(hash[:8])                            // Use the first 8 characters of the hash as a UUID-like string
		finalName := fmt.Sprintf("%s-%s", base[:251-len(uuid)-1], uuid) // Ensure total length is within 253
		return finalName, nil
	}

	return base, nil
}

// watchCSIAddonsNode starts a watcher for a specific CSIAddonsNode resource identified by its name.
// If a CSIAddonsNode is deleted, it logs a warning and attempts to recreate it using mgr.deploy()
func (mgr *Manager) watchCSIAddonsNode(node *csiaddonsv1alpha1.CSIAddonsNode) error {
	// Call deploy on start, this takes care of the cases where
	// a watcher might exit due to an error while trying to
	// recreate CSIAddonsNode in the cluster
	err := mgr.deploy(node)
	if err != nil {
		klog.Fatalf("Failed to create csiaddonsnode: %v", err)
	}

	klog.Infof("Starting watcher for CSIAddonsNode: %s", node.Name)

	dynamicClient, err := dynamic.NewForConfig(mgr.Config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	gvr := schema.GroupVersionResource{
		Group:    csiaddonsv1alpha1.GroupVersion.Group,
		Version:  csiaddonsv1alpha1.GroupVersion.Version,
		Resource: "csiaddonsnodes",
	}

	watcher, err := dynamicClient.Resource(gvr).Namespace(node.Namespace).Watch(context.Background(), v1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", node.Name),
	})
	if err != nil {
		return fmt.Errorf("failed to watch CSIAddonsNode objects: %w", err)
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		switch event.Type {
		case watch.Deleted:
			klog.Infof("WARNING: An active CSIAddonsNode: %s was deleted, it will be recreated", node.Name)

			err := mgr.deploy(node)
			if err != nil {
				return fmt.Errorf("failed to recreate CSIAddonsNode: %w", err)
			}
			klog.Infof("CSIAddonsNode: %s recreated", node.Name)
		}
	}

	// The channel was closed by the API server without any errors
	// Simply log it here and return, the dispatcher is responsible
	// for restarting the watcher
	klog.Infof("Watcher for %s exited gracefully, will be restarted soon", node.Name)

	return nil
}

// DispatchWatcher starts a watcher for the CSIAddonsNode and retries
// if the watcher exits due to an error. It will retry up to a maximum number of
// attempts defined by watcherRetryCount before returning an error.
func (mgr *Manager) DispatchWatcher() error {
	retryCount := 0
	node, err := mgr.getCSIAddonsNode()
	if err != nil {
		return errors.New("failed to get CSIAddonsNode object")
	}

	for retryCount < int(watcherRetryCount) {
		err := mgr.watchCSIAddonsNode(node)
		if err != nil {
			klog.Errorf("Watcher for %s exited, retrying (%d/%d), error: %v", node.Name, retryCount+1, watcherRetryCount, err)

			retryCount++
			time.Sleep(watcherRetryDelay * time.Duration(retryCount))
		} else {
			retryCount = 0
		}
	}
	return fmt.Errorf("watcher for %s reached max retries, giving up", node.Name)
}
