package graph

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	sharedparser "github.com/dhohner/inktrail/internal/parser"
	"github.com/dhohner/inktrail/internal/source"
)

const (
	gitDir    = ".git"
	vendorDir = "vendor"
)

type Function struct {
	Name      string
	Path      string
	StartLine int
	EndLine   int
	Source    string
}

type CallSite struct {
	Path   string
	LineNo int
	Code   string
}

type Graph struct {
	Functions map[string]Function
	Calls     map[string]map[string]bool
	Callers   map[string]map[string]bool
	CallSites map[string]map[string]CallSite

	functionsByCallName map[string][]string
	functionsByPath     map[string][]string
}

type sourceFile struct {
	Path   string
	Source []byte
}

type parsedSource struct {
	Path    string
	Source  []byte
	Doc     *sharedparser.Document
	Package string
	Imports map[string]bool
}

func Build(root string) (*Graph, error) {
	files, err := loadFiles(root)
	if err != nil {
		return nil, err
	}
	return buildFromSources(files)
}

func BuildGit(ref string) (*Graph, error) {
	files, err := loadGitFiles(ref)
	if err != nil {
		return nil, err
	}
	return buildFromSources(files)
}

func loadFiles(root string) ([]sourceFile, error) {
	var files []sourceFile
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !isProductionGoFile(path) {
			return nil
		}
		sourceBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, sourceFile{Path: clean(root, path), Source: sourceBytes})
		return nil
	})
	return files, err
}

func loadGitFiles(ref string) ([]sourceFile, error) {
	out, err := exec.Command("git", "ls-tree", "-r", "--name-only", ref).Output()
	if err != nil {
		return nil, gitErr("git ls-tree failed", err)
	}

	var files []sourceFile
	for _, path := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if path == "" || !isProductionGoFile(path) {
			continue
		}
		sourceBytes, err := exec.Command("git", "show", ref+":"+path).Output()
		if err != nil {
			return nil, gitErr(fmt.Sprintf("git show failed for %s", path), err)
		}
		files = append(files, sourceFile{Path: path, Source: sourceBytes})
	}
	return files, nil
}

func buildFromSources(files []sourceFile) (*Graph, error) {
	g := newGraph()
	parsed, err := parseSources(files)
	if err != nil {
		return nil, err
	}

	defer closeParsedSources(parsed)

	for _, ps := range parsed {
		g.addFunctions(ps)
	}
	for _, ps := range parsed {
		g.addCalls(ps)
	}
	return g, nil
}

func newGraph() *Graph {
	return &Graph{
		Functions:           map[string]Function{},
		Calls:               map[string]map[string]bool{},
		Callers:             map[string]map[string]bool{},
		CallSites:           map[string]map[string]CallSite{},
		functionsByCallName: map[string][]string{},
		functionsByPath:     map[string][]string{},
	}
}

func parseSources(files []sourceFile) ([]parsedSource, error) {
	parsed := make([]parsedSource, 0, len(files))
	for _, sf := range files {
		doc, err := sharedparser.Parse(sharedparser.LanguageGo, sf.Source)
		if err != nil {
			closeParsedSources(parsed)
			return nil, err
		}
		if doc.HasSyntaxError() {
			doc.Close()
			closeParsedSources(parsed)
			return nil, fmt.Errorf("parse %s: syntax error", sf.Path)
		}
		ps := parsedSource{Path: sf.Path, Source: sf.Source, Doc: doc, Imports: map[string]bool{}}
		ps.Package = packageName(doc.RootNode(), sf.Source)
		ps.Imports = importNames(doc.RootNode(), sf.Source)
		parsed = append(parsed, ps)
	}
	return parsed, nil
}

func closeParsedSources(parsed []parsedSource) {
	for _, ps := range parsed {
		ps.Doc.Close()
	}
}

func (g *Graph) addFunctions(ps parsedSource) {
	for _, fn := range functionDeclarations(ps.Doc.RootNode()) {
		body := fn.ChildByFieldName("body")
		if body == nil {
			continue
		}
		name := functionName(ps.Package, &fn, ps.Source)
		r := fn.Range()
		g.Functions[name] = Function{Name: name, Path: ps.Path, StartLine: r.StartLine, EndLine: r.EndLine, Source: nodeSource(ps.Source, int(r.StartByte), int(r.EndByte))}
		g.indexFunction(name)
	}
}

func (g *Graph) addCalls(ps parsedSource) {
	for _, fn := range functionDeclarations(ps.Doc.RootNode()) {
		body := fn.ChildByFieldName("body")
		if body == nil {
			continue
		}
		from := functionName(ps.Package, &fn, ps.Source)
		walk(body, func(n *sharedparser.Node) {
			if n.Kind() != "call_expression" {
				return
			}
			to := callName(n.ChildByFieldName("function"), ps.Imports, ps.Source)
			if to == "" {
				return
			}
			r := n.Range()
			site := CallSite{Path: ps.Path, LineNo: r.StartLine, Code: sourceLine(ps.Source, r.StartLine)}
			for _, candidate := range g.resolveCalls(ps.Package, to) {
				g.addEdge(from, candidate, site)
			}
		})
	}
}

