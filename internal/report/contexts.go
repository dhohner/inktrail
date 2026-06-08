package report

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/dhohner/inktrail/internal/graph"
)

const maxContextExcerptLines = 80

func declarationContexts(g *graph.Graph, changedByFunc map[string][]ChangedLineRange, exclude map[string]bool, compactPaths map[string]bool) []DeclarationContext {
	out := make([]DeclarationContext, 0, len(changedByFunc))
	changed := map[string]bool{}
	for name, changedLines := range changedByFunc {
		changed[name] = true
		if exclude[name] {
			continue
		}
		fn, ok := g.Functions[name]
		if !ok || !supportsDeclarationContext(fn.Path) || compactPaths[fn.Path] {
			continue
		}
		out = append(out, makeDeclarationContext(fn, name, "changed_declaration", "", changedLines))
	}

	seenRelated := map[string]bool{}
	for changedName := range changedByFunc {
		changedFn, ok := g.Functions[changedName]
		if !ok || exclude[changedName] || !supportsDeclarationContext(changedFn.Path) || compactPaths[changedFn.Path] {
			continue
		}
		changedID := symbolID(changedFn)
		for caller := range g.Callers[changedName] {
			out = appendRelatedContext(out, g, caller, changedID, "direct_caller", changed, exclude, seenRelated, compactPaths)
		}
		for callee := range g.Calls[changedName] {
			out = appendRelatedContext(out, g, callee, changedID, "direct_callee", changed, exclude, seenRelated, compactPaths)
		}
		if enclosing := enclosingDeclaration(g, changedName); enclosing != "" {
			out = appendRelatedContext(out, g, enclosing, changedID, "enclosing_declaration", changed, exclude, seenRelated, compactPaths)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		if out[i].Relationship != out[j].Relationship {
			return out[i].Relationship < out[j].Relationship
		}
		return out[i].RelatedTo < out[j].RelatedTo
	})
	return out
}

func appendRelatedContext(out []DeclarationContext, g *graph.Graph, name, relatedTo, relationship string, changed, exclude, seen, compactPaths map[string]bool) []DeclarationContext {
	if changed[name] || exclude[name] {
		return out
	}
	fn, ok := g.Functions[name]
	if !ok || !supportsDeclarationContext(fn.Path) || compactPaths[fn.Path] {
		return out
	}
	key := name + "\x00" + relationship + "\x00" + relatedTo
	if seen[key] {
		return out
	}
	seen[key] = true
	return append(out, makeDeclarationContext(fn, name, relationship, relatedTo, nil))
}

func makeDeclarationContext(fn graph.Function, name, relationship, relatedTo string, changedLines []ChangedLineRange) DeclarationContext {
	content, truncated, omitted := boundedExcerpt(fn.Source, maxContextExcerptLines)
	return DeclarationContext{
		ID:           symbolID(fn),
		Path:         fn.Path,
		Name:         shortName(name),
		Kind:         contextKind(fn),
		LineRange:    LineRange{Start: fn.StartLine, End: fn.EndLine},
		Relationship: relationship,
		RelatedTo:    relatedTo,
		ChangedLines: changedLines,
		Excerpt:      SourceExcerpt{Content: content, Truncated: truncated, OmittedLines: omitted},
	}
}

func enclosingDeclaration(g *graph.Graph, name string) string {
	fn := g.Functions[name]
	var best string
	for otherName, other := range g.Functions {
		if otherName == name || other.Path != fn.Path || other.StartLine > fn.StartLine || other.EndLine < fn.EndLine {
			continue
		}
		if best == "" || declarationSpan(other) < declarationSpan(g.Functions[best]) {
			best = otherName
		}
	}
	return best
}

func declarationSpan(fn graph.Function) int {
	return fn.EndLine - fn.StartLine
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
