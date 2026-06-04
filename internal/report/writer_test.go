package report

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"inktrail/internal/diff"
)

func TestWriteJSONWritesIndentedReport(t *testing.T) {
	r := Report{ChangedSymbols: []string{"app.go::app.F"}}
	var buf bytes.Buffer

	if err := WriteJSON(&buf, r); err != nil {
		t.Fatal(err)
	}

	if !json.Valid(buf.Bytes()) {
		t.Fatalf("invalid json: %s", buf.String())
	}
}

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

	want := "{\"type\":\"summary\",\"files\":1,\"test_files\":0,\"changed_symbols\":1,\"deleted_symbols\":0,\"moved_symbols\":0,\"removed_calls\":0,\"entry_points\":0,\"nodes\":1}\n" +
		"{\"type\":\"file\",\"status\":\"modified\",\"path\":\"app.go\",\"test\":false,\"language\":\"go\",\"classification\":[\"source\"],\"diffstat\":{\"added_lines\":1,\"deleted_lines\":0,\"added_bytes\":19,\"deleted_bytes\":0},\"symbols\":[\"app.go::app.F\"],\"content_ref\":{\"kind\":\"workspace_file\",\"path\":\"app.go\"},\"hunks\":[{\"old_start\":4,\"old_lines\":1,\"new_start\":4,\"new_lines\":1,\"lines\":[{\"op\":\"add\",\"new_line\":4,\"content\":\"if a < b && c > d {\"}]}]}\n" +
		"{\"type\":\"changed_symbol\",\"id\":\"app.go::app.F\"}\n" +
		"{\"type\":\"node\",\"id\":\"app.go::app.F\",\"path\":\"app.go\",\"name\":\"F\",\"kind\":\"function\",\"start_line\":3,\"end_line\":5,\"changed\":true,\"changed_lines\":[{\"start\":4,\"end\":4}],\"package\":\"app\"}\n"
	if got := buf.String(); got != want {
		t.Fatalf("jsonl=%q", got)
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
	if got := file["risk_flags"].([]any); !reflect.DeepEqual(got, []any{"large_added_file"}) {
		t.Fatalf("risk_flags=%#v", got)
	}
}
