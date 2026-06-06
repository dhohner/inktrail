package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dhohner/inktrail/internal/diff"
	"github.com/dhohner/inktrail/internal/graph"
)

func TestAnalyzeWarnsOnceForUnsupportedChangedLanguages(t *testing.T) {
	var out bytes.Buffer
	var warnings bytes.Buffer
	app := New(Dependencies{
		InspectDiff: func(diff.Options) (diff.Result, error) {
			return diff.Result{Files: []diff.FileChange{
				{Status: "modified", Path: "cmd/app.go"},
				{Status: "modified", Path: "src/Main.java"},
				{Status: "modified", Path: "web/app.ts"},
				{Status: "modified", Path: "docs/readme.md"},
			}}, nil
		},
		BuildGraph:    emptyGraph,
		BuildGitGraph: emptyGraph,
		Warnings:      &warnings,
	})

	if err := app.Analyze(nil, &out, false); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(warnings.String(), "unsupported changed file languages skipped"); got != 1 {
		t.Fatalf("warning count=%d, warnings=%q", got, warnings.String())
	}
	if strings.Contains(out.String(), "unsupported changed file languages") {
		t.Fatalf("warning leaked into JSONL stdout: %q", out.String())
	}
	if got := strings.Count(out.String(), "\n"); got != 5 {
		t.Fatalf("JSONL line count=%d, output=%q", got, out.String())
	}
}

func TestAnalyzeDoesNotWarnForUnsupportedTestOnlyFiles(t *testing.T) {
	var out bytes.Buffer
	var warnings bytes.Buffer
	app := New(Dependencies{
		InspectDiff: func(diff.Options) (diff.Result, error) {
			return diff.Result{Files: []diff.FileChange{
				{Status: "modified", Path: "tests/fixtures/input.json", Test: true},
				{Status: "modified", Path: "testdata/output.golden", Test: true},
			}}, nil
		},
		BuildGraph:    emptyGraph,
		BuildGitGraph: emptyGraph,
		Warnings:      &warnings,
	})

	if err := app.Analyze(nil, &out, false); err != nil {
		t.Fatal(err)
	}
	if warnings.Len() != 0 {
		t.Fatalf("warnings=%q, want none", warnings.String())
	}
}

func TestAnalyzeDoesNotWarnForSupportedGoAndJavaFiles(t *testing.T) {
	var out bytes.Buffer
	var warnings bytes.Buffer
	app := New(Dependencies{
		InspectDiff: func(diff.Options) (diff.Result, error) {
			return diff.Result{Files: []diff.FileChange{
				{Status: "modified", Path: "cmd/app.go"},
				{Status: "modified", Path: "src/main/java/example/App.java"},
			}}, nil
		},
		BuildGraph:    emptyGraph,
		BuildGitGraph: emptyGraph,
		Warnings:      &warnings,
	})

	if err := app.Analyze(nil, &out, false); err != nil {
		t.Fatal(err)
	}
	if warnings.Len() != 0 {
		t.Fatalf("warnings=%q, want none", warnings.String())
	}
}

func emptyGraph(string) (*graph.Graph, error) {
	return &graph.Graph{Functions: map[string]graph.Function{}, Calls: map[string]map[string]bool{}, Callers: map[string]map[string]bool{}, CallSites: map[string]map[string]graph.CallSite{}}, nil
}
