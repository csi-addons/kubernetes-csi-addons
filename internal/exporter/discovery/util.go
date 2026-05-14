/*
Copyright 2025 The Kubernetes-CSI-Addons Authors.

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

package discovery

import (
	"fmt"
	"io"
	"os"
)

// maxJSONFileSize limits the size of JSON files we read to prevent memory exhaustion.
const maxJSONFileSize = 1 << 20 // 1 MiB

// readFileLimited opens path and reads at most maxBytes bytes.
// It uses io.LimitReader to bound the read without a separate stat call.
// Returns an error if the file exceeds maxBytes.
func readFileLimited(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	// Read maxBytes+1 so we can detect truncation without a second stat.
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("file %s exceeds size limit of %d bytes", path, maxBytes)
	}
	return data, nil
}
