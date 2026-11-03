package picker

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/ahoffner/alis/internal/config"
	"github.com/ahoffner/alis/internal/registry"
)

func builtins(t *testing.T) *registry.Registry {
	t.Helper()
	dir := t.TempDir()
	reg := registry.Load(config.Paths{
		ShareDir:     "../..",
		UserPacksDir: dir,
		LocalPackDir: dir,
		ConfigFile:   dir + "/config.toml",
	})
	if len(reg.Commands) == 0 {
		t.Fatal("no built-in commands loaded")
	}
	return reg
}

func TestPickerFiltersAndChooses(t *testing.T) {
	m := newModel(builtins(t), lipgloss.NewRenderer(os.Stdout))
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	for _, r := range "gsbr" {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(m.matches) == 0 || m.cmds[m.matches[0]].Name != "gsbr" {
		t.Fatalf("best match for gsbr = %v", m.matches)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.chosen == nil || m.chosen.Name != "gsbr" {
		t.Fatalf("chosen = %v", m.chosen)
	}
}

func TestPickerFitsTheTerminal(t *testing.T) {
	for _, size := range [][2]int{{120, 30}, {70, 20}} {
		m := newModel(builtins(t), lipgloss.NewRenderer(os.Stdout))
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		lines := strings.Split(m.View(), "\n")
		if len(lines) > size[1] {
			t.Errorf("%dx%d: %d lines", size[0], size[1], len(lines))
		}
		for _, l := range lines {
			if w := ansi.StringWidth(l); w > size[0] {
				t.Errorf("%dx%d: line is %d wide: %q", size[0], size[1], w, ansi.Strip(l))
			}
		}
	}
}
