/*
Copyright 2022 The Kubernetes-CSI-Addons Authors.

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
	"testing"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/connection"
	"github.com/csi-addons/spec/lib/go/identity"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestParseEndpoint(t *testing.T) {
	// test empty namespace
	_, _, _, err := parseEndpoint("pod://pod-name:5678")
	assert.Error(t, err)

	// test empty namespace
	_, _, _, err = parseEndpoint("pod://pod-name.:5678")
	assert.Error(t, err)

	namespace, podname, port, err := parseEndpoint("pod://pod-name.csi-addons:5678")
	assert.NoError(t, err)
	assert.Equal(t, namespace, "csi-addons")
	assert.Equal(t, podname, "pod-name")
	assert.Equal(t, port, "5678")

	namespace, podname, port, err = parseEndpoint("pod://csi.pod.ns-cluster.local:5678")
	assert.NoError(t, err)
	assert.Equal(t, namespace, "local")
	assert.Equal(t, podname, "csi.pod.ns-cluster")
	assert.Equal(t, port, "5678")

	// test empty podname
	_, _, _, err = parseEndpoint("pod://.local:5678")
	assert.Error(t, err)

}

func TestParseCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		caps     []*identity.Capability
		expected []string
	}{
		{
			name:     "Empty capabilities",
			caps:     []*identity.Capability{},
			expected: []string{},
		},
		{
			name: "Single capability",
			caps: []*identity.Capability{
				{
					Type: &identity.Capability_Service_{
						Service: &identity.Capability_Service{
							Type: identity.Capability_Service_NODE_SERVICE,
						},
					},
				},
			},
			expected: []string{"service.NODE_SERVICE"},
		},
		{
			name: "Multiple capabilities",
			caps: []*identity.Capability{
				{
					Type: &identity.Capability_Service_{
						Service: &identity.Capability_Service{
							Type: identity.Capability_Service_NODE_SERVICE,
						},
					},
				},
				{
					Type: &identity.Capability_ReclaimSpace_{
						ReclaimSpace: &identity.Capability_ReclaimSpace{
							Type: identity.Capability_ReclaimSpace_ONLINE,
						},
					},
				},
			},
			expected: []string{"service.NODE_SERVICE", "reclaim_space.ONLINE"},
		},
		{
			name: "Same capability with different types",
			caps: []*identity.Capability{
				{
					Type: &identity.Capability_ReclaimSpace_{
						ReclaimSpace: &identity.Capability_ReclaimSpace{
							Type: identity.Capability_ReclaimSpace_ONLINE,
						},
					},
				},
				{
					Type: &identity.Capability_ReclaimSpace_{
						ReclaimSpace: &identity.Capability_ReclaimSpace{
							Type: identity.Capability_ReclaimSpace_OFFLINE,
						},
					},
				},
			},
			expected: []string{"reclaim_space.ONLINE", "reclaim_space.OFFLINE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCapabilities(tt.caps)
			assert.Equal(t, tt.expected, result)
		})
	}
}
func TestGetRetryCountFromReason(t *testing.T) {
	tests := []struct {
		name    string
		reason  string
		want    int
		wantErr bool
	}{
		{
			name:    "empty reason",
			reason:  "",
			want:    0,
			wantErr: false,
		},
		{
			name:    "valid reason",
			reason:  "retry: 2",
			want:    2,
			wantErr: false,
		},
		{
			name:    "valid with extra spaces",
			reason:  "retry:    5",
			want:    5,
			wantErr: false,
		},
		{
			name:    "valid with trailing spaces",
			reason:  "something:  10  ",
			want:    10,
			wantErr: false,
		},
		{
			name:    "no colon",
			reason:  "retry 3",
			want:    0,
			wantErr: true,
		},
		{
			name:    "non-integer value",
			reason:  "retry: abc",
			want:    0,
			wantErr: true,
		},
		{
			name:    "multiple colons",
			reason:  "prefix: 7:extra",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getRetryCountFromReason(tt.reason)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestStaleConnectionKeys(t *testing.T) {
	pod := func(name string) corev1.Pod {
		return corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"}}
	}
	terminating := func(p corev1.Pod) corev1.Pod {
		p.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		return p
	}
	tests := []struct {
		name  string
		conns []string
		key   string
		pods  []corev1.Pod
		want  []string
	}{
		{
			name:  "only the current connection",
			conns: []string{"ns/pod-new"},
			key:   "ns/pod-new",
			pods:  []corev1.Pod{pod("pod-new")},
		},
		{
			name:  "connection to a replaced Pod",
			conns: []string{"ns/pod-old", "ns/pod-new"},
			key:   "ns/pod-new",
			pods:  []corev1.Pod{pod("pod-new")},
			want:  []string{"ns/pod-old"},
		},
		{
			name:  "current connection is kept before its Pod is listed",
			conns: []string{"ns/pod-old", "ns/pod-new"},
			key:   "ns/pod-new",
			want:  []string{"ns/pod-old"},
		},
		{
			name:  "replica on the same node is kept",
			conns: []string{"ns/pod-a", "ns/pod-b"},
			key:   "ns/pod-b",
			pods:  []corev1.Pod{pod("pod-a"), pod("pod-b")},
		},
		{
			name:  "connection to a terminating Pod",
			conns: []string{"ns/pod-old", "ns/pod-new"},
			key:   "ns/pod-new",
			pods:  []corev1.Pod{terminating(pod("pod-old")), pod("pod-new")},
			want:  []string{"ns/pod-old"},
		},
		{
			name:  "current connection is kept while its Pod is terminating",
			conns: []string{"ns/pod-a", "ns/pod-new"},
			key:   "ns/pod-new",
			pods:  []corev1.Pod{pod("pod-a"), terminating(pod("pod-new"))},
		},
		{
			name:  "Pod name is normalized like the pool key",
			conns: []string{"ns/rbd-csi-ceph-com-nodeplugin-a", "ns/rbd-csi-ceph-com-nodeplugin-b"},
			key:   "ns/rbd-csi-ceph-com-nodeplugin-b",
			pods:  []corev1.Pod{pod("rbd.csi.ceph.com-nodeplugin-a"), pod("rbd.csi.ceph.com-nodeplugin-b")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conns := make(map[string]*connection.Connection)
			for _, k := range tt.conns {
				conns[k] = &connection.Connection{}
			}
			assert.ElementsMatch(t, tt.want, staleConnectionKeys(conns, tt.key, "ns", tt.pods))
		})
	}
}

// newTestCSIAddonsNode returns CSIAddonsNode "ns/node-1" with the given endpoint.
func newTestCSIAddonsNode(endpoint string) *csiaddonsv1alpha1.CSIAddonsNode {
	return &csiaddonsv1alpha1.CSIAddonsNode{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1", Namespace: "ns"},
		Spec: csiaddonsv1alpha1.CSIAddonsNodeSpec{
			Driver: csiaddonsv1alpha1.CSIAddonsNodeDriver{Name: "driver", NodeID: "node-1", EndPoint: endpoint},
		},
	}
}

// newTestConnPool returns a pool with a connection of CSIAddonsNode "ns/node-1"
// stored under each of the given keys.
func newTestConnPool(keys ...string) *connection.ConnectionPool {
	pool := connection.NewConnectionPool()
	for _, k := range keys {
		pool.Put(k, &connection.Connection{Namespace: "ns", Name: "node-1"})
	}
	return pool
}

func TestRemoveStaleConnections(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	pod := func(name string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"}}
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod("pod-a"), pod("pod-new")).Build()
	logger := logr.Discard()

	pool := newTestConnPool("ns/pod-old", "ns/pod-a", "ns/pod-new")
	r := &CSIAddonsNodeReconciler{Client: cl, ConnPool: pool}
	r.removeStaleConnections(context.Background(), &logger, newTestCSIAddonsNode("pod://pod-new.ns:9070"), "ns/pod-new")
	assert.Nil(t, pool.GetByKey("ns/pod-old"))
	assert.NotNil(t, pool.GetByKey("ns/pod-a"))
	assert.NotNil(t, pool.GetByKey("ns/pod-new"))

	// Pods outside the CSIAddonsNode namespace are not listed, so nothing is removed.
	pool = newTestConnPool("ns/pod-old", "ns/pod-new")
	r = &CSIAddonsNodeReconciler{Client: cl, ConnPool: pool}
	r.removeStaleConnections(context.Background(), &logger, newTestCSIAddonsNode("pod://pod-new.other:9070"), "ns/pod-new")
	assert.NotNil(t, pool.GetByKey("ns/pod-old"))
}

func TestReconcileDeletionRemovesAllConnections(t *testing.T) {
	scheme := runtime.NewScheme()
	assert.NoError(t, corev1.AddToScheme(scheme))
	assert.NoError(t, csiaddonsv1alpha1.AddToScheme(scheme))
	node := newTestCSIAddonsNode("pod://pod-new.ns:9070")
	node.Finalizers = []string{csiAddonsNodeFinalizer}
	node.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(node).Build()

	pool := newTestConnPool("ns/pod-old", "ns/pod-new")
	r := &CSIAddonsNodeReconciler{Client: cl, ConnPool: pool}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "node-1", Namespace: "ns"}})
	assert.NoError(t, err)
	assert.Empty(t, pool.GetByCSIAddonsNode("ns", "node-1"))
}
