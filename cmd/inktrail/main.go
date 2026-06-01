package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"inktrail/internal/changes"
	"inktrail/internal/graph"
	"inktrail/internal/report"
)

func main() {
	staged := flag.Bool("staged", false, "read staged changes (default when no commits are provided)")
	chains := flag.Bool("chains", false, "print call chains for changed code")
	reportOut := flag.Bool("report", false, "print AI-agent JSON report with changed locations and call sites")
	flag.Parse()

	lines, err := changes.Detect(changes.Options{
		Staged:  *staged,
		Commits: flag.Args(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if !*chains && !*reportOut {
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
	if *reportOut {
		if err := report.WriteJSON(os.Stdout, report.Build(g, lines)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	for _, chain := range g.ChainsForChanged(lines) {
		fmt.Println(strings.Join(chain, " -> "))
	}
}
