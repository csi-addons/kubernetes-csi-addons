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

package controller

import (
	"fmt"
	"testing"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/csi-addons/kubernetes-csi-addons/internal/controller/utils"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func TestGetScheduledTimeForRSJob(t *testing.T) {
	type args struct {
		rsJob *csiaddonsv1alpha1.ReclaimSpaceJob
	}
	scheduledTime := time.Now()
	scheduledTimeString := scheduledTime.Format(time.RFC3339)
	tests := []struct {
		name    string
		args    args
		want    *time.Time
		wantErr bool
	}{
		{
			name: "empty scheduled time string",
			args: args{
				rsJob: &csiaddonsv1alpha1.ReclaimSpaceJob{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							scheduledTimeAnnotation: "",
						},
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "invalid scheduled timestring",
			args: args{
				rsJob: &csiaddonsv1alpha1.ReclaimSpaceJob{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							scheduledTimeAnnotation: "abc",
						},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "missing annotation",
			args: args{
				rsJob: &csiaddonsv1alpha1.ReclaimSpaceJob{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "valid scheduled timestring",
			args: args{
				rsJob: &csiaddonsv1alpha1.ReclaimSpaceJob{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							scheduledTimeAnnotation: scheduledTimeString,
						},
					},
				},
			},
			want:    &scheduledTime,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getScheduledTimeForRSJob(tt.args.rsJob)
			if (err != nil) != tt.wantErr {
				t.Errorf("getScheduledTimeForRSJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			fmt.Println(tt.want, got)
			if tt.want != nil {
				assert.Equal(t, tt.want.Format(time.RFC3339), got.Format(time.RFC3339))
			}
		})
	}
}

func TestGetNextSchedule(t *testing.T) {
	now := time.Now()
	expectedLastMissed := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	mustParse := func(sched string) cron.Schedule {
		s, err := cron.ParseStandard(sched)
		if err != nil {
			t.Fatalf("Failed to parse cron spec %q: %v", sched, err)
		}
		return s
	}
	type args struct {
		rsCronJob *csiaddonsv1alpha1.ReclaimSpaceCronJob
		now       time.Time
	}
	tests := []struct {
		name         string
		args         args
		lastMissed   time.Time
		nextSchedule time.Time
		wantErr      bool
	}{
		{
			name: "Valid schedule, no deadline, last schedule time exists",
			args: args{
				rsCronJob: &csiaddonsv1alpha1.ReclaimSpaceCronJob{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("valid-sched-1"),
					},
					Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
						Schedule: "0 0 * * *",
					},
					Status: csiaddonsv1alpha1.ReclaimSpaceCronJobStatus{
						LastScheduleTime: &metav1.Time{Time: now.Add(-24 * time.Hour).Truncate(24 * time.Hour).Local()},
					},
				},
				now: now,
			},
			lastMissed:   expectedLastMissed,
			nextSchedule: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).Add(time.Hour * 24),
			wantErr:      false,
		},
		{
			name: "Valid schedule, deadline not exceeded, last schedule time exists",
			args: args{
				rsCronJob: &csiaddonsv1alpha1.ReclaimSpaceCronJob{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("valid-sched-2"),
					},
					Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
						Schedule:                "0 0 * * *",
						StartingDeadlineSeconds: ptr.To(int64(86400)),
					},
					Status: csiaddonsv1alpha1.ReclaimSpaceCronJobStatus{
						LastScheduleTime: &metav1.Time{Time: now.Add(-24 * time.Hour).Truncate(24 * time.Hour).Local()},
					},
				},
				now: now,
			},
			lastMissed:   expectedLastMissed,
			nextSchedule: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).Add(time.Hour * 24),
			wantErr:      false,
		},
		{
			name: "Valid schedule, deadline exceeded, last schedule time exists, missed schedules < 100",
			args: args{
				rsCronJob: &csiaddonsv1alpha1.ReclaimSpaceCronJob{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("valid-sched-3"),
					},
					Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
						Schedule:                "*/1 * * * *",
						StartingDeadlineSeconds: ptr.To(int64(5880)),
					},
					Status: csiaddonsv1alpha1.ReclaimSpaceCronJobStatus{
						LastScheduleTime: &metav1.Time{Time: now.Add(-time.Hour * 2)},
					},
				},
				now: now,
			},
			lastMissed:   time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, time.Local),
			nextSchedule: time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, time.Local).Add(time.Minute),
			wantErr:      false,
		},
		{
			name: "Valid schedule, deadline exceeded, last schedule time exists, missed schedules > 100",
			args: args{
				rsCronJob: &csiaddonsv1alpha1.ReclaimSpaceCronJob{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("valid-sched-4"),
					},
					Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
						Schedule:                "*/1 * * * *",
						StartingDeadlineSeconds: ptr.To(int64(6120)),
					},
					Status: csiaddonsv1alpha1.ReclaimSpaceCronJobStatus{
						LastScheduleTime: &metav1.Time{Time: now.Add(-time.Hour * 2)},
					},
				},
				now: now,
			},
			lastMissed:   time.Time{},
			nextSchedule: time.Time{},
			wantErr:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLastMissed, gotNextSchedule, err := getNextSchedule(tt.args.rsCronJob, tt.args.now, 2)
			if (err != nil) != tt.wantErr {
				t.Errorf("getNextSchedule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			sched := mustParse(tt.args.rsCronJob.Spec.Schedule)

			if !tt.lastMissed.IsZero() {
				expectedStaggered := utils.GetStaggeredNext(tt.args.rsCronJob.UID, tt.lastMissed, sched, 2)
				if !gotLastMissed.Equal(expectedStaggered) {
					t.Errorf("getNextSchedule() got last missed = %v, want %v (staggered from %v)", gotLastMissed, expectedStaggered, tt.lastMissed)
				}
			} else if !gotLastMissed.IsZero() {
				t.Errorf("getNextSchedule() got last missed = %v, want zero", gotLastMissed)
			}

			staggered := utils.GetStaggeredNext(tt.args.rsCronJob.UID, tt.nextSchedule, sched, 2)
			if !gotNextSchedule.Equal(staggered) {
				t.Errorf("getNextSchedule() got next schedule = %v, want %v", gotNextSchedule, staggered)
			}
		})
	}
}

