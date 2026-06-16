package report

import (
	"reflect"
	"testing"

	"github.com/dhohner/inktrail/internal/diff"
	"github.com/dhohner/inktrail/internal/graph"
)

func TestBuildWithOptionsFiltersChangedPaths(t *testing.T) {
	g := filterGraph(t)
	result := diff.Result{
		Lines: []diff.Line{{Path: "app.go", LineNo: 3}, {Path: "internal/service/service.go", LineNo: 3}, {Path: "vendor/acme/lib.go", LineNo: 3}},
		Files: []diff.FileChange{{Status: "modified", Path: "app.go"}, {Status: "modified", Path: "internal/service/service.go"}, {Status: "modified", Path: "vendor/acme/lib.go"}},
	}

	r := BuildWithBaseOptions(g, nil, result, FilterOptions{Include: "internal/**/*.go"})
	if got := filePaths(r.Files); !reflect.DeepEqual(got, []string{"internal/service/service.go"}) {
		t.Fatalf("include files=%#v", got)
	}
	if !reflect.DeepEqual(r.ChangedSymbols, []string{"internal/service/service.go::service.Service"}) {
		t.Fatalf("include changed_symbols=%#v", r.ChangedSymbols)
	}

	r = BuildWithBaseOptions(g, nil, result, FilterOptions{Excludes: []string{"**/service.go", "vendor/**"}})
	if got := filePaths(r.Files); !reflect.DeepEqual(got, []string{"app.go"}) {
		t.Fatalf("exclude files=%#v", got)
	}
	if !reflect.DeepEqual(r.ChangedSymbols, []string{"app.go::app.App"}) {
		t.Fatalf("exclude changed_symbols=%#v", r.ChangedSymbols)
	}

	r = BuildWithBaseOptions(g, nil, result, FilterOptions{ExcludeVendor: true})
	if got := filePaths(r.Files); !reflect.DeepEqual(got, []string{"app.go", "internal/service/service.go"}) {
		t.Fatalf("exclude vendor files=%#v", got)
	}
}

func TestBuildWithOptionsRecursiveGlobMatchesTests(t *testing.T) {
	g := filterGraph(t)
	r := BuildWithBaseOptions(g, nil, diff.Result{
		Lines: []diff.Line{{Path: "pkg/app_test.go", LineNo: 3}, {Path: "app.go", LineNo: 3}},
		Files: []diff.FileChange{{Status: "modified", Path: "pkg/app_test.go", Test: true}, {Status: "modified", Path: "app.go"}},
	}, FilterOptions{Include: "**/*_test.go"})
	if got := filePaths(r.Files); !reflect.DeepEqual(got, []string{"pkg/app_test.go"}) {
		t.Fatalf("recursive glob files=%#v", got)
	}
	if r.Summary.Files != 1 || r.Summary.TestFiles != 1 {
		t.Fatalf("summary=%#v", r.Summary)
	}
}

func TestBuildWithOptionsKeepsRelatedContextForPathFilters(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "entry.go", `package app

func Entry() { Changed() }
`)
	write(t, dir, "changed.go", `package app

func Changed() {}
`)
	g, err := graph.Build(dir)
	if err != nil {
		t.Fatal(err)
	}

	r := BuildWithBaseOptions(g, nil, diff.Result{
		Lines: []diff.Line{{Path: "changed.go", LineNo: 3}},
		Files: []diff.FileChange{{Status: "modified", Path: "changed.go"}, {Status: "modified", Path: "entry.go"}},
	}, FilterOptions{Include: "changed.go"})

	if !reflect.DeepEqual(r.ChangedSymbols, []string{"changed.go::app.Changed"}) {
		t.Fatalf("changed_symbols=%#v", r.ChangedSymbols)
	}
	if !reflect.DeepEqual(r.EntryPoints, []string{"entry.go::app.Entry"}) {
		t.Fatalf("entry_points should keep related callers for path filters: %#v", r.EntryPoints)
	}
	if len(r.Nodes) != 2 {
		t.Fatalf("nodes should keep included changes and related callers: %#v", r.Nodes)
	}
	var sawCallerContext bool
	for _, context := range r.Contexts {
		if context.ID == "entry.go::app.Entry" && context.Relationship == "direct_caller" && context.RelatedTo == "changed.go::app.Changed" {
			sawCallerContext = true
		}
	}
	if !sawCallerContext {
		t.Fatalf("missing related caller context: %#v", r.Contexts)
	}
}

