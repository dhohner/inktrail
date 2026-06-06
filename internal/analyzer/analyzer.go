package analyzer

import (
	"github.com/dhohner/inktrail/internal/parser"
)

// Symbol is a language-specific declaration that Inktrail can track in reports.
type Symbol struct {
	Name string
	Node parser.Node
}

// Call is a language-specific call expression. To is intentionally an unresolved
// callable name; graph construction resolves it against known symbols.
type Call struct {
	From string
	To   string
	Site CallSite
}

// CallSite describes where a call expression appears in source.
type CallSite struct {
	Path   string
	LineNo int
	Code   string
}

// Source is the parsed source file passed to language analyzers.
type Source struct {
	Path     string
	Source   []byte
	Doc      *parser.Document
	Language parser.Language
	Package  string
}

// Analyzer contains all language-specific graph extraction behavior.
type Analyzer interface {
	Language() parser.Language
	IsProductionPath(path string) bool
	PackageName(root *parser.Node, source []byte) string
	Symbols(Source) []Symbol
	Calls(Source) []Call
}
