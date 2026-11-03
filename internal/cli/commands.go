package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/ahoffner/alis/internal/manifest"
	"github.com/ahoffner/alis/internal/registry"
	"github.com/ahoffner/alis/internal/shell"
	"github.com/ahoffner/alis/internal/ui"
)

func (a *app) initCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "init <zsh|bash>",
		Short:     "Print the shell integration; add `eval \"$(alis init zsh)\"` to your shell config",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []cobra.Completion{"zsh", "bash"},
		RunE: func(_ *cobra.Command, args []string) error {
			sh, err := shell.Parse(args[0])
			if err != nil {
				return err
			}
			// Problems are deliberately not printed: any output during shell startup is an error
			// for prompts like Powerlevel10k. `alis list` and `alis doctor` report them.
			return shell.Init(os.Stdout, sh, a.registry().Commands, a.paths.LibFile())
		},
	}
}

func (a *app) listCmd() *cobra.Command {
	var pack string
	var long, asJSON bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List every available command, grouped by pack",
		Args:    cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			reg := a.registry()
			if pack != "" && reg.Pack(pack) == nil {
				return fmt.Errorf("no pack named %q (see `alis pack list`)", pack)
			}
			if asJSON {
				return listJSON(reg, pack)
			}
			warnProblems(reg)
			listText(reg, pack, long)
			return nil
		},
	}
	cmd.Flags().StringVarP(&pack, "pack", "p", "", "only show this pack")
	cmd.Flags().BoolVarP(&long, "long", "l", false, "also show what each command runs")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	_ = cmd.RegisterFlagCompletionFunc("pack", a.packNames(""))
	return cmd
}

func warnProblems(reg *registry.Registry) {
	if n := len(reg.Problems); n > 0 {
		noun := "problem"
		if n > 1 {
			noun = "problems"
		}
		fmt.Fprintln(os.Stderr, ui.Warn.Render(fmt.Sprintf("! %d pack %s; some commands may be missing. Run `alis doctor`.", n, noun)))
		fmt.Fprintln(os.Stderr)
	}
}

func listText(reg *registry.Registry, only string, long bool) {
	// Signatures longer than maxWidth get the description on the next line.
	const maxWidth = 28
	width := 0
	for _, c := range reg.Commands {
		if n := utf8.RuneCountInString(signature(c)); n <= maxWidth {
			width = max(width, n)
		}
	}
	descIndent := strings.Repeat(" ", width+4)

	first := true
	for _, ref := range reg.Packs {
		if only != "" && ref.Pack.Name != only {
			continue
		}
		var cmds []*manifest.Command
		for _, c := range reg.Commands {
			if c.Pack == ref.Pack {
				cmds = append(cmds, c)
			}
		}
		if len(cmds) == 0 && !ref.Disabled {
			continue
		}
		if !first {
			fmt.Println()
		}
		first = false
		header := ui.Title.Render(ref.Pack.Name)
		if ref.Pack.Description != "" {
			header += "  " + ui.Dim.Render(ref.Pack.Description)
		}
		if ref.Disabled {
			header += "  " + ui.Warn.Render("(disabled)")
		}
		fmt.Println(header)

		sort.SliceStable(cmds, func(i, j int) bool { return cmds[i].Group < cmds[j].Group })
		group := ""
		for _, c := range cmds {
			if c.Group != group {
				group = c.Group
				if group != "" {
					fmt.Println("  " + ui.Dim.Render(group+":"))
				}
			}
			sig := signature(c)
			styled := ui.Name.Render(c.Name) + ui.Dim.Render(strings.TrimPrefix(sig, c.Name))
			if n := utf8.RuneCountInString(sig); n <= width {
				fmt.Printf("  %s%s%s\n", styled, strings.Repeat(" ", width-n+2), c.Description)
			} else {
				fmt.Printf("  %s\n%s%s\n", styled, descIndent, c.Description)
			}
			if long {
				fmt.Printf("%s%s\n", descIndent, ui.Code.Render("→ "+summary(c)))
			}
		}
	}
	if !long {
		fmt.Println()
		fmt.Println(ui.Dim.Render("`alis info <command>` for details · `alis list -l` to see what each runs · `alis` to search"))
	}
}

func signature(c *manifest.Command) string {
	if c.Usage == "" {
		return c.Name
	}
	return c.Name + " " + c.Usage
}

// summary is a one-line view of what a command runs.
func summary(c *manifest.Command) string {
	if c.Kind() == manifest.KindScript {
		return ui.Tilde(c.ScriptPath())
	}
	body := strings.TrimSpace(c.Body())
	if i := strings.IndexByte(body, '\n'); i >= 0 {
		body = body[:i] + " …"
	}
	return body
}

type jsonCommand struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description,omitempty"`
	Usage       string `json:"usage,omitempty"`
	Pack        string `json:"pack"`
	Source      string `json:"source"`
	Runs        string `json:"runs"`
}

