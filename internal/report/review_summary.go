package report

import (
	"sort"

	"github.com/dhohner/inktrail/internal/diff"
	"github.com/dhohner/inktrail/internal/metadata"
)

func buildReviewSummary(r Report) ReviewSummary {
	fileSymbols := fileSymbolsForReport(r)
	schemaVersion := r.Summary.SchemaVersion
	if schemaVersion == "" {
		schemaVersion = metadata.SchemaVersion
	}
	summary := ReviewSummary{
		SchemaVersion:         schemaVersion,
		ChangedSymbols:        cloneStrings(r.ChangedSymbols),
		DeletedSymbols:        cloneStrings(r.DeletedSymbols),
		RiskyRemovedCallEdges: cloneRemovedCalls(r.RemovedCalls),
		UnsupportedFiles:      unsupportedFiles(r.Warnings),
	}
	for _, file := range r.Files {
		reviewFile := reviewFile(file, fileSymbols[file.Path])
		if file.Test {
			summary.ChangedTestFiles = append(summary.ChangedTestFiles, reviewFile)
			continue
		}
		summary.ChangedProductionFiles = append(summary.ChangedProductionFiles, reviewFile)
	}
	return summary
}

func reviewFile(file diff.FileChange, symbols []string) ReviewFile {
	return ReviewFile{
		Path:     file.Path,
		OldPath:  file.OldPath,
		Status:   file.Status,
		Language: language(file.Path),
		Symbols:  cloneStrings(symbols),
	}
}

func unsupportedFiles(warnings []Warning) []string {
	seen := map[string]bool{}
	var paths []string
	for _, warning := range warnings {
		if warning.Code != "unsupported_language" || warning.Path == "" || seen[warning.Path] {
			continue
		}
		seen[warning.Path] = true
		paths = append(paths, warning.Path)
	}
	sort.Strings(paths)
	return paths
}
