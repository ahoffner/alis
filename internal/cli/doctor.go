package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ahoffner/alis/internal/legacy"
	"github.com/ahoffner/alis/internal/manifest"
	"github.com/ahoffner/alis/internal/registry"
	"github.com/ahoffner/alis/internal/shell"
	"github.com/ahoffner/alis/internal/ui"
)

// fix is a problem doctor can repair.
type fix struct {
	what string
	run  func() error
}

func (a *app) doctorCmd() *cobra.Command {
	var apply, yes bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check your install, shell setup and packs for problems",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			fixes, issues := a.diagnose()
			fmt.Println()
			if len(fixes) > 0 && !apply {
				fmt.Println(ui.Warn.Render(fmt.Sprintf("%d of these can be fixed automatically: run `alis doctor --fix`.", len(fixes))))
				return errSilent
			}
			if len(fixes) > 0 {
				if err := applyFixes(fixes, yes); err != nil {
					return err
				}
				return nil
			}
			if issues > 0 {
				return errSilent
			}
			fmt.Println(ui.OK.Render("Everything looks good."))
			return nil
		},
	}
	cmd.Flags().BoolVar(&apply, "fix", false, "repair what can be repaired (asks first)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "don't ask before fixing")
	return cmd
}

// diagnose prints every check and returns the available fixes and the number of issues.
func (a *app) diagnose() ([]fix, int) {
	reg := a.registry()
	var fixes []fix
	issues := 0
	check := func(ok bool, format string, args ...any) {
		ui.Check(os.Stdout, ok, format, args...)
		if !ok {
			issues++
		}
	}
	section := func(title string) { fmt.Println("\n" + ui.Title.Render(title)) }

	fmt.Println(ui.Title.Render("alis " + a.build.Version))
	if isDir(a.paths.SharedPacksDir()) {
		check(true, "built-in packs: %s", ui.Tilde(a.paths.SharedPacksDir()))
	} else {
		check(false, "built-in packs not found (looked in %q); reinstall alis or set ALIS_SHARE_DIR", a.paths.ShareDir)
	}
	if lib := a.paths.LibFile(); lib != "" {
		check(true, "script helpers: %s", ui.Tilde(lib))
	} else {
		check(false, "script helpers (lib/alis.sh) not found; scripts will fail")
	}
	if path, err := exec.LookPath("alis"); err != nil {
		check(false, "`alis` isn't on your PATH")
	} else {
		check(true, "on PATH: %s", ui.Tilde(path))
	}

	section("Shell")
	for _, sh := range []shell.Shell{shell.Zsh, shell.Bash} {
		rc := rcFile(sh)
		data, err := os.ReadFile(rc)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "alis init") {
			check(true, "%s loads alis", ui.Tilde(rc))
		} else if loginShell() == sh {
			check(false, "%s doesn't load alis; run `alis setup`", ui.Tilde(rc))
		}
	}
	for _, f := range legacy.Detect(legacy.Env{Home: home(), ZshCustom: os.Getenv("ZSH_CUSTOM")}) {
		check(false, "old install: %s (%s)", f.Title, f.Detail)
		fixes = append(fixes, fix{what: "remove " + f.Title, run: f.Fix})
	}

	section("Packs")
	for _, ref := range reg.Packs {
		state := fmt.Sprintf("%d commands", len(ref.Pack.Commands))
		if ref.Disabled {
			state = "disabled"
		}
		check(true, "%s (%s): %s", ref.Pack.Name, ref.Source, state)
	}
	for _, p := range reg.Problems {
		check(false, "%s: %s", ui.Tilde(p.Path), indent(p.Err.Error()))
	}
	for _, s := range reg.Shadows {
		ui.Check(os.Stdout, true, "%s from %s overrides %s", s.Name, s.Winner, s.Loser)
	}

	section("Commands")
	before := issues
	shells := availableShells()
	for _, c := range reg.Commands {
		ref := reg.SourceOf(c)
		switch c.Kind() {
		case manifest.KindScript:
			info, err := os.Stat(c.ScriptPath())
			if err == nil && info.Mode()&0o111 == 0 {
				check(false, "%s: %s isn't executable", c.Name, ui.Tilde(c.ScriptPath()))
				if ref.Source != registry.SourceShared {
					path, mode := c.ScriptPath(), info.Mode()
					fixes = append(fixes, fix{what: "chmod +x " + ui.Tilde(path), run: func() error { return os.Chmod(path, mode|0o755) }})
				}
			}
		case manifest.KindFunction:
			for _, sh := range shells {
				if out, err := exec.Command(string(sh), "-n", "-c", shell.FunctionSource(c)).CombinedOutput(); err != nil {
					check(false, "%s: not valid %s: %s", c.Name, sh, strings.TrimSpace(string(out)))
				}
			}
		}
		if path, err := exec.LookPath(c.Name); err == nil {
			ui.Check(os.Stdout, true, "%s hides %s on your PATH (use `command %s` to reach it)", c.Name, ui.Tilde(path), c.Name)
		}
	}
	for _, ref := range reg.Packs {
		if ref.Disabled {
			continue
		}
		for _, tool := range ref.Pack.Requires {
			if _, err := exec.LookPath(tool); err != nil {
				check(false, "%s pack needs %s, which isn't installed", ref.Pack.Name, tool)
			}
		}
	}
	for _, c := range reg.Commands {
		for _, tool := range c.Requires {
			if _, err := exec.LookPath(tool); err != nil {
				check(false, "%s needs %s, which isn't installed", c.Name, tool)
			}
		}
	}
	if issues == before {
		check(true, "%d commands, no problems", len(reg.Commands))
	}
	return fixes, issues
}

