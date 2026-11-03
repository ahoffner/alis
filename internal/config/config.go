// Package config resolves alis's on-disk locations and reads/writes the user config file.
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/pelletier/go-toml/v2"
)

// Paths are the directories alis reads packs and settings from.
type Paths struct {
	// ShareDir holds packs/ (built-in and brew-installed packs) and lib/alis.sh.
	ShareDir string
	// UserPacksDir holds packs added with `alis pack add` (git clones or symlinks).
	UserPacksDir string
	// LocalPackDir holds the machine-only pack, which is never shared.
	LocalPackDir string
	// ConfigFile is the user's config.toml.
	ConfigFile string
}

// SharedPacksDir is where built-in and brew-installed packs live.
func (p Paths) SharedPacksDir() string { return filepath.Join(p.ShareDir, "packs") }

// LibFile is the helper library pack scripts source via $ALIS_LIB, or "" if it isn't installed.
func (p Paths) LibFile() string {
	if p.ShareDir == "" {
		return ""
	}
	path := filepath.Join(p.ShareDir, "lib", "alis.sh")
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// Resolve works out every path. buildShareDir is the share directory baked in at build time.
func Resolve(buildShareDir string) Paths {
	home, _ := os.UserHomeDir()
	dataHome := envOr("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	configHome := envOr("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	return Paths{
		ShareDir:     resolveShareDir(buildShareDir),
		UserPacksDir: filepath.Join(dataHome, "alis", "packs"),
		LocalPackDir: filepath.Join(configHome, "alis", "local"),
		ConfigFile:   filepath.Join(configHome, "alis", "config.toml"),
	}
}

func resolveShareDir(buildShareDir string) string {
	if dir := os.Getenv("ALIS_SHARE_DIR"); dir != "" {
		return dir
	}
	candidates := []string{buildShareDir}
	// Next to the binary, e.g. a Homebrew keg: <keg>/bin/alis → <keg>/share/alis.
	if exe, err := os.Executable(); err == nil {
		if exe, err = filepath.EvalSymlinks(exe); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", "share", "alis"))
		}
	}
	if prefix := os.Getenv("HOMEBREW_PREFIX"); prefix != "" {
		candidates = append(candidates, filepath.Join(prefix, "share", "alis"))
	}
	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, "packs")); err == nil && info.IsDir() {
			return filepath.Clean(dir)
		}
	}
	return buildShareDir
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Config is the user's config.toml.
type Config struct {
	DisabledPacks    []string `toml:"disabled_packs,omitempty"`
	DisabledCommands []string `toml:"disabled_commands,omitempty"`
}

// Load reads the config file; a missing file is an empty config.
func Load(path string) (*Config, error) {
	c := &Config{}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := toml.Unmarshal(data, c); err != nil {
		return &Config{}, err
	}
	return c, nil
}

// Save writes the config file, creating its directory if needed.
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c *Config) PackDisabled(name string) bool    { return slices.Contains(c.DisabledPacks, name) }
func (c *Config) CommandDisabled(name string) bool { return slices.Contains(c.DisabledCommands, name) }

func (c *Config) SetPackDisabled(name string, disabled bool) {
	c.DisabledPacks = toggle(c.DisabledPacks, name, disabled)
}

func (c *Config) SetCommandDisabled(name string, disabled bool) {
	c.DisabledCommands = toggle(c.DisabledCommands, name, disabled)
}

func toggle(list []string, name string, present bool) []string {
	list = slices.DeleteFunc(list, func(s string) bool { return s == name })
	if present {
		list = append(list, name)
		slices.Sort(list)
	}
	return list
}
