package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
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
	want := "{\"type\":\"summary\",\"schema_version\":\"1.0\",\"files\":0,\"test_files\":0,\"changed_symbols\":0,\"deleted_symbols\":0,\"moved_symbols\":0,\"removed_calls\":0,\"entry_points\":0,\"nodes\":0,\"context_records\":{\"total\":0,\"declaration_context\":0,\"related_declaration_context\":0}}\n"
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

	got, err := newApp().ResolveCommits(nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want staged mode", got)
	}
}

func TestResolveCommitsFallbackUsesHeadWhenCleanAndNothingStaged(t *testing.T) {
	withChangeDetectors(t, boolFunc(false), boolFunc(false))

	got, err := newApp().ResolveCommits(nil, true)
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

	got, err := newApp().ResolveCommits(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want staged mode", got)
	}
}

func TestResolveCommitsFallbackRefusesDirtyWorktree(t *testing.T) {
	withChangeDetectors(t, boolFunc(false), boolFunc(true))

	_, err := newApp().ResolveCommits(nil, true)
	if err == nil {
		t.Fatal("expected dirty worktree error")
	}
}

func TestResolveCommitsExplicitCommitsBypassFallback(t *testing.T) {
	withChangeDetectors(t, func() (bool, error) {
		t.Fatal("staged detector should not be called for explicit commits")
		return false, nil
	}, nil)

	got, err := newApp().ResolveCommits([]string{"main", "HEAD"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "main" || got[1] != "HEAD" {
		t.Fatalf("got %v, want [main HEAD]", got)
	}
}

func TestRunHelpShowsInvocationFormsWithoutError(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := run([]string{"-h"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("help contaminated stdout: %q", out.String())
	}
	help := errOut.String()
	for _, want := range []string{
		"inktrail [--fallback-to-head]",
		"inktrail <commit>",
		"inktrail <base> <head>",
		"inktrail --base <ref> --head <ref>",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help missing %q: %s", want, help)
		}
	}
	if strings.Contains(help, "flag: help requested") || strings.Contains(help, "Options:") {
		t.Fatalf("help included unwanted flag output: %s", help)
	}
}

func TestRunInvalidFlagWritesErrorAndUsageToStderrOnly(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run([]string{"--no-such-flag"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected invalid flag error")
	}
	if out.Len() != 0 {
		t.Fatalf("invalid flag contaminated stdout: %q", out.String())
	}
	stderr := errOut.String()
	for _, want := range []string{
		"flag provided but not defined: -no-such-flag",
		"Usage of inktrail:",
		"inktrail [--fallback-to-head]",
		"inktrail <commit>",
		"inktrail <base> <head>",
		"inktrail --base <ref> --head <ref>",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q: %s", want, stderr)
		}
	}
	if strings.Contains(stderr, "{\"type\"") {
		t.Fatalf("stderr included report JSONL: %s", stderr)
	}
}

func TestRunVersionWritesPlainTextWithoutAnalysis(t *testing.T) {
	withAnalysisDeps(t, func(opts diff.Options) (diff.Result, error) {
		t.Fatal("version should not inspect diffs")
		return diff.Result{}, nil
	}, func(string) (*graph.Graph, error) {
		t.Fatal("version should not build current graph")
		return nil, nil
	}, func(string) (*graph.Graph, error) {
		t.Fatal("version should not build base graph")
		return nil, nil
	})

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := run([]string{"--version"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", errOut.String())
	}
	got := out.String()
	for _, want := range []string{"inktrail", "schema 1.0", "go,java"} {
		if !strings.Contains(got, want) {
			t.Fatalf("version output missing %q: %s", want, got)
		}
	}
}

func TestRunVersionJSONWritesMetadataObject(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := run([]string{"--version", "--json"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Name               string   `json:"name"`
		Version            string   `json:"version"`
		Commit             string   `json:"commit"`
		SchemaVersion      string   `json:"schema_version"`
		SupportedLanguages []string `json:"supported_languages"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid json version output %q: %v", out.String(), err)
	}
	if got.Name != "inktrail" || got.Version == "" || got.Commit == "" || got.SchemaVersion != "1.0" || !reflect.DeepEqual(got.SupportedLanguages, []string{"go", "java"}) {
		t.Fatalf("unexpected version metadata: %#v", got)
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", errOut.String())
	}
}

func TestRunWritesJSONL(t *testing.T) {
	stubEmptyAnalysis(t)

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := run(nil, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", errOut.String())
	}
	if !strings.HasPrefix(out.String(), "{\"type\":\"summary\"") {
		t.Fatalf("stdout is not clean JSONL: %q", out.String())
	}
}

func TestRunFormatJSONLWritesSameAsDefault(t *testing.T) {
	stubEmptyAnalysis(t)

	var defaultOut bytes.Buffer
	var jsonlOut bytes.Buffer
	var errOut bytes.Buffer
	if err := run(nil, &defaultOut, &errOut); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--format", "jsonl"}, &jsonlOut, &errOut); err != nil {
		t.Fatal(err)
	}
	if defaultOut.String() != jsonlOut.String() {
		t.Fatalf("jsonl output differs from default:\ndefault=%q\njsonl=%q", defaultOut.String(), jsonlOut.String())
	}
}

func TestRunFormatJSONWritesReportObject(t *testing.T) {
	stubEmptyAnalysis(t)

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := run([]string{"--format", "json"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var got struct {
		SchemaVersion string `json:"schema_version"`
		Summary       struct {
			SchemaVersion string `json:"schema_version"`
			Files         int    `json:"files"`
		} `json:"summary"`
		Files []json.RawMessage `json:"files"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid json output %q: %v", out.String(), err)
	}
	if got.SchemaVersion != "1.0" || got.Summary.SchemaVersion != "1.0" || got.Summary.Files != 0 || len(got.Files) != 0 {
		t.Fatalf("unexpected json report: %#v", got)
	}
}

func TestRunInvalidGlobPatternDoesNotWriteStdout(t *testing.T) {
	stubEmptyAnalysis(t)

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := run([]string{"--include", "["}, &out, &errOut)
	if err == nil {
		t.Fatal("expected invalid glob error")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout contaminated: %q", out.String())
	}
	if !strings.Contains(err.Error(), "invalid --include pattern") {
		t.Fatalf("unexpected error: %v", err)
	}
	var reported cliError
	if errors.As(err, &reported) {
		t.Fatalf("invalid glob errors must be printable by main, got cliError: %v", err)
	}
}

func TestRunUnsupportedFormatDoesNotWriteStdout(t *testing.T) {
	stubEmptyAnalysis(t)

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := run([]string{"--format", "xml"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected unsupported format error")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout contaminated: %q", out.String())
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunOutputWritesFileAndLeavesStdoutEmpty(t *testing.T) {
	stubEmptyAnalysis(t)
	path := t.TempDir() + "/report.json"

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := run([]string{"--format", "json", "--output", path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout got report data: %q", out.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) || !strings.Contains(string(data), "\"schema_version\":\"1.0\"") {
		t.Fatalf("unexpected output file: %q", string(data))
	}
}

func TestRunOutputFailureDoesNotWriteStdout(t *testing.T) {
	stubEmptyAnalysis(t)
	path := t.TempDir() + "/missing/report.json"

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := run([]string{"--output", path}, &out, &errOut)
	if err == nil {
		t.Fatal("expected output error")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout contaminated: %q", out.String())
	}
}

func TestRunPathFilteringFlags(t *testing.T) {
	withAnalysisDeps(t, func(opts diff.Options) (diff.Result, error) {
		return diff.Result{Files: []diff.FileChange{
			{Status: "modified", Path: "app.go"},
			{Status: "modified", Path: "vendor/acme/lib.go"},
			{Status: "modified", Path: "app_test.go", Test: true},
		}}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	})

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := run([]string{"--include", "**/*.go", "--exclude", "**/*_test.go", "--exclude-vendor", "--changed-only", "--review-summary", "--format", "json"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Summary struct {
			Files int `json:"files"`
		} `json:"summary"`
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
		ReviewSummary *struct {
			ChangedProductionFiles []struct {
				Path string `json:"path"`
			} `json:"changed_production_files"`
		} `json:"review_summary"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid json output %q: %v", out.String(), err)
	}
	if got.Summary.Files != 1 || len(got.Files) != 1 || got.Files[0].Path != "app.go" || got.ReviewSummary == nil || len(got.ReviewSummary.ChangedProductionFiles) != 1 || got.ReviewSummary.ChangedProductionFiles[0].Path != "app.go" {
		t.Fatalf("unexpected filtered report: %#v", got)
	}
}

func TestRunReviewSummaryFlagEmitsJSONLObject(t *testing.T) {
	withAnalysisDeps(t, func(opts diff.Options) (diff.Result, error) {
		return diff.Result{Files: []diff.FileChange{{Status: "modified", Path: "app.go"}, {Status: "modified", Path: "app_test.go", Test: true}}}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	})

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := run([]string{"--review-summary", "--format", "json"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var got struct {
		ReviewSummary *struct {
			ChangedProductionFiles []struct {
				Path string `json:"path"`
			} `json:"changed_production_files"`
			ChangedTestFiles []struct {
				Path string `json:"path"`
			} `json:"changed_test_files"`
		} `json:"review_summary"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid json output %q: %v", out.String(), err)
	}
	if got.ReviewSummary == nil || len(got.ReviewSummary.ChangedProductionFiles) != 1 || got.ReviewSummary.ChangedProductionFiles[0].Path != "app.go" || len(got.ReviewSummary.ChangedTestFiles) != 1 || got.ReviewSummary.ChangedTestFiles[0].Path != "app_test.go" {
		t.Fatalf("unexpected review summary: %#v", got.ReviewSummary)
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

func TestRunNamedRangeMatchesPositionalRangeSemantics(t *testing.T) {
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
	var errOut bytes.Buffer
	if err := run([]string{"--base", "main", "--head", "feature"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(inspected, []string{"main", "feature"}) {
		t.Fatalf("inspected commits=%v, want [main feature]", inspected)
	}
	if !reflect.DeepEqual(baseRefs, []string{"main"}) {
		t.Fatalf("base refs=%v, want [main]", baseRefs)
	}
}

func TestRunRejectsNamedAndPositionalRevisionMixWithoutStdout(t *testing.T) {
	stubEmptyAnalysis(t)

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := run([]string{"--base", "main", "--head", "feature", "extra"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected ambiguous revision error")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout contaminated: %q", out.String())
	}
	if !strings.Contains(err.Error(), "cannot combine --base/--head with positional") {
		t.Fatalf("unexpected error: %v", err)
	}
	var reported cliError
	if errors.As(err, &reported) {
		t.Fatalf("ambiguous revision errors must be printable by main, got cliError: %v", err)
	}
}

func TestRunRejectsIncompleteNamedRangeWithoutStdout(t *testing.T) {
	stubEmptyAnalysis(t)

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := run([]string{"--base", "main"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected incomplete range error")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout contaminated: %q", out.String())
	}
	if !strings.Contains(err.Error(), "--base and --head must be provided together") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRejectsNamedRevisionFlagAfterPositionalWithoutStdout(t *testing.T) {
	stubEmptyAnalysis(t)

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := run([]string{"main", "--base", "other", "--head", "feature"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected ambiguous revision error")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout contaminated: %q", out.String())
	}
	if !strings.Contains(err.Error(), "cannot combine --base/--head with positional") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunRejectsNegativeSizeLimitWithoutStdout(t *testing.T) {
	stubEmptyAnalysis(t)

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := run([]string{"--max-records", "-1"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected size validation error")
	}
	if out.Len() != 0 {
		t.Fatalf("stdout contaminated: %q", out.String())
	}
	if !strings.Contains(err.Error(), "non-negative") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSizeFlagsAffectReportOutput(t *testing.T) {
	withAnalysisDeps(t, func(opts diff.Options) (diff.Result, error) {
		return diff.Result{Files: []diff.FileChange{{Status: "modified", Path: "app.go", Hunks: []diff.Hunk{{Lines: []diff.HunkLine{{Op: "add", NewLine: 1, Content: "a"}, {Op: "add", NewLine: 2, Content: "b"}}}}}}}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	})

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := run([]string{"--format", "json", "--max-lines-per-hunk", "1", "--max-records", "1"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Summary struct {
			Omissions []struct {
				Reason       string `json:"reason"`
				EmittedCount int    `json:"emitted_count"`
			} `json:"omissions"`
		} `json:"summary"`
		Files []struct {
			OmittedLines int `json:"omitted_lines"`
			Hunks        []struct {
				Lines []json.RawMessage `json:"lines"`
			} `json:"hunks"`
		} `json:"files"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid json output %q: %v", out.String(), err)
	}
	if len(got.Files) != 1 || got.Files[0].OmittedLines != 1 || len(got.Files[0].Hunks[0].Lines) != 1 {
		t.Fatalf("hunk limit was not applied: %#v", got.Files)
	}
	var sawHunkOmission bool
	for _, omission := range got.Summary.Omissions {
		if omission.Reason == "max_lines_per_hunk" && omission.EmittedCount == 1 {
			sawHunkOmission = true
		}
	}
	if !sawHunkOmission {
		t.Fatalf("missing hunk omission metadata: %#v", got.Summary.Omissions)
	}
}

func boolFunc(v bool) func() (bool, error) {
	return func() (bool, error) { return v, nil }
}

func stubEmptyAnalysis(t *testing.T) {
	t.Helper()
	withChangeDetectors(t, boolFunc(true), nil)
	withAnalysisDeps(t, func(opts diff.Options) (diff.Result, error) {
		return diff.Result{}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	}, func(string) (*graph.Graph, error) {
		return &graph.Graph{}, nil
	})
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
