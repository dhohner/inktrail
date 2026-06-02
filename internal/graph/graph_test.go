package graph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildCallGraph(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "app.go", `package app

type ControllerA struct{}
type ServiceA struct{}
type ServiceB struct{}
type RepositoryB struct{}

func (c ControllerA) Handle() { ServiceA{}.Do() }
func (s ServiceA) Do() { ServiceB{}.Do() }
func (s ServiceB) Do() { RepositoryB{}.Get() }
func (r RepositoryB) Get() {}
`)

	g, err := Build(dir)
	if err != nil {
		t.Fatal(err)
	}

	if !g.Calls["app.ControllerA.Handle"]["app.ServiceA.Do"] {
		t.Fatalf("missing ControllerA.Handle -> ServiceA.Do edge")
	}
	if !g.Calls["app.ServiceA.Do"]["app.ServiceB.Do"] {
		t.Fatalf("missing ServiceA.Do -> ServiceB.Do edge")
	}
	if !g.Calls["app.ServiceB.Do"]["app.RepositoryB.Get"] {
		t.Fatalf("missing ServiceB.Do -> RepositoryB.Get edge")
	}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
