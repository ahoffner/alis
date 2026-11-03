// Package picker is the fuzzy command finder behind running `alis` with no arguments.
package picker

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"github.com/ahoffner/alis/internal/manifest"
	"github.com/ahoffner/alis/internal/registry"
)

// ErrCancelled is returned when the user closes the picker without choosing.
var ErrCancelled = errors.New("cancelled")

// Pick shows every effective command and returns the one chosen. It draws on /dev/tty, so
// it works while stdout is captured by the shell wrapper.
func Pick(reg *registry.Registry) (*manifest.Command, error) {
	if len(reg.Commands) == 0 {
		return nil, errors.New("no commands available; run `alis doctor`")
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("the picker needs a terminal: %w", err)
	}
	defer tty.Close()

	m := newModel(reg, lipgloss.NewRenderer(tty))
	final, err := tea.NewProgram(m, tea.WithInput(tty), tea.WithOutput(tty), tea.WithAltScreen()).Run()
	if err != nil {
		return nil, err
	}
	if c := final.(*model).chosen; c != nil {
		return c, nil
	}
	return nil, ErrCancelled
}

type styles struct {
	name, dim, selected, border lipgloss.Style
}

type model struct {
	reg     *registry.Registry
	cmds    []*manifest.Command
	labels  []string // what the query is matched against
	input   textinput.Model
	matches []int // indexes into cmds, best first
	cursor  int
	width   int
	height  int
	nameW   int
	chosen  *manifest.Command
	st      styles
}

func newModel(reg *registry.Registry, r *lipgloss.Renderer) *model {
	in := textinput.New()
	in.Prompt = "alis> "
	in.Placeholder = "type to search commands"
	in.Focus()
	m := &model{
		reg:   reg,
		cmds:  reg.Commands,
		input: in,
		st: styles{
			name:     r.NewStyle().Foreground(lipgloss.Color("2")).Bold(true),
			dim:      r.NewStyle().Faint(true),
			selected: r.NewStyle().Reverse(true),
			border:   r.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("8")).Padding(0, 1),
		},
	}
	for _, c := range m.cmds {
		m.labels = append(m.labels, c.Name+" "+c.Description+" "+c.Pack.Name)
		m.nameW = max(m.nameW, len(c.Name))
	}
	m.filter()
	return m
}

func (m *model) filter() {
	m.matches = m.matches[:0]
	m.cursor = 0
	q := strings.TrimSpace(m.input.Value())
	if q == "" {
		for i := range m.cmds {
			m.matches = append(m.matches, i)
		}
		return
	}
	for _, match := range fuzzy.Find(q, m.labels) {
		m.matches = append(m.matches, match.Index)
	}
}

func (m *model) Init() tea.Cmd { return textinput.Blink }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if len(m.matches) > 0 {
				m.chosen = m.cmds[m.matches[m.cursor]]
			}
			return m, tea.Quit
		case "up", "ctrl+p", "ctrl+k":
			m.cursor = max(0, m.cursor-1)
			return m, nil
		case "down", "ctrl+n", "ctrl+j", "tab":
			m.cursor = min(len(m.matches)-1, m.cursor+1)
			return m, nil
		}
	}
	before := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != before {
		m.filter()
	}
	return m, cmd
}

func (m *model) View() string {
	if m.width == 0 {
		return ""
	}
	listW, rows := m.width, max(3, m.height-3)
	side := m.width >= 90
	if side {
		listW = m.width / 2
	} else {
		rows = max(3, (m.height-3)/2)
	}

	// Scroll so the cursor stays visible.
	start := 0
	if m.cursor >= rows {
		start = m.cursor - rows + 1
	}
	var lines []string
	for i := start; i < len(m.matches) && i < start+rows; i++ {
		c := m.cmds[m.matches[i]]
		text := truncate(fmt.Sprintf("%-*s  %s", m.nameW, c.Name, c.Description), listW-len(c.Pack.Name)-6)
		line := fmt.Sprintf("  %s  %s", text, m.st.dim.Render(c.Pack.Name))
		if i == m.cursor {
			line = m.st.selected.Render("> " + text + "  " + c.Pack.Name)
		}
		lines = append(lines, line)
	}
	for len(lines) < rows {
		lines = append(lines, "")
	}
	list := strings.Join(lines, "\n")

	preview := ""
	if len(m.matches) > 0 {
		c := m.cmds[m.matches[m.cursor]]
		pw := m.width - 4
		ph := m.height - rows - 5
		if side {
			pw, ph = m.width-listW-4, rows-2
		}
		text := m.st.name.Render(c.Name) + " " + m.st.dim.Render(c.Usage) + "\n\n" + Describe(m.reg, c)
		preview = m.st.border.Width(pw).Render(clip(lipgloss.NewStyle().Width(pw-2).Render(text), max(1, ph)))
	}

	var body string
	if side {
		body = lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.NewStyle().Width(listW).Render(list), preview)
	} else {
		body = list + "\n" + preview
	}
	status := m.st.dim.Render(fmt.Sprintf("  %d/%d  ·  ↑↓ move  ·  enter choose  ·  esc cancel", len(m.matches), len(m.cmds)))
	return m.input.View() + "\n" + status + "\n" + body
}

func truncate(s string, n int) string {
	r := []rune(s)
	if n < 1 {
		return ""
	}
	if len(r) <= n {
		return s
	}
	return string(r[:max(0, n-1)]) + "…"
}

func clip(s string, lines int) string {
	parts := strings.Split(s, "\n")
	if len(parts) > lines {
		parts = parts[:lines]
	}
	return strings.Join(parts, "\n")
}

// Describe is the plain-text detail shown in the preview pane.
func Describe(reg *registry.Registry, c *manifest.Command) string {
	var b strings.Builder
	if c.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", c.Description)
	}
	src := "?"
	if ref := reg.SourceOf(c); ref != nil {
		src = string(ref.Source)
	}
	fmt.Fprintf(&b, "%s · pack %s (%s)\n\n", c.Kind(), c.Pack.Name, src)
	switch c.Kind() {
	case manifest.KindScript:
		fmt.Fprintf(&b, "runs %s\n", c.ScriptPath())
	default:
		fmt.Fprintf(&b, "%s\n", strings.TrimRight(c.Body(), "\n"))
	}
	if c.Help != "" {
		fmt.Fprintf(&b, "\n%s\n", strings.TrimRight(c.Help, "\n"))
	}
	return b.String()
}
