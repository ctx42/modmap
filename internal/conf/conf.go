// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package conf reads the modmap configuration file describing the maps to
// generate.
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

	// ErrNoOut is returned for a map without an output file.
	ErrNoOut = errors.New("map without an output file")

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

	// Out is the SVG file the map is written to.
	Out string `yaml:"out"`
}

// Config represents the modmap configuration file.
type Config struct {
	// Maps holds the maps the file declares.
	Maps []Map `yaml:"maps"`
}

// Load reads the configuration file at pth. The relative directory and output
// paths it holds are resolved against the directory of the file itself.
func Load(pth string) (*Config, error) {
	data, err := os.ReadFile(pth) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	cfg := &Config{}
	if err = yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse configuration: %w", err)
	}
	if err = cfg.validate(); err != nil {
		return nil, err
	}
	cfg.resolve(filepath.Dir(pth))
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

// validate reports the first problem making the configuration unusable.
func (cfg *Config) validate() error {
	if len(cfg.Maps) == 0 {
		return ErrNoMaps
	}
	seen := make(map[string]bool, len(cfg.Maps))
	for _, mp := range cfg.Maps {
		switch {
		case mp.Name == "":
			return ErrNoName

		case seen[mp.Name]:
			return fmt.Errorf("%w: %s", ErrDupName, mp.Name)

		case len(mp.Dirs) == 0:
			return fmt.Errorf("%w: %s", ErrNoDirs, mp.Name)

		case mp.Out == "":
			return fmt.Errorf("%w: %s", ErrNoOut, mp.Name)
		}
		seen[mp.Name] = true

		for _, glob := range slices.Concat(mp.Include, mp.Exclude) {
			if err := mod.ValidateGlob(glob); err != nil {
				return fmt.Errorf("%s: %w", mp.Name, err)
			}
		}
	}
	return nil
}

// resolve makes every relative path in the configuration absolute by
// resolving it against dir.
func (cfg *Config) resolve(dir string) {
	for idx, mp := range cfg.Maps {
		for jdx, pth := range mp.Dirs {
			cfg.Maps[idx].Dirs[jdx] = absPath(dir, pth)
		}
		cfg.Maps[idx].Out = absPath(dir, mp.Out)
	}
}

// absPath returns pth as an absolute path, resolving a relative one against
// dir.
func absPath(dir, pth string) string {
	if filepath.IsAbs(pth) {
		return pth
	}
	return filepath.Join(dir, pth)
}
