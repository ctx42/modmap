// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package graph

import (
	"github.com/ctx42/modmap/pkg/mod"
)

// newMods returns the modules built from a map of a module path to the module
// paths it directly requires.
func newMods(deps map[string][]string) map[string]*mod.Module {
	mods := make(map[string]*mod.Module, len(deps))
	for pth, reqs := range deps {
		module := &mod.Module{Path: pth}
		for _, req := range reqs {
			hav := mod.Require{Path: req, Ver: "v1.0.0"}
			module.Requires = append(module.Requires, hav)
		}
		mods[pth] = module
	}
	return mods
}

// paths returns the module paths of the nodes.
func paths(nodes []*Node) []string {
	pths := make([]string, 0, len(nodes))
	for _, nod := range nodes {
		pths = append(pths, nod.Path)
	}
	return pths
}

// newGraph returns a graph with the nodes and edges built from a map of a
// module path to the module paths it depends on. It skips the level and
// dependents computation done by [New].
func newGraph(deps map[string][]string) *Graph {
	grp := &Graph{nodes: make(map[string]*Node, len(deps))}
	for pth := range deps {
		grp.nodes[pth] = &Node{Path: pth}
	}
	for pth, edges := range deps {
		for _, dep := range edges {
			if _, ok := grp.nodes[dep]; !ok {
				continue
			}
			nod := grp.nodes[pth]
			nod.Deps = append(nod.Deps, dep)
		}
	}
	return grp
}
