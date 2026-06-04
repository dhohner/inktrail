package report

import (
	"reflect"
	"testing"

	"inktrail/internal/diff"
	"inktrail/internal/graph"
)

func TestBuildIncludesChangedSymbolsEntryPointsNodesAndCallSites(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "app.go", `package app

type ControllerA struct{}
type ServiceA struct{}
type ServiceB struct{}
type RepositoryB struct{}

func (c ControllerA) Handle() { ServiceA{}.Do() }
func (s ServiceA) Do() { ServiceB{}.Do() }
func (s ServiceB) Do() { RepositoryB{}.Get() }
func (r RepositoryB) Get() {}
`)

	g, err := graph.Build(dir)
	if err != nil {
		t.Fatal(err)
	}

	r := Build(g, diff.Result{
		Lines: []diff.Line{{Path: "app.go", LineNo: 10, Content: "func (s ServiceB) Do() { RepositoryB{}.Get() }"}},
		Files: []diff.FileChange{{Status: "modified", Path: "app.go", Hunks: []diff.Hunk{{OldStart: 10, OldLines: 1, NewStart: 10, NewLines: 1}}}},
	})

	if r.Summary.Files != 1 || r.Summary.ChangedSymbols != 1 || r.Summary.EntryPoints != 1 || r.Summary.Nodes != 3 {
		t.Fatalf("summary=%#v", r.Summary)
	}
	if !reflect.DeepEqual(r.Files, []diff.FileChange{{Status: "modified", Path: "app.go", Hunks: []diff.Hunk{{OldStart: 10, OldLines: 1, NewStart: 10, NewLines: 1}}}}) {
		t.Fatalf("files=%#v", r.Files)
	}

	wantChanged := []string{"app.go::app.ServiceB.Do"}
	if !reflect.DeepEqual(r.ChangedSymbols, wantChanged) {
		t.Fatalf("changed_symbols=%#v want=%#v", r.ChangedSymbols, wantChanged)
	}

	wantEntries := []string{"app.go::app.ControllerA.Handle"}
	if !reflect.DeepEqual(r.EntryPoints, wantEntries) {
		t.Fatalf("entry_points=%#v want=%#v", r.EntryPoints, wantEntries)
	}

	nodes := nodesByID(r.Nodes)
	for _, id := range []string{
		"app.go::app.ControllerA.Handle",
		"app.go::app.ServiceA.Do",
		"app.go::app.ServiceB.Do",
	} {
		if _, ok := nodes[id]; !ok {
			t.Fatalf("missing node %s in %#v", id, r.Nodes)
		}
	}

	changed := nodes["app.go::app.ServiceB.Do"]
	if !changed.Changed {
		t.Fatalf("changed node not marked changed: %#v", changed)
	}
	if !reflect.DeepEqual(changed.ChangedLines, []ChangedLineRange{{Start: 10, End: 10}}) {
		t.Fatalf("changed_lines=%#v", changed.ChangedLines)
	}
	if changed.Kind != "method" || changed.Package != "app" || changed.Name != "Do" {
		t.Fatalf("unexpected changed node metadata: %#v", changed)
	}

	serviceA := nodes["app.go::app.ServiceA.Do"]
	if len(serviceA.Calls) != 1 {
		t.Fatalf("ServiceA calls=%#v", serviceA.Calls)
	}
	if serviceA.Calls[0].To != "app.go::app.ServiceB.Do" {
		t.Fatalf("call to=%s", serviceA.Calls[0].To)
	}
	if serviceA.Calls[0].CallSite.Path != "app.go" || serviceA.Calls[0].CallSite.Line != 9 {
		t.Fatalf("call site=%#v", serviceA.Calls[0].CallSite)
	}
}

func TestBuildCompactsAdjacentChangedLinesIntoRanges(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "app.go", `package app

func F() {
	a()
	b()
	c()
	d()
}
`)
	g, err := graph.Build(dir)
	if err != nil {
		t.Fatal(err)
	}

	r := Build(g, diff.Result{Lines: []diff.Line{
		{Path: "app.go", LineNo: 4},
		{Path: "app.go", LineNo: 5},
		{Path: "app.go", LineNo: 7},
	}})
	node := nodesByID(r.Nodes)["app.go::app.F"]
	want := []ChangedLineRange{{Start: 4, End: 5}, {Start: 7, End: 7}}
	if !reflect.DeepEqual(node.ChangedLines, want) {
		t.Fatalf("changed_lines=%#v want=%#v", node.ChangedLines, want)
	}
}

func TestBuildWithBaseIncludesDeletedSymbolsAndRemovedCalls(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "app.go", `package app

type A struct{}
type B struct{}

func (a A) Run() {}
`)
	current, err := graph.Build(dir)
	if err != nil {
		t.Fatal(err)
	}

	oldDir := t.TempDir()
	write(t, oldDir, "app.go", `package app

type A struct{}
type B struct{}

func (a A) Run() { B{}.Gone() }
func (b B) Gone() {}
`)
	old, err := graph.Build(oldDir)
	if err != nil {
		t.Fatal(err)
	}

	r := BuildWithBase(current, old, diff.Result{})

	if !reflect.DeepEqual(r.DeletedSymbols, []string{"app.go::app.B.Gone"}) {
		t.Fatalf("deleted_symbols=%#v", r.DeletedSymbols)
	}
	if len(r.RemovedCalls) != 1 {
		t.Fatalf("removed_calls=%#v", r.RemovedCalls)
	}
	if r.RemovedCalls[0].From != "app.go::app.A.Run" || r.RemovedCalls[0].To != "app.go::app.B.Gone" {
		t.Fatalf("removed_call=%#v", r.RemovedCalls[0])
	}
	if r.Summary.DeletedSymbols != 1 || r.Summary.RemovedCalls != 1 {
		t.Fatalf("summary=%#v", r.Summary)
	}
}

