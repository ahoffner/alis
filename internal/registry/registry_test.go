package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ahoffner/alis/internal/config"
)

func writePack(t *testing.T, dir, manifest string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alis.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testPaths(t *testing.T) config.Paths {
	root := t.TempDir()
	return config.Paths{
		ShareDir:     filepath.Join(root, "share"),
		UserPacksDir: filepath.Join(root, "data", "packs"),
		LocalPackDir: filepath.Join(root, "config", "local"),
		ConfigFile:   filepath.Join(root, "config", "config.toml"),
	}
}

func TestPrecedence(t *testing.T) {
	paths := testPaths(t)
	writePack(t, filepath.Join(paths.SharedPacksDir(), "core"), `
[commands.a]
alias = "core-a"
[commands.b]
alias = "core-b"`)
	writePack(t, filepath.Join(paths.UserPacksDir, "work"), `
[commands.b]
alias = "work-b"
[commands.c]
alias = "work-c"`)
	writePack(t, paths.LocalPackDir, `
[commands.c]
alias = "local-c"`)

	r := Load(paths)
	if len(r.Problems) != 0 {
		t.Fatalf("problems: %v", r.Problems)
	}
	want := map[string]string{"a": "core-a", "b": "work-b", "c": "local-c"}
	if len(r.Commands) != len(want) {
		t.Fatalf("got %d commands, want %d", len(r.Commands), len(want))
	}
	for name, body := range want {
		if c := r.Command(name); c == nil || c.Alias != body {
			t.Errorf("%s = %+v, want %s", name, c, body)
		}
	}
	if len(r.Shadows) != 2 {
		t.Errorf("shadows = %+v, want 2", r.Shadows)
	}
	if ref := r.Pack("local"); ref == nil || ref.Source != SourceLocal {
		t.Errorf("local pack = %+v", ref)
	}
}

func TestDisabled(t *testing.T) {
	paths := testPaths(t)
	writePack(t, filepath.Join(paths.SharedPacksDir(), "core"), `
[commands.a]
alias = "a"
[commands.b]
alias = "b"`)
	writePack(t, filepath.Join(paths.SharedPacksDir(), "extra"), `
[commands.x]
alias = "x"`)
	cfg := &config.Config{DisabledPacks: []string{"extra"}, DisabledCommands: []string{"b"}}
	if err := cfg.Save(paths.ConfigFile); err != nil {
		t.Fatal(err)
	}

	r := Load(paths)
	if len(r.Commands) != 1 || r.Commands[0].Name != "a" {
		t.Fatalf("commands = %v, want just a", r.Commands)
	}
	if ref := r.Pack("extra"); ref == nil || !ref.Disabled {
		t.Errorf("extra should be listed and disabled: %+v", ref)
	}
}

func TestBrokenPackDoesNotStopOthers(t *testing.T) {
	paths := testPaths(t)
	writePack(t, filepath.Join(paths.SharedPacksDir(), "good"), "[commands.ok]\nalias = \"ok\"")
	writePack(t, filepath.Join(paths.UserPacksDir, "bad"), "[commands.x]\nalias = \"a\"\nfunction = \"b\"")
	if err := os.MkdirAll(filepath.Join(paths.UserPacksDir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	r := Load(paths)
	if r.Command("ok") == nil {
		t.Error("good pack's command missing")
	}
	if len(r.Problems) != 2 {
		t.Errorf("problems = %v, want 2 (bad manifest, missing manifest)", r.Problems)
	}
}

func TestLinkedPack(t *testing.T) {
	paths := testPaths(t)
	checkout := filepath.Join(t.TempDir(), "checkout")
	writePack(t, checkout, "name = \"dev\"\n[commands.d]\nalias = \"d\"")
	if err := os.MkdirAll(paths.UserPacksDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(checkout, filepath.Join(paths.UserPacksDir, "dev")); err != nil {
		t.Fatal(err)
	}

	r := Load(paths)
	ref := r.Pack("dev")
	if ref == nil || !ref.Linked {
		t.Fatalf("dev pack = %+v, want linked", ref)
	}
	if want, _ := filepath.EvalSymlinks(checkout); ref.Pack.Dir != want {
		t.Errorf("Dir = %s, want %s", ref.Pack.Dir, want)
	}
}
