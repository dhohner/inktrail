package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"inktrail/internal/changes"
	"inktrail/internal/graph"
)

func main() {
	staged := flag.Bool("staged", false, "read staged changes")
	chains := flag.Bool("chains", false, "print call chains for changed code")
	flag.Parse()

	lines, err := changes.Detect(changes.Options{
		Staged:  *staged,
		Commits: flag.Args(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if !*chains {
		for _, line := range lines {
			fmt.Printf("%s:%d:%s\n", line.Path, line.LineNo, line.Content)
		}
		return
	}

	g, err := graph.Build(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, chain := range g.ChainsForChanged(lines) {
		fmt.Println(strings.Join(chain, " -> "))
	}
}
