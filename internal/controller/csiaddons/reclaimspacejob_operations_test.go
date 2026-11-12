/*
Copyright 2026 The Kubernetes-CSI-Addons Authors.

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
	"fmt"
	"testing"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
)

func operationNames(statuses []csiaddonsv1alpha1.ReclaimSpaceOperationStatus) []csiaddonsv1alpha1.ReclaimSpaceOperation {
	names := make([]csiaddonsv1alpha1.ReclaimSpaceOperation, 0, len(statuses))
	for _, opStatus := range statuses {
		names = append(names, opStatus.Operation)
	}

	return names
}

func TestSyncOperationStatuses(t *testing.T) {
	const unknown = csiaddonsv1alpha1.ReclaimSpaceOperation("Unknown")

	tests := []struct {
		name       string
		operations []csiaddonsv1alpha1.ReclaimSpaceOperation
		want       []csiaddonsv1alpha1.ReclaimSpaceOperation
	}{
		{
			name:       "empty operations use the default",
			operations: nil,
			want:       csiaddonsv1alpha1.DefaultReclaimSpaceOperations(),
		},
		{
			name: "node is ordered before controller",
			operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
			},
			want: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
		},
		{
			name: "operations missing from the order table are kept and ordered last",
			operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				unknown,
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
			},
			want: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
				unknown,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &csiaddonsv1alpha1.ReclaimSpaceJob{
				Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{Operations: tt.operations},
			}

			syncOperationStatuses(job)

			assert.Equal(t, tt.want, operationNames(job.Status.Operations))
			for _, opStatus := range job.Status.Operations {
				assert.Equal(t, csiaddonsv1alpha1.ReclaimSpaceOperationStatePending, opStatus.State)
			}
			// The spec must not be reordered through the status sync.
			assert.Equal(t, tt.operations, job.Spec.Operations)
		})
	}
}

func TestSyncOperationStatusesPreservesExistingEntries(t *testing.T) {
	job := &csiaddonsv1alpha1.ReclaimSpaceJob{
		Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
			Operations: []csiaddonsv1alpha1.ReclaimSpaceOperation{
				csiaddonsv1alpha1.ReclaimSpaceOperationController,
				csiaddonsv1alpha1.ReclaimSpaceOperationNode,
			},
		},
		Status: csiaddonsv1alpha1.ReclaimSpaceJobStatus{
			Operations: []csiaddonsv1alpha1.ReclaimSpaceOperationStatus{{
				Operation: csiaddonsv1alpha1.ReclaimSpaceOperationController,
				State:     csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
				Message:   "done",
			}},
		},
	}

	syncOperationStatuses(job)

	assert.Equal(t, []csiaddonsv1alpha1.ReclaimSpaceOperationStatus{
		{
			Operation: csiaddonsv1alpha1.ReclaimSpaceOperationNode,
			State:     csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
		},
		{
			Operation: csiaddonsv1alpha1.ReclaimSpaceOperationController,
			State:     csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
			Message:   "done",
		},
	}, job.Status.Operations)
}

func TestAllOperationsTerminal(t *testing.T) {
	assert.False(t, allOperationsTerminal(nil), "an empty list must not count as complete")
	assert.False(t, allOperationsTerminal([]csiaddonsv1alpha1.ReclaimSpaceOperationStatus{
		{State: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded},
		{State: csiaddonsv1alpha1.ReclaimSpaceOperationStatePending},
	}))
	assert.True(t, allOperationsTerminal([]csiaddonsv1alpha1.ReclaimSpaceOperationStatus{
		{State: csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded},
		{State: csiaddonsv1alpha1.ReclaimSpaceOperationStateSkipped},
		{State: csiaddonsv1alpha1.ReclaimSpaceOperationStateFailed},
	}))
}

func TestRetryError(t *testing.T) {
	aborted := status.Error(codes.Aborted, "operation in progress")
	unavailable := status.Error(codes.Unavailable, "unavailable")
	notAttached := fmt.Errorf("%w: PVC %q", errVolumeNotAttached, "data-pvc")
	notFound := fmt.Errorf("%w for %q nodeID", errNodeClientNotFound, "node-1")

	t.Run("no errors", func(t *testing.T) {
		assert.Error(t, retryError(nil))
	})

	t.Run("a single failure is returned as-is", func(t *testing.T) {
		assert.Same(t, unavailable, retryError([]error{unavailable}))
	})

	t.Run("the first failure is returned when the node is available", func(t *testing.T) {
		err := retryError([]error{aborted, unavailable})
		assert.Same(t, aborted, err)
		assert.True(t, util.IsAbortedError(err))
	})

	t.Run("node unavailability is detectable alongside other failures", func(t *testing.T) {
		err := retryError([]error{notFound, aborted})
		assert.ErrorIs(t, err, errNodeClientNotFound)
		assert.ErrorIs(t, err, aborted)
		assert.True(t, util.IsAbortedError(err), "gRPC codes must stay detectable")
	})

	t.Run("volume not attached is detectable alongside other failures", func(t *testing.T) {
		err := retryError([]error{unavailable, notAttached})
		assert.ErrorIs(t, err, errVolumeNotAttached)
		assert.ErrorIs(t, err, unavailable)
	})
}

func TestCompleteJobFailedCondition(t *testing.T) {
	r := &ReclaimSpaceJobReconciler{}

	tests := []struct {
		name          string
		statuses      []csiaddonsv1alpha1.ReclaimSpaceOperationStatus
		wantResult    csiaddonsv1alpha1.OperationResult
		wantCondition bool
	}{
		{
			name: "all skipped sets the failed condition",
			statuses: []csiaddonsv1alpha1.ReclaimSpaceOperationStatus{{
				Operation: csiaddonsv1alpha1.ReclaimSpaceOperationNode,
				State:     csiaddonsv1alpha1.ReclaimSpaceOperationStateSkipped,
				Message:   "unsupported",
			}},
			wantResult:    csiaddonsv1alpha1.OperationResultFailed,
			wantCondition: true,
		},
		{
			name: "a failed operation sets the failed condition",
			statuses: []csiaddonsv1alpha1.ReclaimSpaceOperationStatus{
				{
					Operation: csiaddonsv1alpha1.ReclaimSpaceOperationNode,
					State:     csiaddonsv1alpha1.ReclaimSpaceOperationStateFailed,
				},
				{
					Operation: csiaddonsv1alpha1.ReclaimSpaceOperationController,
					State:     csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
				},
			},
			wantResult:    csiaddonsv1alpha1.OperationResultFailed,
			wantCondition: true,
		},
		{
			name: "success clears a stale failed condition",
			statuses: []csiaddonsv1alpha1.ReclaimSpaceOperationStatus{
				{
					Operation: csiaddonsv1alpha1.ReclaimSpaceOperationNode,
					State:     csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
				},
				{
					Operation: csiaddonsv1alpha1.ReclaimSpaceOperationController,
					State:     csiaddonsv1alpha1.ReclaimSpaceOperationStateSkipped,
				},
			},
			wantResult:    csiaddonsv1alpha1.OperationResultSucceeded,
			wantCondition: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &csiaddonsv1alpha1.ReclaimSpaceJob{
				Status: csiaddonsv1alpha1.ReclaimSpaceJobStatus{Operations: tt.statuses},
			}
			// A transient condition left over from a previous reconcile.
			setFailedCondition(&job.Status.Conditions, "waiting for the node", 1)

			r.completeJob(job)

			assert.Equal(t, tt.wantResult, job.Status.Result)
			assert.NotNil(t, job.Status.CompletionTime)
			condition := meta.FindStatusCondition(job.Status.Conditions, conditionFailed)
			if !tt.wantCondition {
				assert.Nil(t, condition)
				return
			}
			if assert.NotNil(t, condition) {
				assert.Equal(t, job.Status.Message, condition.Message)
			}
		})
	}
}

func TestCompleteJobSumsReclaimedSpace(t *testing.T) {
	r := &ReclaimSpaceJobReconciler{}
	job := &csiaddonsv1alpha1.ReclaimSpaceJob{
		Status: csiaddonsv1alpha1.ReclaimSpaceJobStatus{
			Operations: []csiaddonsv1alpha1.ReclaimSpaceOperationStatus{
				{
					Operation:      csiaddonsv1alpha1.ReclaimSpaceOperationNode,
					State:          csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
					ReclaimedSpace: resource.NewQuantity(60, resource.DecimalSI),
				},
				{
					Operation:      csiaddonsv1alpha1.ReclaimSpaceOperationController,
					State:          csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
					ReclaimedSpace: resource.NewQuantity(40, resource.DecimalSI),
				},
			},
		},
	}

	r.completeJob(job)

	assert.Equal(t, csiaddonsv1alpha1.OperationResultSucceeded, job.Status.Result)
	if assert.NotNil(t, job.Status.ReclaimedSpace) {
		assert.Equal(t, int64(100), job.Status.ReclaimedSpace.Value())
	}
}

func TestFailJobExplainsUntouchedOperations(t *testing.T) {
	r := &ReclaimSpaceJobReconciler{}
	job := &csiaddonsv1alpha1.ReclaimSpaceJob{
		Status: csiaddonsv1alpha1.ReclaimSpaceJobStatus{
			Operations: []csiaddonsv1alpha1.ReclaimSpaceOperationStatus{
				{
					Operation: csiaddonsv1alpha1.ReclaimSpaceOperationNode,
					State:     csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
				},
				{
					Operation: csiaddonsv1alpha1.ReclaimSpaceOperationController,
					State:     csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
					Message:   "controller unavailable",
				},
			},
		},
	}

	r.failJob(job, "Time limit reached")

	assert.Equal(t, csiaddonsv1alpha1.OperationResultFailed, job.Status.Result)
	assert.Equal(t, "Time limit reached", job.Status.Operations[0].Message,
		"an operation that was never attempted gets the failure reason")
	assert.Equal(t, "controller unavailable", job.Status.Operations[1].Message,
		"the last attempt's message is kept")
	for _, opStatus := range job.Status.Operations {
		assert.Equal(t, csiaddonsv1alpha1.ReclaimSpaceOperationStateFailed, opStatus.State)
		assert.NotNil(t, opStatus.CompletionTime)
	}
	assert.NotNil(t, meta.FindStatusCondition(job.Status.Conditions, conditionFailed))
}

func TestClassifyOperationOutcomeSkippedKeepsCause(t *testing.T) {
	cause := fmt.Errorf("%w: %v", errNodeReclaimUnsupported, status.Error(codes.Unimplemented, "fstrim not available"))

	outcome := classifyOperationOutcome(csiaddonsv1alpha1.ReclaimSpaceOperationNode, nil, cause)

	assert.Equal(t, csiaddonsv1alpha1.ReclaimSpaceOperationStateSkipped, outcome.state)
	assert.NoError(t, outcome.err)
	assert.Contains(t, outcome.message, "NodeReclaimSpace is not supported")
	assert.Contains(t, outcome.message, "fstrim not available")
}
