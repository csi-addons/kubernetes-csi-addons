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
	"reflect"
	"sync"
	"testing"

	"github.com/csi-addons/spec/lib/go/identity"
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
			if got := NewConnectionPool(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewConnectionPool() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConnectionPool_PutGetDelete(t *testing.T) {

	t.Run(("test single Put, Get, Delete"), func(t *testing.T) {
		cp := NewConnectionPool()
		key1 := "one"
		conn1 := &Connection{
			Capabilities: []*identity.Capability{},
			NodeID:       "one",
			DriverName:   "example.io",
			Timeout:      0,
		}

		cp.Put(key1, conn1)

		conns, unLock := cp.Get()
		if len(*conns) != 1 {
			t.Errorf("connection map should contain only one value: %+v", *conns)
		}
		for k, v := range *conns {
			if k == key1 {
				if !reflect.DeepEqual(*v, *conn1) {
					t.Errorf("expected value %+v is not equal to returned value %+v", Connection{}, *v)
				}
			}
		}
		unLock()

		cp.Delete(key1)

		conns, unLock = cp.Get()
		defer unLock()
		if len(*conns) != 0 {
			t.Errorf("Connection map should be empty: %+v", *conns)
		}
	})

	t.Run(("test Put with same key twice"), func(t *testing.T) {
		cp := NewConnectionPool()
		key1 := "one"
		conn1 := &Connection{
			Capabilities: []*identity.Capability{},
			NodeID:       "one",
			DriverName:   "example.io",
			Timeout:      0,
		}
		conn2 := &Connection{
			Capabilities: []*identity.Capability{},
			NodeID:       "two",
			DriverName:   "two.example.io",
			Timeout:      0,
		}

		cp.Put(key1, conn1)

		conns, unLock := cp.Get()
		for k, v := range *conns {
			if k == key1 {
				if !reflect.DeepEqual(*v, *conn1) {
					t.Errorf("expected value %+v is not equal to returned value %+v", *conn1, *v)
				}
			}
		}
		unLock()

		cp.Put(key1, conn2)

		conns, unLock = cp.Get()
		for k, v := range *conns {
			if k == key1 {
				if !reflect.DeepEqual(*v, *conn2) {
					t.Errorf("expected value %+v is not equal to returned value %+v", *conn2, *v)
				}
			}
		}
		unLock()

		cp.Delete(key1)

		conns, unLock = cp.Get()
		defer unLock()
		if len(*conns) != 0 {
			t.Errorf("Connection map should be empty: %+v", *conns)
		}
	})

	t.Run(("test Delete with same key twice"), func(t *testing.T) {
		cp := NewConnectionPool()
		key1 := "one"
		conn1 := &Connection{
			Capabilities: []*identity.Capability{},
			NodeID:       "one",
			DriverName:   "example.io",
			Timeout:      0,
		}
		cp.Put(key1, conn1)

		conns, unLock := cp.Get()
		for k, v := range *conns {
			if k == key1 {
				if !reflect.DeepEqual(*v, *conn1) {
					t.Errorf("expected value %+v is not equal to returned value %+v", *conn1, *v)
				}
			}
		}
		unLock()

		cp.Delete(key1)
		cp.Delete(key1)

		conns, unLock = cp.Get()
		defer unLock()
		if len(*conns) != 0 {
			t.Errorf("Connection map should be empty: %+v", *conns)
		}
	})

}
