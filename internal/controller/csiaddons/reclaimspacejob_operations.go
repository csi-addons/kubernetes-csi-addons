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
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/util"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// reclaimSpaceOperationOrder is the order in which operations are attempted
// and reported. The spec has set semantics, so a fixed order keeps status and
// messages stable. The node-side reclaim runs first, as the controller-side
// reclaim is mostly redundant once it has completed. Operations that are not
// listed here are attempted last.
var reclaimSpaceOperationOrder = []csiaddonsv1alpha1.ReclaimSpaceOperation{
	csiaddonsv1alpha1.ReclaimSpaceOperationNode,
	csiaddonsv1alpha1.ReclaimSpaceOperationController,
}

// operationOutcome is the result of attempting a single reclaim space
// operation. An outcome with a non-terminal state is attempted again on the
// next reconcile. The fields are combined as follows:
//   - Succeeded: err is nil, reclaimed is set when the driver reported usage.
//   - Skipped: err is nil, message explains why the operation is unsupported.
//   - Pending: err is set and decides how the job is requeued.
//   - Failed: err is set, the operation is not attempted again.
type operationOutcome struct {
	state     csiaddonsv1alpha1.ReclaimSpaceOperationState
	reclaimed *int64
	message   string
	err       error
}

// runOperation dispatches op to the matching reclaim space RPC and classifies
// its result.
func (r *ReclaimSpaceJobReconciler) runOperation(
	ctx context.Context,
	logger logr.Logger,
	target *targetDetails,
	op csiaddonsv1alpha1.ReclaimSpaceOperation) operationOutcome {
	var (
		reclaimed *int64
		err       error
	)

	switch op {
	case csiaddonsv1alpha1.ReclaimSpaceOperationNode:
		reclaimed, err = r.nodeReclaimSpace(ctx, logger, target)
	case csiaddonsv1alpha1.ReclaimSpaceOperationController:
		reclaimed, err = r.controllerReclaimSpace(ctx, logger, target)
	default:
		err = fmt.Errorf("unknown reclaim space operation %q", op)

		return operationOutcome{
			state:   csiaddonsv1alpha1.ReclaimSpaceOperationStateFailed,
			message: err.Error(),
			err:     err,
		}
	}

	return classifyOperationOutcome(op, reclaimed, err)
}

// classifyOperationOutcome maps an RPC result to the state recorded in
// status.operations.
func classifyOperationOutcome(
	op csiaddonsv1alpha1.ReclaimSpaceOperation,
	reclaimed *int64,
	err error) operationOutcome {
	switch {
	case err == nil:
		// The requested operation was successful.
		return operationOutcome{
			state:     csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded,
			reclaimed: reclaimed,
		}
	case errors.Is(err, errNodeReclaimUnsupported), errors.Is(err, errControllerReclaimUnsupported):
		// Operation unsupported, no more attempts. The error explains whether
		// a capability is missing or the RPC is not implemented.
		return operationOutcome{
			state:   csiaddonsv1alpha1.ReclaimSpaceOperationStateSkipped,
			message: err.Error(),
		}
	case errors.Is(err, errVolumeNotAttached), errors.Is(err, errNodeClientNotFound):
		// Either the vol is not attached or node client is not found, retry until exhaustion.
		return operationOutcome{
			state:   csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
			message: err.Error(),
			err:     err,
		}
	default:
		// Something else went wrong which we do not handle in any special manner.
		return operationOutcome{
			state:   csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
			message: fmt.Sprintf("failed to make %s request: %v", operationSide(op), util.GetErrorMessage(err)),
			err:     err,
		}
	}
}

// syncOperationStatuses ensures status.operations contains one entry per
// requested operation, ordered by reclaimSpaceOperationOrder. Existing entries
// are preserved.
func syncOperationStatuses(rsJob *csiaddonsv1alpha1.ReclaimSpaceJob) {
	existing := make(map[csiaddonsv1alpha1.ReclaimSpaceOperation]csiaddonsv1alpha1.ReclaimSpaceOperationStatus, len(rsJob.Status.Operations))
	for _, opStatus := range rsJob.Status.Operations {
		existing[opStatus.Operation] = opStatus
	}

	// Iterate over the requested operations rather than the order table, so
	// that an operation missing from the table is never dropped.
	requested := slices.Clone(rsJob.Spec.EffectiveOperations())
	slices.SortStableFunc(requested, func(a, b csiaddonsv1alpha1.ReclaimSpaceOperation) int {
		return operationRank(a) - operationRank(b)
	})
	requested = slices.Compact(requested)

	statuses := make([]csiaddonsv1alpha1.ReclaimSpaceOperationStatus, 0, len(requested))
	for _, op := range requested {
		if opStatus, ok := existing[op]; ok {
			statuses = append(statuses, opStatus)
			continue
		}

		statuses = append(statuses, csiaddonsv1alpha1.ReclaimSpaceOperationStatus{
			Operation: op,
			State:     csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
		})
	}

	rsJob.Status.Operations = statuses
}

