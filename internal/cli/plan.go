// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/modmap/internal/conf"
	"github.com/ctx42/modmap/pkg/mod"
)

// plan represents the update plan of the changed modules, printed as JSON.
type plan struct {
	// Module is the module path of the changed module when there is only
	// one, kept for the readers of a plan with a single changed module;
	// empty for more.
	Module string `json:"module,omitempty"`

	// Modules holds the sorted module paths of the changed modules.
	Modules []string `json:"modules"`

	// Rounds holds the modules to update by round. A changed module reaching
	// no other changed module is in the first round, so a single changed
	// module is alone there; every round is sorted by module path.
	Rounds [][]planMod `json:"rounds"`

	// Excluded holds the dependents the plan does not update, sorted by
	// module path: the ones a map finding them keeps out, and the ones read
	// from the module cache instead of a scanned directory.
	Excluded []planMod `json:"excluded"`

	// Lag holds, with the "--lag" option, the planned modules whose
	// requirements on other planned modules the update lifts, sorted by
	// module path.
	Lag []lag `json:"lag,omitempty"`
}

// planMod represents a module of the plan.
type planMod struct {
	// Path is the module path.
	Path string `json:"path"`

	// Map is the name of the map that found the module on disk. It is empty
	// for a module read from the module cache.
	Map string `json:"map,omitempty"`

	// Dir is the absolute path of the module directory. It is empty for a
	// module read from the module cache.
	Dir string `json:"dir,omitempty"`
}

// finder represents a map of the configuration file as the plan sees it.
type finder struct {
	name string     // Name of the map.
	dirs []string   // Scanned directories, symbolic links resolved.
	incl mod.Filter // Include globs of the map alone.
	flt  mod.Filter // Include and exclude globs of the map.
}

// holds reports whether one of the map directories holds dir.
func (fnd finder) holds(dir string) bool {
	for _, root := range fnd.dirs {
		if within(dir, root) {
			return true
		}
	}
	return false
}

// finders represents the maps of the configuration file, in the order the
// file declares them.
type finders []finder

// newFinders returns the finders of the maps. The directories are resolved
// the way the scanner resolves them, so the module directories it reports
// are found below them.
func newFinders(maps []conf.Map) finders {
	fnds := make(finders, 0, len(maps))
	for _, mp := range maps {
		fnd := finder{
			name: mp.Name,
			incl: mod.NewFilter(mp.Include, nil),
			flt:  mod.NewFilter(mp.Include, mp.Exclude),
		}
		for _, dir := range mp.Dirs {
			if res, err := filepath.EvalSymlinks(dir); err == nil {
				dir = res
			}
			fnd.dirs = append(fnd.dirs, dir)
		}
		fnds = append(fnds, fnd)
	}
	return fnds
}

// filter returns the filter passing every module path the include globs of
// any map pass. The exclude globs are left out, so a module a map keeps out
// is still found, and reported as excluded.
func (fnds finders) filter() mod.Filter {
	flts := make([]mod.Filter, 0, len(fnds))
	for _, fnd := range fnds {
		flts = append(flts, fnd.incl)
	}
	return mod.AnyOf(flts...)
}

// owner returns the name of the map deciding whether the module at pth, found
// in the directory dir, takes part in the plan, and whether it does. A map
// finds the module when one of its directories holds dir and its include
// globs pass pth. The module takes part when a map finds it and every map
// finding it keeps it, so one exclusion keeps it out. The name is the first
// map keeping it out, else the first map finding it, else the first map
// whose directories hold dir; it is empty for a module not on disk.
func (fnds finders) owner(pth, dir string) (string, bool) {
	if dir == "" {
		return "", false
	}
	var holder, found string
	for _, fnd := range fnds {
		if !fnd.holds(dir) {
			continue
		}
		if holder == "" {
			holder = fnd.name
		}
		if !fnd.incl.Match(pth) {
			continue
		}
		if !fnd.flt.Match(pth) {
			return fnd.name, false
		}
		if found == "" {
			found = fnd.name
		}
	}
	if found != "" {
		return found, true
	}
	return holder, false
}