func TestGetNextSchedule_StaggerPreservesStartingDeadline(t *testing.T) {
	schedule := "0 0 * * *"
	staggerCap := 2
	uid := types.UID("stagger-deadline-rs")
	sched, err := cron.ParseStandard(schedule)
	if err != nil {
		t.Fatalf("Failed to parse schedule: %v", err)
	}

	midnight := time.Date(2026, 7, 15, 0, 0, 0, 0, time.Local)
	prevMidnight := midnight.Add(-24 * time.Hour)
	nextMidnight := midnight.Add(24 * time.Hour)

	staggeredNext := utils.GetStaggeredNext(uid, nextMidnight, sched, staggerCap)
	staggerOffset := staggeredNext.Sub(nextMidnight)

	// Simulate the controller waking up just after the staggered time.
	testNow := midnight.Add(staggerOffset + 10*time.Second)
	// Set StartingDeadlineSeconds smaller than the stagger offset.
	// Without the fix, this would cause the scheduling deadline window
	// to exclude midnight, silently skipping the run.
	startingDeadline := int64(staggerOffset.Seconds() / 2)
	if startingDeadline < 60 {
		startingDeadline = 60
	}

	rsCronJob := &csiaddonsv1alpha1.ReclaimSpaceCronJob{
		ObjectMeta: metav1.ObjectMeta{UID: uid},
		Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
			Schedule:                schedule,
			StartingDeadlineSeconds: ptr.To(startingDeadline),
		},
		Status: csiaddonsv1alpha1.ReclaimSpaceCronJobStatus{
			LastScheduleTime: &metav1.Time{Time: prevMidnight},
		},
	}

	gotLastMissed, _, err := getNextSchedule(rsCronJob, testNow, staggerCap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedStaggered := utils.GetStaggeredNext(uid, midnight, sched, staggerCap)
	if !gotLastMissed.Equal(expectedStaggered) {
		t.Errorf("got last missed = %v, want %v (staggered from %v)", gotLastMissed, expectedStaggered, midnight)
	}

	// The reconciler's "too late" check must not trigger.
	deadline := time.Duration(startingDeadline) * time.Second
	if gotLastMissed.Add(deadline).Before(testNow) {
		t.Errorf("staggered lastMissed %v + deadline %v is before now %v; run would be incorrectly skipped",
			gotLastMissed, deadline, testNow)
	}
}
