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

package condition

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetPVCVolumeHealthAnnotationKey(t *testing.T) {
	assert.Equal(
		t,
		"csiaddons.openshift.io/volumehealth.35d4f4a8-0000-1111-2222-2e9fdf21e086",
		getPVCVolumeHealthAnnotationKey("35d4f4a8-0000-1111-2222-2e9fdf21e086"),
	)
}

func TestGetSinceTimestamp(t *testing.T) {
	empty := getSinceTimestamp("", pvcVolumeHealthAnnotationStateUnhealthy)
	assert.Equal(t, "", empty)

	invalidJSON := getSinceTimestamp("unhealthy", pvcVolumeHealthAnnotationStateUnhealthy)
	assert.Equal(t, "", invalidJSON)

	stateMismatch := getSinceTimestamp(
		`{"state":"healthy","lastChecked":"2026-06-23T05:16:00Z","since":"2026-06-23T04:30:00Z"}`,
		pvcVolumeHealthAnnotationStateUnhealthy,
	)
	assert.Equal(t, "", stateMismatch)

	stateMatch := getSinceTimestamp(
		`{"state":"unhealthy","lastChecked":"2026-06-23T05:16:00Z","since":"2026-06-23T04:30:00Z"}`,
		pvcVolumeHealthAnnotationStateUnhealthy,
	)
	assert.Equal(t, "2026-06-23T04:30:00Z", stateMatch)
}

func TestBuildPVCVolumeHealthAnnotationPatchOnlyTouchesLocalKey(t *testing.T) {
	localNodeUID := "35d4f4a8-0000-1111-2222-2e9fdf21e086"
	remoteNodeUID := "028ab0a3-1111-2222-3333-f2ad83f31389"

	patch, err := buildPVCVolumeHealthAnnotationPatch(map[string]string{
		"other": "value",
		getPVCVolumeHealthAnnotationKey(remoteNodeUID): `{"state":"healthy","lastChecked":"2026-06-23T05:16:00Z","since":"2026-06-23T04:30:00Z","node":"node-b"}`,
	}, localNodeUID, "node-a", false)
	if !assert.NoError(t, err) {
		return
	}

	patchObj := map[string]interface{}{}
	err = json.Unmarshal(patch, &patchObj)
	if !assert.NoError(t, err) {
		return
	}

	patchMetadata, ok := patchObj["metadata"].(map[string]interface{})
	if !assert.True(t, ok) {
		return
	}

	patchAnnotation, ok := patchMetadata["annotations"].(map[string]interface{})
	if !assert.True(t, ok) {
		return
	}

	// The merge-patch should include only the local node annotation key update.
	assert.Len(t, patchAnnotation, 1)
	_, found := patchAnnotation[getPVCVolumeHealthAnnotationKey(remoteNodeUID)]
	assert.False(t, found)

	volumeConditionAnnotationJSON, ok := patchAnnotation[getPVCVolumeHealthAnnotationKey(localNodeUID)].(string)
	if !assert.True(t, ok) {
		return
	}

	volumeConditionAnnotation := pvcVolumeHealthAnnotation{}
	err = json.Unmarshal([]byte(volumeConditionAnnotationJSON), &volumeConditionAnnotation)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, pvcVolumeHealthAnnotationStateUnhealthy, volumeConditionAnnotation.State)
	assert.Equal(t, "node-a", volumeConditionAnnotation.Node)
	// Since the state is changed, the "since" timestamp should be updated.
	assert.Equal(t, volumeConditionAnnotation.LastChecked, volumeConditionAnnotation.Since)
	_, err = time.Parse(time.RFC3339, volumeConditionAnnotation.LastChecked)
	assert.NoError(t, err)
}

func TestBuildPVCVolumeHealthAnnotationPatchPreservesSinceWhenStateUnchanged(t *testing.T) {
	nodeUID := "35d4f4a8-0000-1111-2222-2e9fdf21e086"

	patch, err := buildPVCVolumeHealthAnnotationPatch(map[string]string{
		getPVCVolumeHealthAnnotationKey(nodeUID): `{"state":"unhealthy","lastChecked":"2026-06-23T05:17:00Z","since":"2026-06-23T04:52:00Z","node":"node-a"}`,
	}, nodeUID, "node-a", false)
	if !assert.NoError(t, err) {
		return
	}

	patchObj := map[string]interface{}{}
	err = json.Unmarshal(patch, &patchObj)
	if !assert.NoError(t, err) {
		return
	}

	patchMetadata, ok := patchObj["metadata"].(map[string]interface{})
	if !assert.True(t, ok) {
		return
	}

	patchAnnotation, ok := patchMetadata["annotations"].(map[string]interface{})
	if !assert.True(t, ok) {
		return
	}

	volumeConditionAnnotationJSON, ok := patchAnnotation[getPVCVolumeHealthAnnotationKey(nodeUID)].(string)
	if !assert.True(t, ok) {
		return
	}

	volumeConditionAnnotation := pvcVolumeHealthAnnotation{}
	err = json.Unmarshal([]byte(volumeConditionAnnotationJSON), &volumeConditionAnnotation)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, pvcVolumeHealthAnnotationStateUnhealthy, volumeConditionAnnotation.State)
	assert.Equal(t, "node-a", volumeConditionAnnotation.Node)
	// Since state is unchanged, the original "since" must be preserved.
	assert.Equal(t, "2026-06-23T04:52:00Z", volumeConditionAnnotation.Since)
}

func TestBuildPVCVolumeHealthAnnotationPatchRejectsEmptyNodeUID(t *testing.T) {
	_, err := buildPVCVolumeHealthAnnotationPatch(map[string]string{}, "", "node-a", true)
	assert.Error(t, err)
}