func listJSON(reg *registry.Registry, only string) error {
	out := []jsonCommand{}
	for _, c := range reg.Commands {
		if only != "" && c.Pack.Name != only {
			continue
		}
		runs := c.Body()
		if c.Kind() == manifest.KindScript {
			runs = c.ScriptPath()
		}
		out = append(out, jsonCommand{
			Name: c.Name, Kind: string(c.Kind()), Description: c.Description, Usage: c.Usage,
			Pack: c.Pack.Name, Source: string(reg.SourceOf(c).Source), Runs: runs,
		})
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func (a *app) infoCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "info <command>",
		Short:             "Show what a command does, where it comes from and what it runs",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.commandNames(false),
		RunE: func(_ *cobra.Command, args []string) error {
			reg := a.registry()
			c := reg.Command(args[0])
			if c == nil {
				return fmt.Errorf("no command named %q (see `alis list`)", args[0])
			}
			ref := reg.SourceOf(c)
			fmt.Println(ui.Name.Render(c.Name) + " " + ui.Dim.Render(c.Usage))
			if c.Description != "" {
				fmt.Println(c.Description)
			}
			fmt.Println()
			fmt.Printf("%s %s from pack %s (%s, %s)\n", ui.Dim.Render("Kind:"), c.Kind(), ref.Pack.Name, ref.Source, ui.Tilde(ref.Pack.Dir))
			switch c.Kind() {
			case manifest.KindScript:
				fmt.Printf("%s %s\n", ui.Dim.Render("Runs:"), ui.Code.Render(ui.Tilde(c.ScriptPath())))
			case manifest.KindAlias:
				fmt.Printf("%s %s\n", ui.Dim.Render("Runs:"), ui.Code.Render(c.Alias))
			case manifest.KindFunction:
				fmt.Println(ui.Dim.Render("Runs:"))
				for _, line := range strings.Split(strings.TrimRight(c.Function, "\n"), "\n") {
					fmt.Println("  " + ui.Code.Render(line))
				}
			}
			if c.Help != "" {
				fmt.Println()
				fmt.Println(strings.TrimRight(c.Help, "\n"))
			}
			for _, s := range reg.Shadows {
				if s.Name == c.Name {
					fmt.Println()
					fmt.Println(ui.Warn.Render(fmt.Sprintf("Overrides %s from %s.", c.Name, s.Loser)))
				}
			}
			return nil
		},
	}
}

func (a *app) runCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "run <script> [args…]",
		Short:              "Run a script command (useful from other scripts, where shell functions aren't loaded)",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
		ValidArgsFunction:  a.commandNames(true),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] == "-h" || args[0] == "--help" {
				return cmd.Help()
			}
			c := a.registry().Command(args[0])
			if c == nil {
				return fmt.Errorf("no command named %q (see `alis list`)", args[0])
			}
			if c.Kind() != manifest.KindScript {
				return fmt.Errorf("%s is a shell %s; only scripts can be run with `alis run`", c.Name, c.Kind())
			}
			return a.execScript(c, args[1:])
		},
	}
}

func (a *app) enableCmd(enable bool) *cobra.Command {
	verb, short := "disable", "Hide a command without removing its pack"
	if enable {
		verb, short = "enable", "Re-enable a command hidden with `alis disable`"
	}
	cmd := &cobra.Command{
		Use:   verb + " <command>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			reg := a.registry()
			name := args[0]
			if !enable && reg.Command(name) == nil {
				return fmt.Errorf("no enabled command named %q", name)
			}
			if enable && !reg.Config.CommandDisabled(name) {
				return fmt.Errorf("%q isn't disabled", name)
			}
			reg.Config.SetCommandDisabled(name, !enable)
			if err := reg.Config.Save(a.paths.ConfigFile); err != nil {
				return err
			}
			fmt.Println(ui.OK.Render(fmt.Sprintf("✓ %sd %s", verb, name)))
			return nil
		},
	}
	if enable {
		cmd.ValidArgsFunction = func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
			return a.registry().Config.DisabledCommands, cobra.ShellCompDirectiveNoFileComp
		}
	} else {
		cmd.ValidArgsFunction = a.commandNames(false)
	}
	return cmd
}

const localTemplate = `# Your machine-only alis pack. It's never shared and overrides every other pack.
# Run ` + "`alis list --pack local`" + ` after editing. In a new shell, changes load automatically;
# in this one, run ` + "`alis local`" + ` again or ` + "`eval \"$(alis init zsh)\"`" + `.

description = "Machine-only shortcuts"

# [commands.ls]
# description = "ls with sizes, types and colors"
# alias = "ls -GFh"
`

func (a *app) localCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "local",
		Short: "Edit your machine-only pack (created on first use)",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			path := filepath.Join(a.paths.LocalPackDir, manifest.FileName)
			if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
				if err := os.MkdirAll(a.paths.LocalPackDir, 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(path, []byte(localTemplate), 0o644); err != nil {
					return err
				}
				fmt.Println(ui.OK.Render("✓ Created " + ui.Tilde(path)))
			}
			editor := os.Getenv("VISUAL")
			if editor == "" {
				editor = os.Getenv("EDITOR")
			}
			if editor == "" || !ui.IsTerminal(os.Stdin) {
				fmt.Println(ui.Tilde(path))
				return nil
			}
			c := exec.Command("sh", "-c", editor+` "$1"`, "sh", path)
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := c.Run(); err != nil {
				return err
			}
			a.reload()
			if _, err := manifest.Load(a.paths.LocalPackDir); err != nil {
				fmt.Fprintln(os.Stderr, ui.Warn.Render("! The local pack has a problem: "+err.Error()))
			}
			return nil
		},
	}
}
