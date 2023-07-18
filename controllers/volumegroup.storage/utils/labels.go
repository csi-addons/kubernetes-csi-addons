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
	"github.com/csi-addons/kubernetes-csi-addons/controllers/volumegroup.storage/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func areLabelsMatchLabelSelector(labelsToCheck map[string]string, labelSelector metav1.LabelSelector) (bool, error) {
	selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil {
		return false, &errors.MatchingLabelsAndLabelSelectorError{ErrorMessage: err.Error()}
	}
	return isSelectorMatchesLabels(selector, labelsToCheck), nil
}

func isSelectorMatchesLabels(selector labels.Selector, labelsToCheck map[string]string) bool {
	set := labels.Set(labelsToCheck)
	return selector.Matches(set)
}