// operationRank returns the position of op in reclaimSpaceOperationOrder.
// Unknown operations rank after all known ones.
func operationRank(op csiaddonsv1alpha1.ReclaimSpaceOperation) int {
	if i := slices.Index(reclaimSpaceOperationOrder, op); i >= 0 {
		return i
	}

	return len(reclaimSpaceOperationOrder)
}

// setPendingOperationsMessage sets message on every operation that has not
// reached a terminal state yet.
func setPendingOperationsMessage(statuses []csiaddonsv1alpha1.ReclaimSpaceOperationStatus, message string) {
	for i := range statuses {
		if !statuses[i].State.IsTerminal() {
			statuses[i].Message = message
		}
	}
}

// retryLimitReached returns true when the job used up its retries. Reconcile
// replaces a zero BackoffLimit with the default before calling reconcile, the
// zero check only guards direct callers.
func retryLimitReached(rsJob *csiaddonsv1alpha1.ReclaimSpaceJob) bool {
	return rsJob.Spec.BackoffLimit > 0 && rsJob.Status.Retries >= rsJob.Spec.BackoffLimit
}

// setOperationOutcomeStatus records outcome in the status entry of one operation.
func setOperationOutcomeStatus(
	opStatus *csiaddonsv1alpha1.ReclaimSpaceOperationStatus,
	outcome operationOutcome,
	now time.Time) {
	opStatus.State = outcome.state
	opStatus.Message = outcome.message

	if outcome.reclaimed != nil {
		opStatus.ReclaimedSpace = resource.NewQuantity(*outcome.reclaimed, resource.DecimalSI)
	}

	if outcome.state.IsTerminal() {
		opStatus.CompletionTime = &metav1.Time{Time: now}
	} else {
		opStatus.CompletionTime = nil
	}
}

// allOperationsTerminal returns true when every operation reached a state that
// will not be attempted again. An empty list is never terminal, so a job
// without operations cannot be reported as successful.
func allOperationsTerminal(statuses []csiaddonsv1alpha1.ReclaimSpaceOperationStatus) bool {
	if len(statuses) == 0 {
		return false
	}

	for _, opStatus := range statuses {
		if !opStatus.State.IsTerminal() {
			return false
		}
	}

	return true
}

// operationsInState returns the status entries whose state is one of states.
func operationsInState(
	statuses []csiaddonsv1alpha1.ReclaimSpaceOperationStatus,
	states ...csiaddonsv1alpha1.ReclaimSpaceOperationState,
) []csiaddonsv1alpha1.ReclaimSpaceOperationStatus {
	var result []csiaddonsv1alpha1.ReclaimSpaceOperationStatus
	for _, opStatus := range statuses {
		if slices.Contains(states, opStatus.State) {
			result = append(result, opStatus)
		}
	}

	return result
}

// operationSummary prepares status entry for the job message.
// Like: "x-side reclaim yyy: status.Message", where yyy is current state.
func operationSummary(opStatus csiaddonsv1alpha1.ReclaimSpaceOperationStatus) string {
	summary := fmt.Sprintf("%s-side reclaim %s",
		operationSide(opStatus.Operation),
		strings.ToLower(string(opStatus.State)))
	if opStatus.Message == "" {
		return summary
	}

	return fmt.Sprintf("%s: %s", summary, opStatus.Message)
}

// summarizeOperations joins multiple operation statuses into a summary.
func summarizeOperations(statuses []csiaddonsv1alpha1.ReclaimSpaceOperationStatus) string {
	summaries := make([]string, 0, len(statuses))
	for _, opStatus := range statuses {
		summaries = append(summaries, operationSummary(opStatus))
	}

	return strings.Join(summaries, "; ")
}

// incompleteMessage describes the operations that are still pending, plus the
// skipped ones so that the reason for skipping them stays visible in status.
func incompleteMessage(statuses []csiaddonsv1alpha1.ReclaimSpaceOperationStatus) string {
	incomplete := operationsInState(statuses,
		csiaddonsv1alpha1.ReclaimSpaceOperationStatePending,
		csiaddonsv1alpha1.ReclaimSpaceOperationStateSkipped)
	if len(incomplete) == 0 {
		return "reclaim space operations are not complete"
	}

	return fmt.Sprintf("reclaim space operations not complete: %s", summarizeOperations(incomplete))
}

// retryError returns the error that decides how the job is requeued. When any
// operation waits for the node to become available, all errors are joined so
// that the caller requeues with the fixed node interval instead of the fast
// rate-limited backoff, which would otherwise use up the retries before the
// node is ready. Otherwise the first failure is returned as-is. gRPC status
// codes (for example Aborted) remain detectable in both cases.
func retryError(errs []error) error {
	if len(errs) == 0 {
		return errors.New("reclaim space operations are still pending")
	}

	for _, err := range errs {
		if errors.Is(err, errVolumeNotAttached) || errors.Is(err, errNodeClientNotFound) {
			return errors.Join(errs...)
		}
	}

	return errs[0]
}

