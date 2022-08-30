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
	"testing"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/apis/csiaddons/v1alpha1"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
