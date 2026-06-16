package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/dhohner/inktrail/internal/diff"
)

// SizeOptions bounds emitted report detail while preserving summary metadata.
type SizeOptions struct {
	MaxLinesPerHunk int
	MaxContextLines int
	MaxRecords      int
	BudgetTokens    int
}

func (o SizeOptions) Validate() error {
	if o.MaxLinesPerHunk < 0 || o.MaxContextLines < 0 || o.MaxRecords < 0 || o.BudgetTokens < 0 {
		return fmt.Errorf("size and budget limits must be non-negative")
	}
	return nil
}

func prepareForWrite(r Report, opts SizeOptions, format string) Report {
	r = cloneReportForSizing(withSchemaVersion(r))
	if r.FileSymbols == nil {
		r.FileSymbols = symbolsByPath(r.Nodes)
	}
	r = limitHunkContent(r, opts.MaxLinesPerHunk)
	r = limitContextExcerpts(r, opts.MaxContextLines)
	r.Summary.Omissions = compactContextOmissions(compactHunkOmissions(r.Summary.Omissions))
	if opts.MaxRecords > 0 {
		before := detailRecordCount(r)
		r = trimToRecordCount(r, opts.MaxRecords)
		if after := detailRecordCount(r); after < before {
			r.Summary.Omissions = append(r.Summary.Omissions, Omission{Kind: "records", Reason: "max_records", OriginalCount: before, EmittedCount: after, OmittedCount: before - after})
		}
	}
	if opts.BudgetTokens > 0 {
		r = fitTokenBudget(r, opts, format)
	}
	return r
}

func limitHunkContent(r Report, maxLines int) Report {
	if maxLines <= 0 {
		return r
	}
	for _, file := range r.Files {
		if compactFileRecord(file, diffStat(file), addedContents(file)) {
			continue
		}
		original, omitted := hunkLineCounts(file.Hunks, maxLines)
		if omitted > 0 {
			r.Summary.Omissions = append(r.Summary.Omissions, Omission{Kind: "hunk_lines", Reason: "max_lines_per_hunk", Path: file.Path, OriginalCount: original, EmittedCount: original - omitted, OmittedCount: omitted})
		}
	}
	return r
}

func hunkLineCounts(hunks []diff.Hunk, maxLines int) (int, int) {
	var original, omitted int
	for _, hunk := range hunks {
		original += len(hunk.Lines)
		if len(hunk.Lines) > maxLines {
			omitted += len(hunk.Lines) - maxLines
		}
	}
	return original, omitted
}

func limitContextExcerpts(r Report, maxLines int) Report {
	if maxLines <= 0 {
		return r
	}
	for i := range r.Contexts {
		originalContentLines := len(splitLines(r.Contexts[i].Excerpt.Content))
		originalOmittedLines := r.Contexts[i].Excerpt.OmittedLines
		content, truncated, omitted := boundedExcerpt(r.Contexts[i].Excerpt.Content, maxLines)
		if truncated {
			emittedLines := len(splitLines(content))
			originalLines := originalContentLines + originalOmittedLines
			totalOmitted := originalLines - emittedLines
			r.Contexts[i].Excerpt.Content = content
			r.Contexts[i].Excerpt.Truncated = true
			r.Contexts[i].Excerpt.OmittedLines += omitted
			r.Summary.Omissions = append(r.Summary.Omissions, Omission{Kind: "context_lines", Reason: "max_context_lines", RecordType: r.Contexts[i].Relationship, Path: r.Contexts[i].Path, OriginalCount: originalLines, EmittedCount: emittedLines, OmittedCount: totalOmitted})
		}
	}
	return r
}

func compactHunks(hunks []diff.Hunk, maxLines int) []diff.Hunk {
	out := make([]diff.Hunk, len(hunks))
	copy(out, hunks)
	if maxLines <= 0 {
		return out
	}
	for i := range out {
		if len(out[i].Lines) > maxLines {
			out[i].OmittedLines = len(out[i].Lines) - maxLines
			out[i].Lines = append([]diff.HunkLine(nil), out[i].Lines[:maxLines]...)
		}
	}
	return out
}

func omittedHunkLines(hunks []diff.Hunk) int {
	total := 0
	for _, hunk := range hunks {
		total += hunk.OmittedLines
	}
	return total
}

func upsertOmission(omissions []Omission, next Omission) []Omission {
	for i := range omissions {
		if omissions[i].Kind == next.Kind && omissions[i].Reason == next.Reason && omissions[i].Path == next.Path && omissions[i].RecordType == next.RecordType {
			omissions[i] = next
			return omissions
		}
	}
	return append(omissions, next)
}

func cloneOmissions(in []Omission) []Omission {
	return append([]Omission(nil), in...)
}

