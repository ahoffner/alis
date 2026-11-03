// Package manifest defines the alis.toml pack format and loads it from disk.
package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// FileName is the manifest file every pack directory must contain.
const FileName = "alis.toml"

// Kind is how a command is exposed to the shell.
type Kind string

const (
	KindAlias    Kind = "alias"    // a shell alias; arguments are appended
	KindFunction Kind = "function" // a shell function body, sourced into the user's shell
	KindScript   Kind = "script"   // an executable file in the pack, run as a subprocess
)

// Pack is a named set of commands loaded from one alis.toml.
type Pack struct {
	Name        string              `toml:"name"`
	Description string              `toml:"description"`
	Requires    []string            `toml:"requires"`
	Commands    map[string]*Command `toml:"commands"`

	// Dir is the absolute directory the manifest was loaded from.
	Dir string `toml:"-"`
}

// Command is a single alias, function or script.
type Command struct {
	Description string   `toml:"description"`
	Group       string   `toml:"group"`
	Usage       string   `toml:"usage"`
	Help        string   `toml:"help"`
	Requires    []string `toml:"requires"`

	// Exactly one of these is set.
	Alias    string `toml:"alias"`
	Function string `toml:"function"`
	Script   string `toml:"script"`

	Name string `toml:"-"`
	Pack *Pack  `toml:"-"`
}

var (
	packNameRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	commandNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

	// Names that would break or hijack the shell if defined as an alias or function.
	reservedNames = map[string]bool{
		"alis": true, "if": true, "then": true, "else": true, "elif": true, "fi": true,
		"for": true, "while": true, "until": true, "do": true, "done": true, "case": true,
		"esac": true, "function": true, "select": true, "time": true, "in": true,
		"coproc": true, "foreach": true, "end": true, "repeat": true, "nocorrect": true,
		"command": true, "builtin": true, "exec": true, "eval": true, "source": true,
		"cd": true, "export": true, "unset": true, "unalias": true, "unfunction": true,
	}
)

// Kind reports how the command is exposed. It is only meaningful after validation.
func (c *Command) Kind() Kind {
	switch {
	case c.Alias != "":
		return KindAlias
	case c.Function != "":
		return KindFunction
	default:
		return KindScript
	}
}

// Body is the alias expansion, function body or script path, as written in the manifest.
func (c *Command) Body() string {
	switch c.Kind() {
	case KindAlias:
		return c.Alias
	case KindFunction:
		return c.Function
	default:
		return c.Script
	}
}

// ScriptPath is the absolute path of a script command.
func (c *Command) ScriptPath() string {
	return filepath.Join(c.Pack.Dir, filepath.FromSlash(c.Script))
}

// Sorted returns the pack's commands ordered by name.
func (p *Pack) Sorted() []*Command {
	out := make([]*Command, 0, len(p.Commands))
	for _, c := range p.Commands {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Load reads and validates dir/alis.toml.
func Load(dir string) (*Pack, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(abs, FileName))
	if err != nil {
		return nil, err
	}
	return Parse(data, abs)
}

// Parse decodes and validates manifest data for a pack rooted at dir.
func Parse(data []byte, dir string) (*Pack, error) {
	var p Pack
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		var strict *toml.StrictMissingError
		if errors.As(err, &strict) {
			return nil, fmt.Errorf("unknown field(s):\n%s", strict.String())
		}
		var de *toml.DecodeError
		if errors.As(err, &de) {
			row, col := de.Position()
			return nil, fmt.Errorf("line %d, column %d: %s", row, col, de.Error())
		}
		return nil, err
	}
	p.Dir = dir
	if p.Name == "" {
		p.Name = filepath.Base(dir)
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Pack) validate() error {
	var errs []error
	if !packNameRe.MatchString(p.Name) {
		errs = append(errs, fmt.Errorf("pack name %q must be lowercase letters, digits, '-' or '_'", p.Name))
	}
	for name, c := range p.Commands {
		c.Name = name
		c.Pack = p
		if err := c.validate(); err != nil {
			errs = append(errs, fmt.Errorf("commands.%s: %w", name, err))
		}
	}
	sort.Slice(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
	return errors.Join(errs...)
}

func (c *Command) validate() error {
	if !commandNameRe.MatchString(c.Name) {
		return errors.New("name must start with a letter or '_' and contain only letters, digits, '-' or '_'")
	}
	if reservedNames[c.Name] {
		return fmt.Errorf("%q is reserved by the shell or alis", c.Name)
	}
	set := 0
	for _, v := range []string{c.Alias, c.Function, c.Script} {
		if strings.TrimSpace(v) != "" {
			set++
		}
	}
	if set != 1 {
		return errors.New("set exactly one of alias, function or script")
	}
	if c.Script != "" {
		if filepath.IsAbs(c.Script) || strings.HasPrefix(filepath.Clean(c.Script), "..") {
			return fmt.Errorf("script %q must be a path inside the pack", c.Script)
		}
		info, err := os.Stat(c.ScriptPath())
		if err != nil {
			return fmt.Errorf("script %q not found", c.Script)
		}
		if info.IsDir() {
			return fmt.Errorf("script %q is a directory", c.Script)
		}
	}
	return nil
}
