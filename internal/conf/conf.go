// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package conf reads the modmap configuration file describing the maps to
// draw and the image they are drawn into.
package conf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/ctx42/modmap/pkg/mod"
)

// Configuration file errors.
var (
	// ErrNoMaps is returned for a configuration file declaring no maps.
	ErrNoMaps = errors.New("no maps declared")

	// ErrNoName is returned for a map without a name.
	ErrNoName = errors.New("map without a name")

	// ErrDupName is returned when two maps share a name.
	ErrDupName = errors.New("duplicate map name")

	// ErrNoDirs is returned for a map without directories to scan.
	ErrNoDirs = errors.New("map without directories")

	// ErrEmptyDir is returned for a map listing an empty directory.
	ErrEmptyDir = errors.New("map with an empty directory")

	// ErrNoOut is returned for a configuration without an output file.
	ErrNoOut = errors.New("no output file")

	// ErrUnkMap is returned when a requested map is not declared.
	ErrUnkMap = errors.New("unknown map")
)

// Map describes one map the configuration file declares.
type Map struct {
	// Name is how the map is asked for on the command line.
	Name string `yaml:"name"`

	// Dirs holds the directories scanned for Go modules.
	Dirs []string `yaml:"dirs"`

	// Include holds the module path globs deciding which modules are
	// rendered. An empty list renders every module found.
	Include []string `yaml:"include"`

	// Exclude holds the module path globs removing modules from the map. A
	// module matching both lists is not rendered.
	Exclude []string `yaml:"exclude"`
}

// Config represents the modmap configuration file.
type Config struct {
	// Out is the SVG file the selected maps are drawn into, side by side.
	Out string `yaml:"out"`

	// Maps holds the maps the file declares.
	Maps []Map `yaml:"maps"`
}

// Load reads the configuration file at pth. The relative directory and output
// paths it holds are resolved against the directory of the file itself. A key
// the configuration does not define is an error, so is an "out" key on a map.
func Load(pth string) (*Config, error) {
	data, err := os.ReadFile(pth) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	cfg := &Config{}
	opt := yaml.DisallowUnknownField()
	if err = yaml.UnmarshalWithOptions(data, cfg, opt); err != nil {
		return nil, fmt.Errorf("parse configuration: %w", err)
	}
	cfg.resolve(filepath.Dir(pth))
	if err = cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate %s: %w", pth, err)
	}
	return cfg, nil
}

// Select returns the named maps in the order they are named, or every map
// declared when no names are given.
func (cfg *Config) Select(names []string) ([]Map, error) {
	if len(names) == 0 {
		return cfg.Maps, nil
	}
	maps := make([]Map, 0, len(names))
	for _, name := range names {
		idx := slices.IndexFunc(cfg.Maps, func(mp Map) bool {
			return mp.Name == name
		})
		if idx < 0 {
			return nil, fmt.Errorf(
				"%w: %s (have: %s)",
				ErrUnkMap,
				name,
				strings.Join(cfg.Names(), ", "),
			)
		}
		maps = append(maps, cfg.Maps[idx])
	}
	return maps, nil
}

// Names returns the names of the declared maps.
func (cfg *Config) Names() []string {
	names := make([]string, 0, len(cfg.Maps))
	for _, mp := range cfg.Maps {
		names = append(names, mp.Name)
	}
	return names
}

// validate reports the first problem making the configuration unusable. The
// paths are compared as they are, so they are resolved beforehand.
func (cfg *Config) validate() error {
	if len(cfg.Maps) == 0 {
		return ErrNoMaps
	}
	if cfg.Out == "" {
		return ErrNoOut
	}
	names := make(map[string]bool, len(cfg.Maps))
	for idx, mp := range cfg.Maps {
		switch {
		case mp.Name == "":
			return fmt.Errorf("%w: maps[%d]", ErrNoName, idx)

		case names[mp.Name]:
			return fmt.Errorf("%w: %s", ErrDupName, mp.Name)

		case len(mp.Dirs) == 0:
			return fmt.Errorf("%w: %s", ErrNoDirs, mp.Name)

		case slices.Contains(mp.Dirs, ""):
			return fmt.Errorf("%w: %s", ErrEmptyDir, mp.Name)
		}
		names[mp.Name] = true

		for _, glob := range slices.Concat(mp.Include, mp.Exclude) {
			if err := mod.ValidateGlob(glob); err != nil {
				return fmt.Errorf("%s: %w", mp.Name, err)
			}
		}
	}
	return nil
}

// resolve makes every relative path in the configuration absolute by
// resolving it against dir. Empty paths stay empty.
func (cfg *Config) resolve(dir string) {
	cfg.Out = absPath(dir, cfg.Out)
	for _, mp := range cfg.Maps {
		for idx, pth := range mp.Dirs {
			mp.Dirs[idx] = absPath(dir, pth)
		}
	}
}

// absPath returns pth as a clean absolute path, resolving a relative one
// against dir. An empty pth stays empty.
func absPath(dir, pth string) string {
	if pth == "" {
		return ""
	}
	if filepath.IsAbs(pth) {
		return filepath.Clean(pth)
	}
	return filepath.Join(dir, pth)
}
