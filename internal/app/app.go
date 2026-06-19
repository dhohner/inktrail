package app

import (
	"fmt"
	"io"
	"strings"

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
}

// DefaultDependencies returns the production git, diff, and graph implementations.
func DefaultDependencies() Dependencies {
	return Dependencies{
		HasStagedChanges:   diff.HasStagedChanges,
		HasUnstagedChanges: diff.HasUnstagedChanges,
		InspectDiff:        diff.Inspect,
		BuildGraph:         graph.Build,
		BuildGitGraph:      graph.BuildGit,
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
	Size       report.SizeOptions
	BestEffort bool
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
	if err := opts.Size.Validate(); err != nil {
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
	unsupportedWarnings := unsupportedLanguageWarnings(result.Files)
	if len(unsupportedWarnings) > 0 && !opts.BestEffort {
		return report.Report{}, fmt.Errorf("unsupported production language in %s (use --best-effort to emit partial report with warnings)", unsupportedWarnings[0].Path)
	}

	current, base, graphWarnings, err := a.buildGraphs(baseRef(commits), opts.BestEffort)
	if err != nil {
		return report.Report{}, err
	}
	r := report.BuildFilteredWithBaseOptions(current, base, result, opts.PathFilter)
	r.Warnings = append(r.Warnings, unsupportedWarnings...)
	r.Warnings = append(r.Warnings, graphWarnings...)
	return r, nil
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

type graphBuildResult struct {
	graph *graph.Graph
	err   error
	which string
}

func (a App) buildGraphs(base string, bestEffort bool) (*graph.Graph, *graph.Graph, []report.Warning, error) {
	currentCh := make(chan graphBuildResult, 1)
	baseCh := make(chan graphBuildResult, 1)
	go func() {
		g, err := a.deps.BuildGraph(".")
		currentCh <- graphBuildResult{graph: g, err: err, which: "current"}
	}()
	go func() {
		g, err := a.deps.BuildGitGraph(base)
		baseCh <- graphBuildResult{graph: g, err: err, which: "base"}
	}()

	current := <-currentCh
	baseResult := <-baseCh
	if !bestEffort {
		if current.err != nil {
			return nil, nil, nil, current.err
		}
		if baseResult.err != nil {
			return nil, nil, nil, baseResult.err
		}
		return current.graph, baseResult.graph, nil, nil
	}
	warnings := graphBuildWarnings(current, baseResult)
	if current.err != nil || current.graph == nil {
		// Without a current graph, old/current comparisons would otherwise make
		// every base symbol look deleted. Keep only file-level data in this case.
		current.graph = emptyAnalysisGraph()
		baseResult.graph = emptyAnalysisGraph()
		return current.graph, baseResult.graph, warnings, nil
	}
	if baseResult.err != nil || baseResult.graph == nil {
		baseResult.graph = emptyAnalysisGraph()
	}
	return current.graph, baseResult.graph, warnings, nil
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

func unsupportedLanguageWarnings(files []diff.FileChange) []report.Warning {
	var warnings []report.Warning
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
			warnings = append(warnings, report.Warning{Code: "unsupported_language", Path: path, Message: "unsupported production language skipped for symbol and call graph analysis"})
		}
	}
	return warnings
}

func graphBuildWarnings(results ...graphBuildResult) []report.Warning {
	var warnings []report.Warning
	for _, result := range results {
		if result.err == nil {
			continue
		}
		warnings = append(warnings, report.Warning{Code: graphBuildWarningCode(result.err), Path: graphBuildWarningPath(result.err), Message: fmt.Sprintf("%s graph build failed: %v", result.which, result.err)})
	}
	return warnings
}

func graphBuildWarningCode(err error) string {
	if strings.HasPrefix(err.Error(), "parse ") || strings.Contains(err.Error(), ": syntax error") {
		return "parse_error"
	}
	return "graph_build_failed"
}

func graphBuildWarningPath(err error) string {
	msg := err.Error()
	if strings.HasPrefix(msg, "parse ") {
		rest := strings.TrimPrefix(msg, "parse ")
		if idx := strings.Index(rest, ":"); idx >= 0 {
			return rest[:idx]
		}
	}
	return ""
}

func emptyAnalysisGraph() *graph.Graph {
	return &graph.Graph{Functions: map[string]graph.Function{}, Calls: map[string]map[string]bool{}, Callers: map[string]map[string]bool{}, CallSites: map[string]map[string]graph.CallSite{}}
}
