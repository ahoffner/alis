package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/ahoffner/alis/internal/manifest"
	"github.com/ahoffner/alis/internal/registry"
	"github.com/ahoffner/alis/internal/scaffold"
	"github.com/ahoffner/alis/internal/ui"
)

func (a *app) packCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pack",
		Short: "List, add, remove, update and create packs",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return a.packList() },
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List installed packs",
			Args:  cobra.NoArgs,
			RunE:  func(*cobra.Command, []string) error { return a.packList() },
		},
		a.packAddCmd(),
		a.packRemoveCmd(),
		a.packUpdateCmd(),
		a.packToggleCmd(true),
		a.packToggleCmd(false),
		a.packNewCmd(),
	)
	return cmd
}

func (a *app) packList() error {
	reg := a.registry()
	warnProblems(reg)
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "PACK\tSOURCE\tCOMMANDS\tSTATUS\tDESCRIPTION")
	for _, ref := range reg.Packs {
		status := "enabled"
		if ref.Disabled {
			status = "disabled"
		}
		src := string(ref.Source)
		if ref.Linked {
			src += " → " + ui.Tilde(ref.Pack.Dir)
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", ref.Pack.Name, src, len(ref.Pack.Commands), status, ref.Pack.Description)
	}
	return w.Flush()
}

var githubShorthand = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func (a *app) packAddCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "add <git-url | owner/repo | path>",
		Short: "Install a pack from a git repository, or link a local checkout",
		Long: `Install a pack from a git repository (any URL git understands, including private SSH
URLs) or a GitHub owner/repo shorthand. Pass a local directory to link it instead: edits
show up immediately, which is handy while you write a pack.`,
		Example: `  alis pack add git@github.com:my-org/alis-work.git
  alis pack add someone/alis-docker
  alis pack add ~/Sites/alis-work`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := os.MkdirAll(a.paths.UserPacksDir, 0o755); err != nil {
				return err
			}
			src := expandHome(args[0])
			var (
				p   *manifest.Pack
				err error
			)
			if info, statErr := os.Stat(src); statErr == nil && info.IsDir() {
				p, err = a.linkPack(src, force)
			} else {
				url := args[0]
				if githubShorthand.MatchString(url) {
					url = "https://github.com/" + url + ".git"
				}
				p, err = a.clonePack(url, force)
			}
			if err != nil {
				return err
			}
			fmt.Println(ui.OK.Render(fmt.Sprintf("✓ Added pack %s (%d commands)", p.Name, len(p.Commands))))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "replace an existing pack with the same name")
	return cmd
}

func (a *app) linkPack(dir string, force bool) (*manifest.Pack, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	p, err := manifest.Load(abs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ui.Tilde(filepath.Join(abs, manifest.FileName)), err)
	}
	dest := filepath.Join(a.paths.UserPacksDir, p.Name)
	if err := a.makeRoom(dest, force); err != nil {
		return nil, err
	}
	return p, os.Symlink(abs, dest)
}

func (a *app) clonePack(url string, force bool) (*manifest.Pack, error) {
	tmp, err := os.MkdirTemp(a.paths.UserPacksDir, ".add-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	repo := filepath.Join(tmp, "repo")
	if err := git("", "clone", "--quiet", url, repo); err != nil {
		return nil, fmt.Errorf("git clone %s failed", url)
	}
	p, err := manifest.Load(repo)
	if err != nil {
		return nil, fmt.Errorf("%s isn't a valid pack: %w", url, err)
	}
	// Name the pack from its manifest, not from the temporary clone directory.
	if p.Name == "repo" {
		p.Name = strings.TrimSuffix(filepath.Base(strings.TrimSuffix(url, "/")), ".git")
	}
	dest := filepath.Join(a.paths.UserPacksDir, p.Name)
	if err := a.makeRoom(dest, force); err != nil {
		return nil, err
	}
	return p, os.Rename(repo, dest)
}

func (a *app) makeRoom(dest string, force bool) error {
	if _, err := os.Lstat(dest); err != nil {
		return nil
	}
	if !force {
		return fmt.Errorf("a pack named %q is already installed (use --force to replace it)", filepath.Base(dest))
	}
	return os.RemoveAll(dest)
}

func (a *app) userPack(name string) (*registry.PackRef, error) {
	ref := a.registry().Pack(name)
	if ref == nil {
		return nil, fmt.Errorf("no pack named %q (see `alis pack list`)", name)
	}
	switch ref.Source {
	case registry.SourceShared:
		return nil, fmt.Errorf("%s is installed with alis or Homebrew; use `alis pack disable %s` to turn it off", name, name)
	case registry.SourceLocal:
		return nil, fmt.Errorf("the local pack lives at %s; edit it with `alis local`", ui.Tilde(ref.Path))
	}
	return ref, nil
}

func (a *app) packRemoveCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:               "remove <pack>",
		Aliases:           []string{"rm"},
		Short:             "Remove a pack added with `alis pack add`",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.packNames(registry.SourceUser),
		RunE: func(_ *cobra.Command, args []string) error {
			ref, err := a.userPack(args[0])
			if err != nil {
				return err
			}
			if ref.Linked {
				if err := os.Remove(ref.Path); err != nil {
					return err
				}
				fmt.Println(ui.OK.Render("✓ Unlinked " + args[0] + " (your checkout at " + ui.Tilde(ref.Pack.Dir) + " is untouched)"))
				return nil
			}
			if !force {
				if out, _ := exec.Command("git", "-C", ref.Path, "status", "--porcelain").Output(); len(out) > 0 {
					return fmt.Errorf("%s has uncommitted changes in %s (use --force to delete anyway)", args[0], ui.Tilde(ref.Path))
				}
			}
			if err := os.RemoveAll(ref.Path); err != nil {
				return err
			}
			reg := a.registry()
			if reg.Config.PackDisabled(args[0]) {
				reg.Config.SetPackDisabled(args[0], false)
				_ = reg.Config.Save(a.paths.ConfigFile)
			}
			fmt.Println(ui.OK.Render("✓ Removed " + args[0]))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "delete even with uncommitted changes")
	return cmd
}

