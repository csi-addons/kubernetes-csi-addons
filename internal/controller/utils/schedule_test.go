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

package utils

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	apiTypes "k8s.io/apimachinery/pkg/types"
)

func mustParseSchedule(t *testing.T, spec string) cron.Schedule {
	t.Helper()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	sched, err := parser.Parse(spec)
	if err != nil {
		t.Fatalf("failed to parse schedule %q: %v", spec, err)
	}
	return sched
}

func TestGetStaggeredNext(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		uid          apiTypes.UID
		schedule     string
		cap          int
		expectOffset bool
	}{
		{
			name:         "cap zero returns nextTime unchanged",
			uid:          "test-uid-1",
			schedule:     "@hourly",
			cap:          0,
			expectOffset: false,
		},
		{
			name:         "hourly schedule with cap 1 hour",
			uid:          "test-uid-2",
			schedule:     "@hourly",
			cap:          1,
			expectOffset: true,
		},
		{
			name:         "daily schedule with cap 24 hours",
			uid:          "test-uid-3",
			schedule:     "@daily",
			cap:          24,
			expectOffset: true,
		},
		{
			name:         "daily schedule capped at 1 hour",
			uid:          "test-uid-4",
			schedule:     "@daily",
			cap:          1,
			expectOffset: true,
		},
		{
			name:         "weekly schedule with cap 24 hours",
			uid:          "test-uid-5",
			schedule:     "@weekly",
			cap:          24,
			expectOffset: true,
		},
		{
			name:         "every minute schedule with cap 1 hour",
			uid:          "test-uid-6",
			schedule:     "* * * * *",
			cap:          1,
			expectOffset: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched := mustParseSchedule(t, tt.schedule)
			nextTime := sched.Next(baseTime)

			result := GetStaggeredNext(tt.uid, nextTime, sched, tt.cap)

			// If no offset, result must be same as nextTime
			if !tt.expectOffset {
				if !result.Equal(nextTime) {
					t.Errorf("expected nextTime %v, got %v", nextTime, result)
				}
				return
			}

			// Result cannot be before the next time
			if result.Before(nextTime) {
				t.Errorf("staggered time %v is before nextTime %v", result, nextTime)
			}

			// Result should be within the stagger window
			afterNext := sched.Next(nextTime)
			interval := afterNext.Sub(nextTime)
			maxCap := time.Hour * time.Duration(tt.cap)
			staggerWindow := min(interval, maxCap)

			upperBound := nextTime.Add(staggerWindow)
			if result.After(upperBound) || result.Equal(upperBound) {
				t.Errorf("staggered time %v is outside stagger window [%v, %v)", result, nextTime, upperBound)
			}
		})
	}
}

func TestGetStaggeredNext_Determinism(t *testing.T) {
	baseTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		uid      apiTypes.UID
		schedule string
		cap      int
	}{
		{
			name:     "hourly schedule",
			uid:      "determinism-uid-1",
			schedule: "@hourly",
			cap:      1,
		},
		{
			name:     "daily schedule",
			uid:      "determinism-uid-2",
			schedule: "@daily",
			cap:      24,
		},
		{
			name:     "weekly schedule",
			uid:      "determinism-uid-3",
			schedule: "@weekly",
			cap:      24,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched := mustParseSchedule(t, tt.schedule)
			nextTime := sched.Next(baseTime)

			first := GetStaggeredNext(tt.uid, nextTime, sched, tt.cap)
			for i := range 100 {
				result := GetStaggeredNext(tt.uid, nextTime, sched, tt.cap)
				if !result.Equal(first) {
					t.Fatalf("call %d returned %v, expected %v", i, result, first)
				}
			}
		})
	}
}

func TestGetStaggeredNext_DifferentUIDs(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	sched := mustParseSchedule(t, "@daily")
	nextTime := sched.Next(baseTime)
	cap := 24

	uids := []apiTypes.UID{
		"uid-aaa-111",
		"uid-bbb-222",
		"uid-ccc-333",
		"uid-ddd-444",
		"uid-eee-555",
	}

	results := make(map[time.Time]apiTypes.UID)
	for _, uid := range uids {
		result := GetStaggeredNext(uid, nextTime, sched, cap)
		if prevUID, exists := results[result]; exists {
			// Not a hard failure since hash collisions are possible,
			// but with 5 UIDs it is extremely unlikely.
			t.Errorf("UIDs %q and %q produced the same staggered time %v", prevUID, uid, result)
		}
		results[result] = uid
	}
}
