package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dhohner/inktrail/internal/diff"
	"github.com/dhohner/inktrail/internal/graph"
)

func TestAnalyzeEmptyDiffSkipsGraphBuilds(t *testing.T) {
	withAnalysisDeps(t, func(opts diff.Options) (diff.Result, error) {
		if len(opts.Commits) != 0 {
			t.Fatalf("commits=%v, want staged mode", opts.Commits)
		}
		return diff.Result{}, nil
	}, func(string) (*graph.Graph, error) {
		t.Fatal("current graph should not be built for an empty diff")
		return nil, nil
	}, func(string) (*graph.Graph, error) {
		t.Fatal("base graph should not be built for an empty diff")
		return nil, nil
	})

	var out bytes.Buffer
	if err := analyze(nil, &out, false); err != nil {
		t.Fatal(err)
	}
	want := "{\"type\":\"summary\",\"files\":0,\"test_files\":0,\"changed_symbols\":0,\"deleted_symbols\":0,\"moved_symbols\":0,\"removed_calls\":0,\"entry_points\":0,\"nodes\":0,\"context_records\":{\"total\":0,\"declaration_context\":0,\"related_declaration_context\":0}}\n"
	if got := out.String(); got != want {
		t.Fatalf("output=%q want=%q", got, want)
	}
}

