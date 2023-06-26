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

package v1alpha1

import (
	"errors"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var vrLog = logf.Log.WithName("volumereplication-webhook")

func (v *VolumeReplication) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(v).
		Complete()
}

//+kubebuilder:webhook:path=/validate-replication-storage-openshift-io-v1alpha1-volumereplication,mutating=false,failurePolicy=fail,sideEffects=None,groups=replication.storage.openshift.io,resources=volumereplications,verbs=update,versions=v1alpha1,name=vvolumereplication.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &VolumeReplication{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *VolumeReplication) ValidateCreate() (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *VolumeReplication) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	vrLog.Info("validate update", "name", v.Name)

	oldReplication, ok := old.(*VolumeReplication)
	if !ok {
		return nil, errors.New("error casting old VolumeReplication object")
	}

	var allErrs field.ErrorList

	if !reflect.DeepEqual(oldReplication.Spec.DataSource, v.Spec.DataSource) {
		vrLog.Info("invalid request to change the DataSource", "exiting dataSource", oldReplication.Spec.DataSource, "new dataSource", v.Spec.DataSource)
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("dataSource"), v.Spec.DataSource, "dataSource cannot be changed"))
	}

	if oldReplication.Spec.VolumeReplicationClass != v.Spec.VolumeReplicationClass {
		vrLog.Info("invalid request to change the volumeReplicationClass", "exiting volumeReplicationClass", oldReplication.Spec.VolumeReplicationClass, "new volumeReplicationClass", v.Spec.VolumeReplicationClass)
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("volumeReplicationClass"), v.Spec.VolumeReplicationClass, "volumeReplicationClass cannot be changed"))
	}

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "replication.storage.openshift.io", Kind: "VolumeReplication"},
			v.Name, allErrs)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *VolumeReplication) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}
