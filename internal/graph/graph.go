package graph

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dhohner/inktrail/internal/analyzer"
	"github.com/dhohner/inktrail/internal/analyzer/golang"
	"github.com/dhohner/inktrail/internal/analyzer/java"
	sharedparser "github.com/dhohner/inktrail/internal/parser"
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
	Path     string
	Source   []byte
	Doc      *sharedparser.Document
	Language sharedparser.Language
	Analyzer analyzer.Analyzer
	Package  string
}

var analyzers = analyzer.NewRegistry(
	golang.Analyzer{},
	java.Analyzer{},
)

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
		if !isProductionSourceFile(path) {
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
		if path == "" || !isProductionSourceFile(path) {
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
		analyzer, language, ok := analyzerForPath(sf.Path)
		if !ok || !analyzer.IsProductionPath(sf.Path) {
			continue
		}
		doc, err := sharedparser.Parse(language, sf.Source)
		if err != nil {
			closeParsedSources(parsed)
			return nil, err
		}
		if doc.HasSyntaxError() {
			doc.Close()
			closeParsedSources(parsed)
			return nil, fmt.Errorf("parse %s: syntax error", sf.Path)
		}
		parsed = append(parsed, parsedSource{
			Path:     sf.Path,
			Source:   sf.Source,
			Doc:      doc,
			Language: language,
			Analyzer: analyzer,
			Package:  analyzer.PackageName(doc.RootNode(), sf.Source),
		})
	}
	return parsed, nil
}

func closeParsedSources(parsed []parsedSource) {
	for _, ps := range parsed {
		ps.Doc.Close()
	}
}

func (g *Graph) addFunctions(ps parsedSource) {
	for _, sym := range ps.Analyzer.Symbols(ps.analyzerSource()) {
		r := sym.Node.Range()
		g.Functions[sym.Name] = Function{Name: sym.Name, Path: ps.Path, StartLine: r.StartLine, EndLine: r.EndLine, Source: nodeSource(ps.Source, int(r.StartByte), int(r.EndByte))}
		g.indexFunction(sym.Name)
	}
}

func (g *Graph) addCalls(ps parsedSource) {
	for _, call := range ps.Analyzer.Calls(ps.analyzerSource()) {
		site := CallSite{Path: call.Site.Path, LineNo: call.Site.LineNo, Code: call.Site.Code}
		for _, candidate := range g.resolveCalls(ps.Package, call.To) {
			g.addEdge(call.From, candidate, site)
		}
	}
}

func (ps parsedSource) analyzerSource() analyzer.Source {
	return analyzer.Source{Path: ps.Path, Source: ps.Source, Doc: ps.Doc, Language: ps.Language, Package: ps.Package}
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
	for _, name := range []string{pkg + "." + to, to} {
		if fn, ok := g.Functions[name]; ok {
			return []string{fn.Name}
		}
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

func analyzerForPath(path string) (analyzer.Analyzer, sharedparser.Language, bool) {
	return analyzers.ForPath(path)
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

func isProductionSourceFile(path string) bool {
	analyzer, _, ok := analyzerForPath(path)
	return ok && analyzer.IsProductionPath(path)
}

func gitErr(prefix string, err error) error {
	if exit, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("%s: %s", prefix, bytes.TrimSpace(exit.Stderr))
	}
	return fmt.Errorf("%s: %w", prefix, err)
}
