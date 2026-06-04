package report

import (
	"encoding/json"
	"io"
)

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
