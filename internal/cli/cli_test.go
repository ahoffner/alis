package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// These tests build the real binary and drive it the way a user would.

var bin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "alis-cli-test")
	if err != nil {
		panic(err)
	}
	bin = filepath.Join(dir, "alis")
	if out, err := exec.Command("go", "build", "-o", bin, "../../cmd/alis").CombinedOutput(); err != nil {
		panic(string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// TestStartupInitIsCheap guards shell startup time: `alis init` runs in every new shell, and
// Go runs every imported package's init first. Some terminal libraries build Unicode tables
// there (tcell, go-runewidth >= 0.0.20), which costs 20ms+ on every prompt.
func TestStartupInitIsCheap(t *testing.T) {
	cmd := exec.Command(bin, "--version")
	cmd.Env = append(os.Environ(), "GODEBUG=inittrace=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	// Lines look like: init github.com/mattn/go-runewidth @0.4 ms, 23 ms clock, 9 bytes, 1 allocs
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 7 || f[0] != "init" || f[6] != "clock," {
			continue
		}
		ms, err := strconv.ParseFloat(f[4], 64)
		if err != nil {
			continue
		}
		if ms > 5 {
			t.Errorf("package %s takes %.1fms to initialise, slowing every shell startup", f[1], ms)
		}
	}
}

type env struct {
	t    *testing.T
	home string
	vars []string
}

func newEnv(t *testing.T) *env {
	home := t.TempDir()
	share, _ := filepath.Abs("../..")
	return &env{t: t, home: home, vars: []string{
		"HOME=" + home,
		"PATH=" + filepath.Dir(bin) + ":" + os.Getenv("PATH"),
		"ALIS_SHARE_DIR=" + share,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"XDG_DATA_HOME=" + filepath.Join(home, ".local", "share"),
		"SHELL=/bin/zsh",
		"NO_COLOR=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
	}}
}

// run runs alis and fails the test if it exits non-zero.
func (e *env) run(dir string, args ...string) string {
	e.t.Helper()
	out, err := e.try(dir, args...)
	if err != nil {
		e.t.Fatalf("alis %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func (e *env) try(dir string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = e.vars
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (e *env) names() map[string]string {
	e.t.Helper()
	var cmds []struct{ Name, Pack string }
	if err := json.Unmarshal([]byte(e.run(e.home, "list", "--json")), &cmds); err != nil {
		e.t.Fatal(err)
	}
	out := map[string]string{}
	for _, c := range cmds {
		out[c.Name] = c.Pack
	}
	return out
}

func TestPackLifecycle(t *testing.T) {
	e := newEnv(t)
	if got := e.names(); got["gcbr"] != "git" || got["sup"] != "laravel" || got["cs"] != "php" {
		t.Fatalf("built-in commands missing: %v", got)
	}

	e.run(e.home, "pack", "new", "work")
	dir := filepath.Join(e.home, "alis-work")
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Error("pack new didn't git init")
	}
	e.run(e.home, "pack", "add", dir)
	if got := e.names(); got["work-hello"] != "work" || got["work-example"] != "work" {
		t.Fatalf("linked pack commands missing: %v", got)
	}
	if out := e.run(e.home, "run", "work-example", "a b"); !strings.Contains(out, "Running work-example with: a b") {
		t.Errorf("run work-example output = %q", out)
	}
	if _, err := e.try(e.home, "pack", "add", dir); err == nil {
		t.Error("adding the same pack twice should fail without --force")
	}

	e.run(e.home, "pack", "disable", "work")
	if _, ok := e.names()["work-hello"]; ok {
		t.Error("disabled pack's commands still listed")
	}
	if init := e.run(e.home, "init", "zsh"); strings.Contains(init, "work-hello") {
		t.Error("disabled pack's commands still in init")
	}
	e.run(e.home, "pack", "enable", "work")
	e.run(e.home, "disable", "gco")
	if _, ok := e.names()["gco"]; ok {
		t.Error("disabled command still listed")
	}
	e.run(e.home, "enable", "gco")

	e.run(e.home, "pack", "remove", "work")
	if _, ok := e.names()["work-hello"]; ok {
		t.Error("removed pack's commands still listed")
	}
	if _, err := os.Stat(filepath.Join(dir, "alis.toml")); err != nil {
		t.Error("removing a linked pack deleted the checkout")
	}
}

func TestPackAddFromGit(t *testing.T) {
	e := newEnv(t)
	src := filepath.Join(e.home, "src")
	e.run(e.home, "pack", "new", "team", "--dir", src)
	git(t, e, src, "add", ".")
	git(t, e, src, "commit", "-qm", "init")

	e.run(e.home, "pack", "add", "file://"+src)
	if got := e.names(); got["team-hello"] != "team" {
		t.Fatalf("cloned pack commands missing: %v", got)
	}
	if out := e.run(e.home, "pack", "update"); !strings.Contains(out, "team: up to date") {
		t.Errorf("pack update output = %q", out)
	}
}

func TestBrokenPackKeepsInitSilent(t *testing.T) {
	e := newEnv(t)
	bad := filepath.Join(e.home, ".local", "share", "alis", "packs", "bad")
	if err := os.MkdirAll(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "alis.toml"), []byte("[commands.x]\nalais = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("zsh", "-fc", `eval "$(alis init zsh)"`)
	cmd.Env = e.vars
	if out, err := cmd.CombinedOutput(); err != nil || len(out) > 0 {
		t.Errorf("init with a broken pack: err=%v output=%q", err, out)
	}
	out, err := e.try(e.home, "doctor")
	if err == nil || !strings.Contains(out, "unknown field") {
		t.Errorf("doctor should report the broken pack and fail:\n%s", out)
	}
}

func TestSetupAndLegacyCleanup(t *testing.T) {
	e := newEnv(t)
	zshrc := filepath.Join(e.home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("plugins=(git alis)\nsource $ZSH/oh-my-zsh.sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(e.home, ".oh-my-zsh", "custom", "plugins", "alis"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.run(e.home, "setup", "--yes")
	data, _ := os.ReadFile(zshrc)
	if !strings.Contains(string(data), `eval "$(alis init zsh)"`) || strings.Contains(string(data), "plugins=(git alis)") {
		t.Errorf(".zshrc after setup:\n%s", data)
	}
	if out := e.run(e.home, "setup", "--yes"); !strings.Contains(out, "already loads alis") {
		t.Errorf("second setup should be a no-op:\n%s", out)
	}
	if strings.Count(string(data), "alis init") != 1 {
		t.Error("setup added the init line more than once")
	}
}

// TestGitResolutionScripts runs gcbr and gsbr against a throwaway remote with a real conflict.
func TestGitResolutionScripts(t *testing.T) {
	e := newEnv(t)
	origin := filepath.Join(e.home, "origin.git")
	work := filepath.Join(e.home, "work")
	git(t, e, e.home, "init", "-q", "--bare", "-b", "main", origin)
	git(t, e, e.home, "clone", "-q", origin, work)
	commit := func(branch, content string) {
		if err := os.WriteFile(filepath.Join(work, "f.txt"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		git(t, e, work, "add", "f.txt")
		git(t, e, work, "commit", "-qm", branch+": "+content)
		git(t, e, work, "push", "-q", "origin", branch)
	}
	git(t, e, work, "checkout", "-qb", "main")
	commit("main", "base\n")
	git(t, e, work, "checkout", "-qb", "feature")
	commit("feature", "feature change\n")
	git(t, e, work, "checkout", "-q", "main")
	commit("main", "main change\n")
	git(t, e, work, "checkout", "-q", "feature")

	out := e.run(work, "run", "gcbr", "main")
	if !strings.Contains(out, "Conflicts to resolve") || branch(t, e, work) != "feature-main" {
		t.Fatalf("gcbr didn't leave a conflicted feature-main branch:\n%s", out)
	}

	// Resolve, then land new commits on both sides and sync them in with gsbr.
	if err := os.WriteFile(filepath.Join(work, "f.txt"), []byte("resolved\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, e, work, "commit", "-qam", "resolve")
	if err := os.WriteFile(filepath.Join(work, "other.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, e, work, "checkout", "-q", "main")
	git(t, e, work, "add", "other.txt")
	git(t, e, work, "commit", "-qm", "main: other")
	git(t, e, work, "push", "-q", "origin", "main")
	git(t, e, work, "checkout", "-q", "feature-main")

	out = e.run(work, "run", "gsbr")
	if !strings.Contains(out, "is up to date with 'feature' and 'main'") {
		t.Fatalf("gsbr output:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(work, "other.txt")); err != nil {
		t.Error("gsbr didn't merge the new main commit")
	}
}

func git(t *testing.T, e *env, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = e.vars
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func branch(t *testing.T, e *env, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	cmd.Env = e.vars
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}
