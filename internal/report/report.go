package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"inktrail/internal/diff"
	"inktrail/internal/graph"
)

type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type ChangedLineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
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
	ID           string             `json:"id"`
	Path         string             `json:"path"`
	Name         string             `json:"name"`
	Kind         string             `json:"kind"`
	StartLine    int                `json:"start_line"`
	EndLine      int                `json:"end_line"`
	Calls        []OutgoingCall     `json:"calls,omitempty"`
	Changed      bool               `json:"changed"`
	ChangedLines []ChangedLineRange `json:"changed_lines,omitempty"`
	Boundary     *string            `json:"boundary,omitempty"`
	Package      string             `json:"package"`
}

type RemovedCall struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	CallSite CallSite `json:"call_site"`
}

type DiffStat struct {
	AddedLines   int `json:"added_lines"`
	DeletedLines int `json:"deleted_lines"`
	AddedBytes   int `json:"added_bytes"`
	DeletedBytes int `json:"deleted_bytes"`
}

type ContentRef struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256,omitempty"`
}

type FilePreview struct {
	HeadLines    []string `json:"head_lines,omitempty"`
	TailLines    []string `json:"tail_lines,omitempty"`
	OmittedLines int      `json:"omitted_lines"`
}

type FileRecord struct {
	Type           string       `json:"type"`
	Status         string       `json:"status"`
	OldPath        string       `json:"old_path,omitempty"`
	Path           string       `json:"path"`
	Test           bool         `json:"test"`
	Language       string       `json:"language,omitempty"`
	Classification []string     `json:"classification,omitempty"`
	DiffStat       DiffStat     `json:"diffstat"`
	Symbols        []string     `json:"symbols,omitempty"`
	RiskFlags      []string     `json:"risk_flags,omitempty"`
	ContentRef     *ContentRef  `json:"content_ref,omitempty"`
	Preview        *FilePreview `json:"preview,omitempty"`
	HunksOmitted   bool         `json:"hunks_omitted,omitempty"`
	OmittedLines   int          `json:"omitted_lines,omitempty"`
	Hunks          []diff.Hunk  `json:"hunks,omitempty"`
}

type MovedSymbol struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Summary struct {
	Files          int `json:"files"`
	TestFiles      int `json:"test_files"`
	ChangedSymbols int `json:"changed_symbols"`
	DeletedSymbols int `json:"deleted_symbols"`
	MovedSymbols   int `json:"moved_symbols"`
	RemovedCalls   int `json:"removed_calls"`
	EntryPoints    int `json:"entry_points"`
	Nodes          int `json:"nodes"`
}

type Report struct {
	Summary        Summary           `json:"summary"`
	Files          []diff.FileChange `json:"files"`
	ChangedSymbols []string          `json:"changed_symbols"`
	DeletedSymbols []string          `json:"deleted_symbols"`
	MovedSymbols   []MovedSymbol     `json:"moved_symbols"`
	RemovedCalls   []RemovedCall     `json:"removed_calls"`
	EntryPoints    []string          `json:"entry_points"`
	Nodes          []Node            `json:"nodes"`
}

func Build(g *graph.Graph, result diff.Result) Report {
	return BuildWithBase(g, nil, result)
}

