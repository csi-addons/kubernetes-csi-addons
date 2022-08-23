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

package controllers

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConditionCompleted = "Completed"
	ConditionDegraded  = "Degraded"
	ConditionResyncing = "Resyncing"
)

const (
	Success         = "Success"
	Promoted        = "Promoted"
	Demoted         = "Demoted"
	FailedToPromote = "FailedToPromote"
	FailedToDemote  = "FailedToDemote"
	Error           = "Error"
	VolumeDegraded  = "VolumeDegraded"
	Healthy         = "Healthy"
	ResyncTriggered = "ResyncTriggered"
	FailedToResync  = "FailedToResync"
	NotResyncing    = "NotResyncing"
)

// sets conditions when volume was promoted successfully.
func setPromotedCondition(conditions *[]metav1.Condition, observedGeneration int64) {
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionCompleted,
		Reason:             Promoted,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionDegraded,
		Reason:             Healthy,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionResyncing,
		Reason:             NotResyncing,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
}

// sets conditions when volume promotion was failed.
func setFailedPromotionCondition(conditions *[]metav1.Condition, observedGeneration int64) {
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionCompleted,
		Reason:             FailedToPromote,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionDegraded,
		Reason:             Error,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionResyncing,
		Reason:             NotResyncing,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
}

// sets conditions when volume is demoted and ready to use (resync completed).
func setNotDegradedCondition(conditions *[]metav1.Condition, observedGeneration int64) {
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionDegraded,
		Reason:             Healthy,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionResyncing,
		Reason:             NotResyncing,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
}

// sets conditions when volume was demoted successfully.
func setDemotedCondition(conditions *[]metav1.Condition, observedGeneration int64) {
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionCompleted,
		Reason:             Demoted,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionDegraded,
		Reason:             VolumeDegraded,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionResyncing,
		Reason:             NotResyncing,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
}

// sets conditions when volume demotion was failed.
func setFailedDemotionCondition(conditions *[]metav1.Condition, observedGeneration int64) {
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionCompleted,
		Reason:             FailedToDemote,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionDegraded,
		Reason:             Error,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionResyncing,
		Reason:             NotResyncing,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
}

// sets conditions when volume resync was triggered successfully.
func setResyncCondition(conditions *[]metav1.Condition, observedGeneration int64) {
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionCompleted,
		Reason:             Demoted,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionDegraded,
		Reason:             VolumeDegraded,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionResyncing,
		Reason:             ResyncTriggered,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
}

// sets conditions when volume resync failed.
func setFailedResyncCondition(conditions *[]metav1.Condition, observedGeneration int64) {
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionCompleted,
		Reason:             FailedToResync,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionFalse,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionDegraded,
		Reason:             Error,
		ObservedGeneration: observedGeneration,
		Status:             metav1.ConditionTrue,
	})
	setStatusCondition(conditions, &metav1.Condition{
		Type:               ConditionResyncing,
		Reason:             FailedToResync,
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
