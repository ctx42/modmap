// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package graph

import (
	"testing"

	"github.com/ctx42/testing/pkg/assert"
)

func Test_CycleError_Error(t *testing.T) {
	// --- Given ---
	err := &CycleError{Path: []string{"a", "b", "a"}}

	// --- When ---
	have := err.Error()

	// --- Then ---
	assert.Equal(t, "dependency cycle: a -> b -> a", have)
}

func Test_Graph_checkCycles(t *testing.T) {
	t.Run("acyclic graph", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{"a": {"b"}, "b": nil})

		// --- When ---
		err := grp.checkCycles()

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("diamond is not a cycle", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"a": {"b", "c"},
			"b": {"d"},
			"c": {"d"},
			"d": nil,
		})

		// --- When ---
		err := grp.checkCycles()

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error - cycle carries the path", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": {"a"},
		})

		// --- When ---
		err := grp.checkCycles()

		// --- Then ---
		want := &CycleError{Path: []string{"a", "b", "c", "a"}}
		assert.Equal(t, want, err)
	})

	t.Run("error - module requiring itself", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{"a": {"a"}})

		// --- When ---
		err := grp.checkCycles()

		// --- Then ---
		assert.Equal(t, &CycleError{Path: []string{"a", "a"}}, err)
	})
}
