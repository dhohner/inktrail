package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

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
		fmt.Fprintln(flags.Output(), "  inktrail [--fallback-to-head] [--best-effort] [--review-summary] [--format jsonl|json] [--output <path>]")
		fmt.Fprintln(flags.Output(), "  inktrail [--include <glob>] [--exclude <glob>] [--exclude-vendor] [--changed-only]")
		fmt.Fprintln(flags.Output(), "  inktrail [--max-lines-per-hunk N] [--max-context-lines N] [--max-records N] [--budget-tokens N]")
		fmt.Fprintln(flags.Output(), "  inktrail <commit>")
		fmt.Fprintln(flags.Output(), "  inktrail <base> <head>")
		fmt.Fprintln(flags.Output(), "  inktrail --base <ref> --head <ref>")
		fmt.Fprintln(flags.Output())
		fmt.Fprintln(flags.Output(), "Examples:")
		fmt.Fprintln(flags.Output(), "  inktrail --include 'internal/**/*.go'")
		fmt.Fprintln(flags.Output(), "  inktrail --exclude '**/*_test.go' --exclude-vendor")
		fmt.Fprintln(flags.Output(), "  inktrail --changed-only")
		fmt.Fprintln(flags.Output(), "  inktrail --fallback-to-head --review-summary --output inktrail.jsonl")
		fmt.Fprintln(flags.Output())
		flags.PrintDefaults()
	}
	fallbackToHead := flags.Bool("fallback-to-head", false, "analyze HEAD when no commits are provided, the worktree is clean, and nothing is staged")
	bestEffort := flags.Bool("best-effort", false, "emit partial report records with structured warning records instead of failing on analysis gaps")
	reviewSummary := flags.Bool("review-summary", false, "emit a compact agent-targeted review summary record")
	reportFormat := flags.String("format", "jsonl", "report format: jsonl or json")
	outputPath := flags.String("output", "", "write report to path instead of stdout")
	showVersion := flags.Bool("version", false, "print version metadata and exit")
	jsonVersion := flags.Bool("json", false, "print version metadata as JSON (with --version)")
	base := newRevisionFlag()
	head := newRevisionFlag()
	excludes := newStringListFlag()
	include := flags.String("include", "", "limit report to changed paths matching glob pattern")
	excludeVendor := flags.Bool("exclude-vendor", false, "exclude vendor-scoped files from report consideration")
	changedOnly := flags.Bool("changed-only", false, "limit emitted context to changed files")
	maxLinesPerHunk := flags.Int("max-lines-per-hunk", 0, "maximum changed lines emitted per hunk (0 means unlimited)")
	maxContextLines := flags.Int("max-context-lines", 0, "maximum lines emitted per declaration context excerpt (0 means normal report excerpt behavior)")
	maxRecords := flags.Int("max-records", 0, "maximum detail records emitted after the summary (0 means unlimited)")
	budgetTokens := flags.Int("budget-tokens", 0, "approximate report budget using ceil(serialized_character_count / 4); for planning, not exact tokenizer output (0 means unlimited)")
	flags.Var(excludes, "exclude", "exclude changed paths matching glob pattern (repeatable)")
	flags.Var(base, "base", "base revision for range analysis (requires --head)")
	flags.Var(head, "head", "head revision for range analysis (requires --base)")
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
	commits, err := resolveRevisionArgs(flags.Args(), base, head)
	if err != nil {
		return err
	}

	filters := report.FilterOptions{
		Include:       *include,
		Excludes:      excludes.values,
		ExcludeVendor: *excludeVendor,
		ChangedOnly:   *changedOnly,
	}
	if err := filters.Validate(); err != nil {
		return err
	}
	size := report.SizeOptions{MaxLinesPerHunk: *maxLinesPerHunk, MaxContextLines: *maxContextLines, MaxRecords: *maxRecords, BudgetTokens: *budgetTokens, EmitReviewSummary: *reviewSummary}
	if err := size.Validate(); err != nil {
		return err
	}
	return analyzeWithOptions(commits, out, *fallbackToHead, *bestEffort, *reportFormat, *outputPath, filters, size)
}

func resolveRevisionArgs(positionals []string, base, head *revisionFlag) ([]string, error) {
	for _, arg := range positionals {
		if isNamedRevisionArg(arg) {
			return nil, fmt.Errorf("cannot combine --base/--head with positional revision arguments")
		}
	}
	if base.set || head.set {
		if len(positionals) != 0 {
			return nil, fmt.Errorf("cannot combine --base/--head with positional revision arguments")
		}
		if !base.set || !head.set {
			return nil, fmt.Errorf("--base and --head must be provided together")
		}
		return []string{base.value, head.value}, nil
	}
	return positionals, nil
}

func isNamedRevisionArg(arg string) bool {
	return arg == "--base" || arg == "-base" || strings.HasPrefix(arg, "--base=") || strings.HasPrefix(arg, "-base=") ||
		arg == "--head" || arg == "-head" || strings.HasPrefix(arg, "--head=") || strings.HasPrefix(arg, "-head=")
}

type stringListFlag struct {
	values []string
}

func newStringListFlag() *stringListFlag { return &stringListFlag{} }

func (f *stringListFlag) String() string { return strings.Join(f.values, ",") }

func (f *stringListFlag) Set(value string) error {
	f.values = append(f.values, value)
	return nil
}

type revisionFlag struct {
	value string
	set   bool
}

func newRevisionFlag() *revisionFlag { return &revisionFlag{} }

func (f *revisionFlag) String() string { return f.value }

func (f *revisionFlag) Set(value string) error {
	f.value = value
	f.set = true
	return nil
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
	return analyzeWithOptions(commits, out, fallbackToHead, false, "jsonl", "", report.FilterOptions{}, report.SizeOptions{})
}

func analyzeWithOptions(commits []string, out io.Writer, fallbackToHead, bestEffort bool, format, outputPath string, filters report.FilterOptions, size report.SizeOptions) error {
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
	r, err := newApp().BuildReportWithOptions(commits, fallbackToHead, app.ReportOptions{PathFilter: filters, Size: size, BestEffort: bestEffort})
	if err != nil {
		return err
	}
	if err := writeReport(target, r, size); err != nil {
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

func writerForFormat(format string) (func(io.Writer, report.Report, report.SizeOptions) error, error) {
	switch format {
	case "", "jsonl":
		return report.WriteJSONLWithOptions, nil
	case "json":
		return report.WriteJSONWithOptions, nil
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
