// Package scaffold creates a new pack from the embedded template.
package scaffold

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed template
var files embed.FS

type data struct {
	Name        string
	Description string
	Prefix      string // prepended to the example command names so they don't collide
}

// New writes a pack named name into dir, which must not exist yet, and runs `git init` there.
func New(dir, name string) error {
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s already exists", dir)
	}
	d := data{Name: name, Description: "Shortcuts for " + name, Prefix: name + "-"}

	err := fs.WalkDir(files, "template", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel := strings.TrimSuffix(strings.TrimPrefix(path, "template/"), ".tmpl")
		mode := fs.FileMode(0o644)
		if strings.HasPrefix(rel, "bin/") {
			rel = "bin/" + d.Prefix + strings.TrimPrefix(rel, "bin/")
			mode = 0o755
		}
		src, err := files.ReadFile(path)
		if err != nil {
			return err
		}
		tmpl, err := template.New(rel).Parse(string(src))
		if err != nil {
			return err
		}
		var out bytes.Buffer
		if err := tmpl.Execute(&out, d); err != nil {
			return err
		}
		dest := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, out.Bytes(), mode)
	})
	if err != nil {
		return err
	}
	if git, err := exec.LookPath("git"); err == nil {
		cmd := exec.Command(git, "init", "--quiet", dir)
		return cmd.Run()
	}
	return nil
}
