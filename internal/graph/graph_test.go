package graph

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"inktrail/internal/changes"
)

func TestChainsForChanged(t *testing.T) {
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

	got := g.ChainsForChanged([]changes.Line{{Path: "app.go", LineNo: 10, Content: "func (s ServiceB) Do() { RepositoryB{}.Get() }"}})
	want := [][]string{{"app.ControllerA.Handle", "app.ServiceA.Do", "app.ServiceB.Do", "app.RepositoryB.Get"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