func cloneReportForSizing(r Report) Report {
	r.Files = cloneFileChanges(r.Files)
	r.ChangedSymbols = cloneStrings(r.ChangedSymbols)
	r.DeletedSymbols = cloneStrings(r.DeletedSymbols)
	r.MovedSymbols = cloneMovedSymbols(r.MovedSymbols)
	r.RemovedCalls = cloneRemovedCalls(r.RemovedCalls)
	r.EntryPoints = cloneStrings(r.EntryPoints)
	r.Contexts = cloneDeclarationContexts(r.Contexts)
	r.Nodes = cloneNodes(r.Nodes)
	r.Summary.Omissions = cloneOmissions(r.Summary.Omissions)
	if r.FileSymbols != nil {
		r.FileSymbols = cloneStringMap(r.FileSymbols)
	}
	return r
}

func cloneFileChanges(in []diff.FileChange) []diff.FileChange {
	out := append([]diff.FileChange(nil), in...)
	for i := range out {
		out[i].Hunks = cloneHunks(out[i].Hunks)
	}
	return out
}

func cloneHunks(in []diff.Hunk) []diff.Hunk {
	out := append([]diff.Hunk(nil), in...)
	for i := range out {
		out[i].Lines = append([]diff.HunkLine(nil), out[i].Lines...)
	}
	return out
}

func cloneStringMap(in map[string][]string) map[string][]string {
	out := make(map[string][]string, len(in))
	for k, v := range in {
		out[k] = cloneStrings(v)
	}
	return out
}

func compactHunkOmissions(omissions []Omission) []Omission {
	const hunkOmissionDetailLimit = 20
	count := 0
	for _, omission := range omissions {
		if omission.Kind == "hunk_lines" && omission.Reason == "max_lines_per_hunk" {
			count++
		}
	}
	if count <= hunkOmissionDetailLimit {
		return omissions
	}
	var aggregate Omission
	out := make([]Omission, 0, len(omissions)-count+1)
	for _, omission := range omissions {
		if omission.Kind != "hunk_lines" || omission.Reason != "max_lines_per_hunk" {
			out = append(out, omission)
			continue
		}
		aggregate.Kind = omission.Kind
		aggregate.Reason = omission.Reason
		aggregate.OriginalCount += omission.OriginalCount
		aggregate.EmittedCount += omission.EmittedCount
		aggregate.OmittedCount += omission.OmittedCount
	}
	return append(out, aggregate)
}

func compactContextOmissions(omissions []Omission) []Omission {
	const contextOmissionDetailLimit = 20
	count := 0
	for _, omission := range omissions {
		if omission.Kind == "context_lines" && omission.Reason == "max_context_lines" {
			count++
		}
	}
	if count <= contextOmissionDetailLimit {
		return omissions
	}
	type key struct {
		path       string
		recordType string
	}
	groups := map[key]Omission{}
	out := make([]Omission, 0, len(omissions))
	for _, omission := range omissions {
		if omission.Kind != "context_lines" || omission.Reason != "max_context_lines" {
			out = append(out, omission)
			continue
		}
		k := key{path: omission.Path, recordType: omission.RecordType}
		current := groups[k]
		current.Kind = omission.Kind
		current.Reason = omission.Reason
		current.Path = omission.Path
		current.RecordType = omission.RecordType
		current.OriginalCount += omission.OriginalCount
		current.EmittedCount += omission.EmittedCount
		current.OmittedCount += omission.OmittedCount
		groups[k] = current
	}
	keys := make([]key, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].path == keys[j].path {
			return keys[i].recordType < keys[j].recordType
		}
		return keys[i].path < keys[j].path
	})
	for _, k := range keys {
		out = append(out, groups[k])
	}
	return out
}

func compactOmissionsForBudget(omissions []Omission) []Omission {
	if len(omissions) <= 1 {
		return omissions
	}
	type key struct {
		kind   string
		reason string
	}
	groups := map[key]Omission{}
	for _, omission := range omissions {
		if omission.Kind == "omission_metadata" && omission.Reason == "budget_tokens" {
			continue
		}
		k := key{kind: omission.Kind, reason: omission.Reason}
		current := groups[k]
		current.Kind = omission.Kind
		current.Reason = omission.Reason
		current.OriginalCount += omission.OriginalCount
		current.EmittedCount += omission.EmittedCount
		current.OmittedCount += omission.OmittedCount
		groups[k] = current
	}
	keys := make([]key, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].kind == keys[j].kind {
			return keys[i].reason < keys[j].reason
		}
		return keys[i].kind < keys[j].kind
	})
	out := make([]Omission, 0, len(keys)+1)
	for _, k := range keys {
		out = append(out, groups[k])
	}
	if reduced := len(omissions) - len(out); reduced > 0 {
		emitted := len(out) + 1
		out = append(out, Omission{Kind: "omission_metadata", Reason: "budget_tokens", OriginalCount: len(omissions), EmittedCount: emitted, OmittedCount: len(omissions) - emitted})
	}
	return out
}

