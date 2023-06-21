/*
Copyright 2023 The Kubernetes-CSI-Addons Authors.

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

package messages

var (
	MatchingLabelsAndLabelSelectorFailed = "Could not check if labels are matched with labelSelector, got %s"
	FailedToRemovePVCFromVG              = "Could not remove %s/%s persistentVolumeClaim from %s/%s volumeGroup"
	PVDoesNotExist                       = "%s/%s persistentVolume does not exist"
	UnExpectedPVCError                   = "Got an unexpected error while fetching %s/%s PersistentVolumeClaim"
	FailedToRemovePVFromVGC              = "Could not remove %s persistentVolume from %s/%s volumeGroupContent"
	FailedToAddPVCToVG                   = "Could not add %s/%s persistentVolumeClaim to %s/%s volumeGroup"
	FailedToAddPVToVGC                   = "Could not add %s persistentVolume to %s/%s volumeGroupContent"
	FailedToCreateEvent                  = "Failed to create %s/%s event"
	FailedToGetPV                        = "Failed to get %s persistentVolume"
	PVCIsAlreadyBelongToGroup            = "Failed to add %s/%s persistentVolumeClaim to VolumeGroups %v Because it belongs to other VolumeGroups %v"
	PVCMatchedWithMultipleNewGroups      = "Failed to add %s/%s persistentVolumeClaim to VolumeGroups %v Because it matched more than one new VolumeGroups"
	FailedToGetStorageClass              = "Failed to get %s storageClass"
	FailedToListPVC                      = "Failed to list persistentVolumeClaim"
	FailedToGetStorageClassName          = "Failed to get storageClass name from persistentVolumeClaim %s"
	CannotFindMatchingPVCForPV           = "Cannot find matching persistentVolumeClaim for %s persistentVolume"
	FailToRemovePVCObject                = "Fail To remove %s/%s persistentVolumeClaim object"
)
