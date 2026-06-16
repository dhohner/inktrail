package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dhohner/inktrail/internal/diff"
)

func TestWriteJSONLMaxLinesPerHunkCompactsHunkContent(t *testing.T) {
	r := Report{
		Summary: Summary{Files: 1},
		Files: []diff.FileChange{{
			Status: "modified",
			Path:   "app.go",
			Hunks: []diff.Hunk{{
				OldStart: 1,
				OldLines: 3,
				NewStart: 1,
				NewLines: 3,
				Lines: []diff.HunkLine{
					{Op: "add", NewLine: 1, Content: "a"},
					{Op: "add", NewLine: 2, Content: "b"},
					{Op: "add", NewLine: 3, Content: "c"},
				},
			}},
		}},
	}
	var buf bytes.Buffer
	if err := WriteJSONLWithOptions(&buf, r, SizeOptions{MaxLinesPerHunk: 2}); err != nil {
		t.Fatal(err)
	}

	records := jsonlRecords(t, buf.Bytes())
	file := records[1]
	hunks := file["hunks"].([]any)
	hunk := hunks[0].(map[string]any)
	if got := len(hunk["lines"].([]any)); got != 2 {
		t.Fatalf("lines=%d", got)
	}
	if got := int(hunk["omitted_lines"].(float64)); got != 1 {
		t.Fatalf("hunk omitted_lines=%d", got)
	}
	if got := int(file["omitted_lines"].(float64)); got != 1 {
		t.Fatalf("file omitted_lines=%d", got)
	}
	if file["path"] != "app.go" || file["diffstat"] == nil {
		t.Fatalf("file metadata corrupted: %#v", file)
	}
	if !strings.Contains(buf.String(), `"reason":"max_lines_per_hunk"`) {
		t.Fatalf("summary missing hunk omission metadata: %s", buf.String())
	}
}

func TestWriteJSONLMaxContextLinesReportsTruncation(t *testing.T) {
	r := Report{Summary: Summary{ContextRecords: ContextRecordCounts{Total: 1, DeclarationContext: 1}}, Contexts: []DeclarationContext{{ID: "app.go::app.F", Path: "app.go", Relationship: "changed_declaration", Excerpt: SourceExcerpt{Content: "one\ntwo\nthree"}}}}
	var buf bytes.Buffer
	if err := WriteJSONLWithOptions(&buf, r, SizeOptions{MaxContextLines: 2}); err != nil {
		t.Fatal(err)
	}

	records := jsonlRecords(t, buf.Bytes())
	summary := records[0]
	if summary["omissions"] == nil {
		t.Fatalf("summary missing omissions: %#v", summary)
	}
	context := records[1]
	excerpt := context["excerpt"].(map[string]any)
	if excerpt["content"] != "one\ntwo" || excerpt["truncated"] != true || int(excerpt["omitted_lines"].(float64)) != 1 {
		t.Fatalf("excerpt=%#v", excerpt)
	}
}

func TestWriteJSONLMaxRecordsPreservesSummaryAndReportsOmissions(t *testing.T) {
	r := Report{Summary: Summary{Files: 1, ChangedSymbols: 1, Nodes: 2}, Files: []diff.FileChange{{Status: "modified", Path: "app.go"}}, ChangedSymbols: []string{"app.go::app.F"}, Nodes: []Node{{ID: "app.go::app.F"}, {ID: "app.go::app.G"}}}
	var buf bytes.Buffer
	if err := WriteJSONLWithOptions(&buf, r, SizeOptions{MaxRecords: 2}); err != nil {
		t.Fatal(err)
	}

	records := jsonlRecords(t, buf.Bytes())
	if len(records) != 3 { // summary + 2 detail records
		t.Fatalf("record count=%d records=%#v", len(records), records)
	}
	if records[0]["nodes"] != float64(2) || records[0]["omissions"] == nil {
		t.Fatalf("summary not preserved with omission metadata: %#v", records[0])
	}
	if records[1]["type"] != "file" || records[2]["type"] != "changed_symbol" {
		t.Fatalf("highest priority records not preserved: %#v", records)
	}
}

func TestMaxContextLinesAccountsForPreviouslyOmittedExcerptLines(t *testing.T) {
	r := Report{Summary: Summary{ContextRecords: ContextRecordCounts{Total: 1, DeclarationContext: 1}}, Contexts: []DeclarationContext{{ID: "app.go::app.F", Path: "app.go", Relationship: "changed_declaration", Excerpt: SourceExcerpt{Content: "one\ntwo\nthree", Truncated: true, OmittedLines: 7}}}}
	var buf bytes.Buffer
	if err := WriteJSONWithOptions(&buf, r, SizeOptions{MaxContextLines: 2}); err != nil {
		t.Fatal(err)
	}
	var obj Object
	if err := json.Unmarshal(buf.Bytes(), &obj); err != nil {
		t.Fatal(err)
	}
	if got := obj.Contexts[0].Excerpt.OmittedLines; got != 8 {
		t.Fatalf("excerpt omitted_lines=%d", got)
	}
	omission := obj.Summary.Omissions[0]
	if omission.OriginalCount != 10 || omission.EmittedCount != 2 || omission.OmittedCount != 8 {
		t.Fatalf("unexpected context omission counts: %#v", omission)
	}
}

