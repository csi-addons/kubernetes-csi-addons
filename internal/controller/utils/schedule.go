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
	"hash/fnv"
	"time"

	csiaddonsv1alpha1 "github.com/csi-addons/kubernetes-csi-addons/api/csiaddons/v1alpha1"

	"github.com/robfig/cron/v3"
	apiTypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetStaggeredNext returns a deterministic, UID-based staggered time computed from a schedule.
// The staggered window is capped at a maximum of `cap` but is adjusted for smaller intervals.
func GetStaggeredNext(uid apiTypes.UID, nextTime time.Time, sched cron.Schedule, cap int) time.Time {
	// if cap is set to 0, do not apply any stagger
	if cap == 0 {
		return nextTime
	}

	// cron does not expose interval directly
	// We can determine it by subtracting two consecutive intervals
	afterNext := sched.Next(nextTime)
	interval := afterNext.Sub(nextTime)

	// We do not want the stagger window to be any larger than `cap`
	maxCap := time.Hour * time.Duration(cap)
	staggerWindow := min(interval, maxCap)

	// To prevent the schedule from jumping in bw the reconciles,
	// we use the UID and hash it, this way it remains deterministic
	h := fnv.New32a()
	if _, err := h.Write([]byte(string(uid))); err != nil {
		return nextTime
	}
	hash := h.Sum32()

	// Just a safety net
	stgrWindowSeconds := int64(staggerWindow.Seconds())
	if stgrWindowSeconds <= 0 {
		return nextTime
	}
	offsetSeconds := int64(hash) % stgrWindowSeconds // Modulo ensures it to be < stgrWindowSeconds

	return nextTime.Add(time.Duration(offsetSeconds) * time.Second)
}

func GetSchedule(obj client.Object) string {
	switch v := obj.(type) {
	case *csiaddonsv1alpha1.EncryptionKeyRotationCronJob:
		return v.Spec.Schedule
	case *csiaddonsv1alpha1.ReclaimSpaceCronJob:
		return v.Spec.Schedule
	default:
		return ""
	}
}
