package report

import (
	"encoding/json"
	"io"

	"github.com/dhohner/inktrail/internal/metadata"
)

type Object struct {
	SchemaVersion  string               `json:"schema_version"`
	Summary        Summary              `json:"summary"`
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
	r = withSchemaVersion(r)
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
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(ToObject(r))
}

func ToObject(r Report) Object {
	r = withSchemaVersion(r)
	fileSymbols := symbolsByPath(r.Nodes)
	files := make([]FileRecord, 0, len(r.Files))
	for _, file := range r.Files {
		files = append(files, fileRecord(file, fileSymbols[file.Path]))
	}
	return Object{
		SchemaVersion:  r.Summary.SchemaVersion,
		Summary:        r.Summary,
		Files:          files,
		ChangedSymbols: append([]string(nil), r.ChangedSymbols...),
		DeletedSymbols: append([]string(nil), r.DeletedSymbols...),
		Contexts:       append([]DeclarationContext(nil), r.Contexts...),
		MovedSymbols:   append([]MovedSymbol(nil), r.MovedSymbols...),
		RemovedCalls:   append([]RemovedCall(nil), r.RemovedCalls...),
		EntryPoints:    append([]string(nil), r.EntryPoints...),
		Nodes:          append([]Node(nil), r.Nodes...),
	}
}

func withSchemaVersion(r Report) Report {
	if r.Summary.SchemaVersion == "" {
		r.Summary.SchemaVersion = metadata.SchemaVersion
	}
	return r
}
