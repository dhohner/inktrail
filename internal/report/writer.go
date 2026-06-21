package report

import (
	"encoding/json"
	"io"

	"github.com/dhohner/inktrail/internal/metadata"
)

type Object struct {
	SchemaVersion  string               `json:"schema_version"`
	Summary        Summary              `json:"summary"`
	ReviewSummary  *ReviewSummary       `json:"review_summary,omitempty"`
	Warnings       []Warning            `json:"warnings,omitempty"`
	Files          []FileRecord         `json:"files"`
	ChangedSymbols []string             `json:"changed_symbols"`
	DeletedSymbols []string             `json:"deleted_symbols"`
	Contexts       []DeclarationContext `json:"contexts"`
	MovedSymbols   []MovedSymbol        `json:"moved_symbols"`
	RemovedCalls   []RemovedCall        `json:"removed_calls"`
	EntryPoints    []string             `json:"entry_points"`
	Nodes          []Node               `json:"nodes"`
}

func WriteJSONL(w io.Writer, r Report) error {
	return WriteJSONLWithOptions(w, r, SizeOptions{})
}

func WriteJSONLWithOptions(w io.Writer, r Report, opts SizeOptions) error {
	reviewSource := reviewSummarySource(r)
	return writeJSONLPrepared(w, prepareForWriteWithReviewSource(r, opts, "jsonl", reviewSource), opts, reviewSource)
}

func writeJSONLPrepared(w io.Writer, r Report, opts SizeOptions, reviewSource Report) error {
	r = withSchemaVersion(r)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(struct {
		Type string `json:"type"`
		Summary
	}{Type: "summary", Summary: r.Summary}); err != nil {
		return err
	}
	if opts.EmitReviewSummary {
		if err := enc.Encode(struct {
			Type string `json:"type"`
			ReviewSummary
		}{Type: "review_summary", ReviewSummary: buildReviewSummary(reviewSource)}); err != nil {
			return err
		}
	}
	for _, warning := range r.Warnings {
		if err := enc.Encode(struct {
			Type string `json:"type"`
			Warning
		}{Type: "warning", Warning: warning}); err != nil {
			return err
		}
	}
	fileSymbols := fileSymbolsForReport(r)
	for _, file := range r.Files {
		if err := enc.Encode(fileRecordWithOptions(file, fileSymbols[file.Path], opts)); err != nil {
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
	for _, context := range r.Contexts {
		if err := enc.Encode(struct {
			Type string `json:"type"`
			DeclarationContext
		}{Type: "declaration_context", DeclarationContext: context}); err != nil {
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

func WriteJSON(w io.Writer, r Report) error {
	return WriteJSONWithOptions(w, r, SizeOptions{})
}

func WriteJSONWithOptions(w io.Writer, r Report, opts SizeOptions) error {
	reviewSource := reviewSummarySource(r)
	return writeJSONPrepared(w, prepareForWriteWithReviewSource(r, opts, "json", reviewSource), opts, reviewSource)
}

func writeJSONPrepared(w io.Writer, r Report, opts SizeOptions, reviewSource Report) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(toObjectWithReviewSource(r, opts, reviewSource))
}

func ToObject(r Report) Object {
	return toObjectWithReviewSource(r, SizeOptions{}, reviewSummarySource(r))
}

func toObjectWithOptions(r Report, opts SizeOptions) Object {
	return toObjectWithReviewSource(r, opts, reviewSummarySource(r))
}

func toObjectWithReviewSource(r Report, opts SizeOptions, reviewSource Report) Object {
	r = withSchemaVersion(r)
	fileSymbols := fileSymbolsForReport(r)
	files := make([]FileRecord, 0, len(r.Files))
	for _, file := range r.Files {
		files = append(files, fileRecordWithOptions(file, fileSymbols[file.Path], opts))
	}
	obj := Object{
		SchemaVersion:  r.Summary.SchemaVersion,
		Summary:        r.Summary,
		Warnings:       cloneWarnings(r.Warnings),
		Files:          files,
		ChangedSymbols: cloneStrings(r.ChangedSymbols),
		DeletedSymbols: cloneStrings(r.DeletedSymbols),
		Contexts:       cloneDeclarationContexts(r.Contexts),
		MovedSymbols:   cloneMovedSymbols(r.MovedSymbols),
		RemovedCalls:   cloneRemovedCalls(r.RemovedCalls),
		EntryPoints:    cloneStrings(r.EntryPoints),
		Nodes:          cloneNodes(r.Nodes),
	}
	if opts.EmitReviewSummary {
		reviewSummary := buildReviewSummary(reviewSource)
		obj.ReviewSummary = &reviewSummary
	}
	return obj
}

func reviewSummarySource(r Report) Report {
	r = withSchemaVersion(r)
	if r.FileSymbols == nil {
		r.FileSymbols = symbolsByPath(r.Nodes)
	}
	return r
}

func fileSymbolsForReport(r Report) map[string][]string {
	if r.FileSymbols != nil {
		return r.FileSymbols
	}
	return symbolsByPath(r.Nodes)
}

func cloneStrings(in []string) []string {
	return append([]string{}, in...)
}

func cloneWarnings(in []Warning) []Warning {
	return append([]Warning{}, in...)
}

func cloneDeclarationContexts(in []DeclarationContext) []DeclarationContext {
	return append([]DeclarationContext{}, in...)
}

func cloneMovedSymbols(in []MovedSymbol) []MovedSymbol {
	return append([]MovedSymbol{}, in...)
}

func cloneRemovedCalls(in []RemovedCall) []RemovedCall {
	return append([]RemovedCall{}, in...)
}

func cloneNodes(in []Node) []Node {
	return append([]Node{}, in...)
}

func withSchemaVersion(r Report) Report {
	if r.Summary.SchemaVersion == "" {
		r.Summary.SchemaVersion = metadata.SchemaVersion
	}
	return r
}