func TestBuildWithOptionsPrunesDerivedRecordsToChangedFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "entry.go", `package app

func Entry() { Changed() }
`)
	write(t, dir, "changed.go", `package app

func Changed() {}
`)
	g, err := graph.Build(dir)
	if err != nil {
		t.Fatal(err)
	}

	r := BuildWithBaseOptions(g, nil, diff.Result{
		Lines: []diff.Line{{Path: "changed.go", LineNo: 3}},
		Files: []diff.FileChange{{Status: "modified", Path: "changed.go"}},
	}, FilterOptions{ChangedOnly: true})

	if !reflect.DeepEqual(r.ChangedSymbols, []string{"changed.go::app.Changed"}) {
		t.Fatalf("changed_symbols=%#v", r.ChangedSymbols)
	}
	if len(r.Nodes) != 1 || r.Nodes[0].ID != "changed.go::app.Changed" {
		t.Fatalf("nodes not pruned to changed files: %#v", r.Nodes)
	}
	if len(r.EntryPoints) != 0 {
		t.Fatalf("entry_points should be pruned: %#v", r.EntryPoints)
	}
	for _, context := range r.Contexts {
		if context.Path != "changed.go" {
			t.Fatalf("context from excluded path: %#v", context)
		}
	}
}

func TestFilterOptionsValidateRejectsInvalidGlobPatterns(t *testing.T) {
	for _, opts := range []FilterOptions{
		{Include: "["},
		{Excludes: []string{""}},
		{Excludes: []string{"src/[.go"}},
	} {
		if err := opts.Validate(); err == nil {
			t.Fatalf("Validate(%#v) succeeded, want error", opts)
		}
	}
	if err := (FilterOptions{Include: "**/*_test.go", Excludes: []string{"vendor/**"}}).Validate(); err != nil {
		t.Fatalf("valid filters rejected: %v", err)
	}
}

func TestBuildWithOptionsPrunesDeletedSymbolsAndRemovedCalls(t *testing.T) {
	currentDir := t.TempDir()
	write(t, currentDir, "keep.go", "package app\n\nfunc Keep() {}\n")
	current, err := graph.Build(currentDir)
	if err != nil {
		t.Fatal(err)
	}
	oldDir := t.TempDir()
	write(t, oldDir, "keep.go", "package app\n\nfunc Keep() { Gone() }\n")
	write(t, oldDir, "drop.go", "package app\n\nfunc Gone() {}\n")
	old, err := graph.Build(oldDir)
	if err != nil {
		t.Fatal(err)
	}

	r := BuildWithBaseOptions(current, old, diff.Result{Files: []diff.FileChange{{Status: "modified", Path: "keep.go"}, {Status: "deleted", Path: "drop.go"}}}, FilterOptions{Excludes: []string{"drop.go"}})
	if len(r.DeletedSymbols) != 0 {
		t.Fatalf("deleted_symbols not pruned: %#v", r.DeletedSymbols)
	}
	if len(r.RemovedCalls) != 1 || r.RemovedCalls[0].From != "keep.go::app.Keep" {
		t.Fatalf("removed_calls should keep records tied to included file: %#v", r.RemovedCalls)
	}
}

func filterGraph(t *testing.T) *graph.Graph {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "app.go", "package app\n\nfunc App() {}\n")
	write(t, dir, "internal/service/service.go", "package service\n\nfunc Service() {}\n")
	write(t, dir, "vendor/acme/lib.go", "package acme\n\nfunc Lib() {}\n")
	write(t, dir, "pkg/app_test.go", "package pkg\n\nfunc TestApp() {}\n")
	g, err := graph.Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func filePaths(files []diff.FileChange) []string {
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.Path
	}
	return paths
}
