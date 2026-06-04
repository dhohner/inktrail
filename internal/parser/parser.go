// Package parser provides a small Tree-sitter based parsing foundation for
// source languages that Inktrail explicitly supports.
package parser

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
	treesittergo "github.com/tree-sitter/tree-sitter-go/bindings/go"
	treesitterjava "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

// Language identifies a source language with a bundled Tree-sitter grammar.
type Language string

const (
	LanguageGo   Language = "go"
	LanguageJava Language = "java"
)

var ErrUnsupportedLanguage = errors.New("unsupported parser language")

// Range is a stable source range using 1-based line numbers and byte offsets.
// EndLine is inclusive for non-empty ranges and equals StartLine for single-line
// ranges. Byte offsets are zero-based and half-open: [StartByte, EndByte).
type Range struct {
	StartLine int
	EndLine   int
	StartByte uint
	EndByte   uint
}

// Document owns the native Tree-sitter tree returned from a parse. Call Close
// when finished with the document.
type Document struct {
	Language Language
	Source   []byte
	tree     *sitter.Tree
}

// Close releases native resources associated with the parsed tree.
func (d *Document) Close() {
	if d == nil || d.tree == nil {
		return
	}
	d.tree.Close()
	d.tree = nil
}

// RootRange returns the source range covered by the parse tree root node.
func (d *Document) RootRange() Range {
	if d == nil || d.tree == nil {
		return Range{}
	}
	return rangeFromSitter(d.tree.RootNode().Range())
}

// HasSyntaxError reports whether Tree-sitter marked the parse with errors.
func (d *Document) HasSyntaxError() bool {
	return d != nil && d.tree != nil && d.tree.RootNode().HasError()
}

// ContainsLine reports whether r contains the given 1-based line number.
func (r Range) ContainsLine(line int) bool {
	return line >= r.StartLine && line <= r.EndLine
}

// LanguageForPath returns the supported parser language for path, if any.
func LanguageForPath(path string) (Language, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return LanguageGo, true
	case ".java":
		return LanguageJava, true
	default:
		return "", false
	}
}

// Parse parses source with the requested supported language. The parser is
// closed before returning; the returned Document owns only the parse tree.
func Parse(language Language, source []byte) (*Document, error) {
	grammar, err := grammarFor(language)
	if err != nil {
		return nil, err
	}

	p := sitter.NewParser()
	defer p.Close()
	if err := p.SetLanguage(grammar); err != nil {
		return nil, fmt.Errorf("set %s parser language: %w", language, err)
	}

	tree := p.Parse(source, nil)
	if tree == nil {
		return nil, fmt.Errorf("parse %s source: no tree returned", language)
	}

	return &Document{Language: language, Source: append([]byte(nil), source...), tree: tree}, nil
}

func grammarFor(language Language) (*sitter.Language, error) {
	switch language {
	case LanguageGo:
		return sitter.NewLanguage(treesittergo.Language()), nil
	case LanguageJava:
		return sitter.NewLanguage(treesitterjava.Language()), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLanguage, language)
	}
}

func rangeFromSitter(r sitter.Range) Range {
	startLine := int(r.StartPoint.Row) + 1
	endLine := int(r.EndPoint.Row) + 1
	if r.EndPoint.Column == 0 && endLine > startLine {
		endLine--
	}
	return Range{StartLine: startLine, EndLine: endLine, StartByte: r.StartByte, EndByte: r.EndByte}
}
