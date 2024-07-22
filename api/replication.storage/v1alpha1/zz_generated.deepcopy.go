//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeGroupReplication) DeepCopyInto(out *VolumeGroupReplication) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeGroupReplication.
func (in *VolumeGroupReplication) DeepCopy() *VolumeGroupReplication {
	if in == nil {
		return nil
	}
	out := new(VolumeGroupReplication)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeGroupReplication) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeGroupReplicationClass) DeepCopyInto(out *VolumeGroupReplicationClass) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeGroupReplicationClass.
func (in *VolumeGroupReplicationClass) DeepCopy() *VolumeGroupReplicationClass {
	if in == nil {
		return nil
	}
	out := new(VolumeGroupReplicationClass)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeGroupReplicationClass) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeGroupReplicationClassList) DeepCopyInto(out *VolumeGroupReplicationClassList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VolumeGroupReplicationClass, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeGroupReplicationClassList.
func (in *VolumeGroupReplicationClassList) DeepCopy() *VolumeGroupReplicationClassList {
	if in == nil {
		return nil
	}
	out := new(VolumeGroupReplicationClassList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeGroupReplicationClassList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeGroupReplicationClassSpec) DeepCopyInto(out *VolumeGroupReplicationClassSpec) {
	*out = *in
	if in.Parameters != nil {
		in, out := &in.Parameters, &out.Parameters
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeGroupReplicationClassSpec.
func (in *VolumeGroupReplicationClassSpec) DeepCopy() *VolumeGroupReplicationClassSpec {
	if in == nil {
		return nil
	}
	out := new(VolumeGroupReplicationClassSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeGroupReplicationClassStatus) DeepCopyInto(out *VolumeGroupReplicationClassStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeGroupReplicationClassStatus.
func (in *VolumeGroupReplicationClassStatus) DeepCopy() *VolumeGroupReplicationClassStatus {
	if in == nil {
		return nil
	}
	out := new(VolumeGroupReplicationClassStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeGroupReplicationContent) DeepCopyInto(out *VolumeGroupReplicationContent) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeGroupReplicationContent.
func (in *VolumeGroupReplicationContent) DeepCopy() *VolumeGroupReplicationContent {
	if in == nil {
		return nil
	}
	out := new(VolumeGroupReplicationContent)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeGroupReplicationContent) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeGroupReplicationContentList) DeepCopyInto(out *VolumeGroupReplicationContentList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VolumeGroupReplicationContent, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeGroupReplicationContentList.
func (in *VolumeGroupReplicationContentList) DeepCopy() *VolumeGroupReplicationContentList {
	if in == nil {
		return nil
	}
	out := new(VolumeGroupReplicationContentList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeGroupReplicationContentList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeGroupReplicationContentSource) DeepCopyInto(out *VolumeGroupReplicationContentSource) {
	*out = *in
	if in.VolumeHandles != nil {
		in, out := &in.VolumeHandles, &out.VolumeHandles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeGroupReplicationContentSource.
func (in *VolumeGroupReplicationContentSource) DeepCopy() *VolumeGroupReplicationContentSource {
	if in == nil {
		return nil
	}
	out := new(VolumeGroupReplicationContentSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeGroupReplicationContentSpec) DeepCopyInto(out *VolumeGroupReplicationContentSpec) {
	*out = *in
	out.VolumeGroupReplicationRef = in.VolumeGroupReplicationRef
	in.Source.DeepCopyInto(&out.Source)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeGroupReplicationContentSpec.
func (in *VolumeGroupReplicationContentSpec) DeepCopy() *VolumeGroupReplicationContentSpec {
	if in == nil {
		return nil
	}
	out := new(VolumeGroupReplicationContentSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeGroupReplicationContentStatus) DeepCopyInto(out *VolumeGroupReplicationContentStatus) {
	*out = *in
	if in.PersistentVolumeRefList != nil {
		in, out := &in.PersistentVolumeRefList, &out.PersistentVolumeRefList
		*out = make([]corev1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeGroupReplicationContentStatus.
func (in *VolumeGroupReplicationContentStatus) DeepCopy() *VolumeGroupReplicationContentStatus {
	if in == nil {
		return nil
	}
	out := new(VolumeGroupReplicationContentStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeGroupReplicationList) DeepCopyInto(out *VolumeGroupReplicationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VolumeGroupReplication, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeGroupReplicationList.
func (in *VolumeGroupReplicationList) DeepCopy() *VolumeGroupReplicationList {
	if in == nil {
		return nil
	}
	out := new(VolumeGroupReplicationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeGroupReplicationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeGroupReplicationSource) DeepCopyInto(out *VolumeGroupReplicationSource) {
	*out = *in
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeGroupReplicationSource.
func (in *VolumeGroupReplicationSource) DeepCopy() *VolumeGroupReplicationSource {
	if in == nil {
		return nil
	}
	out := new(VolumeGroupReplicationSource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeGroupReplicationSpec) DeepCopyInto(out *VolumeGroupReplicationSpec) {
	*out = *in
	in.Source.DeepCopyInto(&out.Source)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeGroupReplicationSpec.
func (in *VolumeGroupReplicationSpec) DeepCopy() *VolumeGroupReplicationSpec {
	if in == nil {
		return nil
	}
	out := new(VolumeGroupReplicationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeGroupReplicationStatus) DeepCopyInto(out *VolumeGroupReplicationStatus) {
	*out = *in
	in.VolumeReplicationStatus.DeepCopyInto(&out.VolumeReplicationStatus)
	if in.PersistentVolumeClaimsRefList != nil {
		in, out := &in.PersistentVolumeClaimsRefList, &out.PersistentVolumeClaimsRefList
		*out = make([]corev1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeGroupReplicationStatus.
func (in *VolumeGroupReplicationStatus) DeepCopy() *VolumeGroupReplicationStatus {
	if in == nil {
		return nil
	}
	out := new(VolumeGroupReplicationStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeReplication) DeepCopyInto(out *VolumeReplication) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeReplication.
func (in *VolumeReplication) DeepCopy() *VolumeReplication {
	if in == nil {
		return nil
	}
	out := new(VolumeReplication)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeReplication) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeReplicationClass) DeepCopyInto(out *VolumeReplicationClass) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeReplicationClass.
func (in *VolumeReplicationClass) DeepCopy() *VolumeReplicationClass {
	if in == nil {
		return nil
	}
	out := new(VolumeReplicationClass)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeReplicationClass) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeReplicationClassList) DeepCopyInto(out *VolumeReplicationClassList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VolumeReplicationClass, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeReplicationClassList.
func (in *VolumeReplicationClassList) DeepCopy() *VolumeReplicationClassList {
	if in == nil {
		return nil
	}
	out := new(VolumeReplicationClassList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeReplicationClassList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeReplicationClassSpec) DeepCopyInto(out *VolumeReplicationClassSpec) {
	*out = *in
	if in.Parameters != nil {
		in, out := &in.Parameters, &out.Parameters
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeReplicationClassSpec.
func (in *VolumeReplicationClassSpec) DeepCopy() *VolumeReplicationClassSpec {
	if in == nil {
		return nil
	}
	out := new(VolumeReplicationClassSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeReplicationClassStatus) DeepCopyInto(out *VolumeReplicationClassStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeReplicationClassStatus.
func (in *VolumeReplicationClassStatus) DeepCopy() *VolumeReplicationClassStatus {
	if in == nil {
		return nil
	}
	out := new(VolumeReplicationClassStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeReplicationList) DeepCopyInto(out *VolumeReplicationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VolumeReplication, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeReplicationList.
func (in *VolumeReplicationList) DeepCopy() *VolumeReplicationList {
	if in == nil {
		return nil
	}
	out := new(VolumeReplicationList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VolumeReplicationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeReplicationSpec) DeepCopyInto(out *VolumeReplicationSpec) {
	*out = *in
	in.DataSource.DeepCopyInto(&out.DataSource)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeReplicationSpec.
func (in *VolumeReplicationSpec) DeepCopy() *VolumeReplicationSpec {
	if in == nil {
		return nil
	}
	out := new(VolumeReplicationSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VolumeReplicationStatus) DeepCopyInto(out *VolumeReplicationStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LastStartTime != nil {
		in, out := &in.LastStartTime, &out.LastStartTime
		*out = (*in).DeepCopy()
	}
	if in.LastCompletionTime != nil {
		in, out := &in.LastCompletionTime, &out.LastCompletionTime
		*out = (*in).DeepCopy()
	}
	if in.LastSyncTime != nil {
		in, out := &in.LastSyncTime, &out.LastSyncTime
		*out = (*in).DeepCopy()
	}
	if in.LastSyncBytes != nil {
		in, out := &in.LastSyncBytes, &out.LastSyncBytes
		*out = new(int64)
		**out = **in
	}
	if in.LastSyncDuration != nil {
		in, out := &in.LastSyncDuration, &out.LastSyncDuration
		*out = new(v1.Duration)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VolumeReplicationStatus.
func (in *VolumeReplicationStatus) DeepCopy() *VolumeReplicationStatus {
	if in == nil {
		return nil
	}
	out := new(VolumeReplicationStatus)
	in.DeepCopyInto(out)
	return out
}