func applyFixes(fixes []fix, yes bool) error {
	fmt.Println("Fixes:")
	for _, f := range fixes {
		fmt.Println("  • " + f.what)
	}
	fmt.Println(ui.Dim.Render("  (shell config files are backed up next to themselves as *.alis-backup-<time>)"))
	if !ui.Confirm("Apply these fixes?", yes) {
		return errSilent
	}
	for _, f := range fixes {
		if err := f.run(); err != nil {
			return fmt.Errorf("%s: %w", f.what, err)
		}
		ui.Check(os.Stdout, true, "%s", f.what)
	}
	fmt.Println(ui.Dim.Render("Open a new terminal (or run `exec $SHELL`) to pick up the changes."))
	return nil
}

func (a *app) setupCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:       "setup [zsh|bash]",
		Short:     "Add alis to your shell config and clean up old installs",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []cobra.Completion{"zsh", "bash"},
		RunE: func(_ *cobra.Command, args []string) error {
			sh := loginShell()
			if len(args) == 1 {
				var err error
				if sh, err = shell.Parse(args[0]); err != nil {
					return err
				}
			}
			if sh == "" {
				return fmt.Errorf("couldn't tell your shell from $SHELL; run `alis setup zsh` or `alis setup bash`")
			}
			rc := rcFile(sh)
			data, err := os.ReadFile(rc)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			var fixes []fix
			if strings.Contains(string(data), "alis init") {
				ui.Check(os.Stdout, true, "%s already loads alis", ui.Tilde(rc))
			} else {
				snippet := initSnippet(sh)
				fmt.Printf("This will append to %s:\n\n%s\n", ui.Tilde(rc), ui.Code.Render(strings.TrimSpace(snippet)))
				fixes = append(fixes, fix{what: "add alis to " + ui.Tilde(rc), run: func() error { return appendFile(rc, snippet) }})
			}
			for _, f := range legacy.Detect(legacy.Env{Home: home(), ZshCustom: os.Getenv("ZSH_CUSTOM")}) {
				fixes = append(fixes, fix{what: "remove " + f.Title, run: f.Fix})
			}
			if len(fixes) == 0 {
				fmt.Println(ui.OK.Render("Nothing to do."))
				return nil
			}
			fmt.Println()
			return applyFixes(fixes, yes)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "don't ask first")
	return cmd
}

// initSnippet is what setup adds. The guard keeps startup silent if alis isn't on PATH yet.
func initSnippet(sh shell.Shell) string {
	guard := `(( $+commands[alis] ))`
	if sh == shell.Bash {
		guard = `command -v alis >/dev/null 2>&1`
	}
	return fmt.Sprintf("\n# alis: shell shortcuts from packs (https://github.com/ahoffner/alis)\n%s && eval \"$(alis init %s)\"\n", guard, sh)
}

func appendFile(path, text string) error {
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if err := legacy.Backup(path, data, 0o644); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		text = "\n" + text
	}
	_, err = f.WriteString(text)
	return err
}

func (a *app) updateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update git-installed packs and check for a newer alis",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			err := a.updatePacks(nil)
			if brew, lookErr := exec.LookPath("brew"); lookErr == nil {
				cmd := exec.Command(brew, "outdated", "--quiet", "alis")
				cmd.Env = append(os.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1")
				if out, _ := cmd.Output(); strings.TrimSpace(string(out)) != "" {
					fmt.Println(ui.Warn.Render("A newer alis is available: run `brew upgrade alis`."))
				}
			}
			return err
		},
	}
}

func loginShell() shell.Shell {
	sh, _ := shell.Parse(filepath.Base(os.Getenv("SHELL")))
	return sh
}

func rcFile(sh shell.Shell) string {
	if sh == shell.Zsh {
		if dir := os.Getenv("ZDOTDIR"); dir != "" {
			return filepath.Join(dir, ".zshrc")
		}
		return filepath.Join(home(), ".zshrc")
	}
	return filepath.Join(home(), ".bashrc")
}

func availableShells() []shell.Shell {
	var out []shell.Shell
	for _, sh := range []shell.Shell{shell.Zsh, shell.Bash} {
		if _, err := exec.LookPath(string(sh)); err == nil {
			out = append(out, sh)
		}
	}
	return out
}

func home() string {
	h, _ := os.UserHomeDir()
	return h
}

func indent(s string) string { return strings.ReplaceAll(s, "\n", "\n      ") }
