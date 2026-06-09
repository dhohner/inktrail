package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dhohner/inktrail/internal/diff"
)

func TestWriteHTMLEmbedsCompleteImpactGraphData(t *testing.T) {
	r := Report{
		ChangedSymbols: []string{"pkg.changed"},
		DeletedSymbols: []string{"pkg.deleted"},
		MovedSymbols:   []MovedSymbol{{From: "pkg.old", To: "pkg.new", BodySHA256Equal: true}},
		RemovedCalls:   []RemovedCall{{From: "pkg.caller", To: "pkg.deleted", CallSite: CallSite{Path: "pkg/a.go", Line: 7}}},
		EntryPoints:    []string{"pkg.entry"},
		Contexts:       []DeclarationContext{{ID: "pkg.changed", Path: "pkg/a.go", Name: "changed", Kind: "function", Relationship: "changed_symbol"}},
		Nodes: []Node{
			{ID: "pkg.entry", Path: "pkg/a.go", Name: "entry", Kind: "function", Package: "pkg", Calls: []OutgoingCall{{To: "pkg.changed"}}},
			{ID: "pkg.changed", Path: "pkg/a.go", Name: "changed", Kind: "function", Package: "pkg", Changed: true},
		},
		Files: []diff.FileChange{{Status: "modified", Path: "pkg/a.go"}},
	}

	var out bytes.Buffer
	if err := WriteHTML(&out, r); err != nil {
		t.Fatal(err)
	}
	html := out.String()
	for _, want := range []string{
		`id="impact-graph"`,
		`id="inktrail-report-data"`,
		`"files":[`,
		`"changed_symbols":["pkg.changed"]`,
		`"deleted_symbols":["pkg.deleted"]`,
		`"moved_symbols":[`,
		`"removed_calls":[`,
		`"entry_points":["pkg.entry"]`,
		`"contexts":[`,
		`"nodes":[`,
		`class="impact-grid"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("html missing %q: %s", want, html)
		}
	}
}

func TestWriteHTMLRendersCompleteImpactGraphModel(t *testing.T) {
	r := Report{
		ChangedSymbols: []string{"pkg.changed"},
		DeletedSymbols: []string{"pkg.deleted"},
		MovedSymbols:   []MovedSymbol{{From: "pkg.old", To: "pkg.new", BodySHA256Equal: true}},
		RemovedCalls: []RemovedCall{
			{From: "pkg.missingCaller", To: "pkg.missingCallee", CallSite: CallSite{Path: "pkg/a.go", Line: 8}},
		},
		EntryPoints: []string{"pkg.entry"},
		Contexts:    []DeclarationContext{{ID: "pkg.changed", Path: "pkg/a.go", Name: "changed", Kind: "function"}},
		Nodes: []Node{
			{ID: "pkg.entry", Path: "pkg/a.go", Name: "entry", Kind: "function", Package: "pkg", Calls: []OutgoingCall{{To: "pkg.caller"}}},
			{ID: "pkg.caller", Path: "pkg/a.go", Name: "caller", Kind: "function", Package: "pkg", Calls: []OutgoingCall{{To: "pkg.changed"}}},
			{ID: "pkg.changed", Path: "pkg/a.go", Name: "changed", Kind: "function", Package: "pkg", Changed: true, Calls: []OutgoingCall{{To: "pkg.callee"}}},
			{ID: "pkg.callee", Path: "pkg/b.go", Name: "callee", Kind: "function", Package: "pkg"},
		},
		Files: []diff.FileChange{{Status: "modified", Path: "pkg/a.go"}},
	}

	var out bytes.Buffer
	if err := WriteHTML(&out, r); err != nil {
		t.Fatal(err)
	}
	html := out.String()
	for _, id := range []string{"pkg.entry", "pkg.caller", "pkg.changed", "pkg.callee", "pkg.deleted", "pkg.old", "pkg.new", "pkg.missingCaller", "pkg.missingCallee"} {
		if !strings.Contains(html, `data-id="`+id+`"`) {
			t.Fatalf("graph missing node %q: %s", id, html)
		}
	}
	for _, edge := range []string{
		`data-from="pkg.entry" data-to="pkg.caller" class="graph-edge call"`,
		`data-from="pkg.caller" data-to="pkg.changed" class="graph-edge call"`,
		`data-from="pkg.changed" data-to="pkg.callee" class="graph-edge call"`,
		`data-from="pkg.missingCaller" data-to="pkg.missingCallee" class="graph-edge removed"`,
		`data-from="pkg.old" data-to="pkg.new" class="graph-edge moved"`,
	} {
		if !strings.Contains(html, edge) {
			t.Fatalf("graph missing edge %q: %s", edge, html)
		}
	}
}

func TestWriteHTMLEmptyImpactGraphIsNotBroken(t *testing.T) {
	var out bytes.Buffer
	if err := WriteHTML(&out, Report{}); err != nil {
		t.Fatal(err)
	}
	html := out.String()
	if !strings.Contains(html, "No graph nodes available.") {
		t.Fatalf("empty graph did not render safe message: %s", html)
	}
	if strings.Contains(html, `class="impact-grid"`) || strings.Contains(html, `data-from=`) {
		t.Fatalf("empty graph rendered misleading graph: %s", html)
	}
}

func TestWriteHTMLShowsHunkLineStats(t *testing.T) {
	r := Report{Files: []diff.FileChange{{
		Status: "modified",
		Path:   "cmd/inktrail/main.go",
		Hunks: []diff.Hunk{{Lines: []diff.HunkLine{
			{Op: "delete", OldLine: 1, Content: "old"},
			{Op: "add", NewLine: 1, Content: "new"},
			{Op: "add", NewLine: 2, Content: "next"},
		}}},
	}}}

	var out bytes.Buffer
	if err := WriteHTML(&out, r); err != nil {
		t.Fatal(err)
	}
	html := out.String()
	if !strings.Contains(html, "+2 / -1") {
		t.Fatalf("html did not include hunk stats: %s", html)
	}
	if !strings.Contains(html, ">source</span>") {
		t.Fatalf("html did not include fallback tags: %s", html)
	}
}
