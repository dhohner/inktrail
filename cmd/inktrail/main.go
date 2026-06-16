package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/dhohner/inktrail/internal/report"

	"github.com/dhohner/inktrail/internal/app"
	"github.com/dhohner/inktrail/internal/diff"
	"github.com/dhohner/inktrail/internal/graph"
	"github.com/dhohner/inktrail/internal/metadata"
)

var (
	hasStagedChanges   = diff.HasStagedChanges
	hasUnstagedChanges = diff.HasUnstagedChanges
	inspectDiff        = diff.Inspect
	buildGraph         = graph.Build
	buildGitGraph      = graph.BuildGit
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		var reported cliError
		if !errors.As(err, &reported) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

type cliError struct {
	err error
}

func (e cliError) Error() string { return e.err.Error() }
func (e cliError) Unwrap() error { return e.err }

func run(args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet("inktrail", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Usage of inktrail:")
		fmt.Fprintln(flags.Output(), "  inktrail --version [--json]")
		fmt.Fprintln(flags.Output(), "  inktrail [--fallback-to-head] [--format jsonl|json] [--output <path>]")
		fmt.Fprintln(flags.Output(), "  inktrail <commit>")
		fmt.Fprintln(flags.Output(), "  inktrail <base> <head>")
	}
	fallbackToHead := flags.Bool("fallback-to-head", false, "analyze HEAD when no commits are provided, the worktree is clean, and nothing is staged")
	reportFormat := flags.String("format", "jsonl", "report format: jsonl or json")
	outputPath := flags.String("output", "", "write report to path instead of stdout")
	showVersion := flags.Bool("version", false, "print version metadata and exit")
	jsonVersion := flags.Bool("json", false, "print version metadata as JSON (with --version)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return cliError{err: err}
	}

	if *showVersion {
		return writeVersion(out, *jsonVersion)
	}
	if *jsonVersion {
		return cliError{err: fmt.Errorf("--json requires --version")}
	}

	return analyzeWithOptions(flags.Args(), out, *fallbackToHead, *reportFormat, *outputPath)
}

func writeVersion(out io.Writer, asJSON bool) error {
	info := metadata.Current()
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetEscapeHTML(false)
		return enc.Encode(info)
	}
	_, err := fmt.Fprintf(out, "%s %s (commit %s, schema %s; languages: %s)\n", info.Name, info.Version, info.Commit, info.SchemaVersion, "go,java")
	return err
}

func analyze(commits []string, out io.Writer, fallbackToHead bool) error {
	return analyzeWithOptions(commits, out, fallbackToHead, "jsonl", "")
}

func analyzeWithOptions(commits []string, out io.Writer, fallbackToHead bool, format, outputPath string) error {
	writeReport, err := writerForFormat(format)
	if err != nil {
		return cliError{err: err}
	}
	var target io.Writer = out
	var file *os.File
	if outputPath != "" {
		file, err = os.Create(outputPath)
		if err != nil {
			return cliError{err: fmt.Errorf("write output %q: %w", outputPath, err)}
		}
		defer file.Close()
		target = file
	}
	r, err := newApp().BuildReport(commits, fallbackToHead)
	if err != nil {
		return err
	}
	if err := writeReport(target, r); err != nil {
		return cliError{err: fmt.Errorf("write output: %w", err)}
	}
	if file != nil {
		if err := file.Close(); err != nil {
			return cliError{err: fmt.Errorf("write output %q: %w", outputPath, err)}
		}
		file = nil
	}
	return nil
}

func writerForFormat(format string) (func(io.Writer, report.Report) error, error) {
	switch format {
	case "", "jsonl":
		return report.WriteJSONL, nil
	case "json":
		return report.WriteJSON, nil
	default:
		return nil, fmt.Errorf("unsupported format %q (supported: jsonl, json)", format)
	}
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
