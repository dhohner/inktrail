package source

import "testing"

func TestIsGoTestPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{path: "app/main.go", want: false},
		{path: "app/main_test.go", want: true},
		{path: "test/helper.go", want: true},
		{path: "tests/helper.go", want: true},
		{path: "pkg/testdata/sample.go", want: true},
	}

	for _, tc := range cases {
		if got := IsGoTestPath(tc.path); got != tc.want {
			t.Fatalf("IsGoTestPath(%q)=%v want=%v", tc.path, got, tc.want)
		}
	}
}

func TestIsJavaTestPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{path: "src/main/java/com/acme/App.java", want: false},
		{path: "src/test/java/com/acme/AppTest.java", want: true},
		{path: "service/src/main/java/com/acme/App.java", want: false},
		{path: "service/src/test/java/com/acme/AppTest.java", want: true},
		{path: "service/src/integrationTest/java/com/acme/AppIT.java", want: true},
		{path: "src/test/resources/application.yaml", want: false},
	}

	for _, tc := range cases {
		if got := IsJavaTestPath(tc.path); got != tc.want {
			t.Fatalf("IsJavaTestPath(%q)=%v want=%v", tc.path, got, tc.want)
		}
	}
}
