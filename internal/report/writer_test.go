package report

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/dhohner/inktrail/internal/diff"
)

func TestWriteJSONLWritesOneRecordPerLine(t *testing.T) {
	r := Report{
		Summary: Summary{Files: 1, ChangedSymbols: 1, Nodes: 1},
		Files: []diff.FileChange{{Status: "modified", Path: "app.go", Hunks: []diff.Hunk{{OldStart: 4, OldLines: 1, NewStart: 4, NewLines: 1, Lines: []diff.HunkLine{
			{Op: "add", NewLine: 4, Content: "if a < b && c > d {"},
		}}}}},
		ChangedSymbols: []string{"app.go::app.F"},
		Nodes:          []Node{{ID: "app.go::app.F", Path: "app.go", Name: "F", Kind: "function", StartLine: 3, EndLine: 5, Changed: true, ChangedLines: []ChangedLineRange{{Start: 4, End: 4}}, Package: "app"}},
	}
	var buf bytes.Buffer

	if err := WriteJSONL(&buf, r); err != nil {
		t.Fatal(err)
	}

	want := "{\"type\":\"summary\",\"schema_version\":\"1.0\",\"files\":1,\"test_files\":0,\"changed_symbols\":1,\"deleted_symbols\":0,\"moved_symbols\":0,\"removed_calls\":0,\"entry_points\":0,\"nodes\":1,\"context_records\":{\"total\":0,\"declaration_context\":0,\"related_declaration_context\":0}}\n" +
		"{\"type\":\"file\",\"status\":\"modified\",\"path\":\"app.go\",\"test\":false,\"language\":\"go\",\"classification\":[\"source\"],\"diffstat\":{\"added_lines\":1,\"deleted_lines\":0,\"added_bytes\":19,\"deleted_bytes\":0},\"symbols\":[\"app.go::app.F\"],\"content_ref\":{\"kind\":\"workspace_file\",\"path\":\"app.go\"},\"hunks\":[{\"old_start\":4,\"old_lines\":1,\"new_start\":4,\"new_lines\":1,\"lines\":[{\"op\":\"add\",\"new_line\":4,\"content\":\"if a < b && c > d {\"}]}]}\n" +
		"{\"type\":\"changed_symbol\",\"id\":\"app.go::app.F\"}\n" +
		"{\"type\":\"node\",\"id\":\"app.go::app.F\",\"path\":\"app.go\",\"name\":\"F\",\"kind\":\"function\",\"start_line\":3,\"end_line\":5,\"changed\":true,\"changed_lines\":[{\"start\":4,\"end\":4}],\"package\":\"app\"}\n"
	if got := buf.String(); got != want {
		t.Fatalf("jsonl=%q", got)
	}
}

func TestWriteJSONLEmitsReviewSummaryRecord(t *testing.T) {
	r := Report{
		Summary:        Summary{Files: 2, ChangedSymbols: 1, DeletedSymbols: 1, RemovedCalls: 1},
		Warnings:       []Warning{{Code: "unsupported_language", Path: "web/app.ts", Message: "unsupported production language skipped for symbol and call graph analysis"}},
		Files:          []diff.FileChange{{Status: "modified", Path: "app.go"}, {Status: "modified", Path: "app_test.go", Test: true}, {Status: "modified", Path: "web/app.ts"}},
		ChangedSymbols: []string{"app.go::app.F"},
		DeletedSymbols: []string{"old.go::app.Old"},
		RemovedCalls:   []RemovedCall{{From: "app.go::app.F", To: "old.go::app.Old", CallSite: CallSite{Path: "app.go", Line: 12}}},
		FileSymbols:    map[string][]string{"app.go": {"app.go::app.F"}},
	}
	var buf bytes.Buffer

	if err := WriteJSONLWithOptions(&buf, r, SizeOptions{EmitReviewSummary: true}); err != nil {
		t.Fatal(err)
	}

	records := jsonlRecords(t, buf.Bytes())
	if len(records) < 2 || records[0]["type"] != "summary" || records[1]["type"] != "review_summary" {
		t.Fatalf("records=%#v", records)
	}
	review := records[1]
	if len(review["changed_production_files"].([]any)) != 2 || len(review["changed_test_files"].([]any)) != 1 || len(review["changed_symbols"].([]any)) != 1 || len(review["deleted_symbols"].([]any)) != 1 || len(review["risky_removed_call_edges"].([]any)) != 1 || review["unsupported_files"].([]any)[0] != "web/app.ts" {
		t.Fatalf("review summary=%#v", review)
	}
}

