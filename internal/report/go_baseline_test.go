package report

import (
	"bytes"
	"testing"

	"github.com/dhohner/inktrail/internal/diff"
	"github.com/dhohner/inktrail/internal/graph"
)

func TestGoReportJSONLBaseline(t *testing.T) {
	currentDir := t.TempDir()
	write(t, currentDir, "api.go", `package app

type Controller struct{}
type Service struct{}
type Repo struct{}

func (c Controller) Handle() { Service{}.Do() }
func (s Service) Do() { println("new"); Repo{}.Find() }
func (r Repo) Find() {}
`)
	write(t, currentDir, "worker_new.go", `package app

func Worker() { println("moved and changed") }
`)
	write(t, currentDir, "api_test.go", `package app

import "testing"

func TestService(t *testing.T) { Service{}.Do() }
`)

	baseDir := t.TempDir()
	write(t, baseDir, "api.go", `package app

type Controller struct{}
type Service struct{}
type Repo struct{}
type Old struct{}

func (c Controller) Handle() { Service{}.Do() }
func (s Service) Do() { Repo{}.Find(); Old{}.Gone() }
func (r Repo) Find() {}
func (o Old) Gone() {}
`)
	write(t, baseDir, "worker_old.go", `package app

func Worker() { println("old") }
`)
	write(t, baseDir, "api_test.go", `package app

import "testing"

func TestService(t *testing.T) { Service{}.Do() }
`)

	current, err := graph.Build(currentDir)
	if err != nil {
		t.Fatal(err)
	}
	base, err := graph.Build(baseDir)
	if err != nil {
		t.Fatal(err)
	}

	r := BuildWithBase(current, base, diff.Result{
		Lines: []diff.Line{{Path: "api.go", LineNo: 8}, {Path: "worker_new.go", LineNo: 3}, {Path: "api_test.go", LineNo: 5}},
		Files: []diff.FileChange{
			{Status: "modified", Path: "api.go", Hunks: []diff.Hunk{{OldStart: 8, OldLines: 1, NewStart: 8, NewLines: 1, Lines: []diff.HunkLine{
				{Op: "delete", OldLine: 8, Content: `func (s Service) Do() { Repo{}.Find(); Old{}.Gone() }`},
				{Op: "add", NewLine: 8, Content: `func (s Service) Do() { println("new"); Repo{}.Find() }`},
			}}}},
			{Status: "renamed", OldPath: "worker_old.go", Path: "worker_new.go", Hunks: []diff.Hunk{{OldStart: 3, OldLines: 1, NewStart: 3, NewLines: 1, Lines: []diff.HunkLine{
				{Op: "delete", OldLine: 3, Content: `func Worker() { println("old") }`},
				{Op: "add", NewLine: 3, Content: `func Worker() { println("moved and changed") }`},
			}}}},
			{Status: "modified", Path: "api_test.go", Test: true, Hunks: []diff.Hunk{{OldStart: 5, OldLines: 1, NewStart: 5, NewLines: 1, Lines: []diff.HunkLine{
				{Op: "context", OldLine: 5, NewLine: 5, Content: `func TestService(t *testing.T) { Service{}.Do() }`},
			}}}},
		},
	})

	var buf bytes.Buffer
	if err := WriteJSONL(&buf, r); err != nil {
		t.Fatal(err)
	}

	want := "{\"type\":\"summary\",\"schema_version\":\"1.0\",\"files\":3,\"test_files\":1,\"changed_symbols\":2,\"deleted_symbols\":1,\"moved_symbols\":1,\"removed_calls\":1,\"entry_points\":2,\"nodes\":3,\"context_records\":{\"total\":4,\"declaration_context\":2,\"related_declaration_context\":2}}\n" +
		"{\"type\":\"file\",\"status\":\"modified\",\"path\":\"api.go\",\"test\":false,\"language\":\"go\",\"classification\":[\"source\"],\"diffstat\":{\"added_lines\":1,\"deleted_lines\":1,\"added_bytes\":55,\"deleted_bytes\":53},\"symbols\":[\"api.go::app.Controller.Handle\",\"api.go::app.Service.Do\"],\"content_ref\":{\"kind\":\"workspace_file\",\"path\":\"api.go\"},\"hunks\":[{\"old_start\":8,\"old_lines\":1,\"new_start\":8,\"new_lines\":1,\"lines\":[{\"op\":\"delete\",\"old_line\":8,\"content\":\"func (s Service) Do() { Repo{}.Find(); Old{}.Gone() }\"},{\"op\":\"add\",\"new_line\":8,\"content\":\"func (s Service) Do() { println(\\\"new\\\"); Repo{}.Find() }\"}]}]}\n" +
		"{\"type\":\"file\",\"status\":\"renamed\",\"old_path\":\"worker_old.go\",\"path\":\"worker_new.go\",\"test\":false,\"language\":\"go\",\"classification\":[\"source\"],\"diffstat\":{\"added_lines\":1,\"deleted_lines\":1,\"added_bytes\":46,\"deleted_bytes\":32},\"symbols\":[\"worker_new.go::app.Worker\"],\"content_ref\":{\"kind\":\"workspace_file\",\"path\":\"worker_new.go\"},\"hunks\":[{\"old_start\":3,\"old_lines\":1,\"new_start\":3,\"new_lines\":1,\"lines\":[{\"op\":\"delete\",\"old_line\":3,\"content\":\"func Worker() { println(\\\"old\\\") }\"},{\"op\":\"add\",\"new_line\":3,\"content\":\"func Worker() { println(\\\"moved and changed\\\") }\"}]}]}\n" +
		"{\"type\":\"file\",\"status\":\"modified\",\"path\":\"api_test.go\",\"test\":true,\"language\":\"go\",\"classification\":[\"test\"],\"diffstat\":{\"added_lines\":0,\"deleted_lines\":0,\"added_bytes\":0,\"deleted_bytes\":0},\"content_ref\":{\"kind\":\"workspace_file\",\"path\":\"api_test.go\"},\"hunks\":[{\"old_start\":5,\"old_lines\":1,\"new_start\":5,\"new_lines\":1,\"lines\":[{\"op\":\"context\",\"old_line\":5,\"new_line\":5,\"content\":\"func TestService(t *testing.T) { Service{}.Do() }\"}]}]}\n" +
		"{\"type\":\"changed_symbol\",\"id\":\"api.go::app.Service.Do\"}\n" +
		"{\"type\":\"changed_symbol\",\"id\":\"worker_new.go::app.Worker\"}\n" +
		"{\"type\":\"deleted_symbol\",\"id\":\"api.go::app.Old.Gone\"}\n" +
		"{\"type\":\"declaration_context\",\"id\":\"api.go::app.Controller.Handle\",\"path\":\"api.go\",\"name\":\"Handle\",\"kind\":\"method\",\"line_range\":{\"start\":7,\"end\":7},\"relationship\":\"direct_caller\",\"related_to\":\"api.go::app.Service.Do\",\"excerpt\":{\"content\":\"func (c Controller) Handle() { Service{}.Do() }\",\"truncated\":false,\"omitted_lines\":0}}\n" +
		"{\"type\":\"declaration_context\",\"id\":\"api.go::app.Repo.Find\",\"path\":\"api.go\",\"name\":\"Find\",\"kind\":\"method\",\"line_range\":{\"start\":9,\"end\":9},\"relationship\":\"direct_callee\",\"related_to\":\"api.go::app.Service.Do\",\"excerpt\":{\"content\":\"func (r Repo) Find() {}\",\"truncated\":false,\"omitted_lines\":0}}\n" +
		"{\"type\":\"declaration_context\",\"id\":\"api.go::app.Service.Do\",\"path\":\"api.go\",\"name\":\"Do\",\"kind\":\"method\",\"line_range\":{\"start\":8,\"end\":8},\"relationship\":\"changed_declaration\",\"changed_lines\":[{\"start\":8,\"end\":8}],\"excerpt\":{\"content\":\"func (s Service) Do() { println(\\\"new\\\"); Repo{}.Find() }\",\"truncated\":false,\"omitted_lines\":0}}\n" +
		"{\"type\":\"declaration_context\",\"id\":\"worker_new.go::app.Worker\",\"path\":\"worker_new.go\",\"name\":\"Worker\",\"kind\":\"function\",\"line_range\":{\"start\":3,\"end\":3},\"relationship\":\"changed_declaration\",\"changed_lines\":[{\"start\":3,\"end\":3}],\"excerpt\":{\"content\":\"func Worker() { println(\\\"moved and changed\\\") }\",\"truncated\":false,\"omitted_lines\":0}}\n" +
		"{\"type\":\"moved_symbol\",\"from\":\"worker_old.go::app.Worker\",\"to\":\"worker_new.go::app.Worker\",\"body_sha256_equal\":false}\n" +
		"{\"type\":\"removed_call\",\"from\":\"api.go::app.Service.Do\",\"to\":\"api.go::app.Old.Gone\",\"call_site\":{\"path\":\"api.go\",\"line\":9}}\n" +
		"{\"type\":\"entry_point\",\"id\":\"api.go::app.Controller.Handle\"}\n" +
		"{\"type\":\"entry_point\",\"id\":\"worker_new.go::app.Worker\"}\n" +
		"{\"type\":\"node\",\"id\":\"api.go::app.Controller.Handle\",\"path\":\"api.go\",\"name\":\"Handle\",\"kind\":\"method\",\"start_line\":7,\"end_line\":7,\"calls\":[{\"to\":\"api.go::app.Service.Do\",\"call_site\":{\"path\":\"api.go\",\"line\":7}}],\"changed\":false,\"package\":\"app\"}\n" +
		"{\"type\":\"node\",\"id\":\"api.go::app.Service.Do\",\"path\":\"api.go\",\"name\":\"Do\",\"kind\":\"method\",\"start_line\":8,\"end_line\":8,\"changed\":true,\"changed_lines\":[{\"start\":8,\"end\":8}],\"package\":\"app\"}\n" +
		"{\"type\":\"node\",\"id\":\"worker_new.go::app.Worker\",\"path\":\"worker_new.go\",\"name\":\"Worker\",\"kind\":\"function\",\"start_line\":3,\"end_line\":3,\"changed\":true,\"changed_lines\":[{\"start\":3,\"end\":3}],\"package\":\"app\"}\n"
	if got := buf.String(); got != want {
		t.Fatalf("jsonl baseline drifted:\n%s", got)
	}
}