func TestMaxRecordsTrimsLargestRecordWithinPriorityClass(t *testing.T) {
	r := Report{Summary: Summary{Nodes: 2}, Nodes: []Node{{ID: "large", Name: strings.Repeat("x", 100)}, {ID: "small", Name: "x"}}}
	var buf bytes.Buffer
	if err := WriteJSONLWithOptions(&buf, r, SizeOptions{MaxRecords: 1}); err != nil {
		t.Fatal(err)
	}
	records := jsonlRecords(t, buf.Bytes())
	if len(records) != 2 || records[1]["id"] != "small" {
		t.Fatalf("largest same-priority record was not trimmed first: %#v", records)
	}
}

func TestBudgetOmissionMetadataCountsFinalEmittedEntries(t *testing.T) {
	omissions := compactOmissionsForBudget([]Omission{
		{Kind: "context_lines", Reason: "max_context_lines", OriginalCount: 3, EmittedCount: 1, OmittedCount: 2},
		{Kind: "context_lines", Reason: "max_context_lines", OriginalCount: 3, EmittedCount: 1, OmittedCount: 2},
		{Kind: "context_lines", Reason: "max_context_lines", OriginalCount: 3, EmittedCount: 1, OmittedCount: 2},
	})
	metadata := omissions[len(omissions)-1]
	if metadata.Kind != "omission_metadata" || metadata.OriginalCount != 3 || metadata.EmittedCount != len(omissions) || metadata.OmittedCount != 1 {
		t.Fatalf("unexpected omission metadata counts: %#v in %#v", metadata, omissions)
	}
}

func TestWriteJSONLBudgetTokensUsesApproximateSerializedSize(t *testing.T) {
	r := Report{Summary: Summary{Files: 1, ChangedSymbols: 1, Nodes: 1}, Files: []diff.FileChange{{Status: "modified", Path: "app.go", Hunks: []diff.Hunk{{Lines: []diff.HunkLine{{Op: "add", NewLine: 1, Content: strings.Repeat("x", 200)}}}}}}, ChangedSymbols: []string{"app.go::app.F"}, Nodes: []Node{{ID: "app.go::app.F", Path: "app.go", Name: strings.Repeat("n", 100)}}}
	var full bytes.Buffer
	if err := WriteJSONL(&full, r); err != nil {
		t.Fatal(err)
	}
	budget := len(full.String()) / 8 // smaller than ceil(chars/4) for the full report

	var buf bytes.Buffer
	if err := WriteJSONLWithOptions(&buf, r, SizeOptions{BudgetTokens: budget}); err != nil {
		t.Fatal(err)
	}

	if got := (len(buf.String()) + 3) / 4; got > budget {
		t.Fatalf("estimated tokens=%d budget=%d output=%s", got, budget, buf.String())
	}
	if !strings.Contains(buf.String(), `"reason":"budget_tokens"`) {
		t.Fatalf("missing budget omission metadata: %s", buf.String())
	}
}

func TestWriteJSONLBudgetFloorReportsFinalEstimatedSize(t *testing.T) {
	r := Report{Summary: Summary{Files: 1}, Files: []diff.FileChange{{Status: "modified", Path: "app.go", Hunks: []diff.Hunk{{Lines: []diff.HunkLine{{Op: "add", NewLine: 1, Content: strings.Repeat("x", 200)}}}}}}}
	var buf bytes.Buffer
	if err := WriteJSONLWithOptions(&buf, r, SizeOptions{BudgetTokens: 1}); err != nil {
		t.Fatal(err)
	}

	records := jsonlRecords(t, buf.Bytes())
	finalEstimate := (len(buf.String()) + 3) / 4
	var floor map[string]any
	for _, raw := range records[0]["omissions"].([]any) {
		omission := raw.(map[string]any)
		if omission["reason"] == "budget_floor_exceeded" {
			floor = omission
		}
	}
	if floor == nil {
		t.Fatalf("missing budget floor omission: %s", buf.String())
	}
	if got := int(floor["original_count"].(float64)); got != finalEstimate {
		t.Fatalf("floor original_count=%d final estimate=%d output=%s", got, finalEstimate, buf.String())
	}
}

