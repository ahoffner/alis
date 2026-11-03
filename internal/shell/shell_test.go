package shell

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahoffner/alis/internal/manifest"
)

var update = flag.Bool("update", false, "rewrite golden files")

func loadTricky(t *testing.T) (*manifest.Pack, []*manifest.Command) {
	t.Helper()
	p, err := manifest.Load("testdata/tricky")
	if err != nil {
		t.Fatal(err)
	}
	return p, p.Sorted()
}

func render(t *testing.T, sh Shell) string {
	t.Helper()
	_, cmds := loadTricky(t)
	var b strings.Builder
	if err := Init(&b, sh, cmds, "/lib/alis.sh"); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestGolden(t *testing.T) {
	p, _ := loadTricky(t)
	for _, sh := range []Shell{Zsh, Bash} {
		t.Run(string(sh), func(t *testing.T) {
			got := strings.ReplaceAll(render(t, sh), p.Dir, "$PACKDIR")
			golden := filepath.Join("testdata", "init."+string(sh)+".golden")
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run `go test ./internal/shell -update`)", err)
			}
			if got != string(want) {
				t.Errorf("init %s changed; run `go test ./internal/shell -update` if intended.\n--- got\n%s", sh, got)
			}
		})
	}
}

func TestQuote(t *testing.T) {
	for in, want := range map[string]string{
		"plain":     "'plain'",
		"it's":      `'it'\''s'`,
		`$HOME "x"`: `'$HOME "x"'`,
	} {
		if got := Quote(in); got != want {
			t.Errorf("Quote(%q) = %s, want %s", in, got, want)
		}
	}
}

// shellCommand runs script in a clean, non-interactive instance of sh.
func shellCommand(t *testing.T, sh Shell, script string) *exec.Cmd {
	t.Helper()
	path, err := exec.LookPath(string(sh))
	if err != nil {
		t.Skipf("%s not installed", sh)
	}
	var cmd *exec.Cmd
	if sh == Zsh {
		cmd = exec.Command(path, "-f", "-c", script)
	} else {
		cmd = exec.Command(path, "--norc", "--noprofile", "-c", "shopt -s expand_aliases\n"+script)
	}
	cmd.Env = []string{"HOME=/h", "PATH=" + os.Getenv("PATH")}
	return cmd
}

// TestInitInRealShells evaluates the generated code the way a shell config would.
func TestInitInRealShells(t *testing.T) {
	for _, sh := range []Shell{Zsh, Bash} {
		t.Run(string(sh), func(t *testing.T) {
			init := render(t, sh)

			// Startup must be silent, even when a name we define is already an alias.
			cmd := shellCommand(t, sh, "alias gc='git commit'\neval \"$ALIS_INIT\"")
			cmd.Env = append(cmd.Env, "ALIS_INIT="+init)
			if out, err := cmd.CombinedOutput(); err != nil || len(out) > 0 {
				t.Fatalf("init wasn't silent: err=%v output=%q", err, out)
			}

			// Each command runs in its own eval so it is parsed after init, like a typed command.
			cmd = shellCommand(t, sh, strings.Join([]string{
				"alias gc='git commit'",
				`eval "$ALIS_INIT"`,
				"eval 'q'",
				`eval 'fn a "b c"'`,
				"eval 'usealias'",
				"eval 'gc'",
				"eval 'scr x'",
				// Re-running init with no commands removes what the first run defined.
				`eval "$ALIS_EMPTY"`,
				"eval 'q' 2>/dev/null || echo q-gone",
			}, "\n"))
			var empty strings.Builder
			if err := Init(&empty, sh, nil, ""); err != nil {
				t.Fatal(err)
			}
			cmd.Env = append(cmd.Env, "ALIS_INIT="+init, "ALIS_EMPTY="+empty.String())
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%v\n%s", err, out)
			}
			want := strings.Join([]string{
				`it's /h "x" \ done`,
				"a|b c|",
				`it's /h "x" \ done`,
				"mine",
				"scr:x:/lib/alis.sh",
				"q-gone",
			}, "\n") + "\n"
			if string(out) != want {
				t.Errorf("got:\n%s\nwant:\n%s", out, want)
			}
		})
	}
}

// TestBrokenFunctionIsSilent checks a function with a syntax error is skipped quietly.
func TestBrokenFunctionIsSilent(t *testing.T) {
	p := &manifest.Pack{Name: "broken", Dir: t.TempDir()}
	cmds := []*manifest.Command{
		{Name: "bad", Function: "if then fi }", Pack: p},
		{Name: "good", Alias: "echo good", Pack: p},
	}
	for _, sh := range []Shell{Zsh, Bash} {
		t.Run(string(sh), func(t *testing.T) {
			var b strings.Builder
			if err := Init(&b, sh, cmds, ""); err != nil {
				t.Fatal(err)
			}
			cmd := shellCommand(t, sh, "eval \"$ALIS_INIT\"\neval good")
			cmd.Env = append(cmd.Env, "ALIS_INIT="+b.String())
			out, err := cmd.CombinedOutput()
			if err != nil || string(out) != "good\n" {
				t.Errorf("err=%v output=%q, want only \"good\"", err, out)
			}
		})
	}
}