func TestBuildWithBaseRepresentsMovedSymbolsAsMoves(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "new.go", `package app

func F() {}
`)
	current, err := graph.Build(dir)
	if err != nil {
		t.Fatal(err)
	}

	oldDir := t.TempDir()
	write(t, oldDir, "old.go", `package app

func F() {}
`)
	old, err := graph.Build(oldDir)
	if err != nil {
		t.Fatal(err)
	}

	r := BuildWithBase(current, old, diff.Result{
		Lines: []diff.Line{{Path: "new.go", LineNo: 3}},
		Files: []diff.FileChange{{Status: "added", Path: "new.go", Hunks: []diff.Hunk{{NewStart: 1, NewLines: 3, Lines: []diff.HunkLine{
			{Op: "add", NewLine: 1, Content: "package app"},
			{Op: "add", NewLine: 3, Content: "func F() {}"},
		}}}}},
	})

	wantMoves := []MovedSymbol{{From: "old.go::app.F", To: "new.go::app.F"}}
	if !reflect.DeepEqual(r.MovedSymbols, wantMoves) {
		t.Fatalf("moved_symbols=%#v want=%#v", r.MovedSymbols, wantMoves)
	}
	if len(r.DeletedSymbols) != 0 || len(r.ChangedSymbols) != 0 || len(r.Nodes) != 0 {
		t.Fatalf("deleted_symbols=%#v changed_symbols=%#v nodes=%#v", r.DeletedSymbols, r.ChangedSymbols, r.Nodes)
	}
	if len(r.Files) != 1 || len(r.Files[0].Hunks) != 1 || len(r.Files[0].Hunks[0].Lines) != 1 || r.Files[0].Hunks[0].Lines[0].NewLine != 1 {
		t.Fatalf("files=%#v", r.Files)
	}
	if r.Summary.MovedSymbols != 1 || r.Summary.DeletedSymbols != 0 || r.Summary.ChangedSymbols != 0 {
		t.Fatalf("summary=%#v", r.Summary)
	}
}

func TestBuildWithBaseDoesNotTreatModifiedMovesAsMoved(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "new.go", `package app

func F() { println("new") }
`)
	current, err := graph.Build(dir)
	if err != nil {
		t.Fatal(err)
	}

	oldDir := t.TempDir()
	write(t, oldDir, "old.go", `package app

func F() { println("old") }
`)
	old, err := graph.Build(oldDir)
	if err != nil {
		t.Fatal(err)
	}

	r := BuildWithBase(current, old, diff.Result{Lines: []diff.Line{{Path: "new.go", LineNo: 3}}})

	if len(r.MovedSymbols) != 0 {
		t.Fatalf("moved_symbols=%#v", r.MovedSymbols)
	}
	if !reflect.DeepEqual(r.ChangedSymbols, []string{"new.go::app.F"}) {
		t.Fatalf("changed_symbols=%#v", r.ChangedSymbols)
	}
	if !reflect.DeepEqual(r.DeletedSymbols, []string{"old.go::app.F"}) {
		t.Fatalf("deleted_symbols=%#v", r.DeletedSymbols)
	}
}

func TestBuildWithBaseDoesNotReportRemovedCallsForMovedSymbols(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "new.go", `package app

func F() { G() }
func G() {}
`)
	current, err := graph.Build(dir)
	if err != nil {
		t.Fatal(err)
	}

	oldDir := t.TempDir()
	write(t, oldDir, "old.go", `package app

func F() { G() }
func G() {}
`)
	old, err := graph.Build(oldDir)
	if err != nil {
		t.Fatal(err)
	}

	r := BuildWithBase(current, old, diff.Result{})

	if len(r.MovedSymbols) != 2 {
		t.Fatalf("moved_symbols=%#v", r.MovedSymbols)
	}
	if len(r.RemovedCalls) != 0 {
		t.Fatalf("removed_calls=%#v", r.RemovedCalls)
	}
}

func TestBuildWithBaseDropsFilesWhoseHunksOnlyContainMovedLines(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "new.go", `package app

func F() {}
`)
	current, err := graph.Build(dir)
	if err != nil {
		t.Fatal(err)
	}

	oldDir := t.TempDir()
	write(t, oldDir, "old.go", `package app

func F() {}
`)
	old, err := graph.Build(oldDir)
	if err != nil {
		t.Fatal(err)
	}

	r := BuildWithBase(current, old, diff.Result{
		Lines: []diff.Line{{Path: "new.go", LineNo: 3}},
		Files: []diff.FileChange{{Status: "modified", Path: "new.go", Hunks: []diff.Hunk{{NewStart: 3, NewLines: 1, Lines: []diff.HunkLine{
			{Op: "add", NewLine: 3, Content: "func F() {}"},
		}}}}},
	})

	if len(r.MovedSymbols) != 1 {
		t.Fatalf("moved_symbols=%#v", r.MovedSymbols)
	}
	if len(r.Files) != 0 || r.Summary.Files != 0 {
		t.Fatalf("files=%#v summary=%#v", r.Files, r.Summary)
	}
}