func TestWriteJSONLReviewSummarySurvivesMaxRecordsTrimming(t *testing.T) {
	r := Report{
		Summary:        Summary{Files: 2, ChangedSymbols: 2, Nodes: 2},
		Files:          []diff.FileChange{{Status: "modified", Path: "app.go"}, {Status: "modified", Path: "service.go"}},
		ChangedSymbols: []string{"app.go::app.F", "service.go::app.G"},
		Nodes:          []Node{{ID: "app.go::app.F", Path: "app.go"}, {ID: "service.go::app.G", Path: "service.go"}},
	}
	var buf bytes.Buffer

	if err := WriteJSONLWithOptions(&buf, r, SizeOptions{EmitReviewSummary: true, MaxRecords: 1}); err != nil {
		t.Fatal(err)
	}

	records := jsonlRecords(t, buf.Bytes())
	if len(records) != 3 || records[0]["type"] != "summary" || records[1]["type"] != "review_summary" || records[2]["type"] != "file" {
		t.Fatalf("unexpected records after trimming: %#v", records)
	}
	review := records[1]
	if got := len(review["changed_production_files"].([]any)); got != 2 {
		t.Fatalf("review summary should keep all changed files under max-records, got %d: %#v", got, review)
	}
	if got := len(review["changed_symbols"].([]any)); got != 2 {
		t.Fatalf("review summary should keep all changed symbols under max-records, got %d: %#v", got, review)
	}
	if records[0]["omissions"] == nil {
		t.Fatalf("summary should still report detail record omissions: %#v", records[0])
	}
}

func TestWriteJSONLEmitsWarningRecords(t *testing.T) {
	r := Report{Warnings: []Warning{{Code: "parse_error", Path: "app.go", Message: "current graph build failed: parse app.go: syntax error"}}}
	var buf bytes.Buffer

	if err := WriteJSONL(&buf, r); err != nil {
		t.Fatal(err)
	}

	var records []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	if len(records) != 2 || records[1]["type"] != "warning" || records[1]["code"] != "parse_error" || records[1]["path"] != "app.go" {
		t.Fatalf("records=%#v", records)
	}
}

func TestWriteJSONLCompactsLargeAddedFiles(t *testing.T) {
	var lines []diff.HunkLine
	for i := 1; i <= 301; i++ {
		lines = append(lines, diff.HunkLine{Op: "add", NewLine: i, Content: "func F() {}"})
	}
	r := Report{
		Files: []diff.FileChange{{Status: "added", Path: "large.go", Hunks: []diff.Hunk{{NewStart: 1, NewLines: 301, Lines: lines}}}},
		Nodes: []Node{{ID: "large.go::app.F", Path: "large.go"}},
	}
	var buf bytes.Buffer

	if err := WriteJSONL(&buf, r); err != nil {
		t.Fatal(err)
	}

	var records []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	file := records[1]
	if file["hunks"] != nil || file["hunks_omitted"] != true {
		t.Fatalf("file record did not omit hunks: %#v", file)
	}
	if file["content_ref"] == nil || file["preview"] == nil {
		t.Fatalf("missing lazy content metadata: %#v", file)
	}
	if file["risk_flags"] != nil {
		t.Fatalf("unexpected risk_flags=%#v", file["risk_flags"])
	}
}
