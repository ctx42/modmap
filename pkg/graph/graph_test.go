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

	t.Run("nil module has no dependencies", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{"a": {"b"}, "b": nil})
		mods["b"] = nil

		// --- When ---
		have, err := New(mods)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, 2, have.Len())
		assert.Equal(t, 0, have.nodes["b"].Level)
		assert.Equal(t, 1, have.nodes["a"].Level)
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
		assert.Equal(t, &CycleError{Path: []string{"a", "b", "a"}}, err)
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

func Test_Graph_Cascade(t *testing.T) {
	keepAll := func(string) bool { return true }
	skipW := func(pth string) bool { return pth != "w" }

	t.Run("the changed module comes first", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{
			"a": {"b"},
			"b": {"c"},
			"c": nil,
		})
		grp := must.Value(New(mods))

		// --- When ---
		have := grp.Cascade("c", keepAll)

		// --- Then ---
		want := &Cascade{Rounds: [][]string{{"c"}, {"b"}, {"a"}}}
		assert.Equal(t, want, have)
	})

	t.Run("independent modules share a sorted round", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{
			"z": {"p"},
			"a": {"p"},
			"c": {"a", "z", "p"},
			"p": nil,
		})
		grp := must.Value(New(mods))

		// --- When ---
		have := grp.Cascade("p", keepAll)

		// --- Then ---
		want := &Cascade{Rounds: [][]string{{"p"}, {"a", "z"}, {"c"}}}
		assert.Equal(t, want, have)
	})

	t.Run("a rejected dependent is skipped", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{
			"a": {"t"},
			"w": {"t"},
			"t": nil,
		})
		grp := must.Value(New(mods))

		// --- When ---
		have := grp.Cascade("t", skipW)

		// --- Then ---
		want := &Cascade{
			Rounds:  [][]string{{"t"}, {"a"}},
			Skipped: []string{"w"},
		}
		assert.Equal(t, want, have)
	})

	t.Run("reaching the change through a skipped module", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{
			"a": {"w"},
			"w": {"t"},
			"t": nil,
		})
		grp := must.Value(New(mods))

		// --- When ---
		have := grp.Cascade("t", skipW)

		// --- Then ---
		want := &Cascade{
			Rounds:  [][]string{{"t"}, {"a"}},
			Skipped: []string{"w"},
		}
		assert.Equal(t, want, have)
	})

	t.Run("a skipped module leaves no empty round", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{
			"a": {"w", "b"},
			"w": {"b"},
			"b": {"t"},
			"t": nil,
		})
		grp := must.Value(New(mods))

		// --- When ---
		have := grp.Cascade("t", skipW)

		// --- Then ---
		want := &Cascade{
			Rounds:  [][]string{{"t"}, {"b"}, {"a"}},
			Skipped: []string{"w"},
		}
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
		have := grp.Cascade("lib", keepAll)

		// --- Then ---
		want := &Cascade{Rounds: [][]string{{"lib"}, {"app"}}}
		assert.Equal(t, want, have)
	})

	t.Run("a module without dependents is alone", func(t *testing.T) {
		// --- Given ---
		grp := must.Value(New(newMods(map[string][]string{"a": {"b"}})))

		// --- When ---
		have := grp.Cascade("a", keepAll)

		// --- Then ---
		assert.Equal(t, &Cascade{Rounds: [][]string{{"a"}}}, have)
	})

	t.Run("nil keep keeps every dependent", func(t *testing.T) {
		// --- Given ---
		mods := newMods(map[string][]string{"a": {"b"}, "b": nil})
		grp := must.Value(New(mods))

		// --- When ---
		have := grp.Cascade("b", nil)

		// --- Then ---
		assert.Equal(t, &Cascade{Rounds: [][]string{{"b"}, {"a"}}}, have)
	})

	t.Run("unknown module", func(t *testing.T) {
		// --- Given ---
		grp := must.Value(New(newMods(map[string][]string{"a": nil})))

		// --- When ---
		have := grp.Cascade("b", keepAll)

		// --- Then ---
		assert.Nil(t, have)
	})

	t.Run("rounds agree with the update order", func(t *testing.T) {
		// --- Given ---
		deps := loadDeps(t, "testdata/ctx42_modules.json")
		grp := must.Value(New(newMods(deps)))

		// --- When ---
		have := make(map[string]*Cascade, grp.Len())
		for _, pth := range grp.paths() {
			have[pth] = grp.Cascade(pth, keepAll)
		}

		// --- Then ---
		assert.Len(t, grp.Len(), have)
		for pin, cas := range have {
			ord := grp.Order(pin)
			var cnt int
			for idx, rnd := range cas.Rounds {
				for _, pth := range rnd {
					assert.Equal(t, idx+1, ord[pth])
					cnt++
				}
			}
			assert.Equal(t, len(ord), cnt)
			assert.Nil(t, cas.Skipped)
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

func Test_Graph_paths(t *testing.T) {
	// --- Given ---
	grp := newGraph(map[string][]string{"c": nil, "a": nil, "b": nil})

	// --- When ---
	have := grp.paths()

	// --- Then ---
	assert.Equal(t, []string{"a", "b", "c"}, have)
}

func Test_Graph_setLevels(t *testing.T) {
	t.Run("chain", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{"a": {"b"}, "b": {"c"}, "c": nil})

		// --- When ---
		grp.setLevels()

		// --- Then ---
		assert.Equal(t, 2, grp.nodes["a"].Level)
		assert.Equal(t, 1, grp.nodes["b"].Level)
		assert.Equal(t, 0, grp.nodes["c"].Level)
	})

	t.Run("diamond", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"a": {"b", "c"},
			"b": {"d"},
			"c": {"d"},
			"d": nil,
		})

		// --- When ---
		grp.setLevels()

		// --- Then ---
		assert.Equal(t, 2, grp.nodes["a"].Level)
		assert.Equal(t, 1, grp.nodes["b"].Level)
		assert.Equal(t, 1, grp.nodes["c"].Level)
		assert.Equal(t, 0, grp.nodes["d"].Level)
	})
}

func Test_Graph_setDependents(t *testing.T) {
	// --- Given ---
	grp := newGraph(map[string][]string{
		"a": {"b", "c"},
		"b": {"d"},
		"c": {"d"},
		"d": nil,
	})

	// --- When ---
	grp.setDependents()

	// --- Then ---
	assert.Empty(t, grp.nodes["a"].Dependents)
	assert.Equal(t, []string{"a"}, grp.nodes["b"].Dependents)
	assert.Equal(t, []string{"a"}, grp.nodes["c"].Dependents)
	assert.Equal(t, []string{"a", "b", "c"}, grp.nodes["d"].Dependents)
}

func Test_Graph_group(t *testing.T) {
	t.Run("levels sorted by path", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{"c": nil, "a": nil, "b": nil})
		grp.nodes["b"].Level = 1

		// --- When ---
		grp.group()

		// --- Then ---
		assert.Len(t, 2, grp.Levels)
		want := []*Node{grp.nodes["a"], grp.nodes["c"]}
		assert.Equal(t, want, grp.Levels[0])
		assert.Equal(t, []*Node{grp.nodes["b"]}, grp.Levels[1])
	})

	t.Run("empty graph", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(nil)

		// --- When ---
		grp.group()

		// --- Then ---
		assert.Nil(t, grp.Levels)
	})
}
