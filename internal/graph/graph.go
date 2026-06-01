package graph

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"inktrail/internal/changes"
)

type Function struct {
	Name      string
	Path      string
	StartLine int
	EndLine   int
}

type Graph struct {
	Functions map[string]Function
	Calls     map[string]map[string]bool
	Callers   map[string]map[string]bool
}

func Build(root string) (*Graph, error) {
	g := &Graph{Functions: map[string]Function{}, Calls: map[string]map[string]bool{}, Callers: map[string]map[string]bool{}}
	fset := token.NewFileSet()

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

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}

		pkg := file.Name.Name
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			name := functionName(pkg, fn)
			g.Functions[name] = Function{
				Name:      name,
				Path:      clean(root, path),
				StartLine: fset.Position(fn.Pos()).Line,
				EndLine:   fset.Position(fn.End()).Line,
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
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

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
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
						g.addEdge(from, candidate)
					}
				}
				return true
			})
		}
		return nil
	})
	return g, err
}

func (g *Graph) ChainsForChanged(lines []changes.Line) [][]string {
	changed := map[string]bool{}
	for _, line := range lines {
		for name, fn := range g.Functions {
			if fn.Path == line.Path && line.LineNo >= fn.StartLine && line.LineNo <= fn.EndLine {
				changed[name] = true
			}
		}
	}

	var chains [][]string
	for name := range changed {
		for _, root := range g.rootsTo(name) {
			chains = append(chains, g.forwardChains(root)...)
		}
	}
	return uniqueChains(chains)
}

func (g *Graph) rootsTo(name string) []string {
	seen := map[string]bool{}
	var roots []string
	var walk func(string)
	walk = func(n string) {
		if seen[n] {
			return
		}
		seen[n] = true
		if len(g.Callers[n]) == 0 {
			roots = append(roots, n)
			return
		}
		for caller := range g.Callers[n] {
			walk(caller)
		}
	}
	walk(name)
	sort.Strings(roots)
	return roots
}

func (g *Graph) forwardChains(root string) [][]string {
	var chains [][]string
	var walk func(string, []string, map[string]bool)
	walk = func(n string, path []string, seen map[string]bool) {
		if seen[n] {
			chains = append(chains, append(path, n))
			return
		}
		seen[n] = true
		path = append(path, n)
		if len(g.Calls[n]) == 0 {
			chains = append(chains, path)
			return
		}
		for callee := range g.Calls[n] {
			nextSeen := map[string]bool{}
			for k, v := range seen {
				nextSeen[k] = v
			}
			walk(callee, append([]string{}, path...), nextSeen)
		}
	}
	walk(root, nil, map[string]bool{})
	return chains
}

func (g *Graph) addEdge(from, to string) {
	if from == to {
		return
	}
	if g.Calls[from] == nil {
		g.Calls[from] = map[string]bool{}
	}
	if g.Callers[to] == nil {
		g.Callers[to] = map[string]bool{}
	}
	g.Calls[from][to] = true
	g.Callers[to][from] = true
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

func uniqueChains(chains [][]string) [][]string {
	seen := map[string]bool{}
	var out [][]string
	for _, chain := range chains {
		key := strings.Join(chain, " -> ")
		if !seen[key] {
			seen[key] = true
			out = append(out, chain)
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.Join(out[i], " -> ") < strings.Join(out[j], " -> ") })
	return out
}
