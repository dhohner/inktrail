package app

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/dhohner/inktrail/internal/diff"
	"github.com/dhohner/inktrail/internal/graph"
	"github.com/dhohner/inktrail/internal/report"
)

func TestBuildReportFailsByDefaultForUnsupportedChangedLanguages(t *testing.T) {
	app := New(Dependencies{
		InspectDiff: func(diff.Options) (diff.Result, error) {
			return diff.Result{Files: []diff.FileChange{
				{Status: "modified", Path: "cmd/app.go"},
				{Status: "modified", Path: "web/app.ts"},
			}}, nil
		},
		BuildGraph:    emptyGraph,
		BuildGitGraph: emptyGraph,
	})

	_, err := app.BuildReportWithOptions(nil, false, ReportOptions{})
	if err == nil || !strings.Contains(err.Error(), "unsupported production language") {
		t.Fatalf("err=%v, want unsupported production language", err)
	}
}

func TestBestEffortEmitsUnsupportedLanguageWarnings(t *testing.T) {
	app := New(Dependencies{
		InspectDiff: func(diff.Options) (diff.Result, error) {
			return diff.Result{Files: []diff.FileChange{
				{Status: "modified", Path: "cmd/app.go"},
				{Status: "modified", Path: "web/app.ts"},
				{Status: "modified", Path: "docs/readme.md"},
			}}, nil
		},
		BuildGraph:    emptyGraph,
		BuildGitGraph: emptyGraph,
	})

	r, err := app.BuildReportWithOptions(nil, false, ReportOptions{BestEffort: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 2 {
		t.Fatalf("warnings=%#v, want two unsupported language warnings", r.Warnings)
	}
	if r.Warnings[0].Code != "unsupported_language" || r.Warnings[0].Path != "web/app.ts" {
		t.Fatalf("first warning=%#v", r.Warnings[0])
	}
	if r.Summary.Files != 3 {
		t.Fatalf("summary files=%d, want 3", r.Summary.Files)
	}
}

func TestAnalyzeDoesNotWarnForUnsupportedTestOnlyFiles(t *testing.T) {
	var out bytes.Buffer
	app := New(Dependencies{
		InspectDiff: func(diff.Options) (diff.Result, error) {
			return diff.Result{Files: []diff.FileChange{
				{Status: "modified", Path: "tests/fixtures/input.json", Test: true},
				{Status: "modified", Path: "testdata/output.golden", Test: true},
			}}, nil
		},
		BuildGraph:    emptyGraph,
		BuildGitGraph: emptyGraph,
	})

	if err := app.Analyze(nil, &out, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\"type\":\"warning\"") {
		t.Fatalf("warning record in output: %q", out.String())
	}
}

func TestAnalyzeDoesNotWarnForSupportedGoAndJavaFiles(t *testing.T) {
	var out bytes.Buffer
	app := New(Dependencies{
		InspectDiff: func(diff.Options) (diff.Result, error) {
			return diff.Result{Files: []diff.FileChange{
				{Status: "modified", Path: "cmd/app.go"},
				{Status: "modified", Path: "src/main/java/example/App.java"},
			}}, nil
		},
		BuildGraph:    emptyGraph,
		BuildGitGraph: emptyGraph,
	})

	if err := app.Analyze(nil, &out, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\"type\":\"warning\"") {
		t.Fatalf("warning record in output: %q", out.String())
	}
}

func TestBuildReportWithOptionsFiltersBeforeWarningsAndGraphBuilds(t *testing.T) {
	app := New(Dependencies{
		InspectDiff: func(diff.Options) (diff.Result, error) {
			return diff.Result{Files: []diff.FileChange{{Status: "modified", Path: "web/app.ts"}}}, nil
		},
		BuildGraph: func(string) (*graph.Graph, error) {
			t.Fatal("current graph should not be built when filters remove all files")
			return nil, nil
		},
		BuildGitGraph: func(string) (*graph.Graph, error) {
			t.Fatal("base graph should not be built when filters remove all files")
			return nil, nil
		},
	})

	r, err := app.BuildReportWithOptions(nil, false, ReportOptions{PathFilter: report.FilterOptions{Include: "**/*.go"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Summary.Files != 0 {
		t.Fatalf("summary files=%d, want 0", r.Summary.Files)
	}
	if len(r.Warnings) != 0 {
		t.Fatalf("warnings for excluded file: %#v", r.Warnings)
	}
}

func TestBuildReportFailsByDefaultForGraphBuildErrors(t *testing.T) {
	app := New(Dependencies{
		InspectDiff: func(diff.Options) (diff.Result, error) {
			return diff.Result{Files: []diff.FileChange{{Status: "modified", Path: "cmd/app.go"}}}, nil
		},
		BuildGraph: func(string) (*graph.Graph, error) {
			return nil, errors.New("parse cmd/app.go: syntax error")
		},
		BuildGitGraph: emptyGraph,
	})

	_, err := app.BuildReportWithOptions(nil, false, ReportOptions{})
	if err == nil || !strings.Contains(err.Error(), "syntax error") {
		t.Fatalf("err=%v, want syntax error", err)
	}
}

func TestBestEffortEmitsGraphBuildWarnings(t *testing.T) {
	app := New(Dependencies{
		InspectDiff: func(diff.Options) (diff.Result, error) {
			return diff.Result{Files: []diff.FileChange{{Status: "modified", Path: "cmd/app.go"}}}, nil
		},
		BuildGraph: func(string) (*graph.Graph, error) {
			return nil, errors.New("parse cmd/app.go: syntax error")
		},
		BuildGitGraph: func(string) (*graph.Graph, error) {
			return nil, errors.New("index failed")
		},
	})

	r, err := app.BuildReportWithOptions(nil, false, ReportOptions{BestEffort: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 2 {
		t.Fatalf("warnings=%#v, want 2", r.Warnings)
	}
	if r.Warnings[0].Code != "parse_error" || r.Warnings[0].Path != "cmd/app.go" {
		t.Fatalf("parse warning=%#v", r.Warnings[0])
	}
	if r.Warnings[1].Code != "graph_build_failed" {
		t.Fatalf("graph warning=%#v", r.Warnings[1])
	}
	if r.Summary.Files != 1 {
		t.Fatalf("summary files=%d, want 1", r.Summary.Files)
	}
}

func emptyGraph(string) (*graph.Graph, error) {
	return &graph.Graph{Functions: map[string]graph.Function{}, Calls: map[string]map[string]bool{}, Callers: map[string]map[string]bool{}, CallSites: map[string]map[string]graph.CallSite{}}, nil
}
