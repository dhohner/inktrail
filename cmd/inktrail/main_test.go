package main

import (
	"bytes"
	"reflect"
	"testing"

	"inktrail/internal/diff"
	"inktrail/internal/graph"
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
	want := "{\"type\":\"summary\",\"files\":0,\"test_files\":0,\"changed_symbols\":0,\"deleted_symbols\":0,\"removed_calls\":0,\"entry_points\":0,\"nodes\":0}\n"
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
