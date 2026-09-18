// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package graph

import (
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
)

func Test_New(t *testing.T) {
	t.Run("levels grow with the dependencies", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": nil,
		})

		// --- When ---
		have, err := New(mods)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 0, have.nodes["c"].Level)
		assert.Equal(t, 1, have.nodes["b"].Level)
		assert.Equal(t, 2, have.nodes["a"].Level)
	})

	t.Run("the longest path decides the level", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{
			"a": {"b", "c"},
			"b": {"c"},
			"c": nil,
		})

		// --- When ---
		have, err := New(mods)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 2, have.nodes["a"].Level)
	})

	t.Run("levels are sorted by module path", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{
			"example.com/c": nil,
			"example.com/a": nil,
			"example.com/b": nil,
		})

		// --- When ---
		have, err := New(mods)

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 1, have.Levels)
		want := []string{
			"example.com/a",
			"example.com/b",
			"example.com/c",
		}
		assert.Equal(t, want, paths(have.Levels[0]))
	})

	t.Run("dependents are transitive", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": nil,
			"d": nil,
		})

		// --- When ---
		have, err := New(mods)

		// --- Then ---
		assert.NoError(t, err)
		wDeps := []string{"a", "b"}
		assert.Equal(t, wDeps, have.nodes["c"].Dependents)
		assert.Equal(t, []string{"a"}, have.nodes["b"].Dependents)
		assert.Empty(t, have.nodes["a"].Dependents)
		assert.Empty(t, have.nodes["d"].Dependents)
	})

	t.Run("dependencies outside the graph are ignored", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{"a": {"gone"}})

		// --- When ---
		have, err := New(mods)

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, have.nodes["a"].Deps)
		assert.Equal(t, 0, have.nodes["a"].Level)
	})

	t.Run("no modules give no levels", func(t *testing.T) {
		// --- When ---
		have, err := New(nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, have.Levels)
		assert.Equal(t, 0, have.Len())
	})

	t.Run("error - dependency cycle", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{
			"a": {"b"},
			"b": {"a"},
		})

		// --- When ---
		have, err := New(mods)

		// --- Then ---
		assert.ErrorContain(t, "dependency cycle: a -> b -> a", err)
		assert.Nil(t, have)
	})
}

func Test_Graph_Node(t *testing.T) {
	t.Run("known module", func(t *testing.T) {
		// --- Given ---
		grp := must.Value(New(newMods(map[string][]string{"a": nil})))

		// --- When ---
		have, ok := grp.Node("a")

		// --- Then ---
		assert.True(t, ok)
		assert.Equal(t, "a", have.Path)
	})

	t.Run("unknown module", func(t *testing.T) {
		// --- Given ---
		grp := must.Value(New(newMods(map[string][]string{"a": nil})))

		// --- When ---
		have, ok := grp.Node("b")

		// --- Then ---
		assert.False(t, ok)
		assert.Nil(t, have)
	})
}

func Test_Graph_Len(t *testing.T) {
	// --- Given ---
	mods := newMods(map[string][]string{"a": {"b"}, "b": nil})
	grp := must.Value(New(mods))

	// --- When ---
	have := grp.Len()

	// --- Then ---
	assert.Equal(t, 2, have)
}

func Test_Graph_Order(t *testing.T) {
	t.Run("the changed module comes first", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": nil,
		})
		grp := must.Value(New(mods))

		// --- When ---
		have := grp.Order("c")

		// --- Then ---
		want := map[string]int{"c": 1, "b": 2, "a": 3}
		assert.Equal(t, want, have)
	})

	t.Run("the longest path wins over a shortcut", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{
			"a": {"p"},
			"b": {"p"},
			"c": {"a", "b", "p"},
			"p": nil,
		})
		grp := must.Value(New(mods))

		// --- When ---
		have := grp.Order("p")

		// --- Then ---
		want := map[string]int{"p": 1, "a": 2, "b": 2, "c": 3}
		assert.Equal(t, want, have)
	})

	t.Run("the modules the change relies on are left out", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{
			"app":  {"lib"},
			"lib":  {"core"},
			"core": nil,
		})
		grp := must.Value(New(mods))

		// --- When ---
		have := grp.Order("lib")

		// --- Then ---
		assert.Equal(t, map[string]int{"lib": 1, "app": 2}, have)
	})

	t.Run("a module without dependents is alone", func(t *testing.T) {
		// --- Given ---
		grp := must.Value(New(newMods(map[string][]string{"a": {"b"}})))

		// --- When ---
		have := grp.Order("a")

		// --- Then ---
		assert.Equal(t, map[string]int{"a": 1}, have)
	})

	t.Run("repeated calls agree", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{"a": {"b"}, "b": nil})
		grp := must.Value(New(mods))

		// --- When ---
		have := grp.Order("b")

		// --- Then ---
		assert.Equal(t, grp.Order("b"), have)
	})

	t.Run("unknown module", func(t *testing.T) {
		// --- Given ---
		grp := must.Value(New(newMods(map[string][]string{"a": nil})))

		// --- When ---
		have := grp.Order("b")

		// --- Then ---
		assert.Nil(t, have)
	})

	t.Run("every module comes after the ones it relies on", func(t *testing.T) {
		// --- Given ---
		deps := loadDeps(t, "testdata/ctx42_modules.json")
		grp := must.Value(New(newMods(deps)))

		// --- When ---
		have := make(map[string]map[string]int, grp.Len())
		for _, pth := range grp.paths() {
			have[pth] = grp.Order(pth)
		}

		// --- Then ---
		assert.Len(t, grp.Len(), have)
		for pin, ord := range have {
			assert.Equal(t, 1, ord[pin])
			for pth, num := range ord {
				nod, _ := grp.Node(pth)
				for _, dep := range nod.Deps {
					if _, ok := ord[dep]; !ok {
						continue
					}
					assert.Greater(t, ord[dep], num)
				}
			}
		}
	})
}

func Test_Graph_Widest(t *testing.T) {
	t.Run("the busiest level counts", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{
			"a": {"b"},
			"b": nil,
			"c": nil,
			"d": nil,
		})
		grp := must.Value(New(mods))

		// --- When ---
		have := grp.Widest()

		// --- Then ---
		assert.Equal(t, 3, have)
	})

	t.Run("empty graph", func(t *testing.T) {
		// --- Given ---
		grp := must.Value(New(nil))

		// --- When ---
		have := grp.Widest()

		// --- Then ---
		assert.Equal(t, 0, have)
	})
}
