// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package graph

// Cascade represents the modules to update, round by round, when a module
// changes.
type Cascade struct {
	// Rounds holds the module paths to update by round. The first round
	// holds the changed module alone, and the modules sharing a round may be
	// updated in any order. Every round is sorted by module path.
	Rounds [][]string

	// Skipped holds the sorted module paths of the dependents left out of
	// the rounds.
	Skipped []string
}