func (a *app) packUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "update [pack…]",
		Short:             "Pull the latest version of git-installed packs (all of them by default)",
		ValidArgsFunction: a.packNames(registry.SourceUser),
		RunE: func(_ *cobra.Command, args []string) error {
			return a.updatePacks(args)
		},
	}
}

func (a *app) updatePacks(names []string) error {
	var refs []*registry.PackRef
	if len(names) == 0 {
		for _, ref := range a.registry().Packs {
			if ref.Source == registry.SourceUser {
				refs = append(refs, ref)
			}
		}
	}
	for _, name := range names {
		ref, err := a.userPack(name)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		fmt.Println(ui.Dim.Render("No packs added with `alis pack add`, so nothing to update."))
		return nil
	}
	failed := false
	for _, ref := range refs {
		name := ref.Pack.Name
		switch {
		case ref.Linked:
			ui.Check(os.Stdout, true, "%s: linked to %s, skipped", name, ui.Tilde(ref.Pack.Dir))
		case !isDir(filepath.Join(ref.Path, ".git")):
			ui.Check(os.Stdout, false, "%s: not a git checkout, skipped", name)
		default:
			if err := git(ref.Path, "pull", "--ff-only", "--quiet"); err != nil {
				ui.Check(os.Stdout, false, "%s: git pull failed", name)
				failed = true
				continue
			}
			ui.Check(os.Stdout, true, "%s: up to date", name)
		}
	}
	a.reload()
	if failed {
		return errSilent
	}
	return nil
}

func (a *app) packToggleCmd(enable bool) *cobra.Command {
	verb := "disable"
	if enable {
		verb = "enable"
	}
	return &cobra.Command{
		Use:               verb + " <pack>",
		Short:             strings.ToUpper(verb[:1]) + verb[1:] + " a pack without uninstalling it",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: a.packNames(""),
		RunE: func(_ *cobra.Command, args []string) error {
			reg := a.registry()
			if reg.Pack(args[0]) == nil {
				return fmt.Errorf("no pack named %q (see `alis pack list`)", args[0])
			}
			reg.Config.SetPackDisabled(args[0], !enable)
			if err := reg.Config.Save(a.paths.ConfigFile); err != nil {
				return err
			}
			fmt.Println(ui.OK.Render(fmt.Sprintf("✓ %sd %s", verb, args[0])))
			return nil
		},
	}
}

func (a *app) packNewCmd() *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:     "new <name>",
		Short:   "Create a new pack repository from a template",
		Example: "  alis pack new work            # creates ./alis-work\n  alis pack new work --dir ~/code/my-pack",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			if _, err := manifest.Parse([]byte(fmt.Sprintf("name = %q", name)), "."); err != nil {
				return err
			}
			if dir == "" {
				dir = "alis-" + name
			}
			dir = expandHome(dir)
			if err := scaffold.New(dir, name); err != nil {
				return err
			}
			fmt.Println(ui.OK.Render("✓ Created pack " + name + " in " + ui.Tilde(dir)))
			fmt.Println()
			fmt.Println("Next:")
			fmt.Println("  " + ui.Code.Render("alis pack add "+dir) + ui.Dim.Render("   # link it while you edit alis.toml"))
			fmt.Println("  " + ui.Dim.Render("push it to a (private) git repo, then on other machines:"))
			fmt.Println("  " + ui.Code.Render("alis pack add <git-url>"))
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "directory to create (default ./alis-<name>)")
	return cmd
}

// git runs git with its output on the terminal, so SSH and credential prompts work.
func git(dir string, args ...string) error {
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stderr, os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return err
		}
		return fmt.Errorf("couldn't run git: %w", err)
	}
	return nil
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
