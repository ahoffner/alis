// Package registry discovers packs, applies precedence and config, and yields the effective commands.
package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ahoffner/alis/internal/config"
	"github.com/ahoffner/alis/internal/manifest"
)

// Source is where a pack was found. Later sources take precedence over earlier ones.
type Source string

const (
	SourceShared Source = "shared" // built-in or brew-installed: <share>/packs
	SourceUser   Source = "user"   // added with `alis pack add`
	SourceLocal  Source = "local"  // the machine-only pack in ~/.config/alis/local
)

// PackRef is a discovered pack and where it came from.
type PackRef struct {
	Pack     *manifest.Pack
	Source   Source
	Path     string // directory as discovered (for a linked pack, the symlink)
	Linked   bool   // a symlink to a local checkout, added with `alis pack add <path>`
	Disabled bool
}

// Problem is a pack that could not be loaded. Problems never stop the other packs from loading.
type Problem struct {
	Path string
	Err  error
}

// Shadow records a command or pack hidden by a later one with the same name.
type Shadow struct {
	Name   string
	Winner string // "pack (source)"
	Loser  string
}

// Registry is every discovered pack plus the effective set of commands.
type Registry struct {
	Paths    config.Paths
	Config   *config.Config
	Packs    []*PackRef
	Commands []*manifest.Command // effective commands, sorted by name
	Shadows  []Shadow
	Problems []Problem

	sources map[*manifest.Pack]*PackRef
}

// Load discovers packs under paths. It never fails; anything wrong is recorded in Problems.
func Load(paths config.Paths) *Registry {
	r := &Registry{Paths: paths, sources: map[*manifest.Pack]*PackRef{}}

	cfg, err := config.Load(paths.ConfigFile)
	if err != nil {
		r.Problems = append(r.Problems, Problem{Path: paths.ConfigFile, Err: err})
	}
	r.Config = cfg

	var found []*PackRef
	if paths.ShareDir != "" {
		found = append(found, r.scan(paths.SharedPacksDir(), SourceShared)...)
	}
	found = append(found, r.scan(paths.UserPacksDir, SourceUser)...)
	if ref := r.loadOne(paths.LocalPackDir, SourceLocal, false); ref != nil {
		ref.Pack.Name = "local"
		found = append(found, ref)
	}

	// A later pack with the same name replaces an earlier one entirely.
	byName := map[string]int{}
	for _, ref := range found {
		if i, ok := byName[ref.Pack.Name]; ok {
			r.Shadows = append(r.Shadows, Shadow{Name: "pack " + ref.Pack.Name, Winner: label(ref), Loser: label(r.Packs[i])})
			r.Packs[i] = ref
			continue
		}
		byName[ref.Pack.Name] = len(r.Packs)
		r.Packs = append(r.Packs, ref)
	}

	effective := map[string]*manifest.Command{}
	for _, ref := range r.Packs {
		ref.Disabled = cfg.PackDisabled(ref.Pack.Name)
		r.sources[ref.Pack] = ref
		if ref.Disabled {
			continue
		}
		for _, c := range ref.Pack.Sorted() {
			if cfg.CommandDisabled(c.Name) {
				continue
			}
			if prev, ok := effective[c.Name]; ok {
				r.Shadows = append(r.Shadows, Shadow{Name: c.Name, Winner: label(ref), Loser: label(r.sources[prev.Pack])})
			}
			effective[c.Name] = c
		}
	}
	for _, c := range effective {
		r.Commands = append(r.Commands, c)
	}
	sort.Slice(r.Commands, func(i, j int) bool { return r.Commands[i].Name < r.Commands[j].Name })
	return r
}

// scan loads every pack directory directly under dir, in name order.
func (r *Registry) scan(dir string, src Source) []*PackRef {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		r.Problems = append(r.Problems, Problem{Path: dir, Err: err})
		return nil
	}
	var refs []*PackRef
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		info, err := os.Stat(path) // follows symlinks
		if err != nil {
			r.Problems = append(r.Problems, Problem{Path: path, Err: fmt.Errorf("broken link or unreadable: %w", err)})
			continue
		}
		if !info.IsDir() {
			continue
		}
		if ref := r.loadOne(path, src, e.Type()&fs.ModeSymlink != 0); ref != nil {
			refs = append(refs, ref)
		}
	}
	return refs
}

func (r *Registry) loadOne(path string, src Source, linked bool) *PackRef {
	if _, err := os.Stat(filepath.Join(path, manifest.FileName)); errors.Is(err, fs.ErrNotExist) {
		if src != SourceLocal {
			r.Problems = append(r.Problems, Problem{Path: path, Err: fmt.Errorf("no %s", manifest.FileName)})
		}
		return nil
	}
	dir := path
	if linked {
		if target, err := filepath.EvalSymlinks(path); err == nil {
			dir = target
		}
	}
	p, err := manifest.Load(dir)
	if err != nil {
		r.Problems = append(r.Problems, Problem{Path: filepath.Join(path, manifest.FileName), Err: err})
		return nil
	}
	return &PackRef{Pack: p, Source: src, Path: path, Linked: linked}
}

// Command returns the effective command with that name, or nil.
func (r *Registry) Command(name string) *manifest.Command {
	i := sort.Search(len(r.Commands), func(i int) bool { return r.Commands[i].Name >= name })
	if i < len(r.Commands) && r.Commands[i].Name == name {
		return r.Commands[i]
	}
	return nil
}

// Pack returns the pack with that name, or nil.
func (r *Registry) Pack(name string) *PackRef {
	for _, ref := range r.Packs {
		if ref.Pack.Name == name {
			return ref
		}
	}
	return nil
}

// SourceOf returns the pack reference a command came from.
func (r *Registry) SourceOf(c *manifest.Command) *PackRef { return r.sources[c.Pack] }

func label(ref *PackRef) string { return fmt.Sprintf("%s (%s)", ref.Pack.Name, ref.Source) }
