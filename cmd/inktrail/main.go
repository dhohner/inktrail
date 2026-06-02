package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"inktrail/internal/diff"
	"inktrail/internal/graph"
	"inktrail/internal/report"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("inktrail", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}
	commits := flags.Args()

	result, err := diff.Inspect(diff.Options{Commits: commits})
	if err != nil {
		return err
	}

	current, err := graph.Build(".")
	if err != nil {
		return err
	}
	base, err := graph.BuildGit(baseRef(commits))
	if err != nil {
		return err
	}
	return report.WriteJSON(out, report.BuildWithBase(current, base, result))
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
