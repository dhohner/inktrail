package diff

import (
	"reflect"
	"testing"
)

func TestParseDiffOnlyAddedTargetLines(t *testing.T) {
	diff := []byte(`diff --git a/main.go b/main.go
index 111..222 100644
--- a/main.go
+++ b/main.go
@@ -10,2 +10,2 @@ func main() {
-old()
+new()
@@ -20,0 +21,2 @@ func other() {
+first()
+second()
`)

	got, err := ParseDiff(diff)
	if err != nil {
		t.Fatal(err)
	}

	want := []Line{
		{Path: "main.go", LineNo: 10, Content: "new()"},
		{Path: "main.go", LineNo: 21, Content: "first()"},
		{Path: "main.go", LineNo: 22, Content: "second()"},
	}

	if len(got) != len(want) {
		t.Fatalf("len=%d want=%d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%#v want=%#v", i, got[i], want[i])
		}
	}
}

func TestParseFilesIncludesStatusHunksAndTestFiles(t *testing.T) {
	diff := []byte(`diff --git a/app.go b/app.go
--- a/app.go
+++ b/app.go
@@ -10 +10,2 @@ func main() {
-old()
+new()
+next()
diff --git a/main_test.go b/main_test.go
new file mode 100644
--- /dev/null
+++ b/main_test.go
@@ -0,0 +1 @@
+test()
`)

	got, err := ParseFiles(diff)
	if err != nil {
		t.Fatal(err)
	}

	want := []FileChange{
		{Status: "modified", Path: "app.go", Hunks: []Hunk{{OldStart: 10, OldLines: 1, NewStart: 10, NewLines: 2, Lines: []HunkLine{
			{Op: "delete", OldLine: 10, Content: "old()"},
			{Op: "add", NewLine: 10, Content: "new()"},
			{Op: "add", NewLine: 11, Content: "next()"},
		}}}},
		{Status: "added", Path: "main_test.go", Test: true, Hunks: []Hunk{{OldStart: 0, OldLines: 0, NewStart: 1, NewLines: 1, Lines: []HunkLine{
			{Op: "add", NewLine: 1, Content: "test()"},
		}}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
}

func TestParseDiffIgnoresBlankAddedLines(t *testing.T) {
	diff := []byte(`diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,0 +2,3 @@ func main() {
+
+   
+call()
`)

	got, err := ParseDiff(diff)
	if err != nil {
		t.Fatal(err)
	}

	want := Line{Path: "main.go", LineNo: 4, Content: "call()"}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got=%#v want=%#v", got, []Line{want})
	}
}

func TestParseDiffIgnoresTestCode(t *testing.T) {
	diff := []byte(`diff --git a/main_test.go b/main_test.go
--- a/main_test.go
+++ b/main_test.go
@@ -1,0 +2 @@ func TestMain(t *testing.T) {
+assert()
`)

	got, err := ParseDiff(diff)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got test-code changes: %#v", got)
	}
}

func TestParseDiffIgnoresTestDirs(t *testing.T) {
	diff := []byte(`diff --git a/tests/helper.go b/tests/helper.go
--- a/tests/helper.go
+++ b/tests/helper.go
@@ -1,0 +2 @@ func helper() {
+helper()
`)

	got, err := ParseDiff(diff)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got test-dir changes: %#v", got)
	}
}

func TestParseJavaMavenGradleScopes(t *testing.T) {
	diff := []byte(`diff --git a/src/main/java/com/acme/App.java b/src/main/java/com/acme/App.java
--- a/src/main/java/com/acme/App.java
+++ b/src/main/java/com/acme/App.java
@@ -1,0 +2 @@ class App {
+void run() {}
diff --git a/service/src/test/java/com/acme/AppTest.java b/service/src/test/java/com/acme/AppTest.java
--- a/service/src/test/java/com/acme/AppTest.java
+++ b/service/src/test/java/com/acme/AppTest.java
@@ -1,0 +2 @@ class AppTest {
+void testRun() {}
diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1,0 +1 @@
+# docs
`)

	lines, err := ParseDiff(diff)
	if err != nil {
		t.Fatal(err)
	}
	wantLines := []Line{{Path: "src/main/java/com/acme/App.java", LineNo: 2, Content: "void run() {}"}, {Path: "README.md", LineNo: 1, Content: "# docs"}}
	if !reflect.DeepEqual(lines, wantLines) {
		t.Fatalf("lines=%#v want=%#v", lines, wantLines)
	}

	files, err := ParseFiles(diff)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("files=%#v", files)
	}
	if files[0].Test || !files[1].Test || files[2].Test {
		t.Fatalf("test scopes=%v,%v,%v", files[0].Test, files[1].Test, files[2].Test)
	}
}

func TestParseDiffIgnoresDeletedFiles(t *testing.T) {
	diff := []byte(`diff --git a/dead.go b/dead.go
deleted file mode 100644
--- a/dead.go
+++ /dev/null
@@ -1 +0,0 @@
-gone()
`)

	got, err := ParseDiff(diff)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got deleted-file changes: %#v", got)
	}
}
