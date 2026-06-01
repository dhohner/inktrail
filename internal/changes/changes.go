package changes

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Line identifies one changed line in a file.
type Line struct {
	Path    string
	LineNo  int
	Content string
}

// Options selects which git diff to inspect.
type Options struct {
	Staged  bool
	Commits []string

	// IncludeFormatting keeps whitespace-only and blank-line changes.
	// Default false: skip formatting-only changes because they are not useful for call-chain analysis.
	IncludeFormatting bool
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

// Detect returns only added/modified lines from git diff output.
func Detect(opts Options) ([]Line, error) {
	args := []string{"diff", "--no-ext-diff", "--unified=0"}
	if !opts.IncludeFormatting {
		args = append(args, "--ignore-all-space", "--ignore-blank-lines")
	}

	if opts.Staged {
		args = append(args, "--staged")
	}

	switch len(opts.Commits) {
	case 0:
		// default: unstaged changes, or staged if opts.Staged
	case 1:
		args = append(args, opts.Commits[0]+"^", opts.Commits[0])
	case 2:
		args = append(args, opts.Commits[0], opts.Commits[1])
	default:
		return nil, fmt.Errorf("expected 0, 1, or 2 commits, got %d", len(opts.Commits))
	}

	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git diff failed: %s", strings.TrimSpace(string(exit.Stderr)))
		}
		return nil, err
	}

	return ParseDiff(out)
}

// ParseDiff extracts only target-side changed lines from unified diff bytes.
func ParseDiff(diff []byte) ([]Line, error) {
	var lines []Line
	var path string
	newLine := 0
	inHunk := false

	s := bufio.NewScanner(bytes.NewReader(diff))
	for s.Scan() {
		text := s.Text()

		if strings.HasPrefix(text, "+++ ") {
			path = strings.TrimPrefix(text, "+++ ")
			path = strings.TrimPrefix(path, "b/")
			if path == "/dev/null" {
				path = ""
			}
			inHunk = false
			continue
		}

		if m := hunkHeader.FindStringSubmatch(text); m != nil {
			start, err := strconv.Atoi(m[2])
			if err != nil {
				return nil, err
			}
			newLine = start
			inHunk = true
			continue
		}

		if !inHunk || path == "" || isTestPath(path) {
			continue
		}

		switch {
		case strings.HasPrefix(text, "+") && !strings.HasPrefix(text, "+++"):
			content := strings.TrimPrefix(text, "+")
			if strings.TrimSpace(content) != "" {
				lines = append(lines, Line{Path: path, LineNo: newLine, Content: content})
			}
			newLine++
		case strings.HasPrefix(text, "-") && !strings.HasPrefix(text, "---"):
			// deleted line: no target-side line number; not part of changed code to analyze now
		default:
			newLine++
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func isTestPath(path string) bool {
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == "test" || part == "tests" || part == "testdata" {
			return true
		}
	}
	return strings.HasSuffix(path, "_test.go")
}
