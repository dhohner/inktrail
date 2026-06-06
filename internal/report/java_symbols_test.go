package report

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dhohner/inktrail/internal/diff"
	"github.com/dhohner/inktrail/internal/graph"
)

func TestBuildIncludesChangedJavaProductionSymbols(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "src/main/java/com/acme/Greeter.java")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`package com.acme;
class Greeter {
  void greet() {
    Runnable r = () -> { System.out.println("hi"); };
  }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := graph.Build(root)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	r := Build(g, diff.Result{Lines: []diff.Line{{Path: "src/main/java/com/acme/Greeter.java", LineNo: 4}}})
	want := []string{
		"src/main/java/com/acme/Greeter.java::com.acme.Greeter",
		"src/main/java/com/acme/Greeter.java::com.acme.Greeter.greet",
		"src/main/java/com/acme/Greeter.java::com.acme.Greeter.lambda@4",
	}
	if !reflect.DeepEqual(r.ChangedSymbols, want) {
		t.Fatalf("ChangedSymbols=%#v want %#v", r.ChangedSymbols, want)
	}
}
