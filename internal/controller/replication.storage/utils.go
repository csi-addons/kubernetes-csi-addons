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

package controller

import (
	"context"
	"fmt"
	"time"

	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
	grpcClient "github.com/csi-addons/kubernetes-csi-addons/internal/client"
	conn "github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/csi-addons/spec/lib/go/identity"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetReplicationState(instanceState replicationv1alpha1.ReplicationState) replicationv1alpha1.State {
	switch instanceState {
	case replicationv1alpha1.Primary:
		return replicationv1alpha1.PrimaryState
	case replicationv1alpha1.Secondary:
		return replicationv1alpha1.SecondaryState
	case replicationv1alpha1.Resync:
		return replicationv1alpha1.SecondaryState
	}

	return replicationv1alpha1.UnknownState
}

func GetCurrentReplicationState(instanceStatusState replicationv1alpha1.State) replicationv1alpha1.State {
	if instanceStatusState == "" {
		return replicationv1alpha1.UnknownState
	}

	return instanceStatusState
}

func WaitForVolumeReplicationResource(client client.Client, logger logr.Logger, resourceName string) error {
	unstructuredResource := &unstructured.UnstructuredList{}
	unstructuredResource.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   replicationv1alpha1.GroupVersion.Group,
		Kind:    resourceName,
		Version: replicationv1alpha1.GroupVersion.Version,
	})
	for {
		err := client.List(context.TODO(), unstructuredResource)
		if err == nil {
			return nil
		}
		// return errors other than NoMatch
		if !meta.IsNoMatchError(err) {
			logger.Error(err, "got an unexpected error while waiting for resource", "Resource", resourceName)
			return err
		}
		logger.Info("resource does not exist", "Resource", resourceName)
		time.Sleep(5 * time.Second)
	}
}

type connPoolReconciler interface {
	getConnPool() *conn.ConnectionPool
	getClient() client.Client
	getTimeout() time.Duration
}

func (r *VolumeReplicationReconciler) getConnPool() *conn.ConnectionPool {
	return r.Connpool
}

func (r *VolumeReplicationReconciler) getClient() client.Client {
	return r.Client
}

func (r *VolumeReplicationReconciler) getTimeout() time.Duration {
	return r.Timeout
}

func (r *VolumeGroupReplicationReconciler) getConnPool() *conn.ConnectionPool {
	return r.Connpool
}

func (r *VolumeGroupReplicationReconciler) getClient() client.Client {
	return r.Client
}

func (r *VolumeGroupReplicationReconciler) getTimeout() time.Duration {
	return r.Timeout
}

func (r *VolumeGroupReplicationContentReconciler) getConnPool() *conn.ConnectionPool {
	return r.Connpool
}

func (r *VolumeGroupReplicationContentReconciler) getClient() client.Client {
	return r.Client
}

func (r *VolumeGroupReplicationContentReconciler) getTimeout() time.Duration {
	return r.Timeout
}

func getReplicationClient(ctx context.Context, r connPoolReconciler, driverName, dataSource string) (grpcClient.VolumeReplication, bool, error) {
	conn, err := r.getConnPool().GetLeaderByDriver(ctx, r.getClient(), driverName)
	if err != nil {
		return nil, false, fmt.Errorf("no leader for the ControllerService of driver %q: %w", driverName, err)
	}

	var replicationClient grpcClient.VolumeReplication
	var supportsGetReplicationDestinationInfo bool

	for _, cap := range conn.Capabilities {
		// validate if VOLUME_REPLICATION capability is supported by the driver.
		if cap.GetVolumeReplication() == nil {
			continue
		}

		if cap.GetVolumeReplication().GetType() == identity.Capability_VolumeReplication_GET_REPLICATION_DESTINATION_INFO {
			supportsGetReplicationDestinationInfo = true
			continue
		}

		// validate if VOLUME_REPLICATION capability is enabled by the storage driver.
		if cap.GetVolumeReplication().GetType() == identity.Capability_VolumeReplication_VOLUME_REPLICATION {
			switch dataSource {
			case pvcDataSource:
				replicationClient = grpcClient.NewVolumeReplicationClient(conn.Client, r.getTimeout())
			case volumeGroupReplicationDataSource:
				replicationClient = grpcClient.NewVolumeGroupReplicationClient(conn.Client, r.getTimeout())
			}
		}
	}

	if replicationClient != nil {
		return replicationClient, supportsGetReplicationDestinationInfo, nil
	}

	return nil, false, fmt.Errorf("leading CSIAddonsNode %q for driver %q does not support VolumeReplication", conn.Name, driverName)
}
