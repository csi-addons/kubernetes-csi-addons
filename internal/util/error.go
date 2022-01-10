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

package util

import "google.golang.org/grpc/status"

// GetErrorMessage returns the message from the error if it is a grpc error,
// else returns err.Error().
func GetErrorMessage(err error) string {
	s, ok := status.FromError(err)
	if !ok {
		return err.Error()
	}

	return s.Message()
}
