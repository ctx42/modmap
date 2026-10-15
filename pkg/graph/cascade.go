// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package graph

// Cascade represents the modules to update, round by round, when modules
// change.
type Cascade struct {
	// Rounds holds the module paths to update by round. A changed module
	// reaching no other changed module is in the first round, so a single
	// changed module is alone there. The modules sharing a round may be
	// updated in any order, and every round is sorted by module path.
	Rounds [][]string

	// Skipped holds the sorted module paths of the dependents left out of
	// the rounds.
	Skipped []string
}
