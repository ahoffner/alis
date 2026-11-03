package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseKinds(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "hello"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := Parse([]byte(`
name = "demo"
[commands.a]
alias = "echo a"
[commands.f]
function = "echo f"
[commands.s]
script = "bin/hello"
`), dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Kind{"a": KindAlias, "f": KindFunction, "s": KindScript}
	for name, kind := range want {
		c := p.Commands[name]
		if c.Kind() != kind || c.Name != name || c.Pack != p {
			t.Errorf("%s: kind %s name %q, want %s", name, c.Kind(), c.Name, kind)
		}
	}
	if got := p.Commands["s"].ScriptPath(); got != filepath.Join(dir, "bin", "hello") {
		t.Errorf("ScriptPath = %s", got)
	}
}

func TestParseDefaultsNameToDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mypack")
	p, err := Parse([]byte(`[commands.x]
alias = "ls"`), dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "mypack" {
		t.Errorf("name = %q", p.Name)
	}
}

func TestParseErrors(t *testing.T) {
	tests := map[string]struct{ toml, want string }{
		"unknown field":   {"[commands.x]\nalias = \"ls\"\nalais = \"typo\"", "unknown field"},
		"two kinds":       {"[commands.x]\nalias = \"ls\"\nfunction = \"ls\"", "exactly one"},
		"no kind":         {"[commands.x]\ndescription = \"nothing\"", "exactly one"},
		"bad name":        {"[commands.\"bad name\"]\nalias = \"ls\"", "name must"},
		"reserved":        {"[commands.cd]\nalias = \"ls\"", "reserved"},
		"missing script":  {"[commands.x]\nscript = \"bin/nope\"", "not found"},
		"escaping script": {"[commands.x]\nscript = \"../outside\"", "inside the pack"},
		"bad pack name":   {"name = \"Bad Name\"", "pack name"},
		"syntax":          {"[commands.x\nalias = 1", "line"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(tt.toml), t.TempDir())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestBuiltinPacksAreValid(t *testing.T) {
	dirs, err := filepath.Glob("../../packs/*")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no built-in packs found: %v", err)
	}
	for _, dir := range dirs {
		p, err := Load(dir)
		if err != nil {
			t.Errorf("%s: %v", dir, err)
			continue
		}
		for _, c := range p.Commands {
			if c.Description == "" {
				t.Errorf("%s/%s has no description", p.Name, c.Name)
			}
			if c.Kind() == KindScript {
				info, err := os.Stat(c.ScriptPath())
				if err != nil || info.Mode()&0o111 == 0 {
					t.Errorf("%s/%s: script isn't executable", p.Name, c.Name)
				}
			}
		}
	}
}
