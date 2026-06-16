package app

import (
	"fmt"
	"io"
	"os"

	"github.com/dhohner/inktrail/internal/diff"
	"github.com/dhohner/inktrail/internal/graph"
	"github.com/dhohner/inktrail/internal/parser"
	"github.com/dhohner/inktrail/internal/report"
)

// Dependencies contains side-effecting operations used by App.
// Tests and alternate frontends can replace these without mutating package globals.
type Dependencies struct {
	HasStagedChanges   func() (bool, error)
	HasUnstagedChanges func() (bool, error)
	InspectDiff        func(diff.Options) (diff.Result, error)
	BuildGraph         func(string) (*graph.Graph, error)
	BuildGitGraph      func(string) (*graph.Graph, error)
	Warnings           io.Writer
}

// DefaultDependencies returns the production git, diff, and graph implementations.
func DefaultDependencies() Dependencies {
	return Dependencies{
		HasStagedChanges:   diff.HasStagedChanges,
		HasUnstagedChanges: diff.HasUnstagedChanges,
		InspectDiff:        diff.Inspect,
		BuildGraph:         graph.Build,
		BuildGitGraph:      graph.BuildGit,
		Warnings:           os.Stderr,
	}
}

// App coordinates diff inspection, graph building, and report writing.
type App struct {
	deps Dependencies
}

// New creates an App. Zero-valued dependency functions are filled with defaults.
func New(deps Dependencies) App {
	defaults := DefaultDependencies()
	if deps.HasStagedChanges == nil {
		deps.HasStagedChanges = defaults.HasStagedChanges
	}
	if deps.HasUnstagedChanges == nil {
		deps.HasUnstagedChanges = defaults.HasUnstagedChanges
	}
	if deps.InspectDiff == nil {
		deps.InspectDiff = defaults.InspectDiff
	}
	if deps.BuildGraph == nil {
		deps.BuildGraph = defaults.BuildGraph
	}
	if deps.BuildGitGraph == nil {
		deps.BuildGitGraph = defaults.BuildGitGraph
	}
	if deps.Warnings == nil {
		deps.Warnings = defaults.Warnings
	}
	return App{deps: deps}
}

// Analyze writes a JSONL impact report for staged changes, one commit, or a commit range.
func (a App) Analyze(commits []string, out io.Writer, fallbackToHead bool) error {
	r, err := a.BuildReport(commits, fallbackToHead)
	if err != nil {
		return err
	}
	return report.WriteJSONL(out, r)
}

// ReportOptions configures report construction.
type ReportOptions struct {
	PathFilter report.FilterOptions
}

// BuildReport builds an impact report for staged changes, one commit, or a commit range.
func (a App) BuildReport(commits []string, fallbackToHead bool) (report.Report, error) {
	return a.BuildReportWithOptions(commits, fallbackToHead, ReportOptions{})
}

// BuildReportWithOptions builds an impact report for staged changes, one commit, or a commit range.
func (a App) BuildReportWithOptions(commits []string, fallbackToHead bool, opts ReportOptions) (report.Report, error) {
	commits, err := a.ResolveCommits(commits, fallbackToHead)
	if err != nil {
		return report.Report{}, err
	}
	if err := opts.PathFilter.Validate(); err != nil {
		return report.Report{}, err
	}
	result, err := a.deps.InspectDiff(diff.Options{Commits: commits})
	if err != nil {
		return report.Report{}, err
	}
	result, _ = report.ApplyPathFilter(result, opts.PathFilter)
	if len(result.Files) == 0 {
		return report.Empty(), nil
	}
	warnUnsupportedLanguageFiles(a.deps.Warnings, result.Files)

	current, base, err := a.buildGraphs(baseRef(commits))
	if err != nil {
		return report.Report{}, err
	}
	return report.BuildFilteredWithBaseOptions(current, base, result, opts.PathFilter), nil
}

// ResolveCommits applies agent-safe fallback semantics to CLI commit arguments.
func (a App) ResolveCommits(commits []string, fallbackToHead bool) ([]string, error) {
	if len(commits) != 0 || !fallbackToHead {
		return commits, nil
	}
	staged, err := a.deps.HasStagedChanges()
	if err != nil {
		return nil, err
	}
	if staged {
		return commits, nil
	}
	unstaged, err := a.deps.HasUnstagedChanges()
	if err != nil {
		return nil, err
	}
	if unstaged {
		return nil, fmt.Errorf("refusing to analyze HEAD fallback with unstaged or untracked changes; stage, commit, stash, or pass an explicit commit/range")
	}
	return []string{"HEAD"}, nil
}

func (a App) buildGraphs(base string) (*graph.Graph, *graph.Graph, error) {
	type result struct {
		graph *graph.Graph
		err   error
	}

	currentCh := make(chan result, 1)
	baseCh := make(chan result, 1)
	go func() {
		g, err := a.deps.BuildGraph(".")
		currentCh <- result{graph: g, err: err}
	}()
	go func() {
		g, err := a.deps.BuildGitGraph(base)
		baseCh <- result{graph: g, err: err}
	}()

	current := <-currentCh
	baseResult := <-baseCh
	if current.err != nil {
		return nil, nil, current.err
	}
	if baseResult.err != nil {
		return nil, nil, baseResult.err
	}
	return current.graph, baseResult.graph, nil
}

func baseRef(args []string) string {
	switch len(args) {
	case 0:
		return "HEAD"
	case 1:
		return args[0] + "^"
	default:
		return args[0]
	}
}

func warnUnsupportedLanguageFiles(w io.Writer, files []diff.FileChange) {
	if w == nil {
		return
	}
	for _, file := range files {
		if file.Test {
			continue
		}
		path := file.Path
		if path == "" {
			path = file.OldPath
		}
		if path == "" {
			continue
		}
		if _, ok := parser.LanguageForPath(path); !ok {
			fmt.Fprintln(w, "inktrail: unsupported changed file languages skipped for symbol and call graph analysis; file records will still be emitted")
			return
		}
	}
}
