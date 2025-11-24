/*
Copyright 2025 The Kubernetes-CSI-Addons Authors.

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

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SilentPredicate returns a predicate that rejects every event.
func SilentPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}
}

func StorageClassPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldSC, oldOk := e.ObjectOld.(*storagev1.StorageClass)
			newSC, newOk := e.ObjectNew.(*storagev1.StorageClass)
			if !oldOk || !newOk {
				return false
			}

			// Only trigger if relevant annotations change
			relevantAnnotations := []string{
				KrcJobScheduleTimeAnnotation,
				KrEnableAnnotation,
				RsCronJobScheduleTimeAnnotation,
			}

			return AnnotationValueChanged(oldSC.GetAnnotations(), newSC.GetAnnotations(), relevantAnnotations)
		},
		CreateFunc: func(e event.CreateEvent) bool { return true },
	}
}

func ScMapFunc(r client.Client) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		ok := false
		sc, ok := obj.(*storagev1.StorageClass)
		if !ok {
			return nil
		}

		// List all PVCs using this SC
		pvcList := &corev1.PersistentVolumeClaimList{}
		if err := r.List(ctx, pvcList, client.MatchingFields{StorageClassIndex: sc.Name}); err != nil {
			return nil
		}

		requests := make([]reconcile.Request, len(pvcList.Items))
		for i, pvc := range pvcList.Items {
			requests[i] = reconcile.Request{NamespacedName: types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}}
		}
		return requests
	}
}

// SetupPVCControllerIndexers adds field indexers to the manager to optimize lookups.
//
// The following indexers are set:
//   - PersistentVolumeClaims - indexed by spec.storageClassName
func SetupPVCControllerIndexers(mgr ctrl.Manager) error {
	indices := []struct {
		obj     client.Object
		field   string
		indexFn client.IndexerFunc
	}{
		{
			// Map PVC to SC
			obj:   &corev1.PersistentVolumeClaim{},
			field: StorageClassIndex,
			indexFn: func(rawObj client.Object) []string {
				pvc, ok := rawObj.(*corev1.PersistentVolumeClaim)
				if !ok || (pvc.Spec.StorageClassName == nil) || len(*pvc.Spec.StorageClassName) == 0 {
					return nil
				}
				return []string{*pvc.Spec.StorageClassName}
			},
		},
	}

	for _, index := range indices {
		if err := mgr.GetFieldIndexer().IndexField(
			context.Background(),
			index.obj,
			index.field,
			index.indexFn,
		); err != nil {
			return err
		}
	}

	return nil

}
