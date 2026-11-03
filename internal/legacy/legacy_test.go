package legacy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveBlock(t *testing.T) {
	in := "export A=1\n\n" + blockHeader + "\n### See https://github.com/ahoffner/alis ####\nsource ~/.local/alis/aliases/*\n" + blockFooter + "\nexport B=2\n"
	got, ok := removeBlock(in)
	if !ok || got != "export A=1\nexport B=2\n" {
		t.Errorf("removeBlock = %q, %v", got, ok)
	}
	if _, ok := removeBlock("export A=1\n"); ok {
		t.Error("removeBlock changed a file without the block")
	}
}

func TestRemovePlugin(t *testing.T) {
	for in, want := range map[string]string{
		"plugins=(git alis)\n":                    "plugins=(git)\n",
		"plugins=(alis git docker)\n":             "plugins=(git docker)\n",
		"plugins=(\n  git\n  alis\n)\n":           "plugins=(\n  git\n)\n",
		"# plugins=(rails git)\nplugins=(alis)\n": "# plugins=(rails git)\nplugins=()\n",
	} {
		got, ok := removePlugin(in)
		if !ok || got != want {
			t.Errorf("removePlugin(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	if _, ok := removePlugin("plugins=(git alisx)\n"); ok {
		t.Error("removePlugin matched a different plugin")
	}
}

func TestDetectAndFix(t *testing.T) {
	home := t.TempDir()
	zshrc := filepath.Join(home, ".zshrc")
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(zshrc, "plugins=(git alis)\nsource $ZSH/oh-my-zsh.sh\n\n"+blockHeader+"\nsource x\n"+blockFooter+"\n")
	write(filepath.Join(home, ".oh-my-zsh", "custom", "plugins", "alis", "alis.plugin.zsh"), "print hi\n")
	write(filepath.Join(home, ".local", "alis", "alis"), "#!/bin/bash\n")
	if err := os.MkdirAll(filepath.Join(home, ".local", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, ".local", "alis", "alis"), filepath.Join(home, ".local", "bin", "alis")); err != nil {
		t.Fatal(err)
	}

	findings := Detect(Env{Home: home})
	if len(findings) != 4 {
		t.Fatalf("found %d, want 4: %+v", len(findings), findings)
	}
	for _, f := range findings {
		if err := f.Fix(); err != nil {
			t.Fatalf("%s: %v", f.Title, err)
		}
	}
	if left := Detect(Env{Home: home}); len(left) != 0 {
		t.Errorf("still found after fixing: %+v", left)
	}
	data, _ := os.ReadFile(zshrc)
	if string(data) != "plugins=(git)\nsource $ZSH/oh-my-zsh.sh\n" {
		t.Errorf(".zshrc = %q", data)
	}
	backups, _ := filepath.Glob(zshrc + ".alis-backup-*")
	if len(backups) == 0 {
		t.Error("no backup of .zshrc was made")
	}
}
