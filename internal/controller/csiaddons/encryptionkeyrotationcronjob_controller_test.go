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

package controller

import (
	"context"
	"testing"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/controller/utils"

	"github.com/onsi/ginkgo/v2"
	gomega "github.com/onsi/gomega"
	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestGetNextScheduleForKeyRotation(t *testing.T) {
	now := time.Now()
	expectedLastMissed := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	mustParse := func(sched string) cron.Schedule {
		s, err := cron.ParseStandard(sched)
		if err != nil {
			t.Fatalf("Failed to parse cron spec %q: %v", sched, err)
		}
		return s
	}

	t.Run("Valid schedule, no deadline, last schedule time exists", func(t *testing.T) {
		uid := types.UID("kr-valid-sched-1")
		krcJob := &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{
			ObjectMeta: metav1.ObjectMeta{UID: uid},
			Spec: csiaddonsv1alpha1.EncryptionKeyRotationCronJobSpec{
				Schedule: "0 0 * * *",
			},
			Status: csiaddonsv1alpha1.EncryptionKeyRotationCronJobStatus{
				LastScheduleTime: &metav1.Time{Time: now.Add(-24 * time.Hour).Truncate(24 * time.Hour).Local()},
			},
		}
		gotLastMissed, gotNextSchedule, err := getNextScheduleForKeyRotation(krcJob, now, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sched := mustParse("0 0 * * *")
		expectedStaggered := utils.GetStaggeredNext(uid, expectedLastMissed, sched, 2)
		if !gotLastMissed.Equal(expectedStaggered) {
			t.Errorf("got last missed = %v, want %v", gotLastMissed, expectedStaggered)
		}
		nextSchedule := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).Add(24 * time.Hour)
		expectedNext := utils.GetStaggeredNext(uid, nextSchedule, sched, 2)
		if !gotNextSchedule.Equal(expectedNext) {
			t.Errorf("got next schedule = %v, want %v", gotNextSchedule, expectedNext)
		}
	})

	t.Run("Staggered lastMissed preserves StartingDeadlineSeconds validity", func(t *testing.T) {
		schedule := "0 0 * * *"
		staggerCap := 2
		uid := types.UID("kr-stagger-deadline-1")
		sched, err := cron.ParseStandard(schedule)
		if err != nil {
			t.Fatalf("Failed to parse schedule: %v", err)
		}

		midnight := time.Date(2026, 7, 15, 0, 0, 0, 0, time.Local)
		prevMidnight := midnight.Add(-24 * time.Hour)
		nextMidnight := midnight.Add(24 * time.Hour)

		staggeredNext := utils.GetStaggeredNext(uid, nextMidnight, sched, staggerCap)
		staggerOffset := staggeredNext.Sub(nextMidnight)

		testNow := midnight.Add(staggerOffset + 10*time.Second)
		startingDeadline := int64(staggerOffset.Seconds() / 2)
		if startingDeadline < 60 {
			startingDeadline = 60
		}

		krcJob := &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{
			ObjectMeta: metav1.ObjectMeta{UID: uid},
			Spec: csiaddonsv1alpha1.EncryptionKeyRotationCronJobSpec{
				Schedule:                schedule,
				StartingDeadlineSeconds: ptr.To(startingDeadline),
			},
			Status: csiaddonsv1alpha1.EncryptionKeyRotationCronJobStatus{
				LastScheduleTime: &metav1.Time{Time: prevMidnight},
			},
		}

		gotLastMissed, _, err := getNextScheduleForKeyRotation(krcJob, testNow, staggerCap)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedStaggered := utils.GetStaggeredNext(uid, midnight, sched, staggerCap)
		if !gotLastMissed.Equal(expectedStaggered) {
			t.Errorf("got last missed = %v, want %v (staggered from %v)", gotLastMissed, expectedStaggered, midnight)
		}

		deadline := time.Duration(startingDeadline) * time.Second
		if gotLastMissed.Add(deadline).Before(testNow) {
			t.Errorf("staggered lastMissed %v + deadline %v is before now %v; run would be incorrectly skipped",
				gotLastMissed, deadline, testNow)
		}
	})
}

var _ = ginkgo.Describe("EncryptionKeyRotationCronJob Controller", func() {
	ginkgo.Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		encryptionkeyrotationcronjob := &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{}

		ginkgo.BeforeEach(func() {
			ginkgo.By("creating the custom resource for the Kind EncryptionKeyRotationCronJob")
			err := k8sClient.Get(ctx, typeNamespacedName, encryptionkeyrotationcronjob)
			if err != nil && errors.IsNotFound(err) {
				resource := &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: csiaddonsv1alpha1.EncryptionKeyRotationCronJobSpec{
						Schedule: "@weekly",
						JobSpec: csiaddonsv1alpha1.EncryptionKeyRotationJobTemplateSpec{
							Spec: csiaddonsv1alpha1.EncryptionKeyRotationJobSpec{
								Target: csiaddonsv1alpha1.TargetSpec{
									PersistentVolumeClaim: "rbd-pvc",
								},
							},
						},
					},
				}
				gomega.Expect(k8sClient.Create(ctx, resource)).To(gomega.Succeed())
			}
		})

		ginkgo.AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Cleanup the specific resource instance EncryptionKeyRotationCronJob")
			gomega.Expect(k8sClient.Delete(ctx, resource)).To(gomega.Succeed())
		})
	})
})
