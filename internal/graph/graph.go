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
}

type sourceFile struct {
	Path   string
	Source []byte
}

func Build(root string) (*Graph, error) {
	var files []sourceFile
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || isTestPath(path) {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files = append(files, sourceFile{Path: clean(root, path), Source: source})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return buildFromSources(files)
}

func BuildGit(ref string) (*Graph, error) {
	out, err := exec.Command("git", "ls-tree", "-r", "--name-only", ref).Output()
	if err != nil {
		return nil, gitErr("git ls-tree failed", err)
	}

	var files []sourceFile
	for _, path := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if path == "" || !strings.HasSuffix(path, ".go") || isTestPath(path) {
			continue
		}
		source, err := exec.Command("git", "show", ref+":"+path).Output()
		if err != nil {
			return nil, gitErr(fmt.Sprintf("git show failed for %s", path), err)
		}
		files = append(files, sourceFile{Path: path, Source: source})
	}
	return buildFromSources(files)
}

func buildFromSources(files []sourceFile) (*Graph, error) {
	g := &Graph{Functions: map[string]Function{}, Calls: map[string]map[string]bool{}, Callers: map[string]map[string]bool{}, CallSites: map[string]map[string]CallSite{}}
	fset := token.NewFileSet()
	parsed := map[string]*ast.File{}

	for _, sf := range files {
		file, err := parser.ParseFile(fset, sf.Path, sf.Source, 0)
		if err != nil {
			return nil, err
		}
		parsed[sf.Path] = file
		pkg := file.Name.Name
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := functionName(pkg, fn)
			g.Functions[name] = Function{Name: name, Path: sf.Path, StartLine: fset.Position(fn.Pos()).Line, EndLine: fset.Position(fn.End()).Line}
		}
	}

	for _, sf := range files {
		file := parsed[sf.Path]
		pkg := file.Name.Name
		for _, decl := range file.Decls {
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
				to := callName(call.Fun)
				if to == "" {
					return true
				}
				for candidate := range g.Functions {
					if candidate == pkg+"."+to || strings.HasSuffix(candidate, "."+to) {
						pos := fset.Position(call.Pos())
						g.addEdge(from, candidate, CallSite{Path: sf.Path, LineNo: pos.Line, Code: sourceLine(sf.Source, pos.Line)})
					}
				}
				return true
			})
		}
	}
	return g, nil
}

func (g *Graph) CallSite(from, to string) (CallSite, bool) {
	calls, ok := g.CallSites[from]
	if !ok {
		return CallSite{}, false
	}
	call, ok := calls[to]
	return call, ok
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

func callName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		if recv := selectorReceiver(x.X); recv != "" {
			return recv + "." + x.Sel.Name
		}
		return x.Sel.Name
	default:
		return ""
	}
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

func isTestPath(path string) bool {
	path = filepath.ToSlash(path)
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == "test" || part == "tests" || part == "testdata" {
			return true
		}
	}
	return strings.HasSuffix(path, "_test.go")
}

func gitErr(prefix string, err error) error {
	if exit, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("%s: %s", prefix, bytes.TrimSpace(exit.Stderr))
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
