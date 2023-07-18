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

package utils

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	volumegroupv1 "github.com/csi-addons/kubernetes-csi-addons/apis/volumegroup.storage/v1"
)

func GetVGCObjectFromVG(vgName, Namespace string, vgObject runtimeclient.Object,
	client runtimeclient.Client) (*volumegroupv1.VolumeGroupContent, error) {
	if err := GetNamespacedResourceObject(vgName, Namespace, vgObject, client); err != nil {
		return nil, err
	}
	vgcName := GetVGCName(vgObject.GetUID())
	vgcObj := &volumegroupv1.VolumeGroupContent{}
	err := GetNamespacedResourceObject(vgcName, Namespace, vgcObj, client)
	return vgcObj, err
}

func GetNamespacedResourceObject(name, namespace string, obj runtimeclient.Object, client runtimeclient.Client) error {
	objNamespacedName := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	err := client.Get(context.Background(), objNamespacedName, obj)
	return err
}

func GetVGCName(vgUID types.UID) string {
	return fmt.Sprintf("volumegroup-%s", vgUID)
}

func CreateResourceObject(obj runtimeclient.Object, client runtimeclient.Client) error {
	obj.SetResourceVersion("")
	return client.Create(context.Background(), obj)
}
