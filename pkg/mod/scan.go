// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mod

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
)

// skipDirs holds the names of the directories the scan never descends into.
var skipDirs = []string{".git", "testdata", "vendor"}

// Scanner walks directory trees collecting the Go modules they hold.
type Scanner struct {
	flt  Filter                           // Module path filter.
	logf func(format string, args ...any) // Progress reporter.
}

// NewScanner returns a scanner keeping only the modules passing flt and
// reporting its progress with logf. A nil logf discards progress messages.
func NewScanner(flt Filter, logf func(format string, args ...any)) *Scanner {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Scanner{flt: flt, logf: logf}
}

// Scan walks the roots and returns the modules passing the filter keyed by
// their module path. Every go.mod file found is a module, nested ones
// included, but the directories named in skipDirs are never descended into.
// When more than one root holds the same module path, the first one found
// wins.
func (scn *Scanner) Scan(roots []string) (map[string]*Module, error) {
	mods := make(map[string]*Module)
	for _, root := range roots {
		scn.logf("scanning %s", root)
		if err := scn.walk(root, mods); err != nil {
			return nil, err
		}
	}
	return mods, nil
}

// walk adds every module found under the root to mods.
func (scn *Scanner) walk(root string, mods map[string]*Module) error {
	walker := func(pth string, ent fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ent.IsDir() {
			skip := slices.Contains(skipDirs, ent.Name())
			if pth != root && skip {
				return fs.SkipDir
			}
			return nil
		}
		if ent.Name() != "go.mod" {
			return nil
		}
		mod, err := parseMod(pth, scn.flt)
		if err != nil {
			return err
		}
		if !scn.flt.Match(mod.Path) {
			return nil
		}
		if _, ok := mods[mod.Path]; ok {
			return nil
		}
		mod.Dir = filepath.Dir(pth)
		mods[mod.Path] = mod
		scn.logf("found %s", mod.Path)
		return nil
	}
	if err := filepath.WalkDir(root, walker); err != nil {
		return fmt.Errorf("scan %s: %w", root, err)
	}
	return nil
}
