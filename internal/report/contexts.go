package report

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/dhohner/inktrail/internal/graph"
)

const maxContextExcerptLines = 80

func declarationContexts(g *graph.Graph, changedByFunc map[string][]ChangedLineRange, exclude map[string]bool) []DeclarationContext {
	out := make([]DeclarationContext, 0, len(changedByFunc))
	for name, changedLines := range changedByFunc {
		if exclude[name] {
			continue
		}
		fn := g.Functions[name]
		if !supportsDeclarationContext(fn.Path) {
			continue
		}
		content, truncated, omitted := boundedExcerpt(fn.Source, maxContextExcerptLines)
		out = append(out, DeclarationContext{
			ID:           symbolID(fn),
			Path:         fn.Path,
			Name:         shortName(name),
			Kind:         contextKind(fn),
			LineRange:    LineRange{Start: fn.StartLine, End: fn.EndLine},
			Relationship: "changed_declaration",
			ChangedLines: changedLines,
			Excerpt:      SourceExcerpt{Content: content, Truncated: truncated, OmittedLines: omitted},
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func supportsDeclarationContext(path string) bool {
	switch filepath.Ext(path) {
	case ".go", ".java":
		return true
	default:
		return false
	}
}

func contextKind(fn graph.Function) string {
	switch fn.Kind {
	case "function_declaration":
		return "function"
	case "method_declaration":
		return "method"
	case "constructor_declaration", "compact_constructor_declaration":
		return "constructor"
	case "class_declaration":
		return "class"
	case "interface_declaration":
		return "interface"
	case "enum_declaration":
		return "enum"
	case "record_declaration":
		return "record"
	case "lambda_expression":
		return "lambda"
	case "object_creation_expression":
		return "anonymous_class"
	default:
		return kind(fn.Name)
	}
}

func boundedExcerpt(source string, maxLines int) (string, bool, int) {
	if maxLines <= 0 {
		lines := splitLines(source)
		return "", len(lines) > 0, len(lines)
	}
	lines := splitLines(source)
	if len(lines) <= maxLines {
		return source, false, 0
	}
	return strings.Join(lines[:maxLines], "\n"), true, len(lines) - maxLines
}

func splitLines(source string) []string {
	if source == "" {
		return nil
	}
	trimmed := strings.TrimSuffix(source, "\n")
	if trimmed == "" {
		return []string{""}
	}
	return strings.Split(trimmed, "\n")
}
