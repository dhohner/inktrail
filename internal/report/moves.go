package report

import (
	"sort"
	"strings"

	"inktrail/internal/diff"
	"inktrail/internal/graph"
)

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
