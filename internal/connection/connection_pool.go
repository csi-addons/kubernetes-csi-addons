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

import "sync"

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

// GetByNodeID returns map of connections, filtered with given driverName and optional nodeID.
func (cp *ConnectionPool) GetByNodeID(driverName, nodeID string) map[string]*Connection {
	cp.rwlock.RLock()
	defer cp.rwlock.RUnlock()

	newPool := make(map[string]*Connection)
	for k, v := range cp.pool {
		if v.DriverName != driverName {
			continue
		}
		if nodeID != "" && v.NodeID != nodeID {
			continue
		}
		newPool[k] = v
	}

	return newPool
}
