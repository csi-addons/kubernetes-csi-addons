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
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
)

func TestGetScheduledTime(t *testing.T) {
	t.Parallel()
	const defaultScheduleTime = time.Hour
	logger := testr.New(t)
	testcases := []struct {
		parameters map[string]string
		time       time.Duration
	}{
		{
			parameters: map[string]string{
				"replication.storage.openshift.io/replication-secret-name": "rook-csi-rbd-provisioner",
				"schedulingInterval": "1m",
			},
			time: time.Minute,
		},
		{
			parameters: map[string]string{
				"replication.storage.openshift.io/replication-secret-name": "rook-csi-rbd-provisioner",
			},
			time: defaultScheduleTime,
		},
		{
			parameters: map[string]string{},
			time:       defaultScheduleTime,
		},
		{
			parameters: map[string]string{
				"schedulingInterval": "",
			},
			time: defaultScheduleTime,
		},
		{
			parameters: map[string]string{
				"schedulingInterval": "2mm",
			},
			time: defaultScheduleTime,
		},
		{
			parameters: map[string]string{
				"schedulingInterval": "10s",
			},
			time: 10 * time.Second,
		},
		{
			parameters: map[string]string{
				"schedulingInterval": "4m",
			},
			time: 2 * time.Minute,
		},
		{
			parameters: map[string]string{
				"schedulingInterval": "1h",
			},
			time: 30 * time.Minute,
		},
	}
	for _, tt := range testcases {
		newtt := tt
		t.Run("", func(t *testing.T) {
			t.Parallel()
			if got := getInfoReconcileInterval(newtt.parameters, logger); got != newtt.time {
				t.Errorf("GetSchedluedTime() = %v, want %v", got, newtt.time)
			}
		})
	}
}
