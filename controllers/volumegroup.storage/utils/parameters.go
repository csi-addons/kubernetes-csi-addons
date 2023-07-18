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
	"errors"
	"fmt"
	"strings"
)

func FilterPrefixedParameters(prefix string, param map[string]string) map[string]string {
	newParam := map[string]string{}
	for k, v := range param {
		if !strings.HasPrefix(k, prefix) {
			newParam[k] = v
		}
	}

	return newParam
}

func ValidatePrefixedParameters(param map[string]string) error {
	for k, v := range param {
		if strings.HasPrefix(k, VGAsPrefix) {
			switch k {
			case PrefixedVGSecretNameKey:
				if v == "" {
					return errors.New("secret name cannot be empty")
				}
			case PrefixedVGSecretNamespaceKey:
				if v == "" {
					return errors.New("secret namespace cannot be empty")
				}

			default:

				return fmt.Errorf("found unknown parameter key %q with reserved prefix %s", k, VGAsPrefix)
			}
		}
	}

	return nil
}
