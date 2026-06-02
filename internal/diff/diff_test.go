package diff

import "testing"

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
		{Status: "modified", OldPath: "app.go", Path: "app.go", Hunks: []Hunk{{OldStart: 10, OldLines: 1, NewStart: 10, NewLines: 2}}},
		{Status: "added", OldPath: "main_test.go", Path: "main_test.go", Test: true, Hunks: []Hunk{{OldStart: 0, OldLines: 0, NewStart: 1, NewLines: 1}}},
	}
	if len(got) != len(want) {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
	for i := range want {
		if got[i].Status != want[i].Status || got[i].OldPath != want[i].OldPath || got[i].Path != want[i].Path || got[i].Test != want[i].Test || len(got[i].Hunks) != len(want[i].Hunks) || got[i].Hunks[0] != want[i].Hunks[0] {
			t.Fatalf("got[%d]=%#v want=%#v", i, got[i], want[i])
		}
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
