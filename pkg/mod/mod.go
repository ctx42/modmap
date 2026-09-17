// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package mod discovers Go modules on disk and reads the module paths they
// directly require from their go.mod files.
package mod

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
)

// Module file errors.
var (
	// ErrNoModule is returned for a go.mod file without a module directive.
	ErrNoModule = errors.New("go.mod without a module directive")

	// ErrNoModFile is returned when the go command reports no go.mod file
	// for a module version.
	ErrNoModFile = errors.New("no go.mod file for module version")
)

// cmpRequire orders requirements by module path then version.
func cmpRequire(a, b Require) int {
	if cmp := strings.Compare(a.Path, b.Path); cmp != 0 {
		return cmp
	}
	return strings.Compare(a.Ver, b.Ver)
}

// Require represents a direct requirement of a module.
type Require struct {
	// Path is the module path of the required module.
	Path string

	// Ver is the required module version.
	Ver string
}

// Module represents a Go module and the modules it directly requires.
type Module struct {
	// Path is the module path taken from the "module" directive.
	Path string

	// Dir is the absolute path of the directory holding the go.mod file. It
	// is empty for a module which was not found on disk.
	Dir string

	// Requires holds the direct requirements passing the filter, sorted by
	// module path then version. Indirect requirements are never listed.
	Requires []Require
}

// Deps returns the sorted, unique module paths [Module] requires.
func (mod *Module) Deps() []string {
	pths := make([]string, 0, len(mod.Requires))
	for _, req := range mod.Requires {
		pths = append(pths, req.Path)
	}
	slices.Sort(pths)
	return slices.Compact(pths)
}

// parseMod reads the go.mod file at pth and returns the module it describes.
// Direct requirements not passing flt are dropped.
func parseMod(pth string, flt Filter) (*Module, error) {
	data, err := os.ReadFile(pth) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("read module file: %w", err)
	}
	return parseModData(pth, data, flt)
}

// parseModData returns the module described by the go.mod file content. The
// pth is used in the parse error messages only. Direct requirements not
// passing flt are dropped.
func parseModData(pth string, data []byte, flt Filter) (*Module, error) {
	fil, err := modfile.Parse(pth, data, nil)
	if err != nil {
		return nil, fmt.Errorf("parse module file: %w", err)
	}
	if fil.Module == nil {
		return nil, fmt.Errorf("%w: %s", ErrNoModule, pth)
	}
	mod := &Module{Path: fil.Module.Mod.Path}
	for _, req := range fil.Require {
		if req.Indirect || !flt.Match(req.Mod.Path) {
			continue
		}
		hav := Require{Path: req.Mod.Path, Ver: req.Mod.Version}
		mod.Requires = append(mod.Requires, hav)
	}
	mod.Requires = sortRequires(mod.Requires)
	return mod, nil
}

// sortRequires returns the requirements sorted by module path then version
// with the duplicates removed.
func sortRequires(reqs []Require) []Require {
	slices.SortFunc(reqs, cmpRequire)
	return slices.Compact(reqs)
}
