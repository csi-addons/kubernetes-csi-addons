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
	"fmt"
	"time"

	"github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getSource(dataSource string) v1alpha1.Source {
	switch dataSource {
	case pvcDataSource:
		return v1alpha1.Volume
	case volumeGroupReplicationDataSource:
		return v1alpha1.VolumeGroup
	default:
		return ""
	}
}

// sets conditions when volume was promoted successfully.
func setPromotedCondition(conditions *[]metav1.Condition, observedGeneration int64, dataSource string) {
	source := getSource(dataSource)
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessagePromoted),
		Type:               v1alpha1.ConditionCompleted,
		Reason:             v1alpha1.Promoted,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageHealthy),
		Type:               v1alpha1.ConditionDegraded,
		Reason:             v1alpha1.Healthy,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageNotResyncing),
		Type:               v1alpha1.ConditionResyncing,
		Reason:             v1alpha1.NotResyncing,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
}

// sets conditions when volume promotion was failed.
func setFailedPromotionCondition(conditions *[]metav1.Condition, observedGeneration int64, dataSource, completedMessage, degradedDetailedMessage string) {
	source := getSource(dataSource)
	setStatusCondition(conditions, &metav1.Condition{
		Message:            completedMessage,
		Type:               v1alpha1.ConditionCompleted,
		Reason:             v1alpha1.FailedToPromote,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s: %s", source, v1alpha1.MessageFailedPromoted, degradedDetailedMessage),
		Type:               v1alpha1.ConditionDegraded,
		Reason:             v1alpha1.Error,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageNotResyncing),
		Type:               v1alpha1.ConditionResyncing,
		Reason:             v1alpha1.NotResyncing,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageValidated),
		Type:               v1alpha1.ConditionValidated,
		Reason:             v1alpha1.PrerequisiteMet,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
}

// sets conditions when volume promotion was failed due to failed validation.
func setFailedValidationCondition(conditions *[]metav1.Condition, observedGeneration int64, dataSource, degradedMessage, validationDetailedMessage string) {
	source := getSource(dataSource)
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageFailedPromoted),
		Type:               v1alpha1.ConditionCompleted,
		Reason:             v1alpha1.FailedToPromote,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            degradedMessage,
		Type:               v1alpha1.ConditionDegraded,
		Reason:             v1alpha1.Error,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageNotResyncing),
		Type:               v1alpha1.ConditionResyncing,
		Reason:             v1alpha1.NotResyncing,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s: %s", v1alpha1.MessageFailedPreCondition, validationDetailedMessage),
		Type:               v1alpha1.ConditionValidated,
		Reason:             v1alpha1.PrerequisiteNotMet,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
}

// sets conditions when volume is demoted and ready to use (resync completed).
func setNotDegradedCondition(conditions *[]metav1.Condition, observedGeneration int64, dataSource string) {
	source := getSource(dataSource)
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageDemoted),
		Type:               v1alpha1.ConditionDegraded,
		Reason:             v1alpha1.Healthy,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageNotResyncing),
		Type:               v1alpha1.ConditionResyncing,
		Reason:             v1alpha1.NotResyncing,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
}

// sets conditions when volume was demoted successfully.
func setDemotedCondition(conditions *[]metav1.Condition, observedGeneration int64, dataSource string) {
	source := getSource(dataSource)
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageDemoted),
		Type:               v1alpha1.ConditionCompleted,
		Reason:             v1alpha1.Demoted,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageDegraded),
		Type:               v1alpha1.ConditionDegraded,
		Reason:             v1alpha1.VolumeDegraded,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageNotResyncing),
		Type:               v1alpha1.ConditionResyncing,
		Reason:             v1alpha1.NotResyncing,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
}

// sets conditions when volume demotion was failed.
func setFailedDemotionCondition(conditions *[]metav1.Condition, observedGeneration int64, dataSource, completedMessage, degradedDetailedMessage string) {
	source := getSource(dataSource)
	setStatusCondition(conditions, &metav1.Condition{
		Message:            completedMessage,
		Type:               v1alpha1.ConditionCompleted,
		Reason:             v1alpha1.FailedToDemote,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s: %s", source, v1alpha1.MessageFailedDemoted, degradedDetailedMessage),
		Type:               v1alpha1.ConditionDegraded,
		Reason:             v1alpha1.Error,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageNotResyncing),
		Type:               v1alpha1.ConditionResyncing,
		Reason:             v1alpha1.NotResyncing,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
}

// sets conditions when volume resync was triggered successfully.
func setResyncCondition(conditions *[]metav1.Condition, observedGeneration int64, dataSource string) {
	source := getSource(dataSource)
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageDemoted),
		Type:               v1alpha1.ConditionCompleted,
		Reason:             v1alpha1.Demoted,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageDegraded),
		Type:               v1alpha1.ConditionDegraded,
		Reason:             v1alpha1.VolumeDegraded,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageResyncTriggered),
		Type:               v1alpha1.ConditionResyncing,
		Reason:             v1alpha1.ResyncTriggered,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
}

// sets conditions when volume resync failed.
func setFailedResyncCondition(conditions *[]metav1.Condition, observedGeneration int64, dataSource, completedMessage, degradedDetailedMessage string) {
	source := getSource(dataSource)
	setStatusCondition(conditions, &metav1.Condition{
		Message:            completedMessage,
		Type:               v1alpha1.ConditionCompleted,
		Reason:             v1alpha1.FailedToResync,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s: %s", source, v1alpha1.MessageResyncFailed, degradedDetailedMessage),
		Type:               v1alpha1.ConditionDegraded,
		Reason:             v1alpha1.Error,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Message:            fmt.Sprintf("%s %s", source, v1alpha1.MessageNotResyncing),
		Type:               v1alpha1.ConditionResyncing,
		Reason:             v1alpha1.FailedToResync,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
}

func setStatusCondition(existingConditions *[]metav1.Condition, newCondition *metav1.Condition) {
	if existingConditions == nil {
		existingConditions = &[]metav1.Condition{}
	}

	existingCondition := findCondition(*existingConditions, newCondition.Type)
	if existingCondition == nil {
		newCondition.LastTransitionTime = metav1.NewTime(time.Now())
		*existingConditions = append(*existingConditions, *newCondition)

		return
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		existingCondition.LastTransitionTime = metav1.NewTime(time.Now())
	}

	existingCondition.Message = newCondition.Message
	existingCondition.Reason = newCondition.Reason
	existingCondition.ObservedGeneration = newCondition.ObservedGeneration
}

func findCondition(existingConditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range existingConditions {
		if existingConditions[i].Type == conditionType {
			return &existingConditions[i]
		}
	}

	return nil
}
