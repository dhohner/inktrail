package main

import (
	"flag"
	"fmt"
	"os"

	"inktrail/internal/changes"
)

func main() {
	staged := flag.Bool("staged", false, "read staged changes")
	flag.Parse()

	lines, err := changes.Detect(changes.Options{
		Staged:  *staged,
		Commits: flag.Args(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	for _, line := range lines {
		fmt.Printf("%s:%d:%s\n", line.Path, line.LineNo, line.Content)
	}
}