func BuildWithBase(g, old *graph.Graph, result diff.Result) Report {
	movedSymbols := movedSymbols(g, old)
	currentMovedRanges, oldMovedRanges := movedFunctionRanges(g, old, movedSymbols)
	result.Lines = filterMovedLines(result.Lines, currentMovedRanges)
	result.Files = filterMovedHunkLines(result.Files, currentMovedRanges, oldMovedRanges)

	changedByFunc := changedLineRangesByFunction(g, result.Lines)
	nodeNames, entryPoints := impactedNodes(g, changedByFunc)
	nodes := buildNodes(g, nodeNames, changedByFunc)

	changedSymbols := keysAsSymbolIDsExcluding(g, changedByFunc, movedCurrentNames(movedSymbols))
	entryPointIDs := sortedKeys(entryPoints)
	deletedSymbols := deletedSymbols(g, old, movedOldIDs(movedSymbols))
	removedCalls := removedCalls(g, old, movedSymbolMap(movedSymbols))
	return Report{
		Summary: Summary{
			Files:          len(result.Files),
			TestFiles:      countTestFiles(result.Files),
			ChangedSymbols: len(changedSymbols),
			DeletedSymbols: len(deletedSymbols),
			MovedSymbols:   len(movedSymbols),
			RemovedCalls:   len(removedCalls),
			EntryPoints:    len(entryPointIDs),
			Nodes:          len(nodes),
		},
		Files:          result.Files,
		ChangedSymbols: changedSymbols,
		DeletedSymbols: deletedSymbols,
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

func WriteJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func WriteJSONL(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(struct {
		Type string `json:"type"`
		Summary
	}{Type: "summary", Summary: r.Summary}); err != nil {
		return err
	}
	fileSymbols := symbolsByPath(r.Nodes)
	for _, file := range r.Files {
		if err := enc.Encode(fileRecord(file, fileSymbols[file.Path])); err != nil {
			return err
		}
	}
	for _, id := range r.ChangedSymbols {
		if err := enc.Encode(struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		}{Type: "changed_symbol", ID: id}); err != nil {
			return err
		}
	}
	for _, id := range r.DeletedSymbols {
		if err := enc.Encode(struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		}{Type: "deleted_symbol", ID: id}); err != nil {
			return err
		}
	}
	for _, move := range r.MovedSymbols {
		if err := enc.Encode(struct {
			Type string `json:"type"`
			MovedSymbol
		}{Type: "moved_symbol", MovedSymbol: move}); err != nil {
			return err
		}
	}
	for _, call := range r.RemovedCalls {
		if err := enc.Encode(struct {
			Type string `json:"type"`
			RemovedCall
		}{Type: "removed_call", RemovedCall: call}); err != nil {
			return err
		}
	}
	for _, id := range r.EntryPoints {
		if err := enc.Encode(struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		}{Type: "entry_point", ID: id}); err != nil {
			return err
		}
	}
	for _, node := range r.Nodes {
		if err := enc.Encode(struct {
			Type string `json:"type"`
			Node
		}{Type: "node", Node: node}); err != nil {
			return err
		}
	}
	return nil
}

const (
	largeAddedFileBytes = 15 * 1024
	largeAddedFileLines = 300
	previewLines        = 20
)

func fileRecord(file diff.FileChange, symbols []string) FileRecord {
	stat := diffStat(file)
	added := addedContents(file)
	compact := file.Status == "added" && (stat.AddedBytes > largeAddedFileBytes || stat.AddedLines > largeAddedFileLines || isGenerated(file.Path, added) || isVendorPath(file.Path) || isBinaryPath(file.Path))
	record := FileRecord{
		Type:           "file",
		Status:         file.Status,
		OldPath:        file.OldPath,
		Path:           file.Path,
		Test:           file.Test,
		Language:       language(file.Path),
		Classification: classification(file, added),
		DiffStat:       stat,
		Symbols:        symbols,
		RiskFlags:      riskFlags(file, added, compact),
		ContentRef:     contentRef(file, added),
	}
	if compact {
		record.HunksOmitted = true
		record.OmittedLines = totalHunkLines(file.Hunks)
		record.Preview = preview(added)
		return record
	}
	record.Hunks = file.Hunks
	return record
}

func diffStat(file diff.FileChange) DiffStat {
	var stat DiffStat
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			switch line.Op {
			case "add":
				stat.AddedLines++
				stat.AddedBytes += len(line.Content)
			case "delete":
				stat.DeletedLines++
				stat.DeletedBytes += len(line.Content)
			}
		}
	}
	return stat
}

func addedContents(file diff.FileChange) []string {
	var lines []string
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if line.Op == "add" {
				lines = append(lines, line.Content)
			}
		}
	}
	return lines
}

