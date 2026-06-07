package report

import "github.com/dhohner/inktrail/internal/diff"

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

type SourceExcerpt struct {
	Content      string `json:"content"`
	Truncated    bool   `json:"truncated"`
	OmittedLines int    `json:"omitted_lines"`
}

type DeclarationContext struct {
	ID           string             `json:"id"`
	Path         string             `json:"path"`
	Name         string             `json:"name"`
	Kind         string             `json:"kind"`
	LineRange    LineRange          `json:"line_range"`
	Relationship string             `json:"relationship"`
	ChangedLines []ChangedLineRange `json:"changed_lines,omitempty"`
	Excerpt      SourceExcerpt      `json:"excerpt"`
}

type FileRecord struct {
	Type              string       `json:"type"`
	Status            string       `json:"status"`
	OldPath           string       `json:"old_path,omitempty"`
	Path              string       `json:"path"`
	Test              bool         `json:"test"`
	Language          string       `json:"language,omitempty"`
	Classification    []string     `json:"classification,omitempty"`
	ChangeIntent      []string     `json:"change_intent,omitempty"`
	DiffStat          DiffStat     `json:"diffstat"`
	Symbols           []string     `json:"symbols,omitempty"`
	ContentRef        *ContentRef  `json:"content_ref,omitempty"`
	Preview           *FilePreview `json:"preview,omitempty"`
	HunksOmitted      bool         `json:"hunks_omitted,omitempty"`
	OmittedLines      int          `json:"omitted_lines,omitempty"`
	MovedLinesOmitted int          `json:"moved_lines_omitted,omitempty"`
	Hunks             []diff.Hunk  `json:"hunks,omitempty"`
}

type MovedSymbol struct {
	From            string `json:"from"`
	To              string `json:"to"`
	BodySHA256Equal bool   `json:"body_sha256_equal"`
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
	Contexts       []DeclarationContext `json:"contexts"`
	Nodes          []Node               `json:"nodes"`
}
