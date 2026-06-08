package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dhohner/inktrail/internal/diff"
)

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