func TestAnalyzeBuildsCurrentAndBaseGraphsForNonEmptyDiff(t *testing.T) {
	var currentRoots []string
	var baseRefs []string
	withAnalysisDeps(t, func(opts diff.Options) (diff.Result, error) {
		if !reflect.DeepEqual(opts.Commits, []string{"HEAD"}) {
			t.Fatalf("commits=%v, want [HEAD]", opts.Commits)
		}
		return diff.Result{Files: []diff.FileChange{{Status: "modified", Path: "app.go"}}}, nil
	}, func(root string) (*graph.Graph, error) {
		currentRoots = append(currentRoots, root)
		return &graph.Graph{}, nil
	}, func(ref string) (*graph.Graph, error) {
		baseRefs = append(baseRefs, ref)
		return &graph.Graph{}, nil
	})

	var out bytes.Buffer
	if err := analyze([]string{"HEAD"}, &out, false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(currentRoots, []string{"."}) {
		t.Fatalf("current roots=%v, want [.] ", currentRoots)
	}
	if !reflect.DeepEqual(baseRefs, []string{"HEAD^"}) {
		t.Fatalf("base refs=%v, want [HEAD^]", baseRefs)
	}
}

func TestResolveCommitsNoFallbackKeepsStagedMode(t *testing.T) {
	withChangeDetectors(t, func() (bool, error) {
		t.Fatal("staged detector should not be called without fallback")
		return false, nil
	}, nil)

	got, err := resolveCommits(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want staged mode", got)
	}
}

func TestResolveCommitsFallbackUsesHeadWhenCleanAndNothingStaged(t *testing.T) {
	withChangeDetectors(t, boolFunc(false), boolFunc(false))

	got, err := resolveCommits(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "HEAD" {
		t.Fatalf("got %v, want [HEAD]", got)
	}
}

func TestResolveCommitsFallbackKeepsStagedModeWhenStaged(t *testing.T) {
	withChangeDetectors(t, boolFunc(true), func() (bool, error) {
		t.Fatal("unstaged detector should not be called when staged changes exist")
		return false, nil
	})

	got, err := resolveCommits(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want staged mode", got)
	}
}

func TestResolveCommitsFallbackRefusesDirtyWorktree(t *testing.T) {
	withChangeDetectors(t, boolFunc(false), boolFunc(true))

	_, err := resolveCommits(nil, true)
	if err == nil {
		t.Fatal("expected dirty worktree error")
	}
}

func TestResolveCommitsExplicitCommitsBypassFallback(t *testing.T) {
	withChangeDetectors(t, func() (bool, error) {
		t.Fatal("staged detector should not be called for explicit commits")
		return false, nil
	}, nil)

	got, err := resolveCommits([]string{"main", "HEAD"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "main" || got[1] != "HEAD" {
		t.Fatalf("got %v, want [main HEAD]", got)
	}
}

func TestAnalyzeHTMLWritesTempReportAndAttemptsBrowserOpen(t *testing.T) {
	withAnalysisDeps(t, func(opts diff.Options) (diff.Result, error) {
		return diff.Result{Files: []diff.FileChange{{Status: "modified", Path: "app.go"}}}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	})
	withWorkingDir(t, t.TempDir())

	var opened string
	withBrowser(t, func(path string) error {
		opened = path
		return nil
	})

	var messages bytes.Buffer
	if err := analyzeHTML([]string{"HEAD"}, &messages, false); err != nil {
		t.Fatal(err)
	}
	if opened == "" {
		t.Fatal("browser was not opened")
	}
	if filepath.Dir(opened) != "." || !strings.HasPrefix(filepath.Base(opened), "inktrail-") || filepath.Ext(opened) != ".html" {
		t.Fatalf("report path=%q, want temp html in cwd", opened)
	}
	if _, err := os.Stat(opened); err != nil {
		t.Fatalf("report was not written: %v", err)
	}
	if !strings.Contains(messages.String(), "HTML report written to "+opened) {
		t.Fatalf("report path not visible in messages: %q", messages.String())
	}
}

func TestAnalyzeHTMLBrowserFailureDoesNotFailAnalysis(t *testing.T) {
	withAnalysisDeps(t, func(opts diff.Options) (diff.Result, error) {
		return diff.Result{}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	})
	withWorkingDir(t, t.TempDir())
	withBrowser(t, func(string) error { return errors.New("boom") })

	var messages bytes.Buffer
	if err := analyzeHTML(nil, &messages, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(messages.String(), "could not open browser: boom") {
		t.Fatalf("missing browser failure message: %q", messages.String())
	}
}

func TestRunAgentWritesJSONLAndDoesNotOpenBrowser(t *testing.T) {
	withChangeDetectors(t, boolFunc(true), nil)
	withAnalysisDeps(t, func(opts diff.Options) (diff.Result, error) {
		return diff.Result{}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	})
	withBrowser(t, func(string) error {
		t.Fatal("browser should not open in agent mode")
		return nil
	})

	var out bytes.Buffer
	if err := run([]string{"--agent"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "{\"type\":\"summary\"") {
		t.Fatalf("stdout is not clean JSONL: %q", out.String())
	}
}

func TestRunInteractiveSelectionWritesHumanHTMLReport(t *testing.T) {
	withAnalysisDeps(t, func(opts diff.Options) (diff.Result, error) {
		if !reflect.DeepEqual(opts.Commits, []string{"main", "feature"}) {
			t.Fatalf("commits=%v, want selected range", opts.Commits)
		}
		return diff.Result{}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	})
	withInteractivePrompt(t, []string{"main", "feature"})
	withWorkingDir(t, t.TempDir())
	var opened string
	withBrowser(t, func(path string) error { opened = path; return nil })

	var out bytes.Buffer
	if err := run(nil, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("human run wrote JSONL to stdout: %q", out.String())
	}
	if opened == "" {
		t.Fatal("browser was not opened")
	}
}

func TestAnalyzePassesExplicitRangeAndUsesRangeBase(t *testing.T) {
	var inspected []string
	var baseRefs []string
	withAnalysisDeps(t, func(opts diff.Options) (diff.Result, error) {
		inspected = append([]string(nil), opts.Commits...)
		return diff.Result{Files: []diff.FileChange{{Status: "modified", Path: "app.go"}}}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	}, func(ref string) (*graph.Graph, error) {
		baseRefs = append(baseRefs, ref)
		return &graph.Graph{}, nil
	})

	var out bytes.Buffer
	if err := analyze([]string{"main", "feature"}, &out, true); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(inspected, []string{"main", "feature"}) {
		t.Fatalf("inspected commits=%v, want [main feature]", inspected)
	}
	if !reflect.DeepEqual(baseRefs, []string{"main"}) {
		t.Fatalf("base refs=%v, want [main]", baseRefs)
	}
}

func boolFunc(v bool) func() (bool, error) {
	return func() (bool, error) { return v, nil }
}

func withChangeDetectors(t *testing.T, staged, unstaged func() (bool, error)) {
	t.Helper()
	oldStaged := hasStagedChanges
	oldUnstaged := hasUnstagedChanges
	if staged != nil {
		hasStagedChanges = staged
	}
	if unstaged != nil {
		hasUnstagedChanges = unstaged
	}
	t.Cleanup(func() {
		hasStagedChanges = oldStaged
		hasUnstagedChanges = oldUnstaged
	})
}

func withAnalysisDeps(
	t *testing.T,
	inspect func(diff.Options) (diff.Result, error),
	buildCurrent func(string) (*graph.Graph, error),
	buildBase func(string) (*graph.Graph, error),
) {
	t.Helper()
	oldInspect := inspectDiff
	oldBuild := buildGraph
	oldBuildGit := buildGitGraph
	inspectDiff = inspect
	buildGraph = buildCurrent
	buildGitGraph = buildBase
	t.Cleanup(func() {
		inspectDiff = oldInspect
		buildGraph = oldBuild
		buildGitGraph = oldBuildGit
	})
}

func withBrowser(t *testing.T, open func(string) error) {
	t.Helper()
	oldOpen := openBrowser
	openBrowser = open
	t.Cleanup(func() { openBrowser = oldOpen })
}

func withInteractivePrompt(t *testing.T, selected []string) {
	t.Helper()
	oldTerminal := isTerminal
	oldPrompt := promptAnalysis
	isTerminal = func() bool { return true }
	promptAnalysis = func() ([]string, error) { return selected, nil }
	t.Cleanup(func() {
		isTerminal = oldTerminal
		promptAnalysis = oldPrompt
	})
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatal(err)
		}
	})
}
