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

package connection

import (
	"context"
	"fmt"
	"sync"

	coordination "k8s.io/api/coordination/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/csi-addons/kubernetes-csi-addons/internal/util"
)

//+kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch

// ConnectionPool consists of map of Connection objects and
// methods Put, Get & Delete which operates with required rw locks
// to ensure consistency.
// CSIAddonsNode controller will use Put and Delete methods whereas
// other controllers will make use of Get to choose and connect to sidecar.
type ConnectionPool struct {
	pool   map[string]*Connection
	rwlock *sync.RWMutex
}

// NewConntionPool initializes and returns ConnectionPool object.
func NewConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		pool:   make(map[string]*Connection),
		rwlock: &sync.RWMutex{},
	}
}

// Put adds connection object into map.
func (cp *ConnectionPool) Put(key string, conn *Connection) {
	cp.rwlock.Lock()
	defer cp.rwlock.Unlock()

	oldConn, ok := cp.pool[key]
	if ok {
		oldConn.Close()
	}

	cp.pool[key] = conn
}

// Delete deletes connection object corresponding to given key.
func (cp *ConnectionPool) Delete(key string) {
	cp.rwlock.Lock()
	defer cp.rwlock.Unlock()

	conn, ok := cp.pool[key]
	if ok {
		conn.Close()
	}

	delete(cp.pool, key)
}

// getByDriverName returns map of connections filtered by driverName. This function
// must be called with read lock held.
func (cp *ConnectionPool) getByDriverName(driverName string) map[string]*Connection {
	newPool := make(map[string]*Connection)
	for k, v := range cp.pool {
		if v.DriverName != driverName {
			continue
		}
		newPool[k] = v
	}

	return newPool
}

// GetByNodeID returns map of connections, filtered with given driverName and optional nodeID.
func (cp *ConnectionPool) GetByNodeID(driverName, nodeID string) map[string]*Connection {
	cp.rwlock.RLock()
	defer cp.rwlock.RUnlock()

	pool := cp.getByDriverName(driverName)
	result := make(map[string]*Connection)
	for k, v := range pool {
		// since nodeID is options,check only if it is not empty
		if nodeID != "" && v.NodeID != nodeID {
			continue
		}
		result[k] = v
	}

	return result
}

// getNamespaceByDriverName loops through the connections in the pool and
// returns the Namespace of the first connection that matches the driverName.
func (cp *ConnectionPool) getNamespaceByDriverName(driverName string) (string, error) {
	cp.rwlock.RLock()
	defer cp.rwlock.RUnlock()

	for _, conn := range cp.pool {
		if conn.DriverName != driverName {
			continue
		}

		return conn.Namespace, nil
	}

	// should be impossible to get here, all Connections have a Namespace
	return "", fmt.Errorf("failed to find the namespace where driver %q is running", driverName)
}

// GetLeaderByDriver finds the holder of the lease for the driver, and returns
// the connection to that particular CSI-Addons sidecar.
func (cp *ConnectionPool) GetLeaderByDriver(ctx context.Context, reconciler client.Client, driverName string) (*Connection, error) {
	// detect the Namespace where the driver is deployed
	ns, err := cp.getNamespaceByDriverName(driverName)
	if err != nil {
		return nil, err
	}

	// get the Lease for the driver
	leaseName := util.NormalizeLeaseName(driverName) + "-csi-addons"
	var lease coordination.Lease
	err = reconciler.Get(
		ctx,
		client.ObjectKey{
			Namespace: ns,
			Name:      leaseName,
		},
		&lease)
	if err != nil {
		return nil, fmt.Errorf("no leader found for driver %q: %w", driverName, err)
	} else if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity == "" {
		return nil, fmt.Errorf("lease %q for driver %q does not have a leader", leaseName, driverName)
	}

	// get the connection for the leader
	key := lease.Namespace + "/" + *lease.Spec.HolderIdentity

	cp.rwlock.RLock()
	defer cp.rwlock.RUnlock()

	conn, ok := cp.pool[key]
	if !ok {
		return nil, fmt.Errorf("no connection with key %q found for driver %q: %w", key, driverName, err)
	}

	return conn, nil
}
