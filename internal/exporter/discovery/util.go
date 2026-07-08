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
