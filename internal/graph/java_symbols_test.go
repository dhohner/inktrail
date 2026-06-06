package graph

import "testing"

func TestJavaProductionSymbolsFromTreeSitter(t *testing.T) {
	src := []byte(`package com.acme;
class Greeter {
  Greeter() {}
  void greet() {
    Runnable r = () -> { System.out.println("hi"); };
    Object o = new Object() { void run() {} };
  }
}
interface Welcomer { default void welcome() {} }
record Point(int x) { public Point {} }
enum Mood { HAPPY; void smile() {} }
`)
	g, err := buildFromSources([]sourceFile{{Path: "src/main/java/com/acme/Greeter.java", Source: src}})
	if err != nil {
		t.Fatalf("buildFromSources() error = %v", err)
	}
	want := []string{
		"com.acme.Greeter",
		"com.acme.Greeter.<init>",
		"com.acme.Greeter.greet",
		"com.acme.Greeter.lambda@5",
		"com.acme.Greeter.anonymous@6",
		"com.acme.Greeter.anonymous@6.run",
		"com.acme.Welcomer",
		"com.acme.Welcomer.welcome",
		"com.acme.Point",
		"com.acme.Point.<init>",
		"com.acme.Mood",
		"com.acme.Mood.smile",
	}
	for _, name := range want {
		if _, ok := g.Functions[name]; !ok {
			t.Fatalf("missing Java symbol %s; got %#v", name, g.Functions)
		}
	}
	if got := g.FunctionsContainingLine("src/main/java/com/acme/Greeter.java", 5); len(got) == 0 {
		t.Fatalf("FunctionsContainingLine() did not find Java symbols for changed line")
	}
}

func TestJavaTestSourcesExcludedFromGraph(t *testing.T) {
	files, err := loadFiles(t.TempDir())
	if err != nil || len(files) != 0 {
		t.Fatalf("empty loadFiles()=(%#v,%v)", files, err)
	}
	if isProductionSourceFile("src/test/java/com/acme/GreeterTest.java") {
		t.Fatal("Java test source reported production")
	}
}