// totalReclaimedSpace returns the sum of the space reclaimed by the operations
// that reported it, or nil when no operation did.
func totalReclaimedSpace(statuses []csiaddonsv1alpha1.ReclaimSpaceOperationStatus) *resource.Quantity {
	var (
		reclaimedSpace int64
		reclaimed      bool
	)
	for _, opStatus := range statuses {
		if opStatus.ReclaimedSpace == nil {
			continue
		}
		reclaimedSpace += opStatus.ReclaimedSpace.Value()
		reclaimed = true
	}

	if !reclaimed {
		return nil
	}

	return resource.NewQuantity(reclaimedSpace, resource.DecimalSI)
}

// completeJob records the final result once every requested operation is
// terminal. The Failed condition is set whenever the result is Failed.
func (r *ReclaimSpaceJobReconciler) completeJob(rsJob *csiaddonsv1alpha1.ReclaimSpaceJob) {

	if reclaimedSpace := totalReclaimedSpace(rsJob.Status.Operations); reclaimedSpace != nil {
		rsJob.Status.ReclaimedSpace = reclaimedSpace
	}

	succeeded := operationsInState(rsJob.Status.Operations, csiaddonsv1alpha1.ReclaimSpaceOperationStateSucceeded)
	failed := operationsInState(rsJob.Status.Operations, csiaddonsv1alpha1.ReclaimSpaceOperationStateFailed)
	skipped := operationsInState(rsJob.Status.Operations, csiaddonsv1alpha1.ReclaimSpaceOperationStateSkipped)

	switch {
	// One of the ops failed, other ops might be successful but treat the job as failed.
	case len(failed) > 0:
		rsJob.Status.Result = csiaddonsv1alpha1.OperationResultFailed
		rsJob.Status.Message = fmt.Sprintf("Reclaim Space operation failed: %s", summarizeOperations(failed))
	// Everything was skipped, nothing succeeded.
	case len(succeeded) == 0 && len(skipped) > 0:
		rsJob.Status.Result = csiaddonsv1alpha1.OperationResultFailed
		rsJob.Status.Message = fmt.Sprintf("no reclaim space operation performed for PVC %q: %s",
			rsJob.Spec.Target.PersistentVolumeClaim,
			summarizeOperations(skipped))
	// At least one operation succeeded and none failed. Skipped ones are
	// appended to the message.
	default:
		rsJob.Status.Result = csiaddonsv1alpha1.OperationResultSucceeded
		rsJob.Status.Message = "Reclaim Space operation successfully completed."
		if len(skipped) > 0 {
			rsJob.Status.Message = fmt.Sprintf("%s %s.", rsJob.Status.Message, summarizeOperations(skipped))
		}
	}

	if rsJob.Status.Result == csiaddonsv1alpha1.OperationResultFailed {
		setFailedCondition(&rsJob.Status.Conditions, rsJob.Status.Message, rsJob.Generation)
	} else {
		clearFailedCondition(&rsJob.Status.Conditions)
	}

	rsJob.Status.CompletionTime = &metav1.Time{Time: time.Now()}
}

// failJob marks every operation that did not reach a terminal state as Failed
// and records the job failure.
func (r *ReclaimSpaceJobReconciler) failJob(rsJob *csiaddonsv1alpha1.ReclaimSpaceJob, reason string) {
	now := metav1.Time{Time: time.Now()}
	for i := range rsJob.Status.Operations {
		opStatus := &rsJob.Status.Operations[i]
		if opStatus.State.IsTerminal() {
			continue
		}

		opStatus.State = csiaddonsv1alpha1.ReclaimSpaceOperationStateFailed
		opStatus.CompletionTime = &now
		if opStatus.Message == "" {
			opStatus.Message = reason
		}
	}

	if reclaimedSpace := totalReclaimedSpace(rsJob.Status.Operations); reclaimedSpace != nil {
		rsJob.Status.ReclaimedSpace = reclaimedSpace
	}

	rsJob.Status.Result = csiaddonsv1alpha1.OperationResultFailed
	rsJob.Status.CompletionTime = &now
	if len(rsJob.Status.Operations) == 0 {
		rsJob.Status.Message = reason
	} else {
		rsJob.Status.Message = fmt.Sprintf("%s: %s", reason, summarizeOperations(rsJob.Status.Operations))
	}
	setFailedCondition(&rsJob.Status.Conditions, rsJob.Status.Message, rsJob.Generation)
}

// operationSide returns the side on which op runs, i.e. node or controller.
func operationSide(op csiaddonsv1alpha1.ReclaimSpaceOperation) string {
	switch op {
	case csiaddonsv1alpha1.ReclaimSpaceOperationNode:
		return "node"
	case csiaddonsv1alpha1.ReclaimSpaceOperationController:
		return "controller"
	default:
		return string(op)
	}
}
