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
	"fmt"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
)

func removeByIndexFromPVCList(pvcList []corev1.PersistentVolumeClaim, index int) []corev1.PersistentVolumeClaim {
	return append(pvcList[:index], pvcList[index+1:]...)
}

func removeByIndexFromPVList(pvList []corev1.PersistentVolume,
	index int) []corev1.PersistentVolume {
	return append(pvList[:index], pvList[index+1:]...)
}

func GetStringField(object interface{}, fieldName string) string {
	fieldValue := GetObjectField(object, fieldName)
	if !fieldValue.IsValid() {
		return ""
	}
	if fieldValue.Kind() == reflect.String {
		return fieldValue.String()
	}
	if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
		return ""
	}
	return fieldValue.Elem().String()
}

func GetBoolField(object interface{}, fieldName string) bool {
	fieldValue := GetObjectField(object, fieldName)
	if !fieldValue.IsValid() {
		return false
	}
	if fieldValue.Kind() == reflect.Ptr && fieldValue.IsNil() {
		return false
	}
	return fieldValue.Elem().Bool()
}

func GetObjectField(object interface{}, fieldName string) reflect.Value {
	objectValue := reflect.ValueOf(object)
	if objectValue.Kind() == reflect.Ptr {
		objectValue = objectValue.Elem()
	}
	if !objectValue.IsValid() {
		return reflect.ValueOf(nil)
	}
	for i := 0; i < objectValue.NumField(); i++ {
		fieldValue := objectValue.Field(i)
		fieldType := objectValue.Type().Field(i)
		if fieldType.Name == fieldName {
			return fieldValue
		}
	}
	return reflect.ValueOf(nil)
}

func GetCurrentTime() *metav1.Time {
	metav1NowTime := metav1.NewTime(time.Now())

	return &metav1NowTime
}

func MakeVGName(prefix string, vgUID string) (string, error) {
	if len(vgUID) == 0 {
		return "", fmt.Errorf("Corrupted volumeGroup object, it is missing UID")
	}
	return fmt.Sprintf("%s-%s", prefix, vgUID), nil
}
