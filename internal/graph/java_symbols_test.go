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

func TestJavaCallsResolveLocalTargetsAndSkipExternalReceivers(t *testing.T) {
	src := []byte(`package com.acme;
class A {
  void f() {
    helper();
    B.s();
    new B().g();
    System.out.println("external");
  }
  void helper() {}
  void println(String s) {}
}
class B {
  static void s() {}
  void g() {}
}
`)
	g, err := buildFromSources([]sourceFile{{Path: "src/main/java/com/acme/A.java", Source: src}})
	if err != nil {
		t.Fatalf("buildFromSources() error = %v", err)
	}
	from := "com.acme.A.f"
	for _, to := range []string{"com.acme.A.helper", "com.acme.B.s", "com.acme.B.g"} {
		if !g.Calls[from][to] {
			t.Fatalf("missing edge %s -> %s; calls=%#v", from, to, g.Calls[from])
		}
		if _, ok := g.CallSite(from, to); !ok {
			t.Fatalf("missing call site for %s -> %s", from, to)
		}
	}
	if g.Calls[from]["com.acme.A.println"] {
		t.Fatalf("external System.out.println resolved to local println: %#v", g.Calls[from])
	}
}

func TestJavaUnqualifiedCallsPreferCurrentOwner(t *testing.T) {
	src := []byte(`package com.acme;
class A {
  void f() { helper(); }
  void helper() {}
}
class B {
  void helper() {}
}
`)
	g, err := buildFromSources([]sourceFile{{Path: "src/main/java/com/acme/A.java", Source: src}})
	if err != nil {
		t.Fatalf("buildFromSources() error = %v", err)
	}
	from := "com.acme.A.f"
	if !g.Calls[from]["com.acme.A.helper"] {
		t.Fatalf("missing owner-qualified edge; calls=%#v", g.Calls[from])
	}
	if g.Calls[from]["com.acme.B.helper"] {
		t.Fatalf("unqualified call linked to sibling helper: %#v", g.Calls[from])
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
