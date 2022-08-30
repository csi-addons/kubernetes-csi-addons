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

import (
	"errors"
	"fmt"
	"net"
	"strconv"
)

var ErrInvalidEndpoint = errors.New("invalid endpoint")

// ValidateControllerEndpoint validates given ip address and port,
// returning valid endpoint containing both ip address and port.
func ValidateControllerEndpoint(rawIP, rawPort string) (string, error) {
	ip := net.ParseIP(rawIP)
	if ip == nil {
		return "", fmt.Errorf("%w: invalid controller ip address %q", ErrInvalidEndpoint, rawIP)
	}

	port, err := parsePort(rawPort)
	if err != nil {
		return "", err
	}

	if ip.To4() != nil {
		return fmt.Sprintf("%s:%d", ip.String(), port), nil
	}

	return fmt.Sprintf("[%s]:%d", ip.String(), port), nil
}

// BuildEndpointURL returns a URL formatted address of the pod that has this
// sidecar running.
// The format of the URL depends on the arguments passed to this function, it
// will return either
//   - a ValidateControllerEndpoint() if rawIP is set
//   - pod://<pod>:<port> if pod is set, and rawIP and namespace are not set
//   - pod://<pod>.<namespace>:<port> if pod, namespace are set, and rawIP not
func BuildEndpointURL(rawIP, rawPort, pod, namespace string) (string, error) {
	if rawIP != "" {
		return ValidateControllerEndpoint(rawIP, rawPort)
	}

	if pod == "" && namespace == "" {
		return "", fmt.Errorf("%w: missing IP-address or Pod and Namespace", ErrInvalidEndpoint)
	}

	port, err := parsePort(rawPort)
	if err != nil {
		return "", err
	}

	endpoint := "pod://" + pod

	// namespace is optional
	if namespace != "" {
		endpoint += "." + namespace
	}

	return endpoint + fmt.Sprintf(":%d", port), nil
}

// parsePort takes a network port as string and converts that to an integer.
func parsePort(rawPort string) (uint64, error) {
	port, err := strconv.ParseUint(rawPort, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid controller port %q: %v", ErrInvalidEndpoint, rawPort, err)
	}

	return port, nil
}
