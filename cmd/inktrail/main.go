package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"inktrail/internal/diff"
	"inktrail/internal/graph"
	"inktrail/internal/report"
)

type analysisMode int

const (
	modeStaged analysisMode = iota
	modeCommit
	modeRange
)

type modeOption struct {
	label       string
	description string
	placeholder string
	help        string
}

var modeOptions = []modeOption{
	{
		label:       "Analyze staged files",
		description: "Review exactly what is staged for your next commit.",
	},
	{
		label:       "Analyze a commit",
		description: "Inspect one commit against its parent.",
		placeholder: "HEAD",
		help:        "commit ref, e.g. 'HEAD' or 'abc1234'",
	},
	{
		label:       "Analyze a commit range",
		description: "Compare two refs, branches, tags, or SHAs.",
		placeholder: "main HEAD",
		help:        "commit range, e.g. 'main HEAD' or 'main..HEAD'",
	},
}

var (
	appStyle = lipgloss.NewStyle().
			Margin(1, 2).
			Padding(1, 3).
			Border(lipgloss.DoubleBorder()).
			BorderForeground(lipgloss.Color("63")).
			Width(76)

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))

	subtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).MarginTop(1)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)

	selectedItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("63")).Bold(true).Padding(0, 1).Width(66)
	itemStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Padding(0, 1).Width(66)
	descriptionStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).PaddingLeft(4)
	keyStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("238")).Padding(0, 1)
	inputBoxStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("99")).Padding(1, 2).Width(66)
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
	if len(commits) == 0 {
		selected, err := promptAnalysis()
		if err != nil {
			return err
		}
		commits = selected
	}

	return analyze(commits, out)
}

func analyze(commits []string, out io.Writer) error {
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

func promptAnalysis() ([]string, error) {
	model := newPromptModel()
	result, err := tea.NewProgram(model).Run()
	if err != nil {
		return nil, err
	}
	m := result.(promptModel)
	if m.cancelled {
		return nil, fmt.Errorf("cancelled")
	}
	return m.commits(), nil
}

type promptModel struct {
	cursor    int
	mode      analysisMode
	input     textinput.Model
	entering  bool
	cancelled bool
	err       string
}

func newPromptModel() promptModel {
	input := textinput.New()
	input.CharLimit = 120
	input.Width = 50
	input.Prompt = "➜ "
	input.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	input.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
	input.PlaceholderStyle = subtleStyle
	return promptModel{input: input}
}

func (m promptModel) Init() tea.Cmd { return nil }

func (m promptModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if !m.entering {
				m.mode = analysisMode(m.cursor)
				if m.mode == modeStaged {
					return m, tea.Quit
				}
				option := modeOptions[m.mode]
				m.entering = true
				m.err = ""
				m.input.Placeholder = option.placeholder
				m.input.Focus()
				return m, nil
			}
			value := strings.TrimSpace(m.input.Value())
			if value == "" {
				m.err = "value required"
				return m, nil
			}
			if m.mode == modeRange && len(parseRange(value)) != 2 {
				m.err = "enter range as <from> <to> or <from>..<to>"
				return m, nil
			}
			return m, tea.Quit
		case "up", "k":
			if !m.entering && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if !m.entering && m.cursor < len(modeOptions)-1 {
				m.cursor++
			}
		case "backspace":
			if m.entering && m.input.Value() == "" {
				m.entering = false
				m.err = ""
				m.input.Blur()
				return m, nil
			}
		}
	}
	if m.entering {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m promptModel) View() string {
	if m.entering {
		return m.inputView()
	}

	var b strings.Builder
	b.WriteString(headerView("Choose an analysis target") + "\n\n")
	for i, option := range modeOptions {
		if m.cursor == i {
			b.WriteString(selectedItemStyle.Render("▸ "+option.label) + "\n")
		} else {
			b.WriteString(itemStyle.Render("  "+option.label) + "\n")
		}
		b.WriteString(descriptionStyle.Render(option.description) + "\n")
		if i != len(modeOptions)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString(helpStyle.Render(keyStyle.Render("↑/↓") + " move  " + keyStyle.Render("j/k") + " move  " + keyStyle.Render("enter") + " select  " + keyStyle.Render("esc") + " quit"))
	return appStyle.Render(b.String())
}

func (m promptModel) inputView() string {
	var b strings.Builder
	option := modeOptions[m.mode]
	b.WriteString(headerView(option.label) + "\n\n")
	b.WriteString(inputBoxStyle.Render(m.input.View()) + "\n")
	if m.err != "" {
		b.WriteString("\n" + errorStyle.Render("✕ "+m.err) + "\n")
	}
	b.WriteString("\n" + subtleStyle.Render("Hint: "+option.help) + "\n")
	b.WriteString(helpStyle.Render(keyStyle.Render("enter") + " submit  " + keyStyle.Render("backspace") + " back  " + keyStyle.Render("esc") + " quit"))
	return appStyle.Render(b.String())
}

func headerView(subtitle string) string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center, titleStyle.Render("inktrail"), " ", subtleStyle.Render("code-change cartography")),
		subtitleStyle.Render(subtitle),
	)
}

func (m promptModel) commits() []string {
	switch m.mode {
	case modeCommit:
		return []string{strings.TrimSpace(m.input.Value())}
	case modeRange:
		return parseRange(m.input.Value())
	default:
		return nil
	}
}

func parseRange(raw string) []string {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "..") {
		parts := strings.SplitN(raw, "..", 2)
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		if left == "" || right == "" {
			return nil
		}
		return []string{left, right}
	}
	parts := strings.Fields(raw)
	if len(parts) != 2 {
		return nil
	}
	return parts
}