func TestMaxRecordsPreservesFileSymbolsWhenNodesAreTrimmed(t *testing.T) {
	r := Report{
		Summary: Summary{Files: 1, Nodes: 1},
		Files:   []diff.FileChange{{Status: "modified", Path: "app.go"}},
		Nodes:   []Node{{ID: "app.go::app.F", Path: "app.go", Name: "F"}},
	}
	var buf bytes.Buffer
	if err := WriteJSONLWithOptions(&buf, r, SizeOptions{MaxRecords: 1}); err != nil {
		t.Fatal(err)
	}

	records := jsonlRecords(t, buf.Bytes())
	if len(records) != 2 {
		t.Fatalf("record count=%d records=%#v", len(records), records)
	}
	symbols, ok := records[1]["symbols"].([]any)
	if !ok || len(symbols) != 1 || symbols[0] != "app.go::app.F" {
		t.Fatalf("file symbols not preserved after trimming nodes: %#v", records[1])
	}
}

func TestWriteJSONDoesNotMutateInputWhenSizing(t *testing.T) {
	r := Report{Contexts: []DeclarationContext{{ID: "app.go::app.F", Path: "app.go", Relationship: "changed_declaration", Excerpt: SourceExcerpt{Content: "one\ntwo"}}}}
	var buf bytes.Buffer
	if err := WriteJSONWithOptions(&buf, r, SizeOptions{MaxContextLines: 1}); err != nil {
		t.Fatal(err)
	}
	if got := r.Contexts[0].Excerpt.Content; got != "one\ntwo" {
		t.Fatalf("input context was mutated: %q", got)
	}
	if len(r.Summary.Omissions) != 0 {
		t.Fatalf("input summary omissions were mutated: %#v", r.Summary.Omissions)
	}
}

func TestWriteJSONCompactsManyHunkOmissions(t *testing.T) {
	files := make([]diff.FileChange, 25)
	for i := range files {
		files[i] = diff.FileChange{Status: "modified", Path: "app.go", Hunks: []diff.Hunk{{Lines: []diff.HunkLine{{Op: "add", NewLine: 1, Content: "a"}, {Op: "add", NewLine: 2, Content: "b"}}}}}
	}
	r := Report{Summary: Summary{Files: len(files)}, Files: files}
	var buf bytes.Buffer
	if err := WriteJSONWithOptions(&buf, r, SizeOptions{MaxLinesPerHunk: 1}); err != nil {
		t.Fatal(err)
	}
	var obj Object
	if err := json.Unmarshal(buf.Bytes(), &obj); err != nil {
		t.Fatal(err)
	}
	if len(obj.Summary.Omissions) != 1 {
		t.Fatalf("hunk omissions were not compacted: %#v", obj.Summary.Omissions)
	}
	omission := obj.Summary.Omissions[0]
	if omission.Kind != "hunk_lines" || omission.Reason != "max_lines_per_hunk" || omission.OriginalCount != 50 || omission.EmittedCount != 25 || omission.OmittedCount != 25 {
		t.Fatalf("unexpected compacted hunk omission: %#v", omission)
	}
}

func TestWriteJSONBudgetCompactsOmissionMetadataUnderBudgetPressure(t *testing.T) {
	contexts := make([]DeclarationContext, 40)
	for i := range contexts {
		contexts[i] = DeclarationContext{ID: "app.go::app.F", Path: "app.go", Relationship: "changed_declaration", Excerpt: SourceExcerpt{Content: "one\ntwo\nthree"}}
	}
	r := Report{Summary: Summary{ContextRecords: ContextRecordCounts{Total: len(contexts), DeclarationContext: len(contexts)}}, Contexts: contexts}

	var full bytes.Buffer
	if err := WriteJSONWithOptions(&full, r, SizeOptions{MaxContextLines: 1}); err != nil {
		t.Fatal(err)
	}
	budget := ((len(full.String()) + 3) / 4) / 3

	var buf bytes.Buffer
	if err := WriteJSONWithOptions(&buf, r, SizeOptions{MaxContextLines: 1, BudgetTokens: budget}); err != nil {
		t.Fatal(err)
	}
	if got := (len(buf.String()) + 3) / 4; got > budget {
		t.Fatalf("estimated tokens=%d budget=%d output=%s", got, budget, buf.String())
	}
	var obj Object
	if err := json.Unmarshal(buf.Bytes(), &obj); err != nil {
		t.Fatal(err)
	}
	if len(obj.Summary.Omissions) >= len(contexts) {
		t.Fatalf("omission metadata was not compacted: %d omissions", len(obj.Summary.Omissions))
	}
	if len(obj.ChangedSymbols) != 0 || obj.ChangedSymbols == nil || obj.Contexts == nil {
		t.Fatalf("JSON object slices should encode as empty arrays after trimming: %#v", obj)
	}
}

func jsonlRecords(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("unmarshal %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}
