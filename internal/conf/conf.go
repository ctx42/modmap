// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package conf reads the modmap configuration file describing the groups to
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
	// ErrNoGroups is returned for a configuration file declaring no groups.
	ErrNoGroups = errors.New("no groups declared")

	// ErrNoName is returned for a group without a name.
	ErrNoName = errors.New("group without a name")

	// ErrDupName is returned when two groups share a name.
	ErrDupName = errors.New("duplicate group name")

	// ErrNoDirs is returned for a group without directories to scan.
	ErrNoDirs = errors.New("group without directories")

	// ErrEmptyDir is returned for a group listing an empty directory.
	ErrEmptyDir = errors.New("group with an empty directory")

	// ErrNoOut is returned for a configuration without an output file.
	ErrNoOut = errors.New("no output file")

	// ErrUnkGroup is returned when a requested group is not declared.
	ErrUnkGroup = errors.New("unknown group")

	// ErrMapsKey is returned for a configuration file using the "maps" key,
	// which was renamed to "groups".
	ErrMapsKey = errors.New("maps was renamed to groups")
)

// Group describes one group the configuration file declares.
type Group struct {
	// Name is how the group is asked for on the command line.
	Name string `yaml:"name"`

	// Dirs holds the directories scanned for Go modules.
	Dirs []string `yaml:"dirs"`

	// Include holds the module path globs deciding which modules are
	// rendered. An empty list renders every module found.
	Include []string `yaml:"include"`

	// Exclude holds the module path globs removing modules from the group.
	// A module matching both lists is not rendered.
	Exclude []string `yaml:"exclude"`
}

// Config represents the modmap configuration file.
type Config struct {
	// Out is the SVG file the selected groups are drawn into, side by side.
	Out string `yaml:"out"`

	// Groups holds the groups the file declares.
	Groups []Group `yaml:"groups"`
}

// Load reads the configuration file at pth. The relative directory and output
// paths it holds are resolved against the directory of the file itself. A key
// the configuration does not define is an error, so is an "out" key on a group.
func Load(pth string) (*Config, error) {
	data, err := os.ReadFile(pth) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	cfg := &Config{}
	opt := yaml.DisallowUnknownField()
	if err = yaml.UnmarshalWithOptions(data, cfg, opt); err != nil {
		if hasKey(data, "maps") {
			err = ErrMapsKey
		}
		return nil, fmt.Errorf("parse configuration: %w", err)
	}
	cfg.resolve(filepath.Dir(pth))
	if err = cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate %s: %w", pth, err)
	}
	return cfg, nil
}

// Select returns the named groups in the order they are named, or every group
// declared when no names are given.
func (cfg *Config) Select(names []string) ([]Group, error) {
	if len(names) == 0 {
		return cfg.Groups, nil
	}
	groups := make([]Group, 0, len(names))
	for _, name := range names {
		idx := slices.IndexFunc(cfg.Groups, func(grp Group) bool {
			return grp.Name == name
		})
		if idx < 0 {
			return nil, fmt.Errorf(
				"%w: %s (have: %s)",
				ErrUnkGroup,
				name,
				strings.Join(cfg.Names(), ", "),
			)
		}
		groups = append(groups, cfg.Groups[idx])
	}
	return groups, nil
}

// Names returns the names of the declared groups.
func (cfg *Config) Names() []string {
	names := make([]string, 0, len(cfg.Groups))
	for _, grp := range cfg.Groups {
		names = append(names, grp.Name)
	}
	return names
}

// validate reports the first problem making the configuration unusable. The
// paths are compared as they are, so they are resolved beforehand.
func (cfg *Config) validate() error {
	if len(cfg.Groups) == 0 {
		return ErrNoGroups
	}
	if cfg.Out == "" {
		return ErrNoOut
	}
	names := make(map[string]bool, len(cfg.Groups))
	for idx, grp := range cfg.Groups {
		switch {
		case grp.Name == "":
			return fmt.Errorf("%w: groups[%d]", ErrNoName, idx)

		case names[grp.Name]:
			return fmt.Errorf("%w: %s", ErrDupName, grp.Name)

		case len(grp.Dirs) == 0:
			return fmt.Errorf("%w: %s", ErrNoDirs, grp.Name)

		case slices.Contains(grp.Dirs, ""):
			return fmt.Errorf("%w: %s", ErrEmptyDir, grp.Name)
		}
		names[grp.Name] = true

		for _, glob := range slices.Concat(grp.Include, grp.Exclude) {
			if err := mod.ValidateGlob(glob); err != nil {
				return fmt.Errorf("%s: %w", grp.Name, err)
			}
		}
	}
	return nil
}

// resolve makes every relative path in the configuration absolute by
// resolving it against dir. Empty paths stay empty.
func (cfg *Config) resolve(dir string) {
	cfg.Out = absPath(dir, cfg.Out)
	for _, grp := range cfg.Groups {
		for idx, pth := range grp.Dirs {
			grp.Dirs[idx] = absPath(dir, pth)
		}
	}
}

// hasKey reports whether the YAML document in data is a mapping holding the
// top level key.
func hasKey(data []byte, key string) bool {
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}
	_, ok := doc[key]
	return ok
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
