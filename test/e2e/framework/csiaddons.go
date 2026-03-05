/*
Copyright 2026 The Kubernetes-CSI-Addons Authors.

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
	"strings"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	replicationv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/replication.storage/v1alpha1"
)

// CreateReclaimSpaceJob creates a ReclaimSpaceJob
func (f *Framework) CreateReclaimSpaceJob(name string, pvcName string) *csiaddonsv1alpha1.ReclaimSpaceJob {
	ginkgo.By(fmt.Sprintf("Creating ReclaimSpaceJob %s for PVC %s", name, pvcName))

	rsJob := &csiaddonsv1alpha1.ReclaimSpaceJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
		Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
			Target: csiaddonsv1alpha1.TargetSpec{
				PersistentVolumeClaim: pvcName,
			},
			BackoffLimit:         6,
			RetryDeadlineSeconds: 600,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Create(ctx, rsJob)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create ReclaimSpaceJob")

	f.TrackResource(rsJob)

	return rsJob
}

// WaitForReclaimSpaceJobComplete waits for a ReclaimSpaceJob to complete
func (f *Framework) WaitForReclaimSpaceJobComplete(name string) *csiaddonsv1alpha1.ReclaimSpaceJob {
	ginkgo.By(fmt.Sprintf("Waiting for ReclaimSpaceJob %s to complete", name))

	rsJob := &csiaddonsv1alpha1.ReclaimSpaceJob{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("job-complete"))
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, f.GetTimeout("job-complete"), true, func(ctx context.Context) (bool, error) {
		err := f.Client.Get(ctx, client.ObjectKey{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		}, rsJob)
		if err != nil {
			return false, err
		}
		return rsJob.Status.Result != "", nil
	})

	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "ReclaimSpaceJob did not complete in time")
	return rsJob
}

// GetReclaimSpaceJob gets a ReclaimSpaceJob
func (f *Framework) GetReclaimSpaceJob(name string) *csiaddonsv1alpha1.ReclaimSpaceJob {
	rsJob := &csiaddonsv1alpha1.ReclaimSpaceJob{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: f.GetNamespaceName(),
	}, rsJob)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to get ReclaimSpaceJob")
	return rsJob
}

// CreateReclaimSpaceCronJob creates a ReclaimSpaceCronJob
func (f *Framework) CreateReclaimSpaceCronJob(name string, pvcName string, schedule string) *csiaddonsv1alpha1.ReclaimSpaceCronJob {
	ginkgo.By(fmt.Sprintf("Creating ReclaimSpaceCronJob %s for PVC %s with schedule %s", name, pvcName, schedule))

	rsCronJob := &csiaddonsv1alpha1.ReclaimSpaceCronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
		Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
			Schedule: schedule,
			JobSpec: csiaddonsv1alpha1.ReclaimSpaceJobTemplateSpec{
				Spec: csiaddonsv1alpha1.ReclaimSpaceJobSpec{
					Target: csiaddonsv1alpha1.TargetSpec{
						PersistentVolumeClaim: pvcName,
					},
					BackoffLimit:         3,
					RetryDeadlineSeconds: 600,
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Create(ctx, rsCronJob)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create ReclaimSpaceCronJob")

	f.TrackResource(rsCronJob)

	return rsCronJob
}

// GetReclaimSpaceCronJob gets a ReclaimSpaceCronJob
func (f *Framework) GetReclaimSpaceCronJob(name string) *csiaddonsv1alpha1.ReclaimSpaceCronJob {
	rsCronJob := &csiaddonsv1alpha1.ReclaimSpaceCronJob{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: f.GetNamespaceName(),
	}, rsCronJob)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to get ReclaimSpaceCronJob")
	return rsCronJob
}

// ListReclaimSpaceJobs lists ReclaimSpaceJobs with optional label selector
func (f *Framework) ListReclaimSpaceJobs(labelSelector map[string]string) *csiaddonsv1alpha1.ReclaimSpaceJobList {
	jobList := &csiaddonsv1alpha1.ReclaimSpaceJobList{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	listOpts := []client.ListOption{
		client.InNamespace(f.GetNamespaceName()),
	}
	if labelSelector != nil {
		listOpts = append(listOpts, client.MatchingLabels(labelSelector))
	}

	err := f.Client.List(ctx, jobList, listOpts...)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to list ReclaimSpaceJobs")
	return jobList
}

// WaitForReclaimSpaceJobCreation waits for at least one ReclaimSpaceJob to be created by a CronJob
func (f *Framework) WaitForReclaimSpaceJobCreation(cronJobName string, timeout time.Duration) *csiaddonsv1alpha1.ReclaimSpaceJob {
	ginkgo.By(fmt.Sprintf("Waiting for ReclaimSpaceJob to be created by CronJob %s", cronJobName))

	var createdJob *csiaddonsv1alpha1.ReclaimSpaceJob
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		jobList := f.ListReclaimSpaceJobs(nil)
		for i := range jobList.Items {
			job := &jobList.Items[i]
			// Check if this job is owned by the specified CronJob
			for _, ownerRef := range job.OwnerReferences {
				if ownerRef.Kind == "ReclaimSpaceCronJob" && ownerRef.Name == cronJobName {
					createdJob = job
					return true, nil
				}
			}
		}
		return false, nil
	})

	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "No ReclaimSpaceJob was created in time")
	return createdJob
}

// CreateEncryptionKeyRotationJob creates an EncryptionKeyRotationJob
func (f *Framework) CreateEncryptionKeyRotationJob(name string, pvcName string) *csiaddonsv1alpha1.EncryptionKeyRotationJob {
	ginkgo.By(fmt.Sprintf("Creating EncryptionKeyRotationJob %s for PVC %s", name, pvcName))

	ekrJob := &csiaddonsv1alpha1.EncryptionKeyRotationJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
		Spec: csiaddonsv1alpha1.EncryptionKeyRotationJobSpec{
			Target: csiaddonsv1alpha1.TargetSpec{
				PersistentVolumeClaim: pvcName,
			},
			BackoffLimit:         6,
			RetryDeadlineSeconds: 600,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Create(ctx, ekrJob)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create EncryptionKeyRotationJob")

	f.TrackResource(ekrJob)

	return ekrJob
}

// WaitForEncryptionKeyRotationJobComplete waits for an EncryptionKeyRotationJob to complete
func (f *Framework) WaitForEncryptionKeyRotationJobComplete(name string) *csiaddonsv1alpha1.EncryptionKeyRotationJob {
	ginkgo.By(fmt.Sprintf("Waiting for EncryptionKeyRotationJob %s to complete", name))

	ekrJob := &csiaddonsv1alpha1.EncryptionKeyRotationJob{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("job-complete"))
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, f.GetTimeout("job-complete"), true, func(ctx context.Context) (bool, error) {
		err := f.Client.Get(ctx, client.ObjectKey{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		}, ekrJob)
		if err != nil {
			return false, err
		}
		return ekrJob.Status.Result != "", nil
	})

	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "EncryptionKeyRotationJob did not complete in time")
	return ekrJob
}

// CreateEncryptionKeyRotationCronJob creates an EncryptionKeyRotationCronJob
func (f *Framework) CreateEncryptionKeyRotationCronJob(name string, pvcName string, schedule string) *csiaddonsv1alpha1.EncryptionKeyRotationCronJob {
	ginkgo.By(fmt.Sprintf("Creating EncryptionKeyRotationCronJob %s for PVC %s with schedule %s", name, pvcName, schedule))

	ekrCronJob := &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
		Spec: csiaddonsv1alpha1.EncryptionKeyRotationCronJobSpec{
			Schedule: schedule,
			JobSpec: csiaddonsv1alpha1.EncryptionKeyRotationJobTemplateSpec{
				Spec: csiaddonsv1alpha1.EncryptionKeyRotationJobSpec{
					Target: csiaddonsv1alpha1.TargetSpec{
						PersistentVolumeClaim: pvcName,
					},
					BackoffLimit:         3,
					RetryDeadlineSeconds: 600,
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Create(ctx, ekrCronJob)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create EncryptionKeyRotationCronJob")

	f.TrackResource(ekrCronJob)

	return ekrCronJob
}

// GetEncryptionKeyRotationCronJob gets an EncryptionKeyRotationCronJob
func (f *Framework) GetEncryptionKeyRotationCronJob(name string) *csiaddonsv1alpha1.EncryptionKeyRotationCronJob {
	ekrCronJob := &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: f.GetNamespaceName(),
	}, ekrCronJob)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to get EncryptionKeyRotationCronJob")
	return ekrCronJob
}

// ListEncryptionKeyRotationJobs lists EncryptionKeyRotationJobs with optional label selector
func (f *Framework) ListEncryptionKeyRotationJobs(labelSelector map[string]string) *csiaddonsv1alpha1.EncryptionKeyRotationJobList {
	jobList := &csiaddonsv1alpha1.EncryptionKeyRotationJobList{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	listOpts := []client.ListOption{
		client.InNamespace(f.GetNamespaceName()),
	}
	if labelSelector != nil {
		listOpts = append(listOpts, client.MatchingLabels(labelSelector))
	}

	err := f.Client.List(ctx, jobList, listOpts...)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to list EncryptionKeyRotationJobs")
	return jobList
}

// WaitForEncryptionKeyRotationJobCreation waits for at least one EncryptionKeyRotationJob to be created by a CronJob
func (f *Framework) WaitForEncryptionKeyRotationJobCreation(cronJobName string, timeout time.Duration) *csiaddonsv1alpha1.EncryptionKeyRotationJob {
	ginkgo.By(fmt.Sprintf("Waiting for EncryptionKeyRotationJob to be created by CronJob %s", cronJobName))

	var createdJob *csiaddonsv1alpha1.EncryptionKeyRotationJob
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		jobList := f.ListEncryptionKeyRotationJobs(nil)
		for i := range jobList.Items {
			job := &jobList.Items[i]
			// Check if this job is owned by the specified CronJob
			for _, ownerRef := range job.OwnerReferences {
				if ownerRef.Kind == "EncryptionKeyRotationCronJob" && ownerRef.Name == cronJobName {
					createdJob = job
					return true, nil
				}
			}
		}
		return false, nil
	})

	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "No EncryptionKeyRotationJob was created in time")
	return createdJob
}

// CreateNetworkFence creates a NetworkFence
func (f *Framework) CreateNetworkFence(name string, cidrs []string, fenceState csiaddonsv1alpha1.FenceState) *csiaddonsv1alpha1.NetworkFence {
	ginkgo.By(fmt.Sprintf("Creating NetworkFence %s", name))

	nf := &csiaddonsv1alpha1.NetworkFence{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
		Spec: csiaddonsv1alpha1.NetworkFenceSpec{
			Driver:     f.GetCSIDriverName(),
			Cidrs:      cidrs,
			FenceState: fenceState,
		},
	}

	// Add NetworkFence-specific parameters if configured
	nfParams := f.GetNetworkFenceResourceParameters()
	if len(nfParams) > 0 {
		nf.Spec.Parameters = nfParams
	}

	// Add secret if configured
	secretConfig := f.GetNetworkFenceSecret()
	if secretConfig.Name != "" && secretConfig.Namespace != "" {
		nf.Spec.Secret = csiaddonsv1alpha1.SecretSpec{
			Name:      secretConfig.Name,
			Namespace: secretConfig.Namespace,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Create(ctx, nf)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create NetworkFence")

	f.TrackResource(nf)

	return nf
}

// GetNetworkFence gets a NetworkFence
func (f *Framework) GetNetworkFence(name string) *csiaddonsv1alpha1.NetworkFence {
	nf := &csiaddonsv1alpha1.NetworkFence{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: f.GetNamespaceName(),
	}, nf)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to get NetworkFence")
	return nf
}

// CreateNetworkFenceWithClass creates a NetworkFence using NetworkFenceClass
func (f *Framework) CreateNetworkFenceWithClass(name string, cidrs []string, fenceState csiaddonsv1alpha1.FenceState, className string) *csiaddonsv1alpha1.NetworkFence {
	ginkgo.By(fmt.Sprintf("Creating NetworkFence %s with NetworkFenceClass %s", name, className))

	nf := &csiaddonsv1alpha1.NetworkFence{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
		Spec: csiaddonsv1alpha1.NetworkFenceSpec{
			NetworkFenceClassName: className,
			Cidrs:                 cidrs,
			FenceState:            fenceState,
		},
	}

	// Add NetworkFence-specific parameters if configured
	nfParams := f.GetNetworkFenceResourceParameters()
	if len(nfParams) > 0 {
		nf.Spec.Parameters = nfParams
	}

	// Add secret if configured (optional when using NetworkFenceClass)
	secretConfig := f.GetNetworkFenceSecret()
	if secretConfig.Name != "" && secretConfig.Namespace != "" {
		nf.Spec.Secret = csiaddonsv1alpha1.SecretSpec{
			Name:      secretConfig.Name,
			Namespace: secretConfig.Namespace,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Create(ctx, nf)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create NetworkFence with class")

	f.TrackResource(nf)

	return nf
}

// UpdateNetworkFence updates a NetworkFence
func (f *Framework) UpdateNetworkFence(nf *csiaddonsv1alpha1.NetworkFence) *csiaddonsv1alpha1.NetworkFence {
	ginkgo.By(fmt.Sprintf("Updating NetworkFence %s", nf.Name))

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Update(ctx, nf)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to update NetworkFence")
	return nf
}

// WaitForNetworkFenceResult waits for a NetworkFence to have a result
func (f *Framework) WaitForNetworkFenceResult(name string, expectedResult csiaddonsv1alpha1.FencingOperationResult) *csiaddonsv1alpha1.NetworkFence {
	ginkgo.By(fmt.Sprintf("Waiting for NetworkFence %s to have result %s", name, expectedResult))

	nf := &csiaddonsv1alpha1.NetworkFence{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, f.GetTimeout("operation"), true, func(ctx context.Context) (bool, error) {
		err := f.Client.Get(ctx, client.ObjectKey{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		}, nf)
		if err != nil {
			return false, err
		}
		return nf.Status.Result == expectedResult, nil
	})

	gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("NetworkFence did not reach result %s in time", expectedResult))
	return nf
}

// WaitForNetworkFenceMessage waits for a NetworkFence to have a specific message in its status
func (f *Framework) WaitForNetworkFenceMessage(name string, expectedMessage string) *csiaddonsv1alpha1.NetworkFence {
	ginkgo.By(fmt.Sprintf("Waiting for NetworkFence %s to have message containing %q", name, expectedMessage))

	nf := &csiaddonsv1alpha1.NetworkFence{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, f.GetTimeout("operation"), true, func(ctx context.Context) (bool, error) {
		err := f.Client.Get(ctx, client.ObjectKey{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		}, nf)
		if err != nil {
			return false, err
		}
		// Check if the message contains the expected substring and the result is Succeeded
		return nf.Status.Result == csiaddonsv1alpha1.FencingOperationResultSucceeded &&
			strings.Contains(nf.Status.Message, expectedMessage), nil
	})

	gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("NetworkFence did not have message containing %q in time", expectedMessage))
	return nf
}

// CreateNetworkFenceClass creates a NetworkFenceClass
func (f *Framework) CreateNetworkFenceClass(name string, provisioner string, parameters map[string]string) *csiaddonsv1alpha1.NetworkFenceClass {
	ginkgo.By(fmt.Sprintf("Creating NetworkFenceClass %s", name))

	nfc := &csiaddonsv1alpha1.NetworkFenceClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: csiaddonsv1alpha1.NetworkFenceClassSpec{
			Provisioner: provisioner,
			Parameters:  parameters,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Create(ctx, nfc)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create NetworkFenceClass")

	f.TrackResource(nfc)

	return nfc
}

// GetNetworkFenceClass gets a NetworkFenceClass
func (f *Framework) GetNetworkFenceClass(name string) *csiaddonsv1alpha1.NetworkFenceClass {
	nfc := &csiaddonsv1alpha1.NetworkFenceClass{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Get(ctx, client.ObjectKey{Name: name}, nfc)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to get NetworkFenceClass")
	return nfc
}

// CreateVolumeReplicationClass creates a VolumeReplicationClass
func (f *Framework) CreateVolumeReplicationClass(name string, provisioner string, parameters map[string]string) *replicationv1alpha1.VolumeReplicationClass {
	ginkgo.By(fmt.Sprintf("Creating VolumeReplicationClass %s", name))

	vrc := &replicationv1alpha1.VolumeReplicationClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: replicationv1alpha1.VolumeReplicationClassSpec{
			Provisioner: provisioner,
			Parameters:  parameters,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Create(ctx, vrc)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create VolumeReplicationClass")

	f.TrackResource(vrc)

	return vrc
}

// CreateVolumeReplication creates a VolumeReplication
func (f *Framework) CreateVolumeReplication(name string, pvcName string, vrcName string, replicationState replicationv1alpha1.ReplicationState) *replicationv1alpha1.VolumeReplication {
	ginkgo.By(fmt.Sprintf("Creating VolumeReplication %s for PVC %s", name, pvcName))

	vr := &replicationv1alpha1.VolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		},
		Spec: replicationv1alpha1.VolumeReplicationSpec{
			VolumeReplicationClass: vrcName,
			ReplicationState:       replicationState,
			DataSource: corev1.TypedLocalObjectReference{
				Kind: "PersistentVolumeClaim",
				Name: pvcName,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Create(ctx, vr)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create VolumeReplication")

	f.TrackResource(vr)

	return vr
}

// WaitForVolumeReplicationState waits for a VolumeReplication to reach a specific state
func (f *Framework) WaitForVolumeReplicationState(name string, expectedState replicationv1alpha1.State) *replicationv1alpha1.VolumeReplication {
	ginkgo.By(fmt.Sprintf("Waiting for VolumeReplication %s to reach state %s", name, expectedState))

	vr := &replicationv1alpha1.VolumeReplication{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("replication-sync"))
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, f.GetTimeout("replication-sync"), true, func(ctx context.Context) (bool, error) {
		err := f.Client.Get(ctx, client.ObjectKey{
			Name:      name,
			Namespace: f.GetNamespaceName(),
		}, vr)
		if err != nil {
			return false, err
		}
		return vr.Status.State == expectedState, nil
	})

	gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("VolumeReplication did not reach state %s in time", expectedState))
	return vr
}

// GetVolumeReplication gets a VolumeReplication
func (f *Framework) GetVolumeReplication(name string) *replicationv1alpha1.VolumeReplication {
	vr := &replicationv1alpha1.VolumeReplication{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: f.GetNamespaceName(),
	}, vr)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to get VolumeReplication")
	return vr
}

// UpdateVolumeReplication updates a VolumeReplication
func (f *Framework) UpdateVolumeReplication(vr *replicationv1alpha1.VolumeReplication) {
	ginkgo.By(fmt.Sprintf("Updating VolumeReplication %s", vr.Name))

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Update(ctx, vr)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to update VolumeReplication")
}

// DeleteResource deletes a resource
func (f *Framework) DeleteResource(obj client.Object) {
	ginkgo.By(fmt.Sprintf("Deleting resource %s/%s", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName()))

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Delete(ctx, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to delete resource")
	}
}

// WaitForResourceDeleted waits for a resource to be deleted
func (f *Framework) WaitForResourceDeleted(obj client.Object, timeout time.Duration) {
	ginkgo.By(fmt.Sprintf("Waiting for resource %s/%s to be deleted", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName()))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		err := f.Client.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})

	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Resource was not deleted in time")
}

// CreateVolumeGroupReplicationClass creates a VolumeGroupReplicationClass
func (f *Framework) CreateVolumeGroupReplicationClass(name string, provisioner string, parameters map[string]string) *replicationv1alpha1.VolumeGroupReplicationClass {
	ginkgo.By(fmt.Sprintf("Creating VolumeGroupReplicationClass %s", name))

	vgrc := &replicationv1alpha1.VolumeGroupReplicationClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: replicationv1alpha1.VolumeGroupReplicationClassSpec{
			Provisioner: provisioner,
			Parameters:  parameters,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Create(ctx, vgrc)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to create VolumeGroupReplicationClass")

	f.TrackResource(vgrc)

	return vgrc
}

// GetVolumeGroupReplicationClass gets a VolumeGroupReplicationClass
func (f *Framework) GetVolumeGroupReplicationClass(name string) *replicationv1alpha1.VolumeGroupReplicationClass {
	vgrc := &replicationv1alpha1.VolumeGroupReplicationClass{}
	ctx, cancel := context.WithTimeout(context.Background(), f.GetTimeout("operation"))
	defer cancel()

	err := f.Client.Get(ctx, client.ObjectKey{Name: name}, vgrc)
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Failed to get VolumeGroupReplicationClass")
	return vgrc
}
