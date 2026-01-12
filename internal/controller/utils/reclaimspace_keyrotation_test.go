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
	"testing"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"
	"github.com/robfig/cron/v3"
	apitypes "k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSetSpec(t *testing.T) {
	tests := []struct {
		name     string
		obj      client.Object
		schedule string
		pvcName  string
		validate func(t *testing.T, obj client.Object)
	}{
		{
			name:     "SetSpec with EncryptionKeyRotationCronJob",
			obj:      &csiaddonsv1alpha1.EncryptionKeyRotationCronJob{},
			schedule: "@weekly",
			pvcName:  "test-pvc",
			validate: func(t *testing.T, obj client.Object) {
				ekrCronJob := obj.(*csiaddonsv1alpha1.EncryptionKeyRotationCronJob)
				if ekrCronJob.Spec.Schedule != "@weekly" {
					t.Errorf("expected schedule '@weekly', got '%s'", ekrCronJob.Spec.Schedule)
				}
				if ekrCronJob.Spec.JobSpec.Spec.Target.PersistentVolumeClaim != "test-pvc" {
					t.Errorf("expected pvcName 'test-pvc', got '%s'", ekrCronJob.Spec.JobSpec.Spec.Target.PersistentVolumeClaim)
				}
				// We set it by default when setting spec
				if ekrCronJob.Annotations[CSIAddonsStateAnnotation] != CSIAddonsStateManaged {
					t.Error("expected CSIAddonsStateManaged annotation")
				}
			},
		},
		{
			name:     "SetSpec with ReclaimSpaceCronJob",
			obj:      &csiaddonsv1alpha1.ReclaimSpaceCronJob{},
			schedule: "@daily",
			pvcName:  "another-pvc",
			validate: func(t *testing.T, obj client.Object) {
				rsCronJob := obj.(*csiaddonsv1alpha1.ReclaimSpaceCronJob)
				if rsCronJob.Spec.Schedule != "@daily" {
					t.Errorf("expected schedule '@daily', got '%s'", rsCronJob.Spec.Schedule)
				}
				if rsCronJob.Spec.JobSpec.Spec.Target.PersistentVolumeClaim != "another-pvc" {
					t.Errorf("expected pvcName 'another-pvc', got '%s'", rsCronJob.Spec.JobSpec.Spec.Target.PersistentVolumeClaim)
				}
				// We set it by default when setting spec
				if rsCronJob.Annotations[CSIAddonsStateAnnotation] != CSIAddonsStateManaged {
					t.Error("expected CSIAddonsStateManaged annotation")
				}
			},
		},
		{
			name:     "SetSpec with nil object",
			obj:      nil,
			schedule: "@monthly",
			pvcName:  "test-pvc",
			validate: func(t *testing.T, obj client.Object) {
				// No validation needed, we should not panic
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetSpec(tt.obj, tt.schedule, tt.pvcName)
			tt.validate(t, tt.obj)
		})
	}
}

func TestGetStaggeredNext(t *testing.T) {
	baseTime, _ := time.Parse(time.RFC3339, "2026-01-01T12:00:00Z")

	mustParse := func(sched string) cron.Schedule {
		s, err := cron.ParseStandard(sched)
		if err != nil {
			t.Fatalf("Failed to parse cron spec %q: %v", sched, err)
		}
		return s
	}

	tests := []struct {
		name     string
		uid      apitypes.UID
		schedule string
		baseNext time.Time
		maxDelta time.Duration // The staggered offset msut be less than this value
	}{
		{
			name:     "Hourly schedule - capped at 1h interval",
			uid:      apitypes.UID("job-hourly"),
			schedule: "@hourly",
			baseNext: baseTime,
			maxDelta: 1 * time.Hour,
		},
		{
			name:     "Daily schedule - capped at 2h max cap",
			uid:      apitypes.UID("job-daily"),
			schedule: "@daily",
			baseNext: baseTime,
			maxDelta: 2 * time.Hour,
		},
		{
			name:     "1 Minute schedule - capped at 1m interval",
			uid:      apitypes.UID("job-minute"),
			schedule: "* * * * *",
			baseNext: baseTime,
			maxDelta: 1 * time.Minute,
		},
		{
			name:     "13 Minute schedule - capped at 13m interval",
			uid:      apitypes.UID("job-13m"),
			schedule: "*/13 * * * *",
			baseNext: baseTime,
			maxDelta: 13 * time.Minute,
		},
		{
			// This last test case is special, avoid modifications
			name:     "Determinism Check A (Run 1)", // Do not modify, used to check when to store the result
			uid:      apitypes.UID("static-uid"),    // Do not modify, used to assert determinisim below
			schedule: "@daily",
			baseNext: baseTime,
			maxDelta: 2 * time.Hour,
		},
	}

	// Will store the result of the above special test case
	var deterministicResult time.Time
	var collisionResult time.Time

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sched := mustParse(tc.schedule)
			got := GetStaggeredNext(tc.uid, tc.baseNext, sched)

			// Staggered time must be >= original time
			if got.Before(tc.baseNext) {
				t.Errorf("Result %v is before base time %v", got, tc.baseNext)
			}

			// Offset must be < min(cronInterval, maxCapof2hrs)
			offset := got.Sub(tc.baseNext)
			if offset >= tc.maxDelta {
				t.Errorf("Offset %v exceeds allowed max delta %v", offset, tc.maxDelta)
			}

			// Store results of the last test case for determinism
			if tc.name == "Determinism Check A (Run 1)" {
				deterministicResult = got
				collisionResult = got
			}
		})

		// to ensure the run after the above TCs
		if i == len(tests)-1 {
			t.Run("Determinism Check B (Run 2 - Same UID)", func(t *testing.T) {
				sched := mustParse("@daily")
				got := GetStaggeredNext("static-uid", baseTime, sched)

				if !got.Equal(deterministicResult) {
					t.Errorf("Expected %v, got %v. Result not deterministic.", deterministicResult, got)
				}
			})

			t.Run("Collision Check (Different UID)", func(t *testing.T) {
				sched := mustParse("@daily")
				got := GetStaggeredNext("different-uid", baseTime, sched)

				// It is theoretically possible for hashes to collide, but highly unlikely
				// especially within a 2hr window
				if got.Equal(collisionResult) {
					t.Logf("Warning: Different UIDs resulted in exact same time %v. "+
						"This is rare but possible (hash collision).", got)
				} else {
					t.Logf("Verified different UIDs produced different offsets: %v vs %v",
						collisionResult.Sub(baseTime), got.Sub(baseTime))
				}
			})
		}
	}
}
