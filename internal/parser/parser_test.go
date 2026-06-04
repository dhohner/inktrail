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
			r := doc.RootRange()
			if r.StartLine != 1 || r.EndLine != tt.endLine {
				t.Fatalf("RootRange() lines = %d-%d, want 1-%d", r.StartLine, r.EndLine, tt.endLine)
			}
			if !r.ContainsLine(3) || r.ContainsLine(tt.endLine+1) {
				t.Fatalf("ContainsLine() did not provide stable changed-line containment for %+v", r)
			}
			if r.StartByte != 0 || r.EndByte != uint(len(tt.source)) {
				t.Fatalf("RootRange() bytes = %d-%d, want 0-%d", r.StartByte, r.EndByte, len(tt.source))
			}
		})
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

	if got := doc.RootRange(); got != (Range{}) {
		t.Fatalf("RootRange() after Close = %+v, want zero value", got)
	}
}
