package graph

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"inktrail/internal/source"
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
	Path   string
	Source []byte
	File   *ast.File
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
	fset := token.NewFileSet()
	parsed, err := parseSources(fset, files)
	if err != nil {
		return nil, err
	}

	for _, ps := range parsed {
		g.addFunctions(fset, ps)
	}
	for _, ps := range parsed {
		g.addCalls(fset, ps)
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

func parseSources(fset *token.FileSet, files []sourceFile) ([]parsedSource, error) {
	parsed := make([]parsedSource, 0, len(files))
	for _, sf := range files {
		file, err := parser.ParseFile(fset, sf.Path, sf.Source, 0)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, parsedSource{Path: sf.Path, Source: sf.Source, File: file})
	}
	return parsed, nil
}

func (g *Graph) addFunctions(fset *token.FileSet, ps parsedSource) {
	pkg := ps.File.Name.Name
	for _, decl := range ps.File.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		name := functionName(pkg, fn)
		g.Functions[name] = Function{Name: name, Path: ps.Path, StartLine: fset.Position(fn.Pos()).Line, EndLine: fset.Position(fn.End()).Line}
		g.indexFunction(name)
	}
}

func (g *Graph) addCalls(fset *token.FileSet, ps parsedSource) {
	pkg := ps.File.Name.Name
	imports := importNames(ps.File)
	for _, decl := range ps.File.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		from := functionName(pkg, fn)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			to := callName(call.Fun, imports)
			if to == "" {
				return true
			}
			pos := fset.Position(call.Pos())
			site := CallSite{Path: ps.Path, LineNo: pos.Line, Code: sourceLine(ps.Source, pos.Line)}
			for _, candidate := range g.resolveCalls(pkg, to) {
				g.addEdge(from, candidate, site)
			}
			return true
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

func functionName(pkg string, fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return pkg + "." + fn.Name.Name
	}
	return pkg + "." + recvName(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

func recvName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.StarExpr:
		return recvName(x.X)
	default:
		return "unknown"
	}
}

func callName(expr ast.Expr, imports map[string]bool) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		if recv := selectorReceiver(x.X); recv != "" {
			return recv + "." + x.Sel.Name
		}
		if ident, ok := x.X.(*ast.Ident); ok && imports[ident.Name] {
			return ""
		}
		return x.Sel.Name
	default:
		return ""
	}
}

func importNames(file *ast.File) map[string]bool {
	imports := map[string]bool{}
	for _, spec := range file.Imports {
		if spec.Name != nil {
			if spec.Name.Name != "." && spec.Name.Name != "_" {
				imports[spec.Name.Name] = true
			}
			continue
		}
		path := strings.Trim(spec.Path.Value, "\"")
		if _, name := filepath.Split(path); name != "" {
			imports[name] = true
		}
	}
	return imports
}

func selectorReceiver(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.CompositeLit:
		return typeName(x.Type)
	case *ast.UnaryExpr:
		if x.Op == token.AND {
			return selectorReceiver(x.X)
		}
	}
	return ""
}

func typeName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	case *ast.StarExpr:
		return typeName(x.X)
	}
	return ""
}

func sourceLine(source []byte, lineNo int) string {
	lines := strings.Split(string(source), "\n")
	if lineNo < 1 || lineNo > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[lineNo-1])
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
