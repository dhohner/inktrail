package report

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dhohner/inktrail/internal/diff"
	"github.com/dhohner/inktrail/internal/graph"
)

func TestDeclarationContextExcerptTruncatesDeterministically(t *testing.T) {
	var lines []string
	for i := 1; i <= maxContextExcerptLines+3; i++ {
		lines = append(lines, fmt.Sprintf("line %03d", i))
	}
	g := &graph.Graph{Functions: map[string]graph.Function{
		"app.F": {Name: "app.F", Path: "app.go", Kind: "function_declaration", StartLine: 1, EndLine: len(lines), Source: strings.Join(lines, "\n")},
	}}

	first := declarationContexts(g, map[string][]ChangedLineRange{"app.F": {{Start: 2, End: 2}}}, nil, nil)
	second := declarationContexts(g, map[string][]ChangedLineRange{"app.F": {{Start: 2, End: 2}}}, nil, nil)

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("contexts first=%#v second=%#v", first, second)
	}
	if first[0].Excerpt != second[0].Excerpt {
		t.Fatalf("excerpt not stable: %#v != %#v", first[0].Excerpt, second[0].Excerpt)
	}
	if !first[0].Excerpt.Truncated || first[0].Excerpt.OmittedLines != 3 {
		t.Fatalf("missing truncation metadata: %#v", first[0].Excerpt)
	}
	gotLines := strings.Split(first[0].Excerpt.Content, "\n")
	if len(gotLines) != maxContextExcerptLines || gotLines[len(gotLines)-1] != "line 080" || strings.Contains(first[0].Excerpt.Content, "line 081") {
		t.Fatalf("excerpt content not bounded: %q", first[0].Excerpt.Content)
	}
}

func TestBuildDoesNotEmitDeclarationContextsForCompactedAddedFiles(t *testing.T) {
	var sourceLines []string
	var hunkLines []diff.HunkLine
	for i := 1; i <= largeAddedFileLines+1; i++ {
		line := fmt.Sprintf("// filler %03d", i)
		sourceLines = append(sourceLines, line)
		hunkLines = append(hunkLines, diff.HunkLine{Op: "add", NewLine: i, Content: line})
	}
	sourceLines[0] = "package app"
	sourceLines[1] = "func F() {"
	sourceLines[len(sourceLines)-1] = "}"
	hunkLines[0].Content = sourceLines[0]
	hunkLines[1].Content = sourceLines[1]
	hunkLines[len(hunkLines)-1].Content = "}"

	g := &graph.Graph{Functions: map[string]graph.Function{
		"app.F": {Name: "app.F", Path: "large.go", Kind: "function_declaration", StartLine: 2, EndLine: len(sourceLines), Source: strings.Join(sourceLines[1:], "\n")},
	}}

	r := Build(g, diff.Result{
		Lines: []diff.Line{{Path: "large.go", LineNo: 2}},
		Files: []diff.FileChange{{Status: "added", Path: "large.go", Hunks: []diff.Hunk{{NewStart: 1, NewLines: len(hunkLines), Lines: hunkLines}}}},
	})

	if len(r.Contexts) != 0 {
		t.Fatalf("compacted added file emitted contexts: %#v", r.Contexts)
	}
}
