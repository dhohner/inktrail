package report

import (
	"os"
	"path/filepath"
	"testing"
)

func nodesByID(nodes []Node) map[string]Node {
	out := map[string]Node{}
	for _, node := range nodes {
		out[node.ID] = node
	}
	return out
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
