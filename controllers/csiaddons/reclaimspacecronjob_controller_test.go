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

package controllers

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/apis/csiaddons/v1alpha1"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
					Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
						Schedule: "0 0 * * *",
					},
					Status: csiaddonsv1alpha1.ReclaimSpaceCronJobStatus{
						LastScheduleTime: &metav1.Time{Time: now.Add(-time.Hour)},
					},
				},
				now: now,
			},
			lastMissed:   time.Time{},
			nextSchedule: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).Add(time.Hour * 24),
			wantErr:      false,
		},
		{
			name: "Valid schedule, deadline not exceeded, last schedule time exists",
			args: args{
				rsCronJob: &csiaddonsv1alpha1.ReclaimSpaceCronJob{
					Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
						Schedule:                "0 0 * * *",
						StartingDeadlineSeconds: ptr.To(int64(3600)),
					},
					Status: csiaddonsv1alpha1.ReclaimSpaceCronJobStatus{
						LastScheduleTime: &metav1.Time{Time: now.Add(-time.Hour)},
					},
				},
				now: now,
			},
			lastMissed:   time.Time{},
			nextSchedule: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).Add(time.Hour * 24),
			wantErr:      false,
		},
		{
			name: "Valid schedule, deadline exceeded, last schedule time exists, missed schedules < 100",
			args: args{
				rsCronJob: &csiaddonsv1alpha1.ReclaimSpaceCronJob{
					Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
						Schedule:                "*/1 * * * *",
						StartingDeadlineSeconds: ptr.To(int64(6000)),
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
					Spec: csiaddonsv1alpha1.ReclaimSpaceCronJobSpec{
						Schedule:                "*/1 * * * *",
						StartingDeadlineSeconds: ptr.To(int64(6060)),
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
			gotLastMissed, gotNextSchedule, err := getNextSchedule(tt.args.rsCronJob, tt.args.now)
			if (err != nil) != tt.wantErr {
				t.Errorf("getNextSchedule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotLastMissed, tt.lastMissed) {
				t.Errorf("getNextSchedule() got last missed = %v, want %v", gotLastMissed, tt.lastMissed)
			}
			if !reflect.DeepEqual(gotNextSchedule, tt.nextSchedule) {
				t.Errorf("getNextSchedule() got next schedule = %v, want %v", gotNextSchedule, tt.nextSchedule)
			}
		})
	}
}
