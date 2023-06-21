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

package envtest

import (
	volumegroupv1 "github.com/csi-addons/kubernetes-csi-addons/apis/volumegroup.storage/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	VG = &volumegroupv1.VolumeGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      VGName,
			Namespace: Namespace,
		},
		Spec: volumegroupv1.VolumeGroupSpec{
			VolumeGroupClassName: &VGClassName,
			Source: volumegroupv1.VolumeGroupSource{
				Selector: &metav1.LabelSelector{
					MatchLabels: FakeMatchLabels,
				},
			},
		},
	}
	VGClass = &volumegroupv1.VolumeGroupClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: VGClassName,
		},
		Driver:     DriverName,
		Parameters: StorageClassParameters,
	}
	Secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName,
			Namespace: Namespace,
		},
	}
	PVC = &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PVCName,
			Namespace: Namespace,
			Labels:    FakeMatchLabels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &SCName,
			VolumeName:       PVName,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *resource.NewQuantity(100*1024*1024, resource.BinarySI),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}
	StorageClass = &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: SCName,
		},
		Provisioner: DriverName,
	}
	PV = &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PVName,
			Namespace: Namespace,
		},
		Spec: corev1.PersistentVolumeSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: *resource.NewQuantity(100*1024*1024, resource.BinarySI),
			},
			StorageClassName:              SCName,
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       DriverName,
					FSType:       "ext4",
					VolumeHandle: "volumeHandle",
				},
			},
			ClaimRef: &corev1.ObjectReference{
				Name:      PVCName,
				Namespace: Namespace,
			},
		},
	}
)