func contentRef(file diff.FileChange, added []string) *ContentRef {
	if file.Path == "" || file.Status == "deleted" {
		return nil
	}
	ref := &ContentRef{Kind: "workspace_file", Path: file.Path}
	if file.Status == "added" {
		sum := sha256.Sum256([]byte(strings.Join(added, "\n")))
		ref.SHA256 = hex.EncodeToString(sum[:])
	}
	return ref
}

func preview(lines []string) *FilePreview {
	if len(lines) == 0 {
		return nil
	}
	limit := previewLines
	if len(lines) <= limit*2 {
		return &FilePreview{HeadLines: lines, OmittedLines: 0}
	}
	return &FilePreview{
		HeadLines:    append([]string(nil), lines[:limit]...),
		TailLines:    append([]string(nil), lines[len(lines)-limit:]...),
		OmittedLines: len(lines) - limit*2,
	}
}

func totalHunkLines(hunks []diff.Hunk) int {
	total := 0
	for _, hunk := range hunks {
		total += len(hunk.Lines)
	}
	return total
}

func symbolsByPath(nodes []Node) map[string][]string {
	out := map[string][]string{}
	seen := map[string]map[string]bool{}
	for _, node := range nodes {
		if seen[node.Path] == nil {
			seen[node.Path] = map[string]bool{}
		}
		if seen[node.Path][node.ID] {
			continue
		}
		seen[node.Path][node.ID] = true
		out[node.Path] = append(out[node.Path], node.ID)
	}
	for path := range out {
		sort.Strings(out[path])
	}
	return out
}

func classification(file diff.FileChange, added []string) []string {
	var out []string
	if file.Test {
		out = append(out, "test")
	}
	if isVendorPath(file.Path) {
		out = append(out, "vendor")
	}
	if isGenerated(file.Path, added) {
		out = append(out, "generated")
	}
	if isBinaryPath(file.Path) {
		out = append(out, "binary")
	}
	if len(out) == 0 {
		out = append(out, "source")
	}
	return out
}

func riskFlags(file diff.FileChange, added []string, compact bool) []string {
	flags := map[string]bool{}
	if compact {
		flags["large_added_file"] = true
	}
	for _, c := range classification(file, added) {
		if c != "source" {
			flags[c] = true
		}
	}
	content := strings.ToLower(strings.Join(added, "\n"))
	checks := map[string][]string{
		"auth":           {"auth", "oauth", "jwt", "session", "permission"},
		"secret":         {"secret", "password", "passwd", "api_key", "apikey", "token"},
		"crypto":         {"crypto", "sha256", "md5", "encrypt", "decrypt", "cipher"},
		"external_input": {"http.", "url.", "json.", "formvalue", "query", "body"},
		"command_exec":   {"exec.", "os/exec", "system("},
		"sql":            {"select ", "insert ", "update ", "delete ", "sql."},
	}
	for flag, needles := range checks {
		for _, needle := range needles {
			if strings.Contains(content, needle) {
				flags[flag] = true
				break
			}
		}
	}
	return sortedKeys(flags)
}

func isGenerated(path string, lines []string) bool {
	lowerPath := strings.ToLower(path)
	if strings.Contains(lowerPath, "generated") || strings.Contains(lowerPath, ".pb.go") || strings.Contains(lowerPath, "_gen.") {
		return true
	}
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "code generated") || strings.Contains(lower, "do not edit") || strings.Contains(lower, "auto-generated") {
			return true
		}
	}
	return false
}

func isVendorPath(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		switch part {
		case "vendor", "node_modules", "third_party", "dist", "build":
			return true
		}
	}
	return false
}

func isBinaryPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".pdf", ".zip", ".gz", ".tar", ".jar", ".woff", ".woff2", ".ttf", ".otf", ".mp4", ".mov":
		return true
	default:
		return false
	}
}

func language(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".py":
		return "python"
	case ".rb":
		return "ruby"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	default:
		return strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	}
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

