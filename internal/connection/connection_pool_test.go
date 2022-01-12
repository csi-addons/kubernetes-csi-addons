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
	"sync"
	"testing"

	"github.com/csi-addons/spec/lib/go/identity"
	"github.com/stretchr/testify/assert"
)

func TestNewConnectionPool(t *testing.T) {
	tests := []struct {
		name string
		want *ConnectionPool
	}{
		{
			name: "Get new ConnectionPool object",
			want: &ConnectionPool{
				pool:   make(map[string]*Connection),
				rwlock: &sync.RWMutex{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := NewConnectionPool()
			assert.Equal(t, tt.want, res)
		})
	}
}

func TestConnectionPool_PutGetDelete(t *testing.T) {

	t.Run(("test single Put, Get, Get without nodeID, Delete"), func(t *testing.T) {
		cp := NewConnectionPool()
		driverName := "example.io"
		nodeID := "one"
		key1 := "one"
		conn1 := &Connection{
			Capabilities: []*identity.Capability{},
			NodeID:       nodeID,
			DriverName:   driverName,
			Timeout:      0,
		}

		cp.Put(key1, conn1)

		conns := cp.GetByNodeID(driverName, nodeID)
		assert.Equal(t, 1, len(conns))
		for k, v := range conns {
			if k == key1 {
				assert.Equal(t, *conn1, *v)
			}
		}

		// nodeID is optional
		conns = cp.GetByNodeID(driverName, "")
		assert.Equal(t, 1, len(conns))
		for k, v := range conns {
			if k == key1 {
				assert.Equal(t, *conn1, *v)
			}
		}

		// non-matching driverName
		conns = cp.GetByNodeID("", "")
		assert.Equal(t, 0, len(conns))

		cp.Delete(key1)

		conns = cp.GetByNodeID(driverName, nodeID)
		assert.Empty(t, conns)
	})

	t.Run(("test Put with same key twice"), func(t *testing.T) {
		cp := NewConnectionPool()
		driverName := "example.io"
		nodeID := "one"
		key1 := "one"
		conn1 := &Connection{
			Capabilities: []*identity.Capability{},
			NodeID:       nodeID,
			DriverName:   driverName,
			Timeout:      0,
		}

		conn2 := &Connection{
			Capabilities: []*identity.Capability{
				{
					Type: &identity.Capability_Service_{
						Service: &identity.Capability_Service{
							Type: 0,
						},
					},
				},
			},
			NodeID:     nodeID,
			DriverName: driverName,
			Timeout:    0,
		}

		cp.Put(key1, conn1)

		conns := cp.GetByNodeID(driverName, nodeID)
		for k, v := range conns {
			if k == key1 {
				assert.Equal(t, *conn1, *v)
			}
		}

		cp.Put(key1, conn2)

		conns = cp.GetByNodeID(driverName, nodeID)
		for k, v := range conns {
			if k == key1 {
				assert.Equal(t, *conn2, *v)
			}
		}

		cp.Delete(key1)

		conns = cp.GetByNodeID(driverName, nodeID)
		assert.Empty(t, conns)
	})

	t.Run(("test Delete with same key twice"), func(t *testing.T) {
		cp := NewConnectionPool()
		driverName := "example.io"
		nodeID := "one"
		key1 := "one"
		conn1 := &Connection{
			Capabilities: []*identity.Capability{},
			NodeID:       nodeID,
			DriverName:   driverName,
			Timeout:      0,
		}
		cp.Put(key1, conn1)

		conns := cp.GetByNodeID(driverName, nodeID)
		for k, v := range conns {
			if k == key1 {
				assert.Equal(t, *conn1, *v)
			}
		}

		cp.Delete(key1)
		cp.Delete(key1)

		conns = cp.GetByNodeID(driverName, nodeID)
		assert.Empty(t, conns)
	})

	t.Run(("test Put after Get, verify no change in first Get variable"), func(t *testing.T) {
		cp := NewConnectionPool()
		driverName := "example.io"
		nodeID := "one"
		key1 := "one"
		conn1 := &Connection{
			Capabilities: []*identity.Capability{},
			NodeID:       nodeID,
			DriverName:   driverName,
			Timeout:      0,
		}
		cp.Put(key1, conn1)

		conns := cp.GetByNodeID(driverName, nodeID)
		for k, v := range conns {
			if k == key1 {
				assert.Equal(t, *conn1, *v)
			}
		}

		conn2 := &Connection{
			Capabilities: []*identity.Capability{},
			NodeID:       nodeID,
			DriverName:   driverName,
			Timeout:      1,
		}
		cp.Put(key1, conn2)
		assert.Equal(t, 1, len(conns))
		for k, v := range conns {
			if k == key1 {
				assert.Equal(t, *conn1, *v)
			}
		}
	})
}

func TestConnectionPool_getByDriverName(t *testing.T) {
	type fields struct {
		pool   map[string]*Connection
		rwlock *sync.RWMutex
	}
	type args struct {
		driverName string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   map[string]*Connection
	}{
		{
			name: "matching driverName present",
			fields: fields{
				pool: map[string]*Connection{
					"one": {
						DriverName: "driver-1",
					},
					"two": {
						DriverName: "driver-2",
					},
					"three": {
						DriverName: "driver-1",
					},
				},
				rwlock: &sync.RWMutex{},
			},
			args: args{
				driverName: "driver-1",
			},
			want: map[string]*Connection{
				"one": {
					DriverName: "driver-1",
				},
				"three": {
					DriverName: "driver-1",
				},
			},
		},
		{
			name: "matching driverName absent",
			fields: fields{
				pool: map[string]*Connection{
					"one": {
						DriverName: "driver-1",
					},
					"two": {
						DriverName: "driver-2",
					},
				},
				rwlock: &sync.RWMutex{},
			},
			args: args{
				driverName: "driver-3",
			},
			want: map[string]*Connection{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := &ConnectionPool{
				pool:   tt.fields.pool,
				rwlock: tt.fields.rwlock,
			}
			res := cp.getByDriverName(tt.args.driverName)
			assert.Equal(t, tt.want, res)
		})
	}
}
