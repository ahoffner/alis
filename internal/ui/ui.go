// Package ui holds terminal styles and prompts shared by the CLI.
package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var (
	Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	Name  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	Dim   = lipgloss.NewStyle().Faint(true)
	Code  = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	Warn  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	Err   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	OK    = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
)

// Check prints a doctor-style status line.
func Check(w io.Writer, ok bool, format string, a ...any) {
	mark := OK.Render("✓")
	if !ok {
		mark = Warn.Render("!")
	}
	fmt.Fprintf(w, "  %s %s\n", mark, fmt.Sprintf(format, a...))
}

// IsTerminal reports whether f is attached to a terminal.
func IsTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

// Confirm asks a yes/no question on stderr, defaulting to no. assumeYes skips the prompt.
// It returns false without asking when stdin is not a terminal.
func Confirm(question string, assumeYes bool) bool {
	if assumeYes {
		return true
	}
	if !IsTerminal(os.Stdin) {
		fmt.Fprintf(os.Stderr, "%s (not a terminal; pass --yes to proceed)\n", question)
		return false
	}
	fmt.Fprintf(os.Stderr, "%s %s ", Warn.Render(question), Dim.Render("[y/N]"))
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}

// Tilde shortens a path under the home directory to ~/….
func Tilde(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" && (path == home || strings.HasPrefix(path, home+string(os.PathSeparator))) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