func keysAsSymbolIDs(g *graph.Graph, values map[string][]ChangedLineRange) []string {
	return keysAsSymbolIDsExcluding(g, values, nil)
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

func movedSymbols(current, old *graph.Graph) []MovedSymbol {
	if old == nil {
		return nil
	}
	var out []MovedSymbol
	for name, oldFn := range old.Functions {
		currentFn, ok := current.Functions[name]
		if !ok || oldFn.Path == currentFn.Path || oldFn.Source != currentFn.Source {
			continue
		}
		out = append(out, MovedSymbol{From: symbolID(oldFn), To: symbolID(currentFn)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From == out[j].From {
			return out[i].To < out[j].To
		}
		return out[i].From < out[j].From
	})
	return out
}

func movedCurrentNames(moves []MovedSymbol) map[string]bool {
	out := map[string]bool{}
	for _, move := range moves {
		_, name, ok := strings.Cut(move.To, "::")
		if ok {
			out[name] = true
		}
	}
	return out
}

func movedOldIDs(moves []MovedSymbol) map[string]bool {
	out := map[string]bool{}
	for _, move := range moves {
		out[move.From] = true
	}
	return out
}

func movedSymbolMap(moves []MovedSymbol) map[string]string {
	out := map[string]string{}
	for _, move := range moves {
		out[move.From] = move.To
	}
	return out
}

func remapMovedSymbol(id string, moves map[string]string) string {
	if to, ok := moves[id]; ok {
		return to
	}
	return id
}

func movedFunctionRanges(current, old *graph.Graph, moves []MovedSymbol) (map[string][]LineRange, map[string][]LineRange) {
	currentRanges := map[string][]LineRange{}
	oldRanges := map[string][]LineRange{}
	if old == nil {
		return currentRanges, oldRanges
	}
	for _, move := range moves {
		_, currentName, ok := strings.Cut(move.To, "::")
		if ok {
			if fn, exists := current.Functions[currentName]; exists {
				currentRanges[fn.Path] = append(currentRanges[fn.Path], LineRange{Start: fn.StartLine, End: fn.EndLine})
			}
		}
		_, oldName, ok := strings.Cut(move.From, "::")
		if ok {
			if fn, exists := old.Functions[oldName]; exists {
				oldRanges[fn.Path] = append(oldRanges[fn.Path], LineRange{Start: fn.StartLine, End: fn.EndLine})
			}
		}
	}
	return currentRanges, oldRanges
}

func filterMovedLines(lines []diff.Line, movedRanges map[string][]LineRange) []diff.Line {
	if len(movedRanges) == 0 {
		return lines
	}
	out := make([]diff.Line, 0, len(lines))
	for _, line := range lines {
		if inRanges(line.LineNo, movedRanges[line.Path]) {
			continue
		}
		out = append(out, line)
	}
	return out
}

func filterMovedHunkLines(files []diff.FileChange, currentMovedRanges, oldMovedRanges map[string][]LineRange) []diff.FileChange {
	if len(currentMovedRanges) == 0 && len(oldMovedRanges) == 0 {
		return files
	}
	out := make([]diff.FileChange, 0, len(files))
	for _, file := range files {
		filteredFile := file
		filteredFile.Hunks = make([]diff.Hunk, 0, len(file.Hunks))
		oldPath := file.OldPath
		if oldPath == "" {
			oldPath = file.Path
		}
		for _, hunk := range file.Hunks {
			filteredHunk := hunk
			filteredHunk.Lines = make([]diff.HunkLine, 0, len(hunk.Lines))
			for _, line := range hunk.Lines {
				if line.Op == "add" && inRanges(line.NewLine, currentMovedRanges[file.Path]) {
					continue
				}
				if line.Op == "del" && inRanges(line.OldLine, oldMovedRanges[oldPath]) {
					continue
				}
				filteredHunk.Lines = append(filteredHunk.Lines, line)
			}
			if len(hunk.Lines) == 0 || len(filteredHunk.Lines) > 0 {
				filteredFile.Hunks = append(filteredFile.Hunks, filteredHunk)
			}
		}
		if len(file.Hunks) == 0 || len(filteredFile.Hunks) > 0 {
			out = append(out, filteredFile)
		}
	}
	return out
}

func inRanges(lineNo int, ranges []LineRange) bool {
	for _, r := range ranges {
		if lineNo >= r.Start && lineNo <= r.End {
			return true
		}
	}
	return false
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
