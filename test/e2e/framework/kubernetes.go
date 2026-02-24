/*
Copyright 2024 The Kubernetes-CSI-Addons Authors.

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

package framework

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreatePVC creates a PersistentVolumeClaim and waits for it to be bound
func (f *Framework) CreatePVC(name string, size string, storageClass string) *corev1.PersistentVolumeClaim {
	By(fmt.Sprintf("Creating PVC %s with size %s", name, size))

	if storageClass == "" {
		storageClass = f.GetStorageClassName()
	}

	if size == "" {
		size = f.GetVolumeSize()
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{f.GetAccessMode()},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
			StorageClassName: &storageClass,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("pvc-create"))
	defer cancel()

	err := f.Client.Create(ctx, pvc)
	Expect(err).NotTo(HaveOccurred(), "Failed to create PVC")

	f.TrackResource(pvc)

	return pvc
}

// CreateBlockPVC creates a PersistentVolumeClaim with Block volume mode
func (f *Framework) CreateBlockPVC(name string, size string, storageClass string) *corev1.PersistentVolumeClaim {
	By(fmt.Sprintf("Creating Block mode PVC %s with size %s", name, size))

	if storageClass == "" {
		storageClass = f.GetStorageClassName()
	}

	if size == "" {
		size = f.GetVolumeSize()
	}

	volumeMode := corev1.PersistentVolumeBlock
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{f.GetAccessMode()},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
			StorageClassName: &storageClass,
			VolumeMode:       &volumeMode,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("pvc-create"))
	defer cancel()

	err := f.Client.Create(ctx, pvc)
	Expect(err).NotTo(HaveOccurred(), "Failed to create Block mode PVC")

	f.TrackResource(pvc)

	return pvc
}

// WaitForPVCBound waits for a PVC to be bound
func (f *Framework) WaitForPVCBound(pvcName string) *corev1.PersistentVolumeClaim {
	By(fmt.Sprintf("Waiting for PVC %s to be bound", pvcName))

	pvc := &corev1.PersistentVolumeClaim{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("pvc-bound"))
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, f.GetTimeout("pvc-bound"), true, func(ctx context.Context) (bool, error) {
		err := f.Client.Get(ctx, client.ObjectKey{
			Name:      pvcName,
			Namespace: f.GetNamespaceName(),
		}, pvc)
		if err != nil {
			return false, err
		}
		return pvc.Status.Phase == corev1.ClaimBound, nil
	})

	Expect(err).NotTo(HaveOccurred(), "PVC did not become bound in time")
	return pvc
}

// GetPVC gets a PersistentVolumeClaim
func (f *Framework) GetPVC(name string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: f.GetNamespaceName(),
	}, pvc)
	Expect(err).NotTo(HaveOccurred(), "Failed to get PVC")
	return pvc
}

// GetPV gets a PersistentVolume by name
func (f *Framework) GetPV(name string) *corev1.PersistentVolume {
	pv := &corev1.PersistentVolume{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Get(ctx, client.ObjectKey{Name: name}, pv)
	Expect(err).NotTo(HaveOccurred(), "Failed to get PV")
	return pv
}

// DeletePVC deletes a PersistentVolumeClaim
func (f *Framework) DeletePVC(name string) {
	By(fmt.Sprintf("Deleting PVC %s", name))

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Delete(ctx, pvc)
	if err != nil && !apierrors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred(), "Failed to delete PVC")
	}
}

// CreatePod creates a pod that uses a PVC
func (f *Framework) CreatePod(name string, pvcName string, command []string) *corev1.Pod {
	By(fmt.Sprintf("Creating pod %s with PVC %s", name, pvcName))

	if command == nil {
		command = []string{"sleep", "3600"}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "test-container",
					Image:   "busybox:latest",
					Command: command,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "test-volume",
							MountPath: "/mnt/test",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "test-volume",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("pod-start"))
	defer cancel()

	err := f.Client.Create(ctx, pod)
	Expect(err).NotTo(HaveOccurred(), "Failed to create pod")

	f.TrackResource(pod)

	return pod
}

// CreatePodWithBlockDevice creates a pod that uses a PVC as a block device
func (f *Framework) CreatePodWithBlockDevice(name string, pvcName string, command []string) *corev1.Pod {
	By(fmt.Sprintf("Creating pod %s with block device PVC %s", name, pvcName))

	if command == nil {
		command = []string{"sleep", "3600"}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "test-container",
					Image:   "busybox:latest",
					Command: command,
					VolumeDevices: []corev1.VolumeDevice{
						{
							Name:       "test-volume",
							DevicePath: "/dev/xvda",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "test-volume",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("pod-start"))
	defer cancel()

	err := f.Client.Create(ctx, pod)
	Expect(err).NotTo(HaveOccurred(), "Failed to create pod with block device")

	f.TrackResource(pod)

	return pod
}

// WaitForPodRunning waits for a pod to be running
func (f *Framework) WaitForPodRunning(podName string) *corev1.Pod {
	By(fmt.Sprintf("Waiting for pod %s to be running", podName))

	pod := &corev1.Pod{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("pod-start"))
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, f.GetTimeout("pod-start"), true, func(ctx context.Context) (bool, error) {
		err := f.Client.Get(ctx, client.ObjectKey{
			Name:      podName,
			Namespace: f.GetNamespaceName(),
		}, pod)
		if err != nil {
			return false, err
		}
		return pod.Status.Phase == corev1.PodRunning, nil
	})

	Expect(err).NotTo(HaveOccurred(), "Pod did not become running in time")
	return pod
}

// DeletePod deletes a pod
func (f *Framework) DeletePod(name string) {
	By(fmt.Sprintf("Deleting pod %s", name))

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("pod-delete"))
	defer cancel()

	err := f.Client.Delete(ctx, pod)
	if err != nil && !apierrors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred(), "Failed to delete pod")
	}
}

// WaitForPodDeleted waits for a pod to be deleted
func (f *Framework) WaitForPodDeleted(podName string) {
	By(fmt.Sprintf("Waiting for pod %s to be deleted", podName))

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("pod-delete"))
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, f.GetTimeout("pod-delete"), true, func(ctx context.Context) (bool, error) {
		pod := &corev1.Pod{}
		err := f.Client.Get(ctx, client.ObjectKey{
			Name:      podName,
			Namespace: f.GetNamespaceName(),
		}, pod)
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})

	Expect(err).NotTo(HaveOccurred(), "Pod was not deleted in time")
}

// GetStorageClass gets a storage class by name
func (f *Framework) GetStorageClass(name string) *storagev1.StorageClass {
	sc := &storagev1.StorageClass{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Get(ctx, client.ObjectKey{Name: name}, sc)
	Expect(err).NotTo(HaveOccurred(), "Failed to get storage class")
	return sc
}

// ListStorageClasses lists all storage classes
func (f *Framework) ListStorageClasses() *storagev1.StorageClassList {
	scList := &storagev1.StorageClassList{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.List(ctx, scList)
	Expect(err).NotTo(HaveOccurred(), "Failed to list storage classes")
	return scList
}

// CreateSecret creates a secret
func (f *Framework) CreateSecret(name string, data map[string][]byte) *corev1.Secret {
	By(fmt.Sprintf("Creating secret %s", name))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
		Data: data,
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Create(ctx, secret)
	Expect(err).NotTo(HaveOccurred(), "Failed to create secret")

	f.TrackResource(secret)

	return secret
}
