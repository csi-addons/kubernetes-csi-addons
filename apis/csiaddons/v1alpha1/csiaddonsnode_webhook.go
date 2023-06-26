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
var csnLog = logf.Log.WithName("csiaddonsnode-webhook")

func (c *CSIAddonsNode) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

//+kubebuilder:webhook:path=/validate-csiaddons-openshift-io-v1alpha1-csiaddonsnode,mutating=false,failurePolicy=fail,sideEffects=None,groups=csiaddons.openshift.io,resources=csiaddonsnodes,verbs=update,versions=v1alpha1,name=vcsiaddonsnode.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &CSIAddonsNode{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *CSIAddonsNode) ValidateCreate() (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *CSIAddonsNode) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	csnLog.Info("validate update", "name", c.Name)

	oldCSIAddonsNode, ok := old.(*CSIAddonsNode)
	if !ok {
		return nil, errors.New("error casting CSIAddonsNode object")
	}

	var allErrs field.ErrorList

	if c.Spec.Driver.NodeID != oldCSIAddonsNode.Spec.Driver.NodeID {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "driver", "nodeID"), c.Spec.Driver.NodeID, "nodeID cannot be updated"))
	}

	if c.Spec.Driver.Name != oldCSIAddonsNode.Spec.Driver.Name {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "driver", "name"), c.Spec.Driver.Name, "name cannot be updated"))
	}

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "csiaddons.openshift.io", Kind: "CSIAddonsNode"},
			c.Name, allErrs)
	}

	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *CSIAddonsNode) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}
