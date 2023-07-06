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
var nfLog = logf.Log.WithName("networkfence-webhook")

func (n *NetworkFence) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(n).
		Complete()
}

//+kubebuilder:webhook:path=/validate-csiaddons-openshift-io-v1alpha1-networkfence,mutating=false,failurePolicy=fail,sideEffects=None,groups=csiaddons.openshift.io,resources=networkfences,verbs=update,versions=v1alpha1,name=vnetworkfence.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &NetworkFence{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (n *NetworkFence) ValidateCreate() (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (n *NetworkFence) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	nfLog.Info("validate update", "name", n.Name)

	oldNetworkFence, ok := old.(*NetworkFence)
	if !ok {
		return nil, errors.New("error casting NetworkFence object")
	}

	var allErrs field.ErrorList
	if n.Spec.Driver != oldNetworkFence.Spec.Driver {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("driver"), n.Spec.Driver, "driver cannot be changed"))
	}

	if !reflect.DeepEqual(n.Spec.Parameters, oldNetworkFence.Spec.Parameters) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("parameters"), n.Spec.Parameters, "parameters cannot be changed"))
	}

	if n.Spec.Secret.Name != oldNetworkFence.Spec.Secret.Name {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "secret", "name"), n.Spec.Secret, "secret name cannot be changed"))
	}

	if n.Spec.Secret.Namespace != oldNetworkFence.Spec.Secret.Namespace {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "secret", "namespace"), n.Spec.Secret, "secret namespace cannot be changed"))
	}

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "csiaddons.openshift.io", Kind: "NetworkFence"},
			n.Name, allErrs)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (n *NetworkFence) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}