func detailRecordCount(r Report) int {
	return len(r.Files) + len(r.ChangedSymbols) + len(r.DeletedSymbols) + len(r.Contexts) + len(r.MovedSymbols) + len(r.RemovedCalls) + len(r.EntryPoints) + len(r.Nodes)
}

func trimToRecordCount(r Report, max int) Report {
	for detailRecordCount(r) > max {
		r = trimOneLowestPriorityRecord(r)
	}
	return r
}

func fitTokenBudget(r Report, opts SizeOptions, format string) Report {
	if estimateReportTokens(r, opts, format) <= opts.BudgetTokens {
		return r
	}
	r.Summary.Omissions = compactOmissionsForBudget(r.Summary.Omissions)
	before := detailRecordCount(r)
	if before > 0 {
		base := r
		best, ok := reportForLargestFittingRecordCount(base, before, opts, format)
		if ok {
			return best
		}
		r = withBudgetRecordOmission(trimToRecordCount(base, 0), before, 0)
	}
	if estimateReportTokens(r, opts, format) > opts.BudgetTokens {
		r = withBudgetFloorOmission(r, opts, format)
	}
	return r
}

func reportForLargestFittingRecordCount(base Report, originalRecords int, opts SizeOptions, format string) (Report, bool) {
	low, high := 0, originalRecords
	var best Report
	ok := false
	for low <= high {
		mid := low + (high-low)/2
		candidate := withBudgetRecordOmission(trimToRecordCount(base, mid), originalRecords, mid)
		if estimateReportTokens(candidate, opts, format) <= opts.BudgetTokens {
			best = candidate
			ok = true
			low = mid + 1
			continue
		}
		high = mid - 1
	}
	return best, ok
}

func withBudgetRecordOmission(r Report, originalRecords, emittedRecords int) Report {
	r.Summary.Omissions = upsertOmission(cloneOmissions(r.Summary.Omissions), Omission{Kind: "records", Reason: "budget_tokens", OriginalCount: originalRecords, EmittedCount: emittedRecords, OmittedCount: originalRecords - emittedRecords})
	return r
}

func withBudgetFloorOmission(r Report, opts SizeOptions, format string) Report {
	for {
		estimate := estimateReportTokens(r, opts, format)
		next := r
		next.Summary.Omissions = upsertOmission(cloneOmissions(next.Summary.Omissions), Omission{Kind: "budget", Reason: "budget_floor_exceeded", OriginalCount: estimate, EmittedCount: opts.BudgetTokens, OmittedCount: estimate - opts.BudgetTokens})
		next.Summary.Omissions = compactOmissionsForBudget(next.Summary.Omissions)
		if estimateReportTokens(next, opts, format) == estimateReportTokens(r, opts, format) {
			return next
		}
		r = next
	}
}

func trimOneLowestPriorityRecord(r Report) Report {
	switch {
	case len(r.Nodes) > 0:
		r.Nodes = removeLargestJSON(r.Nodes)
	case len(r.EntryPoints) > 0:
		r.EntryPoints = removeLargestString(r.EntryPoints)
	case len(r.RemovedCalls) > 0:
		r.RemovedCalls = removeLargestJSON(r.RemovedCalls)
	case len(r.MovedSymbols) > 0:
		r.MovedSymbols = removeLargestJSON(r.MovedSymbols)
	case len(r.Contexts) > 0:
		r.Contexts = removeLargestJSON(r.Contexts)
	case len(r.DeletedSymbols) > 0:
		r.DeletedSymbols = removeLargestString(r.DeletedSymbols)
	case len(r.ChangedSymbols) > 0:
		r.ChangedSymbols = removeLargestString(r.ChangedSymbols)
	case len(r.Files) > 0:
		r.Files = removeLargestJSON(r.Files)
	}
	return r
}

func removeLargestString(in []string) []string {
	if len(in) == 0 {
		return in
	}
	largest := 0
	for i := 1; i < len(in); i++ {
		if len(in[i]) >= len(in[largest]) {
			largest = i
		}
	}
	return append(in[:largest], in[largest+1:]...)
}

func removeLargestJSON[T any](in []T) []T {
	if len(in) == 0 {
		return in
	}
	largest := 0
	largestSize := jsonSize(in[0])
	for i := 1; i < len(in); i++ {
		if size := jsonSize(in[i]); size >= largestSize {
			largest = i
			largestSize = size
		}
	}
	return append(in[:largest], in[largest+1:]...)
}

func jsonSize(v any) int {
	b, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return len(b)
}

func estimateReportTokens(r Report, opts SizeOptions, format string) int {
	var buf bytes.Buffer
	opts.BudgetTokens = 0
	if format == "json" {
		_ = writeJSONWithOptions(&buf, r, opts)
	} else {
		_ = writeJSONLWithOptions(&buf, r, opts)
	}
	return int(math.Ceil(float64(buf.Len()) / 4.0))
}
