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
)

// log is for logging in this package.
var vrcLog = logf.Log.WithName("volumereplicationclass-webhook")

func (v *VolumeReplicationClass) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(v).
		Complete()
}

//+kubebuilder:webhook:path=/validate-replication-storage-openshift-io-v1alpha1-volumereplicationclass,mutating=false,failurePolicy=fail,sideEffects=None,groups=replication.storage.openshift.io,resources=volumereplicationclasses,verbs=update,versions=v1alpha1,name=vvolumereplicationclass.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &VolumeReplicationClass{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v *VolumeReplicationClass) ValidateCreate() error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v *VolumeReplicationClass) ValidateUpdate(old runtime.Object) error {
	vrcLog.Info("validate update", "name", v.Name)

	oldReplicationClass, ok := old.(*VolumeReplicationClass)
	if !ok {
		return errors.New("error casting old VolumeReplicationClass object")
	}

	var allErrs field.ErrorList
	if oldReplicationClass.Spec.Provisioner != v.Spec.Provisioner {
		vrcLog.Info("invalid request to change the provisioner", "exiting provisioner", oldReplicationClass.Spec.Provisioner, "new provisioner", v.Spec.Provisioner)
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("provisioner"), v.Spec.Provisioner, "provisioner cannot be changed"))
	}

	if !reflect.DeepEqual(oldReplicationClass.Spec.Parameters, v.Spec.Parameters) {
		vrcLog.Info("invalid request to change the parameters", "exiting parameters", oldReplicationClass.Spec.Parameters, "new parameters", v.Spec.Parameters)
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("parameters"), v.Spec.Parameters, "parameters cannot be changed"))
	}

	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "replication.storage.openshift.io", Kind: "VolumeReplicationClass"},
		v.Name, allErrs)

}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v *VolumeReplicationClass) ValidateDelete() error {
	return nil
}
