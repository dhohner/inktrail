package report

import (
	"strings"

	"github.com/dhohner/inktrail/internal/metadata"
)

func pruneFilteredDerivedRecords(r Report, allowedPaths PathSet) Report {
	allowedIDs := idsByPath(r.ChangedSymbols, allowedPaths)

	r.ChangedSymbols = filterIDsByPath(r.ChangedSymbols, allowedPaths)
	r.DeletedSymbols = filterIDsByPath(r.DeletedSymbols, allowedPaths)

	moves := r.MovedSymbols[:0]
	for _, move := range r.MovedSymbols {
		if pathAllowed(symbolPath(move.From), allowedPaths) || pathAllowed(symbolPath(move.To), allowedPaths) {
			moves = append(moves, move)
			allowedIDs[move.From] = true
			allowedIDs[move.To] = true
		}
	}
	r.MovedSymbols = moves

	removed := r.RemovedCalls[:0]
	for _, call := range r.RemovedCalls {
		if pathAllowed(symbolPath(call.From), allowedPaths) || pathAllowed(symbolPath(call.To), allowedPaths) || pathAllowed(call.CallSite.Path, allowedPaths) {
			removed = append(removed, call)
		}
	}
	r.RemovedCalls = removed

	contexts := r.Contexts[:0]
	for _, context := range r.Contexts {
		if pathAllowed(context.Path, allowedPaths) || pathAllowed(symbolPath(context.RelatedTo), allowedPaths) {
			contexts = append(contexts, context)
			allowedIDs[context.ID] = true
		}
	}
	r.Contexts = contexts

	for _, node := range r.Nodes {
		allowedIDs[node.ID] = true
	}
	filterNodeCalls(r.Nodes, allowedIDs, nil)
	r.EntryPoints = filterEntryPointsByNode(r.EntryPoints, allowedIDs)

	return r
}

func pruneChangedOnlyRecords(r Report, allowedPaths PathSet) Report {
	allowedIDs := idsByPath(append(append([]string{}, r.ChangedSymbols...), r.DeletedSymbols...), allowedPaths)

	r.ChangedSymbols = filterIDsByPath(r.ChangedSymbols, allowedPaths)
	r.DeletedSymbols = filterIDsByPath(r.DeletedSymbols, allowedPaths)

	moves := r.MovedSymbols[:0]
	for _, move := range r.MovedSymbols {
		if pathAllowed(symbolPath(move.From), allowedPaths) || pathAllowed(symbolPath(move.To), allowedPaths) {
			moves = append(moves, move)
			allowedIDs[move.From] = true
			allowedIDs[move.To] = true
		}
	}
	r.MovedSymbols = moves

	removed := r.RemovedCalls[:0]
	for _, call := range r.RemovedCalls {
		if pathAllowed(symbolPath(call.From), allowedPaths) || pathAllowed(symbolPath(call.To), allowedPaths) || pathAllowed(call.CallSite.Path, allowedPaths) {
			removed = append(removed, call)
		}
	}
	r.RemovedCalls = removed

	contexts := r.Contexts[:0]
	for _, context := range r.Contexts {
		if pathAllowed(context.Path, allowedPaths) && (context.RelatedTo == "" || pathAllowed(symbolPath(context.RelatedTo), allowedPaths)) {
			contexts = append(contexts, context)
			allowedIDs[context.ID] = true
		}
	}
	r.Contexts = contexts

	nodes := r.Nodes[:0]
	for _, node := range r.Nodes {
		if !pathAllowed(node.Path, allowedPaths) {
			continue
		}
		allowedIDs[node.ID] = true
		nodes = append(nodes, node)
	}
	filterNodeCalls(nodes, allowedIDs, &allowedPaths)
	r.Nodes = nodes
	r.EntryPoints = filterEntryPointsByPath(r.EntryPoints, allowedIDs, allowedPaths)

	return r
}

// Empty returns an initialized report with an accurate zero-value summary.
func Empty() Report {
	return Report{Summary: summarize(Report{})}
}

func summarize(r Report) Summary {
	return Summary{
		SchemaVersion:  metadata.SchemaVersion,
		Files:          len(r.Files),
		TestFiles:      countTestFiles(r.Files),
		ChangedSymbols: len(r.ChangedSymbols),
		DeletedSymbols: len(r.DeletedSymbols),
		MovedSymbols:   len(r.MovedSymbols),
		RemovedCalls:   len(r.RemovedCalls),
		EntryPoints:    len(r.EntryPoints),
		Nodes:          len(r.Nodes),
		ContextRecords: contextRecordCounts(r.Contexts),
	}
}

func idsByPath(ids []string, allowed PathSet) map[string]bool {
	out := map[string]bool{}
	for _, id := range ids {
		if pathAllowed(symbolPath(id), allowed) {
			out[id] = true
		}
	}
	return out
}

func filterIDsByPath(ids []string, allowed PathSet) []string {
	out := ids[:0]
	for _, id := range ids {
		if pathAllowed(symbolPath(id), allowed) {
			out = append(out, id)
		}
	}
	return out
}

func filterNodeCalls(nodes []Node, allowedIDs map[string]bool, allowedPaths *PathSet) {
	for i := range nodes {
		calls := nodes[i].Calls[:0]
		for _, call := range nodes[i].Calls {
			if !allowedIDs[call.To] {
				continue
			}
			if allowedPaths != nil && !pathAllowed(call.CallSite.Path, *allowedPaths) {
				continue
			}
			calls = append(calls, call)
		}
		nodes[i].Calls = calls
	}
}

func filterEntryPointsByNode(entries []string, allowedIDs map[string]bool) []string {
	out := entries[:0]
	for _, id := range entries {
		if allowedIDs[id] {
			out = append(out, id)
		}
	}
	return out
}

func filterEntryPointsByPath(entries []string, allowedIDs map[string]bool, allowedPaths PathSet) []string {
	out := entries[:0]
	for _, id := range entries {
		if allowedIDs[id] && pathAllowed(symbolPath(id), allowedPaths) {
			out = append(out, id)
		}
	}
	return out
}

func pathAllowed(p string, allowed PathSet) bool {
	return allowed.Contains(p)
}

func symbolPath(id string) string {
	path, _, _ := strings.Cut(id, "::")
	return path
}
