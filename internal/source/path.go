package source

import (
	"path/filepath"
	"strings"
)

// IsTestPath reports whether path belongs to test-only scope.
func IsTestPath(path string) bool {
	path = filepath.ToSlash(path)
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == "test" || part == "tests" || part == "testdata" {
			return true
		}
	}
	return strings.HasSuffix(path, "_test.go") || isJavaTestPath(path)
}

// IsGoTestPath reports whether path belongs to Go test-only scope.
func IsGoTestPath(path string) bool {
	return IsTestPath(path) && strings.HasSuffix(filepath.ToSlash(path), ".go")
}

// IsJavaTestPath reports whether path belongs to Java test-only scope.
func IsJavaTestPath(path string) bool {
	return isJavaTestPath(filepath.ToSlash(path))
}

func isJavaTestPath(path string) bool {
	if !strings.HasSuffix(path, ".java") {
		return false
	}
	parts := strings.Split(path, "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] == "src" && parts[i+2] == "java" {
			switch parts[i+1] {
			case "test", "integrationTest", "functionalTest", "e2eTest":
				return true
			}
		}
	}
	return false
}
