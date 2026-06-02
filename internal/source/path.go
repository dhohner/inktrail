package source

import (
	"path/filepath"
	"strings"
)

// IsGoTestPath reports whether path belongs to Go test-only scope.
func IsGoTestPath(path string) bool {
	path = filepath.ToSlash(path)
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == "test" || part == "tests" || part == "testdata" {
			return true
		}
	}
	return strings.HasSuffix(path, "_test.go")
}
