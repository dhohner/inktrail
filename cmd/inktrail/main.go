package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"inktrail/internal/app"
	"inktrail/internal/diff"
	"inktrail/internal/graph"
	"inktrail/internal/ui"
)

var (
	hasStagedChanges   = diff.HasStagedChanges
	hasUnstagedChanges = diff.HasUnstagedChanges
	inspectDiff        = diff.Inspect
	buildGraph         = graph.Build
	buildGitGraph      = graph.BuildGit
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
	noUI := flags.Bool("no-ui", false, "skip the interactive selector; analyze staged changes, or HEAD if nothing is staged")
	flags.BoolVar(noUI, "agent", false, "alias for --no-ui")
	if err := flags.Parse(args); err != nil {
		return err
	}

	commits := flags.Args()
	if len(commits) == 0 && !*noUI && stdioIsTerminal() {
		selected, err := ui.PromptAnalysis()
		if err != nil {
			return err
		}
		commits = selected
	}

	return analyze(commits, out, *noUI)
}

func stdioIsTerminal() bool {
	stdin, err := os.Stdin.Stat()
	if err != nil || stdin.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	stdout, err := os.Stdout.Stat()
	if err != nil || stdout.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	return true
}

func analyze(commits []string, out io.Writer, fallbackToHead bool) error {
	return newApp().Analyze(commits, out, fallbackToHead)
}

func resolveCommits(commits []string, fallbackToHead bool) ([]string, error) {
	return newApp().ResolveCommits(commits, fallbackToHead)
}

func newApp() app.App {
	return app.New(app.Dependencies{
		HasStagedChanges:   hasStagedChanges,
		HasUnstagedChanges: hasUnstagedChanges,
		InspectDiff:        inspectDiff,
		BuildGraph:         buildGraph,
		BuildGitGraph:      buildGitGraph,
	})
}
