// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/modmap/internal/conf"
	"github.com/ctx42/modmap/pkg/mod"
)

// plan represents the update plan of a changed module, printed as JSON.
type plan struct {
	// Module is the module path of the changed module.
	Module string `json:"module"`

	// Map is the name of the map the plan is limited to.
	Map string `json:"map"`

	// Rounds holds the modules to update by round. The first round holds
	// the changed module alone, and every round is sorted by module path.
	Rounds [][]planMod `json:"rounds"`

	// Excluded holds the dependents the plan does not update, sorted by
	// module path: the ones outside the map, and the ones read from the
	// module cache instead of a scanned directory.
	Excluded []planMod `json:"excluded"`
}

// planMod represents a module of the plan.
type planMod struct {
	// Path is the module path.
	Path string `json:"path"`

	// Dir is the absolute path of the module directory. It is empty for a
	// module read from the module cache.
	Dir string `json:"dir,omitempty"`
}

// runPlan prints the update plan of the changed module to stdout as JSON.
// The directories of every map the configuration declares are scanned, so
// the dependents outside the map the plan is limited to are found and
// reported as excluded. Only the dependents the map keeps and found in a
// scanned directory are planned.
func runPlan(
	ctx context.Context,
	rng *ring.Ring,
	cfg *config,
	fil *conf.Config,
) error {

	sel, err := fil.Select(cfg.names)
	if err != nil {
		return err
	}
	mp := sel[0]
	flt := mod.NewFilter(mp.Include, mp.Exclude)
	if !flt.Match(cfg.plan) {
		const format = "%w: %s (map: %s)"
		return fmt.Errorf(format, errModNotInMap, cfg.plan, mp.Name)
	}
	mods, grp, err := build(ctx, rng, logger(rng), combine(fil.Out, fil.Maps))
	if err != nil {
		return err
	}

	keep := func(pth string) bool {
		return flt.Match(pth) && mods[pth].Dir != ""
	}
	cas := grp.Cascade(cfg.plan, keep)
	if cas == nil {
		return fmt.Errorf("%w: %s", errModNotFound, cfg.plan)
	}
	if mods[cfg.plan].Dir == "" {
		return fmt.Errorf("%w: %s", errModNotOnDisk, cfg.plan)
	}

	pln := plan{
		Module:   cfg.plan,
		Map:      mp.Name,
		Rounds:   make([][]planMod, 0, len(cas.Rounds)),
		Excluded: make([]planMod, 0, len(cas.Skipped)),
	}
	for _, rnd := range cas.Rounds {
		pms := make([]planMod, 0, len(rnd))
		for _, pth := range rnd {
			pms = append(pms, planMod{Path: pth, Dir: mods[pth].Dir})
		}
		pln.Rounds = append(pln.Rounds, pms)
	}
	for _, pth := range cas.Skipped {
		pm := planMod{Path: pth, Dir: mods[pth].Dir}
		pln.Excluded = append(pln.Excluded, pm)
	}

	enc := json.NewEncoder(rng.Stdout())
	enc.SetIndent("", "  ")
	if err = enc.Encode(pln); err != nil {
		return fmt.Errorf("write plan: %w", err)
	}
	return nil
}
