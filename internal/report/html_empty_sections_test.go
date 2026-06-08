package report

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteHTMLSkipsEmptySections(t *testing.T) {
	var out bytes.Buffer
	if err := WriteHTML(&out, Report{}); err != nil {
		t.Fatal(err)
	}
	html := out.String()
	for _, heading := range []string{"<h2>Changed symbols</h2>", "<h2>Deleted symbols</h2>", "<h2>Moved symbols</h2>"} {
		if strings.Contains(html, heading) {
			t.Fatalf("html rendered empty section %q: %s", heading, html)
		}
	}
}
