/*
Copyright 2023 The Kubernetes-CSI-Addons Authors.

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

package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/pkg/messages"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createSuccessNamespacedObjectEvent(logger logr.Logger, client client.Client, object client.Object,
	message, reason string) error {
	event := generateEvent(object, reason, message, normalEventType)
	logger.Info(fmt.Sprintf(messages.CreateEventForNamespacedObject, object.GetNamespace(), object.GetName(),
		object.GetObjectKind().GroupVersionKind().Kind, message))
	return createEvent(logger, client, event)
}

func createNamespacedObjectErrorEvent(logger logr.Logger, client client.Client, object client.Object,
	errorMessage, reason string) error {
	event := generateEvent(object, reason, errorMessage, warningEventType)
	logger.Info(fmt.Sprintf(messages.CreateEventForNamespacedObject, object.GetNamespace(), object.GetName(),
		object.GetObjectKind().GroupVersionKind().Kind, errorMessage))
	return createEvent(logger, client, event)
}

func generateEvent(object client.Object, reason, message, eventType string) *corev1.Event {
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: object.GetNamespace(),
			Name:      fmt.Sprintf("%s.%s", object.GetName(), generateString()),
		},
		ReportingController: vgController,
		InvolvedObject: corev1.ObjectReference{
			Kind:       object.GetObjectKind().GroupVersionKind().Kind,
			APIVersion: object.GetObjectKind().GroupVersionKind().Version,
			Name:       object.GetName(),
			Namespace:  object.GetNamespace(),
			UID:        object.GetUID(),
		},
		Reason:  reason,
		Message: message,
		Type:    eventType,
		FirstTimestamp: metav1.Time{
			Time: time.Now(),
		},
	}
}

func createEvent(logger logr.Logger, client client.Client, event *corev1.Event) error {
	err := client.Create(context.TODO(), event)
	if err != nil {
		logger.Error(err, fmt.Sprintf(messages.FailedToCreateEvent, event.Namespace, event.Name))
		return err
	}
	logger.Info(fmt.Sprintf(messages.EventCreated, event.Namespace, event.Name))
	return nil
}