func (g *Graph) indexFunction(name string) {
	fn := g.Functions[name]
	g.functionsByPath[fn.Path] = append(g.functionsByPath[fn.Path], name)

	seen := map[string]bool{}
	for _, key := range []string{shortCallableName(name), lastNameSegment(name)} {
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		g.functionsByCallName[key] = append(g.functionsByCallName[key], name)
	}
}

func (g *Graph) resolveCalls(pkg, to string) []string {
	if fn, ok := g.Functions[pkg+"."+to]; ok {
		return []string{fn.Name}
	}
	return g.functionsByCallName[to]
}

func (g *Graph) CallSite(from, to string) (CallSite, bool) {
	calls, ok := g.CallSites[from]
	if !ok {
		return CallSite{}, false
	}
	call, ok := calls[to]
	return call, ok
}

// FunctionsContainingLine returns production functions in path whose range includes lineNo.
func (g *Graph) FunctionsContainingLine(path string, lineNo int) []Function {
	var out []Function
	if g.functionsByPath == nil {
		for _, fn := range g.Functions {
			if fn.Path == path && lineNo >= fn.StartLine && lineNo <= fn.EndLine {
				out = append(out, fn)
			}
		}
		return out
	}
	for _, name := range g.functionsByPath[path] {
		fn := g.Functions[name]
		if lineNo >= fn.StartLine && lineNo <= fn.EndLine {
			out = append(out, fn)
		}
	}
	return out
}

func (g *Graph) addEdge(from, to string, call CallSite) {
	if from == to {
		return
	}
	if g.Calls[from] == nil {
		g.Calls[from] = map[string]bool{}
	}
	if g.Callers[to] == nil {
		g.Callers[to] = map[string]bool{}
	}
	if g.CallSites[from] == nil {
		g.CallSites[from] = map[string]CallSite{}
	}
	g.Calls[from][to] = true
	g.Callers[to][from] = true
	g.CallSites[from][to] = call
}

func packageName(root *sharedparser.Node, source []byte) string {
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

func functionDeclarations(root *sharedparser.Node) []sharedparser.Node {
	var out []sharedparser.Node
	for _, child := range root.NamedChildren() {
		switch child.Kind() {
		case "function_declaration", "method_declaration":
			out = append(out, child)
		}
	}
	return out
}

func functionName(pkg string, fn *sharedparser.Node, source []byte) string {
	name := "unknown"
	if nameNode := fn.ChildByFieldName("name"); nameNode != nil {
		name = nameNode.Text(source)
	}
	if fn.Kind() != "method_declaration" {
		return pkg + "." + name
	}
	return pkg + "." + recvName(fn.ChildByFieldName("receiver"), source) + "." + name
}

func recvName(receiver *sharedparser.Node, source []byte) string {
	if receiver == nil {
		return "unknown"
	}
	var found string
	walk(receiver, func(n *sharedparser.Node) {
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

func callName(expr *sharedparser.Node, imports map[string]bool, source []byte) string {
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

func importNames(root *sharedparser.Node, source []byte) map[string]bool {
	imports := map[string]bool{}
	walk(root, func(n *sharedparser.Node) {
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

func selectorReceiver(expr *sharedparser.Node, source []byte) string {
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

func typeName(expr *sharedparser.Node, source []byte) string {
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

func walk(root *sharedparser.Node, visit func(*sharedparser.Node)) {
	if root == nil {
		return
	}
	visit(root)
	for _, child := range root.NamedChildren() {
		c := child
		walk(&c, visit)
	}
}

func lastSelectorPart(text string) string {
	text = strings.TrimPrefix(text, "*")
	parts := strings.Split(text, ".")
	return parts[len(parts)-1]
}

func sourceLine(source []byte, lineNo int) string {
	lines := strings.Split(string(source), "\n")
	if lineNo < 1 || lineNo > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[lineNo-1])
}

func nodeSource(source []byte, start, end int) string {
	if start < 0 || end < start || start > len(source) {
		return ""
	}
	if end > len(source) {
		end = len(source)
	}
	return string(source[start:end])
}

func clean(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}

func shortCallableName(name string) string {
	parts := strings.Split(name, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[1:], ".")
	}
	return name
}

func lastNameSegment(name string) string {
	parts := strings.Split(name, ".")
	return parts[len(parts)-1]
}

func shouldSkipDir(name string) bool {
	return name == gitDir || name == vendorDir
}

func isProductionGoFile(path string) bool {
	return strings.HasSuffix(path, ".go") && !source.IsGoTestPath(path)
}

func gitErr(prefix string, err error) error {
	if exit, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("%s: %s", prefix, bytes.TrimSpace(exit.Stderr))
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
