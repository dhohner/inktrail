package report

import (
	"testing"

	"github.com/dhohner/inktrail/internal/graph"
)

func TestDeclarationContextsIncludeJavaChangedSymbols(t *testing.T) {
	g := &graph.Graph{Functions: map[string]graph.Function{
		"com.acme.Greeter": {Name: "com.acme.Greeter", Path: "src/main/java/com/acme/Greeter.java", Kind: "class_declaration", StartLine: 2, EndLine: 6, Source: "class Greeter {\n  Greeter() {\n    this.name = \"hi\";\n  }\n}"},
		"com.acme.Greeter.<init>": {Name: "com.acme.Greeter.<init>", Path: "src/main/java/com/acme/Greeter.java", Kind: "constructor_declaration", StartLine: 3, EndLine: 5, Source: "Greeter() {\n  this.name = \"hi\";\n}"},
		"com.acme.Greeter.greet": {Name: "com.acme.Greeter.greet", Path: "src/main/java/com/acme/Greeter.java", Kind: "method_declaration", StartLine: 7, EndLine: 9, Source: "void greet() {\n  System.out.println(name);\n}"},
	}}

	got := declarationContexts(g, map[string][]ChangedLineRange{
		"com.acme.Greeter":        {{Start: 4, End: 4}},
		"com.acme.Greeter.<init>": {{Start: 4, End: 4}},
		"com.acme.Greeter.greet":  {{Start: 8, End: 8}},
	}, nil)

	if len(got) != 3 {
		t.Fatalf("declarationContexts() len=%d want 3: %#v", len(got), got)
	}
	byID := map[string]DeclarationContext{}
	for _, ctx := range got {
		byID[ctx.ID] = ctx
		if ctx.Path == "" || ctx.Name == "" || ctx.Relationship == "" || ctx.Excerpt.Content == "" {
			t.Fatalf("missing required context metadata: %#v", ctx)
		}
	}
	if byID["src/main/java/com/acme/Greeter.java::com.acme.Greeter"].Kind != "class" {
		t.Fatalf("class context kind=%q", byID["src/main/java/com/acme/Greeter.java::com.acme.Greeter"].Kind)
	}
	if byID["src/main/java/com/acme/Greeter.java::com.acme.Greeter.<init>"].Kind != "constructor" {
		t.Fatalf("constructor context kind=%q", byID["src/main/java/com/acme/Greeter.java::com.acme.Greeter.<init>"].Kind)
	}
	if byID["src/main/java/com/acme/Greeter.java::com.acme.Greeter.greet"].Kind != "method" {
		t.Fatalf("method context kind=%q", byID["src/main/java/com/acme/Greeter.java::com.acme.Greeter.greet"].Kind)
	}
}
