# alis

Shell shortcuts (aliases, functions and scripts) organised into shareable **packs**.

alis comes with packs for Git, Laravel Sail and PHP tooling. You can add your own: keep
private ones (work shortcuts, say) in their own git repo and install them on just the
machines that need them.

```console
$ alis list
git  Git shortcuts and branch-resolution helpers
  gc <branch>             Check out a branch and pull it
  gcbr <target-branch>    Create a conflict-resolution branch <current>-<target> with <target> merged in
  gco                     Check out the branch you were on before
  …
```

Run `alis` with no arguments for a fuzzy finder over every command. Pick one and it's put on
your prompt, ready for arguments.

## Install

```bash
brew install ahoffner/tap/alis
alis setup
```

`alis setup` adds one line to your `~/.zshrc` (or `~/.bashrc`):

```bash
eval "$(alis init zsh)"
```

If you use oh-my-zsh, the line has to come after `source $ZSH/oh-my-zsh.sh`. Appending it to
the end of the file, which is what `setup` does, takes care of that. Init prints nothing and
takes a few milliseconds, so it's safe with Powerlevel10k's instant prompt.

`alis setup` also offers to remove the alis 1.x or 2.x installs listed under
[Upgrading from 1.x or 2.x](#upgrading-from-1x-or-2x).

## Using it

| Command | What it does |
|---|---|
| `alis` | Search every command; the one you pick is put on your prompt |
| `alis list [-l] [--pack p]` | List commands by pack (`-l` shows what each runs) |
| `alis info <cmd>` | Description, usage, source pack and exactly what it runs |
| `alis doctor [--fix]` | Check the install, shell setup and packs |
| `alis disable <cmd>` / `alis enable <cmd>` | Hide or restore a single command |
| `alis local` | Edit your machine-only pack |
| `alis update` | Update git-installed packs and check for a new alis |

Upgrade alis itself with `brew upgrade alis`.

## Built-in packs

| Pack | Commands |
|---|---|
| **git** | `gco` previous branch · `gc <branch>` checkout and pull · `gsal` apply latest stash · `gcbr <target>` make a conflict-resolution branch · `gsbr [target]` sync a resolution branch with both sides |
| **laravel** | `sail` (uses `./sail` or `vendor/bin/sail`) · `sup` · `san` · `sam` · `sac` · `tink` · `st [filter] [paths…] [--stop]` run tests |
| **php** | `cs` PHP CS Fixer · `phan` Phan on changed files · `phanall` · `prep` (cs then phanall) · `p` PHPUnit |

Turn off a whole pack with `alis pack disable laravel`.

## Packs

A pack is a directory with an `alis.toml`. alis looks for packs in three places. Later
places win when two packs define the same command:

1. `$(brew --prefix)/share/alis/packs/`: the built-in packs, plus any packs installed by other Homebrew formulae.
2. `~/.local/share/alis/packs/`: packs added with `alis pack add`.
3. `~/.config/alis/local/`: your machine-only pack (`alis local`). It's never shared.

```bash
alis pack add git@github.com:my-org/alis-work.git   # any git URL, including private SSH ones
alis pack add someone/alis-docker                   # GitHub shorthand
alis pack add ~/code/alis-work                      # link a local checkout while you work on it
alis pack list
alis pack update [name]
alis pack disable work / alis pack enable work
alis pack remove work
```

New commands are live in your current shell straight away, because the `alis` shell function
reloads the integration after any `pack`, `enable`, `disable`, `local` or `update` command.

### Making a pack for work (or anything else)

```bash
alis pack new work --dir ~/code/alis-work   # alis.toml, an example script, a README, git init
alis pack add ~/code/alis-work              # link it while you edit
```

Push it to a private repo, then on each machine that needs it run
`alis pack add <git-url>`. Nothing from the pack goes into this repository.

### `alis.toml`

```toml
name = "work"                      # defaults to the directory name
description = "Work shortcuts"
requires = ["docker"]              # optional: tools `alis doctor` checks for

[commands.sup]
description = "Start the containers"
alias = "sail up -d"               # a shell alias; arguments are appended

[commands.swe]
description = "Switch the app to another environment"
usage = "<env>"
function = '''
sail artisan switch:env "$1"
'''                                # a shell function: runs in your shell, can cd/export

[commands.deploy]
description = "Deploy the current branch"
usage = "[--dry-run]"
script = "bin/deploy"              # an executable in the pack, run as a subprocess
help = "Longer text shown by `alis info deploy`."
```

Every command sets exactly one of `alias`, `function` or `script`. Optional fields are
`description`, `usage`, `help`, `group` (a sub-heading in `alis list`) and `requires`.

**Functions** must work in both zsh and bash if you use both. `alis doctor` syntax-checks them
in every shell you have installed.

**Scripts** can be in any language, as long as they're executable with a shebang. Bash
scripts can use the helpers in [`lib/alis.sh`](lib/alis.sh): `info`, `success`, `warn`,
`error`, `die`, `confirm`, `prompt`, `run`, `require_git_repo`, `require_clean_tree` and
`has_conflicts`. Load them with:

```bash
source "${ALIS_LIB:?run this through alis}"
```

To call a script from somewhere shell functions aren't loaded (another script, say), use
`alis run <name> [args…]`.

### Shipping a pack with Homebrew

A pack can also be a formula in your own tap. Install its files into `share/alis/packs/<name>`:

```ruby
def install
  (share/"alis/packs/work").install Dir["*"]
end
```

## Upgrading from 1.x or 2.x

Older versions put a `### MANAGED BY ALIS` block in your shell config and copied files to
`~/.local/alis`, or ran as an oh-my-zsh plugin that printed a banner at startup. `alis setup`
and `alis doctor --fix` find and remove all of these. Shell config files are backed up first,
as `*.alis-backup-<time>`.

Commands that moved:

- `swl`/`swd`/`sws` and `dockroot` were specific to one workplace. They now belong in a
  private pack. `alis pack new` gives you a starting point.
- `ls='ls -GFh'` is personal taste rather than a shared default. Add it to `alis local` if
  you want it.

## Development

```bash
make test               # go test ./... (includes real zsh/bash integration tests)
make lint               # go vet + shellcheck
eval "$(make -s dev)"   # use this checkout's binary and packs in your current shell
```

Releasing: push a `vX.Y.Z` tag. The release workflow creates a GitHub release and updates
`Formula/alis.rb` in [ahoffner/homebrew-tap](https://github.com/ahoffner/homebrew-tap) (using the
`HOMEBREW_TAP_TOKEN` secret). [`packaging/homebrew/alis.rb`](packaging/homebrew/alis.rb) is the formula's template.
