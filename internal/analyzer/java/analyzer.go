package java

import (
	"fmt"
	"os"
	"strings"

	"github.com/dhohner/inktrail/internal/analyzer"
	"github.com/dhohner/inktrail/internal/parser"
	"github.com/dhohner/inktrail/internal/source"
)

type Analyzer struct{}

func (Analyzer) Language() parser.Language { return parser.LanguageJava }

func (Analyzer) IsProductionPath(path string) bool {
	return strings.HasSuffix(path, ".java") && !source.IsJavaTestPath(path)
}

func (Analyzer) PackageName(root *parser.Node, source []byte) string {
	for _, child := range root.NamedChildren() {
		if child.Kind() != "package_declaration" {
			continue
		}
		children := child.NamedChildren()
		if len(children) > 0 {
			return children[len(children)-1].Text(source)
		}
	}
	return "default"
}

func (Analyzer) Symbols(src analyzer.Source) []analyzer.Symbol {
	var out []analyzer.Symbol
	var walkJava func(*parser.Node, []string)
	walkJava = func(n *parser.Node, owners []string) {
		if n == nil {
			return
		}
		switch n.Kind() {
		case "class_declaration", "interface_declaration", "enum_declaration", "record_declaration":
			name := javaName(n, src.Source)
			if name == "" {
				javaWarnUnstable(n)
				return
			}
			qualified := append(append([]string{}, owners...), name)
			out = append(out, analyzer.Symbol{Name: javaQualifiedName(src.Package, qualified), Node: *n})
			for _, child := range n.NamedChildren() {
				c := child
				walkJava(&c, qualified)
			}
			return
		case "method_declaration", "constructor_declaration", "compact_constructor_declaration":
			name := javaName(n, src.Source)
			if n.Kind() != "method_declaration" {
				name = "<init>"
			}
			if name == "" || len(owners) == 0 {
				javaWarnUnstable(n)
			} else {
				out = append(out, analyzer.Symbol{Name: javaQualifiedName(src.Package, append(append([]string{}, owners...), name)), Node: *n})
			}
		case "lambda_expression":
			if len(owners) == 0 {
				javaWarnUnstable(n)
			} else {
				r := n.Range()
				out = append(out, analyzer.Symbol{Name: javaQualifiedName(src.Package, append(append([]string{}, owners...), fmt.Sprintf("lambda@%d", r.StartLine))), Node: *n})
			}
		case "object_creation_expression":
			if javaHasChildKind(n, "class_body") {
				if len(owners) == 0 {
					javaWarnUnstable(n)
				} else {
					r := n.Range()
					anonOwners := append(append([]string{}, owners...), fmt.Sprintf("anonymous@%d", r.StartLine))
					out = append(out, analyzer.Symbol{Name: javaQualifiedName(src.Package, anonOwners), Node: *n})
					for _, child := range n.NamedChildren() {
						c := child
						walkJava(&c, anonOwners)
					}
					return
				}
			}
		}
		for _, child := range n.NamedChildren() {
			c := child
			walkJava(&c, owners)
		}
	}
	walkJava(src.Doc.RootNode(), nil)
	return out
}

func (Analyzer) Calls(analyzer.Source) []analyzer.Call {
	return nil
}

func javaName(n *parser.Node, source []byte) string {
	if name := n.ChildByFieldName("name"); name != nil {
		return name.Text(source)
	}
	for _, child := range n.NamedChildren() {
		switch child.Kind() {
		case "identifier", "type_identifier":
			return child.Text(source)
		}
	}
	return ""
}

func javaHasChildKind(n *parser.Node, kind string) bool {
	for _, child := range n.NamedChildren() {
		if child.Kind() == kind {
			return true
		}
	}
	return false
}

func javaQualifiedName(pkg string, parts []string) string {
	prefix := pkg
	if prefix == "" {
		prefix = "default"
	}
	return prefix + "." + strings.Join(parts, ".")
}

func javaWarnUnstable(n *parser.Node) {
	if n == nil {
		return
	}
	r := n.Range()
	fmt.Fprintf(os.Stderr, "inktrail: Java symbol extraction skipped unstable %s at line %d\n", n.Kind(), r.StartLine)
}
