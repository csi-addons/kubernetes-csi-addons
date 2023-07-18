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
	ReconcileVG                    = "Reconciling VolumeGroup"
	UnableToCreateVGController     = "Unable to create VolumeGroup controller"
	UnableToCreateVGCController    = "Unable to create VolumeGroupContent controller"
	PVCNotFound                    = "%s/%s persistentVolumeClaim not found"
	ListVGs                        = "Listing volumeGroups"
	CheckIfPVCMatchesVG            = "Checking if %s/%s persistentVolumeClaim is matches %s/%s volumeGroup"
	PVCMatchedToVG                 = "%s/%s persistentVolumeClaim is matched with %s/%s volumeGroup"
	PVCNotMatchedToVG              = "%s/%s persistentVolumeClaim is not matched with %s/%s volumeGroup"
	RemovePVCFromVG                = "Removing %s/%s persistentVolumeClaim from %s/%s volumeGroup"
	RemovedPVCFromVG               = "Successfully removed %s/%s persistentVolumeClaim from %s/%s volumeGroup"
	PVCDoesNotHavePV               = "PersistentVolumeClaim does not Have persistentVolume"
	GetPVOfPVC                     = "Get matching persistentVolume from %s/%s persistentVolumeClaim"
	GetVGC                         = "Get %s/%s volumeGroupContent"
	GetVG                          = "Get %s/%s volumeGroup"
	RemovePVFromVGC                = "Removing %s/%s persistentVolume from %s/%s volumeGroupContent"
	RemovedPVFromVGC               = "Successfully removed %s persistentVolume from %s/%s volumeGroupContent"
	FailedToModifyVG               = "Failed to modify %s/%s volumeGroup"
	AddPVCToVG                     = "Adding %s/%s persistentVolumeClaim to %s/%s volumeGroup"
	AddedPVCToVG                   = "Successfully added %s/%s persistentVolumeClaim to %s/%s volumeGroup"
	AddPVToVG                      = "Adding %s persistentVolume to %s/%s volumeGroup"
	AddedPVToVGC                   = "Successfully added %s persistentVolume to %s/%s volumeGroupContent"
	ModifyVG                       = "Modifying %s volumeGroupID with %v volumeIDs"
	ModifiedVG                     = "Successfully modified %s volumeGroupID"
	CreateEventForNamespacedObject = "Creating event for %s/%s %s, with [%s] message"
	EventCreated                   = "Successfully Created  %s/%s event"
	UpdateVGStatus                 = "Updating status of %s/%s volumeGroup"
	GetPVC                         = "Getting %s/%s persistentVolumeClaim"
	GetPV                          = "Getting %s persistentVolume"
	PVCIsNotInBoundPhase           = "PersistentVolumeClaim is not in bound phase, stopping the reconcile, when it will be in bound phase, reconcile will continue"
	StorageClassHasVGParameter     = "StorageClass %s contain parameter volume_group for claim %s/%s. volumegroup feature is not supported"
	ListPVCs                       = "Listing PersistentVolumeClaims"
	VGCreated                      = "Successfully Created  %s/%s volumeGroup"
	VGCCreated                     = "Successfully Created  %s/%s volumeGroupContent"
	RetryUpdateVGStatus            = "Retry update %s/%s volumeGroup status due to conflict error"
	RetryUpdateVGCtStatus          = "Retry update %s/%s volumeGroupContent status due to conflict error"
	RetryUpdateFinalizer           = "Retry update finalizer due to conflict error"
	NonVolumeGroupFinalizers       = "%s/%s have a non-volumegroup finalizers"
	VgIsStillExist                 = "can not delete %s/%s volumeGroupContent because volumeGroup is still exist"
	DeletePVCsUnderVGC             = "Deleting persistentVolumeClaims under %s/%s volumeGroupContent"
	DeletePVC                      = "Deleting %s/%s persistentVolumeClaim"
)
