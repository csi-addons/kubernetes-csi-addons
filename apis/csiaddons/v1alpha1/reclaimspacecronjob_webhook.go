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
var rscjLog = logf.Log.WithName("reclaimspacecronjob-webhook")

func (r *ReclaimSpaceCronJob) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-csiaddons-openshift-io-v1alpha1-reclaimspacecronjob,mutating=false,failurePolicy=fail,sideEffects=None,groups=csiaddons.openshift.io,resources=reclaimspacecronjobs,verbs=update,versions=v1alpha1,name=vreclaimspacecronjob.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ReclaimSpaceCronJob{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ReclaimSpaceCronJob) ValidateCreate() (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ReclaimSpaceCronJob) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	rscjLog.Info("validate update", "name", r.Name)

	oldReclaimSpaceCronJob, ok := old.(*ReclaimSpaceCronJob)
	if !ok {
		return nil, errors.New("error casting ReclaimSpaceCronJob object")
	}

	var allErrs field.ErrorList

	if r.Spec.JobSpec.Spec.Target.PersistentVolumeClaim != oldReclaimSpaceCronJob.Spec.JobSpec.Spec.Target.PersistentVolumeClaim {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "jobTemplate", "spec", "target", "persistentVolumeClaim"), r.Spec.JobSpec.Spec.Target.PersistentVolumeClaim, "persistentVolumeClaim cannot be changed"))
	}

	if len(allErrs) != 0 {
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: "csiaddons.openshift.io", Kind: "ReclaimSpaceCronJob"},
			r.Name, allErrs)
	}
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ReclaimSpaceCronJob) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}
