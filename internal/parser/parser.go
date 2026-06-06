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

type languageSpec struct {
	Language   Language
	Extensions []string
	Grammar    func() *sitter.Language
}

const (
	LanguageGo   Language = "go"
	LanguageJava Language = "java"
)

var (
	ErrUnsupportedLanguage = errors.New("unsupported parser language")

	languageSpecs = []languageSpec{
		{
			Language:   LanguageGo,
			Extensions: []string{".go"},
			Grammar:    func() *sitter.Language { return sitter.NewLanguage(treesittergo.Language()) },
		},
		{
			Language:   LanguageJava,
			Extensions: []string{".java"},
			Grammar:    func() *sitter.Language { return sitter.NewLanguage(treesitterjava.Language()) },
		},
	}
)

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

// Node is a language-neutral view over a Tree-sitter node. It intentionally
// exposes only stable source shape needed by language analyzers.
type Node struct {
	node sitter.Node
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
	root := d.RootNode()
	if root == nil {
		return Range{}
	}
	return root.Range()
}

// RootNode returns the root syntax node, or nil after Close.
func (d *Document) RootNode() *Node {
	if d == nil || d.tree == nil {
		return nil
	}
	return &Node{node: *d.tree.RootNode()}
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
	ext := strings.ToLower(filepath.Ext(path))
	for _, spec := range languageSpecs {
		for _, candidate := range spec.Extensions {
			if ext == candidate {
				return spec.Language, true
			}
		}
	}
	return "", false
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
	for _, spec := range languageSpecs {
		if spec.Language == language {
			return spec.Grammar(), nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrUnsupportedLanguage, language)
}

// Kind returns this node's grammar kind.
func (n *Node) Kind() string {
	if n == nil {
		return ""
	}
	return n.node.Kind()
}

// Range returns this node's stable source range.
func (n *Node) Range() Range {
	if n == nil {
		return Range{}
	}
	return rangeFromSitter(n.node.Range())
}

// Text returns the source bytes covered by this node as UTF-8 text.
func (n *Node) Text(source []byte) string {
	if n == nil {
		return ""
	}
	return n.node.Utf8Text(source)
}

// NamedChildren returns this node's named children in source order.
func (n *Node) NamedChildren() []Node {
	if n == nil {
		return nil
	}
	cursor := n.node.Walk()
	defer cursor.Close()
	return wrapNodes(n.node.NamedChildren(cursor))
}

// ChildByFieldName returns this node's child for a grammar field, if any.
func (n *Node) ChildByFieldName(fieldName string) *Node {
	if n == nil {
		return nil
	}
	child := n.node.ChildByFieldName(fieldName)
	if child == nil {
		return nil
	}
	return &Node{node: *child}
}

func wrapNodes(nodes []sitter.Node) []Node {
	out := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, Node{node: node})
	}
	return out
}

func rangeFromSitter(r sitter.Range) Range {
	startLine := int(r.StartPoint.Row) + 1
	endLine := int(r.EndPoint.Row) + 1
	if r.EndPoint.Column == 0 && endLine > startLine {
		endLine--
	}
	return Range{StartLine: startLine, EndLine: endLine, StartByte: r.StartByte, EndByte: r.EndByte}
}
