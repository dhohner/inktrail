package report

import (
	"sort"
	"strings"

	"github.com/dhohner/inktrail/internal/diff"
	"github.com/dhohner/inktrail/internal/graph"
	"github.com/dhohner/inktrail/internal/metadata"
)

func Build(g *graph.Graph, result diff.Result) Report {
	return BuildWithBase(g, nil, result)
}

func BuildWithBase(g, old *graph.Graph, result diff.Result) Report {
	movedSymbols := movedSymbols(g, old)
	equalMoves := equalBodyMoves(movedSymbols)
	currentMovedRanges, oldMovedRanges := movedFunctionRanges(g, old, equalMoves)
	result.Lines = filterMovedLines(result.Lines, currentMovedRanges)
	result.Files = filterMovedHunkLines(result.Files, currentMovedRanges, oldMovedRanges)

	changedByFunc := changedLineRangesByFunction(g, result.Lines)
	nodeNames, entryPoints := impactedNodes(g, changedByFunc)
	nodes := buildNodes(g, nodeNames, changedByFunc)

	changedSymbols := keysAsSymbolIDsExcluding(g, changedByFunc, movedCurrentNames(equalMoves))
	contexts := declarationContexts(g, changedByFunc, movedCurrentNames(equalMoves), compactPaths(result.Files))
	entryPointIDs := sortedKeys(entryPoints)
	deletedSymbols := deletedSymbols(g, old, movedOldIDs(movedSymbols))
	removedCalls := removedCalls(g, old, movedSymbolMap(movedSymbols))
	return Report{
		Summary: Summary{
			SchemaVersion:  metadata.SchemaVersion,
			Files:          len(result.Files),
			TestFiles:      countTestFiles(result.Files),
			ChangedSymbols: len(changedSymbols),
			DeletedSymbols: len(deletedSymbols),
			MovedSymbols:   len(movedSymbols),
			RemovedCalls:   len(removedCalls),
			EntryPoints:    len(entryPointIDs),
			Nodes:          len(nodes),
			ContextRecords: contextRecordCounts(contexts),
		},
		Files:          result.Files,
		ChangedSymbols: changedSymbols,
		DeletedSymbols: deletedSymbols,
		Contexts:       contexts,
		MovedSymbols:   movedSymbols,
		RemovedCalls:   removedCalls,
		EntryPoints:    entryPointIDs,
		Nodes:          nodes,
	}
}

func changedLineRangesByFunction(g *graph.Graph, lines []diff.Line) map[string][]ChangedLineRange {
	lineNosByFunction := map[string][]int{}
	for _, line := range lines {
		for _, fn := range g.FunctionsContainingLine(line.Path, line.LineNo) {
			lineNosByFunction[fn.Name] = append(lineNosByFunction[fn.Name], line.LineNo)
		}
	}

	changedByFunc := map[string][]ChangedLineRange{}
	for name, lineNos := range lineNosByFunction {
		changedByFunc[name] = compactLineRanges(lineNos)
	}
	return changedByFunc
}

func impactedNodes(g *graph.Graph, changedByFunc map[string][]ChangedLineRange) (map[string]bool, map[string]bool) {
	nodeNames := map[string]bool{}
	entryPoints := map[string]bool{}
	for _, chain := range chainsToChanged(g, changedByFunc) {
		if len(chain) == 0 {
			continue
		}
		entryPoints[symbolID(g.Functions[chain[0]])] = true
		for _, name := range chain {
			nodeNames[name] = true
		}
	}
	return nodeNames, entryPoints
}

func buildNodes(g *graph.Graph, nodeNames map[string]bool, changedByFunc map[string][]ChangedLineRange) []Node {
	nodes := make([]Node, 0, len(nodeNames))
	for name := range nodeNames {
		fn := g.Functions[name]
		changedLines := changedByFunc[name]
		nodes = append(nodes, Node{
			ID:           symbolID(fn),
			Path:         fn.Path,
			Name:         shortName(name),
			Kind:         kind(name),
			StartLine:    fn.StartLine,
			EndLine:      fn.EndLine,
			Calls:        relevantCalls(g, name, nodeNames),
			Changed:      len(changedLines) > 0,
			ChangedLines: changedLines,
			Boundary:     nil,
			Package:      packageName(name),
		})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	return nodes
}

func chainsToChanged(g *graph.Graph, changedByFunc map[string][]ChangedLineRange) [][]string {
	seenChains := map[string]bool{}
	var chains [][]string
	for changedName := range changedByFunc {
		var walk func(string, []string, map[string]bool)
		walk = func(name string, suffix []string, seen map[string]bool) {
			if seen[name] {
				return
			}
			seen[name] = true
			path := append([]string{name}, suffix...)
			callers := g.Callers[name]
			if len(callers) == 0 {
				key := strings.Join(path, "->")
				if !seenChains[key] {
					seenChains[key] = true
					chains = append(chains, path)
				}
				return
			}
			for caller := range callers {
				nextSeen := map[string]bool{}
				for k, v := range seen {
					nextSeen[k] = v
				}
				walk(caller, path, nextSeen)
			}
		}
		walk(changedName, nil, map[string]bool{})
	}
	sort.Slice(chains, func(i, j int) bool { return strings.Join(chains[i], "->") < strings.Join(chains[j], "->") })
	return chains
}

func countTestFiles(files []diff.FileChange) int {
	count := 0
	for _, file := range files {
		if file.Test {
			count++
		}
	}
	return count
}

func contextRecordCounts(contexts []DeclarationContext) ContextRecordCounts {
	counts := ContextRecordCounts{Total: len(contexts)}
	for _, context := range contexts {
		if context.Relationship == "changed_declaration" {
			counts.DeclarationContext++
		} else {
			counts.RelatedDeclarationContext++
		}
	}
	return counts
}

func packageName(name string) string {
	pkg, _, _ := strings.Cut(name, ".")
	return pkg
}

func shortName(name string) string {
	parts := strings.Split(name, ".")
	return parts[len(parts)-1]
}

func kind(name string) string {
	if strings.Count(name, ".") >= 2 {
		return "method"
	}
	return "function"
}
