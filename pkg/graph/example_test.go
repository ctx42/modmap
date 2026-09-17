// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package graph_test

import (
	"fmt"

	"github.com/ctx42/modmap/pkg/graph"
	"github.com/ctx42/modmap/pkg/mod"
)

func ExampleNew() {
	mods := map[string]*mod.Module{
		"example.com/app": {
			Path: "example.com/app",
			Requires: []mod.Require{
				{Path: "example.com/lib", Ver: "v1.0.0"},
			},
		},
		"example.com/lib": {
			Path: "example.com/lib",
			Requires: []mod.Require{
				{Path: "example.com/core", Ver: "v1.0.0"},
			},
		},
		"example.com/core": {Path: "example.com/core"},
	}

	grp, err := graph.New(mods)
	if err != nil {
		fmt.Println(err)
		return
	}

	for level, nodes := range grp.Levels {
		for _, nod := range nodes {
			fmt.Println(level, nod.Path, nod.Dependents)
		}
	}
	// Output:
	// 0 example.com/core [example.com/app example.com/lib]
	// 1 example.com/lib [example.com/app]
	// 2 example.com/app []
}

func ExampleCycleError() {
	mods := map[string]*mod.Module{
		"example.com/a": {
			Path: "example.com/a",
			Requires: []mod.Require{
				{Path: "example.com/b", Ver: "v1.0.0"},
			},
		},
		"example.com/b": {
			Path: "example.com/b",
			Requires: []mod.Require{
				{Path: "example.com/a", Ver: "v1.0.0"},
			},
		},
	}

	_, err := graph.New(mods)

	fmt.Println(err)
	// Output:
	// dependency cycle: example.com/a -> example.com/b -> example.com/a
}
