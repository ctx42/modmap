// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package graph

// Node represents a module drawn on the map.
type Node struct {
	// Path is the module path.
	Path string

	// Level is the level the module is drawn on. Level zero holds the
	// modules without dependencies.
	Level int

	// Deps holds the sorted module paths the module depends on directly.
	Deps []string

	// Dependents holds the sorted module paths of the modules reaching this
	// module, directly or through other modules. It is the set of modules
	// which have to be updated when this module changes.
	Dependents []string

	// leveled marks the node level as computed.
	leveled bool
}
