package golang

import (
	"path/filepath"
	"strings"

	"github.com/dhohner/inktrail/internal/analyzer"
	"github.com/dhohner/inktrail/internal/parser"
	"github.com/dhohner/inktrail/internal/source"
)

type Analyzer struct{}

func (Analyzer) Language() parser.Language { return parser.LanguageGo }

func (Analyzer) IsProductionPath(path string) bool {
	return strings.HasSuffix(path, ".go") && !source.IsGoTestPath(path)
}

func (Analyzer) PackageName(root *parser.Node, source []byte) string {
	for _, child := range root.NamedChildren() {
		if child.Kind() != "package_clause" {
			continue
		}
		for _, pkgChild := range child.NamedChildren() {
			if pkgChild.Kind() == "package_identifier" {
				return pkgChild.Text(source)
			}
		}
	}
	return "unknown"
}

func (Analyzer) Symbols(src analyzer.Source) []analyzer.Symbol {
	var out []analyzer.Symbol
	for _, fn := range functionDeclarations(src.Doc.RootNode()) {
		if body := fn.ChildByFieldName("body"); body == nil {
			continue
		}
		out = append(out, analyzer.Symbol{Name: functionName(src.Package, &fn, src.Source), Node: fn})
	}
	return out
}

func (Analyzer) Calls(src analyzer.Source) []analyzer.Call {
	var out []analyzer.Call
	imports := importNames(src.Doc.RootNode(), src.Source)
	for _, fn := range functionDeclarations(src.Doc.RootNode()) {
		body := fn.ChildByFieldName("body")
		if body == nil {
			continue
		}
		from := functionName(src.Package, &fn, src.Source)
		walk(body, func(n *parser.Node) {
			if n.Kind() != "call_expression" {
				return
			}
			to := callName(n.ChildByFieldName("function"), imports, src.Source)
			if to == "" {
				return
			}
			r := n.Range()
			out = append(out, analyzer.Call{From: from, To: to, Site: analyzer.CallSite{Path: src.Path, LineNo: r.StartLine, Code: sourceLine(src.Source, r.StartLine)}})
		})
	}
	return out
}

func functionDeclarations(root *parser.Node) []parser.Node {
	var out []parser.Node
	for _, child := range root.NamedChildren() {
		switch child.Kind() {
		case "function_declaration", "method_declaration":
			out = append(out, child)
		}
	}
	return out
}

func functionName(pkg string, fn *parser.Node, source []byte) string {
	name := "unknown"
	if nameNode := fn.ChildByFieldName("name"); nameNode != nil {
		name = nameNode.Text(source)
	}
	if fn.Kind() != "method_declaration" {
		return pkg + "." + name
	}
	return pkg + "." + receiverName(fn.ChildByFieldName("receiver"), source) + "." + name
}

func receiverName(receiver *parser.Node, source []byte) string {
	if receiver == nil {
		return "unknown"
	}
	var found string
	walk(receiver, func(n *parser.Node) {
		if found != "" {
			return
		}
		switch n.Kind() {
		case "type_identifier", "qualified_type":
			found = lastSelectorPart(n.Text(source))
		}
	})
	if found == "" {
		return "unknown"
	}
	return found
}

func callName(expr *parser.Node, imports map[string]bool, source []byte) string {
	if expr == nil {
		return ""
	}
	switch expr.Kind() {
	case "identifier":
		return expr.Text(source)
	case "selector_expression":
		field := expr.ChildByFieldName("field")
		if field == nil {
			return ""
		}
		operand := expr.ChildByFieldName("operand")
		if recv := selectorReceiver(operand, source); recv != "" {
			return recv + "." + field.Text(source)
		}
		if operand != nil && operand.Kind() == "identifier" && imports[operand.Text(source)] {
			return ""
		}
		return field.Text(source)
	default:
		return ""
	}
}

func importNames(root *parser.Node, source []byte) map[string]bool {
	imports := map[string]bool{}
	walk(root, func(n *parser.Node) {
		if n.Kind() != "import_spec" {
			return
		}
		children := n.NamedChildren()
		if len(children) == 0 {
			return
		}
		if children[0].Kind() == "package_identifier" {
			name := children[0].Text(source)
			if name != "." && name != "_" {
				imports[name] = true
			}
			return
		}
		for _, child := range children {
			if child.Kind() == "interpreted_string_literal" || child.Kind() == "raw_string_literal" {
				path := strings.Trim(child.Text(source), "\"`")
				if _, name := filepath.Split(path); name != "" {
					imports[name] = true
				}
			}
		}
	})
	return imports
}

func selectorReceiver(expr *parser.Node, source []byte) string {
	if expr == nil {
		return ""
	}
	switch expr.Kind() {
	case "composite_literal":
		return typeName(expr.ChildByFieldName("type"), source)
	case "unary_expression", "parenthesized_expression":
		for _, child := range expr.NamedChildren() {
			c := child
			if recv := selectorReceiver(&c, source); recv != "" {
				return recv
			}
		}
	}
	return ""
}

func typeName(expr *parser.Node, source []byte) string {
	if expr == nil {
		return ""
	}
	switch expr.Kind() {
	case "type_identifier", "identifier", "field_identifier":
		return expr.Text(source)
	case "qualified_type", "selector_expression":
		return lastSelectorPart(expr.Text(source))
	case "pointer_type":
		for _, child := range expr.NamedChildren() {
			c := child
			if name := typeName(&c, source); name != "" {
				return name
			}
		}
	}
	return ""
}

func walk(root *parser.Node, visit func(*parser.Node)) {
	if root == nil {
		return
	}
	visit(root)
	for _, child := range root.NamedChildren() {
		c := child
		walk(&c, visit)
	}
}

func sourceLine(source []byte, lineNo int) string {
	lines := strings.Split(string(source), "\n")
	if lineNo < 1 || lineNo > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[lineNo-1])
}

func lastSelectorPart(text string) string {
	text = strings.TrimPrefix(text, "*")
	parts := strings.Split(text, ".")
	return parts[len(parts)-1]
}
