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

package version

import (
	"fmt"
	"runtime"
)

var (
	// GitCommit tell the latest git commit image is built from.
	GitCommit string
	// Version tells the release version.
	Version string
)

func PrintVersion() {
	fmt.Println("Version:", Version)
	fmt.Println("Git Commit:", GitCommit)
	fmt.Println("Go Version:", runtime.Version())
	fmt.Println("Compiler:", runtime.Compiler)
	fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}
