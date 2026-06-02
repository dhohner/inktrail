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
