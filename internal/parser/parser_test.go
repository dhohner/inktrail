package parser

import (
	"errors"
	"testing"
)

func TestParseSupportedLanguages(t *testing.T) {
	tests := []struct {
		name     string
		language Language
		source   []byte
		endLine  int
	}{
		{
			name:     "go",
			language: LanguageGo,
			source: []byte(`package sample

func Add(a int, b int) int {
	return a + b
}
`),
			endLine: 5,
		},
		{
			name:     "java",
			language: LanguageJava,
			source: []byte(`package com.example;

public class Greeter {
    public String greet(String name) {
        return "Hello, " + name;
    }
}
`),
			endLine: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse(tt.language, tt.source)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			defer doc.Close()

			if doc.HasSyntaxError() {
				t.Fatalf("expected representative %s source to parse without syntax errors", tt.language)
			}
			r := doc.RootNode().Range()
			if r.StartLine != 1 || r.EndLine != tt.endLine {
				t.Fatalf("root range lines = %d-%d, want 1-%d", r.StartLine, r.EndLine, tt.endLine)
			}
			if r.StartByte != 0 || r.EndByte != uint(len(tt.source)) {
				t.Fatalf("root range bytes = %d-%d, want 0-%d", r.StartByte, r.EndByte, len(tt.source))
			}
		})
	}
}

func TestDocumentRootNodeExposesLanguageNeutralTraversal(t *testing.T) {
	doc, err := Parse(LanguageJava, []byte(`class Greeter {
    String greet() { return "hi"; }
}
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	defer doc.Close()

	root := doc.RootNode()
	if root == nil || root.Kind() != "program" {
		t.Fatalf("RootNode() = %#v, want Java program node", root)
	}
	children := root.NamedChildren()
	if len(children) != 1 || children[0].Kind() != "class_declaration" {
		t.Fatalf("RootNode().NamedChildren() = %#v, want class declaration", children)
	}
	name := children[0].ChildByFieldName("name")
	if name == nil || name.Text(doc.Source) != "Greeter" {
		t.Fatalf("class name node = %#v", name)
	}
	if got := children[0].Range(); got.StartLine != 1 || got.EndLine != 3 {
		t.Fatalf("class range = %+v, want lines 1-3", got)
	}
}

func TestUnsupportedLanguagesStayOutsideFoundation(t *testing.T) {
	if _, ok := LanguageForPath("README.md"); ok {
		t.Fatal("LanguageForPath() reported markdown as supported")
	}

	_, err := Parse(Language("ruby"), []byte("puts 'hello'\n"))
	if !errors.Is(err, ErrUnsupportedLanguage) {
		t.Fatalf("Parse() error = %v, want ErrUnsupportedLanguage", err)
	}
}

func TestDocumentCloseIsIdempotent(t *testing.T) {
	doc, err := Parse(LanguageGo, []byte("package main\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	doc.Close()
	doc.Close()

	if got := doc.RootNode(); got != nil {
		t.Fatalf("RootNode() after Close = %#v, want nil", got)
	}
}
