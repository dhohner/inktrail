package report

import (
	"fmt"
	"path"
	"strings"

	"github.com/dhohner/inktrail/internal/diff"
)

// FilterOptions narrows changed paths before report records are emitted.
type FilterOptions struct {
	Include       string
	Excludes      []string
	ExcludeVendor bool
	ChangedOnly   bool
}

func (o FilterOptions) active() bool {
	return o.Include != "" || len(o.Excludes) != 0 || o.ExcludeVendor || o.ChangedOnly
}

// Validate reports malformed glob filters before analysis starts.
func (o FilterOptions) Validate() error {
	if o.Include != "" {
		if err := validateGlobPattern(o.Include); err != nil {
			return fmt.Errorf("invalid --include pattern %q: %w", o.Include, err)
		}
	}
	for _, exclude := range o.Excludes {
		if err := validateGlobPattern(exclude); err != nil {
			return fmt.Errorf("invalid --exclude pattern %q: %w", exclude, err)
		}
	}
	return nil
}

// PathSet stores normalized report paths allowed by the active path filter.
type PathSet struct {
	paths map[string]bool
}

func newPathSet() PathSet {
	return PathSet{paths: map[string]bool{}}
}

func (s PathSet) Add(path string) {
	if path = cleanReportPath(path); path != "" {
		s.paths[path] = true
	}
}

func (s PathSet) AddFile(file diff.FileChange) {
	for _, p := range fileFilterPaths(file) {
		s.Add(p)
	}
}

func (s PathSet) Contains(path string) bool {
	return s.paths[cleanReportPath(path)]
}

// ApplyPathFilter narrows diff records to changed files that pass opts.
func ApplyPathFilter(result diff.Result, opts FilterOptions) (diff.Result, PathSet) {
	if !opts.active() {
		paths := newPathSet()
		for _, file := range result.Files {
			paths.AddFile(file)
		}
		return result, paths
	}

	allowed := newPathSet()
	files := make([]diff.FileChange, 0, len(result.Files))
	for _, file := range result.Files {
		if includeFile(file, opts) {
			files = append(files, file)
			allowed.AddFile(file)
		}
	}

	lines := make([]diff.Line, 0, len(result.Lines))
	for _, line := range result.Lines {
		if allowed.Contains(line.Path) {
			lines = append(lines, line)
		}
	}
	return diff.Result{Lines: lines, Files: files}, allowed
}

func filterResult(result diff.Result, opts FilterOptions) (diff.Result, PathSet) {
	return ApplyPathFilter(result, opts)
}

func includeFile(file diff.FileChange, opts FilterOptions) bool {
	paths := fileFilterPaths(file)
	if len(paths) == 0 {
		return false
	}
	if opts.Include != "" {
		matched := false
		for _, p := range paths {
			if globMatch(opts.Include, p) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if opts.ExcludeVendor {
		for _, p := range paths {
			if isVendorPath(p) {
				return false
			}
		}
	}
	for _, exclude := range opts.Excludes {
		for _, p := range paths {
			if globMatch(exclude, p) {
				return false
			}
		}
	}
	return true
}

func fileFilterPaths(file diff.FileChange) []string {
	seen := map[string]bool{}
	var paths []string
	for _, p := range []string{file.Path, file.OldPath} {
		p = cleanReportPath(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		paths = append(paths, p)
	}
	return paths
}

func cleanReportPath(p string) string {
	p = strings.TrimPrefix(p, "a/")
	p = strings.TrimPrefix(p, "b/")
	return strings.Trim(strings.ReplaceAll(p, "\\", "/"), "/")
}

func globMatch(pattern, name string) bool {
	pattern = cleanReportPath(pattern)
	name = cleanReportPath(name)
	if pattern == "" {
		return name == ""
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func validateGlobPattern(pattern string) error {
	pattern = cleanReportPath(pattern)
	if pattern == "" {
		return fmt.Errorf("empty pattern")
	}
	for _, segment := range strings.Split(pattern, "/") {
		if segment == "**" {
			continue
		}
		if _, err := path.Match(segment, ""); err != nil {
			return err
		}
	}
	return nil
}

func matchSegments(pattern, name []string) bool {
	if len(pattern) == 0 {
		return len(name) == 0
	}
	if pattern[0] == "**" {
		if matchSegments(pattern[1:], name) {
			return true
		}
		for i := range name {
			if matchSegments(pattern[1:], name[i+1:]) {
				return true
			}
		}
		return false
	}
	if len(name) == 0 {
		return false
	}
	ok, err := path.Match(pattern[0], name[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(pattern[1:], name[1:])
}
