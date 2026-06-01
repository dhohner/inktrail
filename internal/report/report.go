package report

import (
	"encoding/json"
	"io"
	"sort"
	"strings"

	"inktrail/internal/changes"
	"inktrail/internal/graph"
)

type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type ChangedLine struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

type CallSite struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

type OutgoingCall struct {
	To       string   `json:"to"`
	CallSite CallSite `json:"call_site"`
}

type Node struct {
	ID           string         `json:"id"`
	Path         string         `json:"path"`
	Name         string         `json:"name"`
	Kind         string         `json:"kind"`
	StartLine    int            `json:"start_line"`
	EndLine      int            `json:"end_line"`
	Calls        []OutgoingCall `json:"calls,omitempty"`
	Changed      bool           `json:"changed"`
	ChangedLines []ChangedLine  `json:"changed_lines,omitempty"`
	Boundary     *string        `json:"boundary"`
	Package      string         `json:"package"`
	File         string         `json:"file"`
	LineRange    LineRange      `json:"lineRange"`
}

type Report struct {
	ChangedSymbols []string `json:"changed_symbols"`
	EntryPoints    []string `json:"entry_points"`
	Nodes          []Node   `json:"nodes"`
}

func Build(g *graph.Graph, changed []changes.Line) Report {
	changedByFunc := map[string][]ChangedLine{}
	changedLines := make([]ChangedLine, 0, len(changed))
	for _, line := range changed {
		changedLine := ChangedLine{Path: line.Path, Line: line.LineNo}
		changedLines = append(changedLines, changedLine)
		for name, fn := range g.Functions {
			if fn.Path == line.Path && line.LineNo >= fn.StartLine && line.LineNo <= fn.EndLine {
				changedByFunc[name] = append(changedByFunc[name], changedLine)
			}
		}
	}

	chains := chainsToChanged(g, changedByFunc)
	nodeNames := map[string]bool{}
	entryPoints := map[string]bool{}
	for _, chain := range chains {
		if len(chain) == 0 {
			continue
		}
		entryPoints[symbolID(g.Functions[chain[0]])] = true
		for _, name := range chain {
			nodeNames[name] = true
		}
	}

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
			File:         fn.Path,
			LineRange:    LineRange{Start: fn.StartLine, End: fn.EndLine},
		})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	changedSymbols := keysAsSymbolIDs(g, changedByFunc)
	return Report{
		ChangedSymbols: changedSymbols,
		EntryPoints:    sortedKeys(entryPoints),
		Nodes:          nodes,
	}
}

func chainsToChanged(g *graph.Graph, changedByFunc map[string][]ChangedLine) [][]string {
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

func WriteJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func symbolID(fn graph.Function) string {
	return fn.Path + "::" + fn.Name
}

func relevantCalls(g *graph.Graph, name string, relevant map[string]bool) []OutgoingCall {
	calls := g.Calls[name]
	out := make([]OutgoingCall, 0, len(calls))
	seen := map[string]bool{}
	for callee := range calls {
		if !relevant[callee] {
			continue
		}
		to := symbolID(g.Functions[callee])
		if seen[to] {
			continue
		}
		seen[to] = true
		call, ok := g.CallSite(name, callee)
		if !ok {
			continue
		}
		out = append(out, OutgoingCall{To: to, CallSite: CallSite{Path: call.Path, Line: call.LineNo}})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].To < out[j].To })
	return out
}

func keysAsSymbolIDs(g *graph.Graph, values map[string][]ChangedLine) []string {
	ids := make([]string, 0, len(values))
	for name := range values {
		ids = append(ids, symbolID(g.Functions[name]))
	}
	sort.Strings(ids)
	return ids
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
