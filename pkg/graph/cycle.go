// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package graph

import (
	"slices"
	"strings"
)

// Node colors used while looking for cycles.
const (
	white = iota // Not visited yet.
	gray         // On the current path.
	black        // Visited, with every dependency visited.
)

// CycleError represents a dependency cycle in the module graph.
type CycleError struct {
	// Path holds the module paths forming the cycle with the first module
	// repeated as the last one.
	Path []string
}

func (err *CycleError) Error() string {
	return "dependency cycle: " + strings.Join(err.Path, " -> ")
}

// checkCycles returns [CycleError] for the first dependency cycle it finds.
func (grp *Graph) checkCycles() error {
	state := make(map[string]int, len(grp.nodes))
	var stack []string

	var visit func(pth string) error
	visit = func(pth string) error {
		state[pth] = gray
		stack = append(stack, pth)
		for _, dep := range grp.nodes[pth].Deps {
			switch state[dep] {
			case gray:
				idx := slices.Index(stack, dep)
				cyc := append(slices.Clone(stack[idx:]), dep)
				return &CycleError{Path: cyc}

			case white:
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[pth] = black
		return nil
	}

	for _, pth := range grp.paths() {
		if state[pth] != white {
			continue
		}
		if err := visit(pth); err != nil {
			return err
		}
	}
	return nil
}
