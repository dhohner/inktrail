package diff

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"inktrail/internal/source"
)

// Line identifies one changed line in a file.
type Line struct {
	Path    string
	LineNo  int
	Content string
}

// Hunk identifies changed line ranges without changed content.
type Hunk struct {
	OldStart int `json:"old_start"`
	OldLines int `json:"old_lines"`
	NewStart int `json:"new_start"`
	NewLines int `json:"new_lines"`
}

// FileChange identifies one changed file without changed content.
type FileChange struct {
	Status  string `json:"status"`
	OldPath string `json:"old_path,omitempty"`
	Path    string `json:"path"`
	Test    bool   `json:"test"`
	Hunks   []Hunk `json:"hunks,omitempty"`
}

// Result contains diff-derived review metadata.
type Result struct {
	Lines []Line
	Files []FileChange
}

// Options selects which git diff to inspect.
type Options struct {
	Commits []string

	// IncludeFormatting keeps whitespace-only and blank-line changes.
	// Default false: skip formatting-only changes because they are not useful for call-chain analysis.
	IncludeFormatting bool
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// Inspect returns changed lines plus file/hunk metadata from git diff output.
func Inspect(opts Options) (Result, error) {
	out, err := gitDiff(opts)
	if err != nil {
		return Result{}, err
	}
	return Parse(out)
}

// Detect returns only added/modified non-test code lines from git diff output.
func Detect(opts Options) ([]Line, error) {
	result, err := Inspect(opts)
	return result.Lines, err
}

func gitDiff(opts Options) ([]byte, error) {
	args := []string{"diff", "--no-ext-diff", "--unified=0"}
	if !opts.IncludeFormatting {
		args = append(args, "--ignore-all-space", "--ignore-blank-lines")
	}

	switch len(opts.Commits) {
	case 0:
		// default: staged changes only; unstaged changes are intentionally ignored
		args = append(args, "--staged")
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
	return out, nil
}

// Parse extracts target-side changed lines and content-free file/hunk metadata from unified diff bytes.
func Parse(raw []byte) (Result, error) {
	lines, err := ParseDiff(raw)
	if err != nil {
		return Result{}, err
	}
	files, err := ParseFiles(raw)
	if err != nil {
		return Result{}, err
	}
	return Result{Lines: lines, Files: files}, nil
}

// ParseDiff extracts only target-side changed, non-test code lines from unified diff bytes.
func ParseDiff(raw []byte) ([]Line, error) {
	var lines []Line
	var path string
	newLine := 0
	inHunk := false

	s := bufio.NewScanner(bytes.NewReader(raw))
	for s.Scan() {
		text := s.Text()

		if strings.HasPrefix(text, "+++ ") {
			path = cleanDiffPath(strings.TrimPrefix(text, "+++ "))
			if path == "/dev/null" {
				path = ""
			}
			inHunk = false
			continue
		}

		if m := hunkHeader.FindStringSubmatch(text); m != nil {
			start, err := strconv.Atoi(m[3])
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

// ParseFiles extracts file status and hunk ranges without changed content.
func ParseFiles(raw []byte) ([]FileChange, error) {
	var files []FileChange
	current := -1

	s := bufio.NewScanner(bytes.NewReader(raw))
	for s.Scan() {
		text := s.Text()
		switch {
		case strings.HasPrefix(text, "diff --git "):
			oldPath, path := parseDiffGitPaths(text)
			files = append(files, FileChange{Status: "modified", OldPath: oldPath, Path: path, Test: isTestPath(path)})
			current = len(files) - 1
		case current >= 0 && strings.HasPrefix(text, "new file mode "):
			files[current].Status = "added"
		case current >= 0 && strings.HasPrefix(text, "deleted file mode "):
			files[current].Status = "deleted"
		case current >= 0 && strings.HasPrefix(text, "rename from "):
			files[current].Status = "renamed"
			files[current].OldPath = strings.TrimPrefix(text, "rename from ")
		case current >= 0 && strings.HasPrefix(text, "rename to "):
			files[current].Path = strings.TrimPrefix(text, "rename to ")
			files[current].Test = isTestPath(files[current].Path)
		case current >= 0 && strings.HasPrefix(text, "+++ "):
			path := cleanDiffPath(strings.TrimPrefix(text, "+++ "))
			if path != "/dev/null" {
				files[current].Path = path
				files[current].Test = isTestPath(path)
			}
		case current >= 0:
			m := hunkHeader.FindStringSubmatch(text)
			if m == nil {
				continue
			}
			hunk, err := parseHunk(m)
			if err != nil {
				return nil, err
			}
			files[current].Hunks = append(files[current].Hunks, hunk)
		}
	}
	return files, s.Err()
}

func parseHunk(m []string) (Hunk, error) {
	oldStart, err := strconv.Atoi(m[1])
	if err != nil {
		return Hunk{}, err
	}
	oldLines := parseCount(m[2])
	newStart, err := strconv.Atoi(m[3])
	if err != nil {
		return Hunk{}, err
	}
	return Hunk{OldStart: oldStart, OldLines: oldLines, NewStart: newStart, NewLines: parseCount(m[4])}, nil
}

func parseCount(raw string) int {
	if raw == "" {
		return 1
	}
	count, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return count
}

func parseDiffGitPaths(line string) (string, string) {
	parts := strings.Split(line, " ")
	if len(parts) < 4 {
		return "", ""
	}
	return cleanDiffPath(parts[2]), cleanDiffPath(parts[3])
}

func cleanDiffPath(path string) string {
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	return path
}

func isTestPath(path string) bool {
	return source.IsGoTestPath(path)
}
