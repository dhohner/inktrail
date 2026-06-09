package report

import (
	"sort"

	"github.com/dhohner/inktrail/internal/graph"
)

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

func deletedSymbols(current, old *graph.Graph, movedOldIDs map[string]bool) []string {
	if old == nil {
		return nil
	}
	var ids []string
	currentIDs := map[string]bool{}
	for _, fn := range current.Functions {
		currentIDs[symbolID(fn)] = true
	}
	for _, fn := range old.Functions {
		id := symbolID(fn)
		if !currentIDs[id] && !movedOldIDs[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func removedCalls(current, old *graph.Graph, moved map[string]string) []RemovedCall {
	if old == nil {
		return nil
	}
	currentEdges := map[string]bool{}
	for from, calls := range current.Calls {
		fromFn, ok := current.Functions[from]
		if !ok {
			continue
		}
		for to := range calls {
			toFn, ok := current.Functions[to]
			if ok {
				currentEdges[symbolID(fromFn)+"->"+symbolID(toFn)] = true
			}
		}
	}

	var out []RemovedCall
	for from, calls := range old.Calls {
		fromFn, ok := old.Functions[from]
		if !ok {
			continue
		}
		for to := range calls {
			toFn, ok := old.Functions[to]
			if !ok {
				continue
			}
			fromID := symbolID(fromFn)
			toID := symbolID(toFn)
			if currentEdges[remapMovedSymbol(fromID, moved)+"->"+remapMovedSymbol(toID, moved)] {
				continue
			}
			call, _ := old.CallSite(from, to)
			out = append(out, RemovedCall{From: fromID, To: toID, CallSite: CallSite{Path: call.Path, Line: call.LineNo}})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From == out[j].From {
			return out[i].To < out[j].To
		}
		return out[i].From < out[j].From
	})
	return out
}

func compactLineRanges(lines []int) []ChangedLineRange {
	if len(lines) == 0 {
		return nil
	}
	sort.Ints(lines)
	var ranges []ChangedLineRange
	start := lines[0]
	end := lines[0]
	for _, line := range lines[1:] {
		if line == end {
			continue
		}
		if line == end+1 {
			end = line
			continue
		}
		ranges = append(ranges, ChangedLineRange{Start: start, End: end})
		start = line
		end = line
	}
	ranges = append(ranges, ChangedLineRange{Start: start, End: end})
	return ranges
}

func keysAsSymbolIDsExcluding(g *graph.Graph, values map[string][]ChangedLineRange, exclude map[string]bool) []string {
	ids := make([]string, 0, len(values))
	for name := range values {
		if exclude[name] {
			continue
		}
		ids = append(ids, symbolID(g.Functions[name]))
	}
	sort.Strings(ids)
	return ids
}
