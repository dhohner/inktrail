package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/dhohner/inktrail/internal/app"
	"github.com/dhohner/inktrail/internal/diff"
	"github.com/dhohner/inktrail/internal/graph"
	"github.com/dhohner/inktrail/internal/report"
	"github.com/dhohner/inktrail/internal/ui"
)

var (
	hasStagedChanges   = diff.HasStagedChanges
	hasUnstagedChanges = diff.HasUnstagedChanges
	inspectDiff        = diff.Inspect
	buildGraph         = graph.Build
	buildGitGraph      = graph.BuildGit
	openBrowser        = openPathInBrowser
	isTerminal         = stdioIsTerminal
	promptAnalysis     = ui.PromptAnalysis
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
	humanReport := false
	if len(commits) == 0 && !*noUI && isTerminal() {
		selected, err := promptAnalysis()
		if err != nil {
			return err
		}
		commits = selected
		humanReport = true
	}

	if humanReport {
		return analyzeHTML(commits, os.Stderr, false)
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

func analyzeHTML(commits []string, messages io.Writer, fallbackToHead bool) error {
	r, err := newApp().BuildReport(commits, fallbackToHead)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(".", "inktrail-*.html")
	if err != nil {
		return err
	}
	path := file.Name()
	if err := report.WriteHTML(file, r); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	fmt.Fprintf(messages, "inktrail: HTML report written to %s\n", path)
	if err := openBrowser(path); err != nil {
		fmt.Fprintf(messages, "inktrail: could not open browser: %v\n", err)
	}
	return nil
}

func openPathInBrowser(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
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
