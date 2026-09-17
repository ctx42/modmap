// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package graph turns discovered Go modules into the layered graph the map is
// drawn from: every module sits one level above the modules it depends on.
package graph

import (
	"maps"
	"slices"

	"github.com/ctx42/modmap/pkg/mod"
)

// Graph represents the layered module dependency graph.
type Graph struct {
	// Levels holds the nodes by level. Index zero holds the modules without
	// dependencies, and every level is sorted by module path.
	Levels [][]*Node

	// nodes holds every node keyed by module path.
	nodes map[string]*Node
}

// New returns the graph of the given modules. A dependency pointing outside
// mods is ignored, so the graph never carries a module the filter dropped. It
// returns [CycleError] when the modules depend on each other in a circle.
func New(mods map[string]*mod.Module) (*Graph, error) {
	grp := &Graph{nodes: make(map[string]*Node, len(mods))}
	for pth := range mods {
		grp.nodes[pth] = &Node{Path: pth}
	}
	for pth, module := range mods {
		nod := grp.nodes[pth]
		for _, dep := range module.Deps() {
			if _, ok := grp.nodes[dep]; !ok {
				continue
			}
			nod.Deps = append(nod.Deps, dep)
		}
	}
	if err := grp.checkCycles(); err != nil {
		return nil, err
	}
	grp.setLevels()
	grp.setDependents()
	grp.group()
	return grp, nil
}

// Node returns the node of the module path and whether the graph has it.
func (grp *Graph) Node(pth string) (*Node, bool) {
	nod, ok := grp.nodes[pth]
	return nod, ok
}

// Len returns the number of modules in the graph.
func (grp *Graph) Len() int { return len(grp.nodes) }

// Widest returns the number of modules on the level holding the most of them.
func (grp *Graph) Widest() int {
	var widest int
	for _, lvl := range grp.Levels {
		widest = max(widest, len(lvl))
	}
	return widest
}

// paths returns the sorted module paths of every node in the graph.
func (grp *Graph) paths() []string {
	return slices.Sorted(maps.Keys(grp.nodes))
}

// setLevels assigns every node the level one above the highest level of the
// modules it depends on.
func (grp *Graph) setLevels() {
	var level func(pth string) int
	level = func(pth string) int {
		nod := grp.nodes[pth]
		if nod.leveled {
			return nod.Level
		}
		for _, dep := range nod.Deps {
			nod.Level = max(nod.Level, level(dep)+1)
		}
		nod.leveled = true
		return nod.Level
	}
	for _, pth := range grp.paths() {
		_ = level(pth)
	}
}

// setDependents assigns every node the sorted module paths of the modules
// reaching it, directly or through other modules.
func (grp *Graph) setDependents() {
	rdep := make(map[string][]string, len(grp.nodes))
	for _, pth := range grp.paths() {
		for _, dep := range grp.nodes[pth].Deps {
			rdep[dep] = append(rdep[dep], pth)
		}
	}
	for pth, nod := range grp.nodes {
		seen := make(map[string]bool)
		queue := slices.Clone(rdep[pth])
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			if seen[cur] {
				continue
			}
			seen[cur] = true
			queue = append(queue, rdep[cur]...)
		}
		delete(seen, pth)
		nod.Dependents = slices.Sorted(maps.Keys(seen))
	}
}

// group fills the levels with the nodes, each level sorted by module path.
func (grp *Graph) group() {
	if len(grp.nodes) == 0 {
		return
	}
	var top int
	for _, nod := range grp.nodes {
		top = max(top, nod.Level)
	}
	grp.Levels = make([][]*Node, top+1)
	for _, pth := range grp.paths() {
		nod := grp.nodes[pth]
		grp.Levels[nod.Level] = append(grp.Levels[nod.Level], nod)
	}
}
