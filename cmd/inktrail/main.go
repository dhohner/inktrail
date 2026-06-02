package main

import (
	"flag"
	"fmt"
	"os"

	"inktrail/internal/diff"
	"inktrail/internal/graph"
	"inktrail/internal/report"
)

func main() {
	flag.Parse()

	result, err := diff.Inspect(diff.Options{Commits: flag.Args()})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	g, err := graph.Build(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	old, err := graph.BuildGit(baseRef(flag.Args()))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := report.WriteJSON(os.Stdout, report.BuildWithBase(g, old, result)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func baseRef(args []string) string {
	switch len(args) {
	case 0:
		return "HEAD"
	case 1:
		return args[0] + "^"
	default:
		return args[0]
	}
}
