// Package cli implements the alis command line.
package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ahoffner/alis/internal/config"
	"github.com/ahoffner/alis/internal/manifest"
	"github.com/ahoffner/alis/internal/picker"
	"github.com/ahoffner/alis/internal/registry"
	"github.com/ahoffner/alis/internal/ui"
)

// Build carries values set at link time.
type Build struct {
	Version  string
	ShareDir string
}

type app struct {
	build Build
	paths config.Paths
	reg   *registry.Registry
}

func (a *app) registry() *registry.Registry {
	if a.reg == nil {
		a.reg = registry.Load(a.paths)
	}
	return a.reg
}

// reload drops the cached registry after something on disk changed.
func (a *app) reload() { a.reg = nil }

// errSilent ends the command with exit status 1 without printing anything more.
var errSilent = errors.New("")

// Execute runs the CLI and returns the process exit status.
func Execute(b Build) int {
	a := &app{build: b, paths: config.Resolve(b.ShareDir)}
	root := a.rootCmd()
	if err := root.Execute(); err != nil {
		if !errors.Is(err, errSilent) && !errors.Is(err, picker.ErrCancelled) {
			fmt.Fprintln(os.Stderr, ui.Err.Render("alis: "+err.Error()))
		}
		return 1
	}
	return 0
}

func (a *app) rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "alis",
		Short: "Shell shortcuts, organised into shareable packs",
		Long: `alis turns packs of aliases, functions and scripts (alis.toml files) into shell
shortcuts. Load it from your shell config with:

  eval "$(alis init zsh)"

Run alis with no arguments to search every command.`,
		Version:       a.build.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return a.runByName(cmd, args)
			}
			if !ui.IsTerminal(os.Stdin) || !ui.IsTerminal(os.Stdout) {
				return cmd.Help()
			}
			c, err := picker.Pick(a.registry())
			if err != nil {
				return err
			}
			fmt.Println(pickText(c))
			fmt.Fprintln(os.Stderr, ui.Dim.Render("With shell integration loaded, `alis` puts this on your prompt for you."))
			return nil
		},
	}
	root.AddGroup(&cobra.Group{ID: "use", Title: "Commands:"}, &cobra.Group{ID: "manage", Title: "Setup and packs:"})
	for _, c := range []*cobra.Command{a.listCmd(), a.infoCmd(), a.runCmd()} {
		c.GroupID = "use"
		root.AddCommand(c)
	}
	for _, c := range []*cobra.Command{
		a.packCmd(), a.enableCmd(true), a.enableCmd(false), a.localCmd(),
		a.initCmd(), a.setupCmd(), a.doctorCmd(), a.updateCmd(),
	} {
		c.GroupID = "manage"
		root.AddCommand(c)
	}
	root.AddCommand(a.pickCmd())
	root.SetCompletionCommandGroupID("manage")
	root.SetHelpCommandGroupID("manage")
	return root
}

// runByName handles `alis <command> [args]`: scripts run, shell commands get pointed at the shell.
func (a *app) runByName(cmd *cobra.Command, args []string) error {
	c := a.registry().Command(args[0])
	if c == nil {
		msg := fmt.Sprintf("unknown command %q", args[0])
		if s := cmd.SuggestionsFor(args[0]); len(s) > 0 {
			msg += "; did you mean " + strings.Join(s, ", ") + "?"
		}
		return errors.New(msg + " (see `alis list`)")
	}
	if c.Kind() == manifest.KindScript {
		return a.execScript(c, args[1:])
	}
	return fmt.Errorf("%s is a shell %s, so run it directly in your shell (it's loaded by `eval \"$(alis init zsh)\"`)", c.Name, c.Kind())
}

// execScript replaces this process with a pack script, with $ALIS_LIB set.
func (a *app) execScript(c *manifest.Command, args []string) error {
	env := append(os.Environ(), "ALIS_LIB="+a.paths.LibFile())
	path := c.ScriptPath()
	return syscall.Exec(path, append([]string{path}, args...), env)
}

// pickText is what the picker puts on the prompt: the name, plus a space if it takes arguments.
func pickText(c *manifest.Command) string {
	if c.Usage != "" {
		return c.Name + " "
	}
	return c.Name
}

func (a *app) pickCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "pick",
		Short:  "Open the picker and print the chosen command (used by the shell wrapper)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := picker.Pick(a.registry())
			if err != nil {
				return err
			}
			fmt.Print(pickText(c))
			return nil
		},
	}
}

// commandNames completes effective command names.
func (a *app) commandNames(onlyScripts bool) cobra.CompletionFunc {
	return func(_ *cobra.Command, args []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveDefault
		}
		var out []cobra.Completion
		for _, c := range a.registry().Commands {
			if onlyScripts && c.Kind() != manifest.KindScript {
				continue
			}
			out = append(out, cobra.CompletionWithDesc(c.Name, c.Description))
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// packNames completes pack names, optionally only those of one source.
func (a *app) packNames(only registry.Source) cobra.CompletionFunc {
	return func(_ *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
		var out []cobra.Completion
		for _, ref := range a.registry().Packs {
			if only != "" && ref.Source != only {
				continue
			}
			out = append(out, cobra.CompletionWithDesc(ref.Pack.Name, ref.Pack.Description))
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}