// runPlan prints the update plan of the changed modules to stdout as JSON.
// The plan spans every map the configuration declares: the changed modules
// and every module depending on one of them, directly or through other
// modules. A dependent is planned when the maps finding it on disk keep it,
// and reported as excluded otherwise. Every changed module has to be planned.
func runPlan(
	ctx context.Context,
	rng *ring.Ring,
	cfg *config,
	fil *conf.Config,
) error {

	fnds := newFinders(fil.Maps)
	spc := combine(fil.Out, fil.Maps)
	spc.Filter = fnds.filter()
	mods, grp, err := build(ctx, rng, logger(rng), spc)
	if err != nil {
		return err
	}

	roots := slices.Sorted(slices.Values(cfg.plan))
	roots = slices.Compact(roots)
	for _, root := range roots {
		if _, ok := grp.Node(root); !ok {
			return fmt.Errorf("%w: %s", errModNotFound, root)
		}
		dir := mods[root].Dir
		if dir == "" {
			return fmt.Errorf("%w: %s", errModNotOnDisk, root)
		}
		// The scan finds modules on disk only below the map directories,
		// so one of the maps names the changed module.
		if name, ok := fnds.owner(root, dir); !ok {
			const format = "%w: %s (map: %s)"
			return fmt.Errorf(format, errModExcluded, root, name)
		}
	}

	keep := func(pth string) bool {
		_, ok := fnds.owner(pth, mods[pth].Dir)
		return ok
	}
	entry := func(pth string) planMod {
		name, _ := fnds.owner(pth, mods[pth].Dir)
		return planMod{Path: pth, Map: name, Dir: mods[pth].Dir}
	}
	cas := grp.CascadeFrom(roots, keep)
	pln := plan{
		Modules:  roots,
		Rounds:   make([][]planMod, 0, len(cas.Rounds)),
		Excluded: make([]planMod, 0, len(cas.Skipped)),
	}
	for _, rnd := range cas.Rounds {
		pms := make([]planMod, 0, len(rnd))
		for _, pth := range rnd {
			pms = append(pms, entry(pth))
		}
		pln.Rounds = append(pln.Rounds, pms)
	}
	for _, pth := range cas.Skipped {
		pln.Excluded = append(pln.Excluded, entry(pth))
	}
	if len(roots) == 1 {
		pln.Module = roots[0]
	}
	if cfg.lag {
		if pln.Lag, err = lags(ctx, mods, cas.Rounds); err != nil {
			return err
		}
	}

	enc := json.NewEncoder(rng.Stdout())
	enc.SetIndent("", "  ")
	if err = enc.Encode(pln); err != nil {
		return fmt.Errorf("write plan: %w", err)
	}
	return nil
}

// runModules prints to stdout as JSON the modules on disk the maps keep, as
// the plan names them, sorted by module path: every module a map finds in one
// of its directories and every map finding it keeps.
func runModules(ctx context.Context, rng *ring.Ring, fil *conf.Config) error {
	fnds := newFinders(fil.Maps)
	spc := combine(fil.Out, fil.Maps)
	spc.Filter = fnds.filter()
	mods, _, err := build(ctx, rng, logger(rng), spc)
	if err != nil {
		return err
	}
	pms := make([]planMod, 0, len(mods))
	for _, pth := range slices.Sorted(maps.Keys(mods)) {
		dir := mods[pth].Dir
		if name, ok := fnds.owner(pth, dir); ok {
			pms = append(pms, planMod{Path: pth, Map: name, Dir: dir})
		}
	}
	enc := json.NewEncoder(rng.Stdout())
	enc.SetIndent("", "  ")
	if err = enc.Encode(pms); err != nil {
		return fmt.Errorf("write modules: %w", err)
	}
	return nil
}
