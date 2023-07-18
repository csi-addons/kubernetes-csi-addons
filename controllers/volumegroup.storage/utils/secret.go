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

	volumegroupv1 "github.com/csi-addons/kubernetes-csi-addons/apis/volumegroup.storage/v1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getSecretData(client client.Client, logger logr.Logger, name, namespace string) (map[string]string, error) {
	namespacedName := types.NamespacedName{Name: name, Namespace: namespace}
	secret := &corev1.Secret{}
	err := client.Get(context.TODO(), namespacedName, secret)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Error(err, "secret not found", "Secret Name", name, "Secret Namespace", namespace)

			return nil, err
		}
		logger.Error(err, "error getting secret", "Secret Name", name, "Secret Namespace", namespace)

		return nil, err
	}

	return convertMap(secret.Data), nil
}

func convertMap(oldMap map[string][]byte) map[string]string {
	newMap := make(map[string]string)

	for key, val := range oldMap {
		newMap[key] = string(val)
	}

	return newMap
}

func GetSecretDataFromClass(client client.Client, vgClass *volumegroupv1.VolumeGroupClass, logger logr.Logger) (map[string]string, error) {
	secretName, secretNamespace := GetSecretCred(vgClass)
	secret := make(map[string]string)
	var err error
	if secretName != "" && secretNamespace != "" {
		secret, err = getSecretData(client, logger, secretName, secretNamespace)
		if err != nil {
			return nil, err
		}
	}
	return secret, nil
}

func GetSecretCred(vgClass *volumegroupv1.VolumeGroupClass) (string, string) {
	secretName := vgClass.Parameters[PrefixedVGSecretNameKey]
	secretNamespace := vgClass.Parameters[PrefixedVGSecretNamespaceKey]
	return secretName, secretNamespace
}
