// Package legacy finds and removes installs of alis 1.x/2.x, which conflict with 3.x.
package legacy

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	blockHeader = "### MANAGED BY ALIS DO NOT EDIT ####"
	blockFooter = "### END ALIS MANAGED ALIASES ####"
)

// Finding is one leftover from an old install, with a fix that removes it.
type Finding struct {
	Title  string
	Detail string
	Fix    func() error
}

// Env is where to look; tests point it at a temp dir.
type Env struct {
	Home      string
	ZshCustom string // $ZSH_CUSTOM, or empty for ~/.oh-my-zsh/custom
}

// Detect returns every legacy install artifact it can find.
func Detect(env Env) []Finding {
	var out []Finding
	for _, rc := range []string{".zshrc", ".bashrc", ".bash_profile"} {
		path := filepath.Join(env.Home, rc)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if _, ok := removeBlock(string(data)); ok {
			out = append(out, Finding{
				Title:  "alis 1.x block in ~/" + rc,
				Detail: "the \"" + blockHeader + "\" section sources the old aliases and adds ~/.local/alis to PATH",
				Fix:    func() error { return rewrite(path, removeBlock) },
			})
		}
	}

	oldDir := filepath.Join(env.Home, ".local", "alis")
	if info, err := os.Stat(oldDir); err == nil && info.IsDir() {
		out = append(out, Finding{
			Title:  "alis 1.x install at ~/.local/alis",
			Detail: "old copy of the scripts",
			Fix:    func() error { return os.RemoveAll(oldDir) },
		})
	}
	oldLink := filepath.Join(env.Home, ".local", "bin", "alis")
	if target, err := os.Readlink(oldLink); err == nil && strings.Contains(target, filepath.Join(".local", "alis")) {
		out = append(out, Finding{
			Title:  "alis 1.x link at ~/.local/bin/alis",
			Detail: "points at the old install and can shadow the new binary on PATH",
			Fix:    func() error { return os.Remove(oldLink) },
		})
	}

	custom := env.ZshCustom
	if custom == "" {
		custom = filepath.Join(env.Home, ".oh-my-zsh", "custom")
	}
	pluginDir := filepath.Join(custom, "plugins", "alis")
	zshrc := filepath.Join(env.Home, ".zshrc")
	data, _ := os.ReadFile(zshrc)
	_, inPlugins := removePlugin(string(data))
	_, statErr := os.Stat(pluginDir)
	if inPlugins || statErr == nil {
		out = append(out, Finding{
			Title:  "alis 2.x oh-my-zsh plugin",
			Detail: "`alis` in plugins=(…) of ~/.zshrc and/or " + pluginDir + "; it prints a banner at startup",
			Fix: func() error {
				// Edit plugins=(…) first: a listed plugin with no directory makes oh-my-zsh warn.
				if inPlugins {
					if err := rewrite(zshrc, removePlugin); err != nil {
						return err
					}
				}
				return os.RemoveAll(pluginDir)
			},
		})
	}
	return out
}

// rewrite applies edit to a file, keeping a timestamped backup next to it.
func rewrite(path string, edit func(string) (string, bool)) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, changed := edit(string(data))
	if !changed {
		return nil
	}
	if err := Backup(path, data, info.Mode().Perm()); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), info.Mode().Perm())
}

// Backup saves data as path.alis-backup-<unix time>. If several edits land in the same
// second, the first backup (the original file) is kept.
func Backup(path string, data []byte, perm os.FileMode) error {
	backup := fmt.Sprintf("%s.alis-backup-%d", path, time.Now().Unix())
	f, err := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// removeBlock deletes the alis 1.x managed block (header through footer) and the blank
// line the installer put before it.
func removeBlock(s string) (string, bool) {
	start := strings.Index(s, blockHeader)
	if start < 0 {
		return s, false
	}
	end := strings.Index(s[start:], blockFooter)
	if end < 0 {
		return s, false
	}
	end = start + end + len(blockFooter)
	if end < len(s) && s[end] == '\n' {
		end++
	}
	for start > 0 && s[start-1] == '\n' && (start < 2 || s[start-2] == '\n') {
		start--
	}
	return s[:start] + s[end:], true
}

var pluginsRe = regexp.MustCompile(`(?m)^(\s*plugins=\()([^)]*)(\))`)

// removePlugin drops `alis` from oh-my-zsh's plugins=(…) list.
func removePlugin(s string) (string, bool) {
	changed := false
	out := pluginsRe.ReplaceAllStringFunc(s, func(m string) string {
		parts := pluginsRe.FindStringSubmatch(m)
		fields := strings.Fields(parts[2])
		if !slices.Contains(fields, "alis") {
			return m
		}
		changed = true
		fields = slices.DeleteFunc(fields, func(f string) bool { return f == "alis" })
		if strings.Contains(parts[2], "\n") {
			return parts[1] + "\n  " + strings.Join(fields, "\n  ") + "\n" + parts[3]
		}
		return parts[1] + strings.Join(fields, " ") + parts[3]
	})
	return out, changed
}
