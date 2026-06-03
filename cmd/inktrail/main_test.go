package main

import "testing"

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
