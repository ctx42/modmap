// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/iokit"
	"github.com/ctx42/testkit/pkg/oskit"

	"github.com/ctx42/modmap/internal/conf"
)

func Test_runPlan(t *testing.T) {
	t.Run("dependents outside the map are excluded", func(t *testing.T) {
		// --- Given ---
		cnf, src := planMaps(t)
		fil := must.Value(conf.Load(cnf))
		names := []string{"ctx42"}
		cfg := &config{plan: "github.com/ctx42/testing", names: names}
		tst := ringtest.New(t).WetStdout().WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.NoError(t, err)
		want := plan{
			Module: "github.com/ctx42/testing",
			Map:    "ctx42",
			Rounds: [][]planMod{
				{
					{
						Path: "github.com/ctx42/testing",
						Dir:  filepath.Join(src, "ctx42", "testing"),
					},
				},
				{
					{
						Path: "github.com/ctx42/a",
						Dir:  filepath.Join(src, "ctx42", "a"),
					},
					{
						Path: "github.com/ctx42/d",
						Dir:  filepath.Join(src, "ctx42", "d"),
					},
				},
			},
			Excluded: []planMod{
				{
					Path: "github.com/customer/b",
					Dir:  filepath.Join(src, "customer", "b"),
				},
			},
		}
		wJSON := must.Value(json.MarshalIndent(want, "", "  "))
		assert.Equal(t, string(wJSON)+"\n", tst.Stdout())

		assert.Contain(t, "graph has 4 modules", tst.Stderr())
	})

	t.Run("two runs print the same plan", func(t *testing.T) {
		// --- Given ---
		cnf, _ := planMaps(t)
		fil := must.Value(conf.Load(cnf))
		names := []string{"ctx42"}
		cfg := &config{plan: "github.com/ctx42/testing", names: names}
		tst := ringtest.New(t).WetStdout().WetStderr()
		rng := tst.Ring()
		must.Nil(runPlan(t.Context(), rng, cfg, fil))
		first := tst.Stdout()
		tst.ResetStdout()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, first, tst.Stdout())

		assert.Contain(t, "graph has 4 modules", tst.Stderr())
	})

	t.Run("a module without dependents is alone", func(t *testing.T) {
		// --- Given ---
		cnf, src := planMaps(t)
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "github.com/ctx42/a", names: []string{"ctx42"}}
		tst := ringtest.New(t).WetStdout().WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.NoError(t, err)
		have := plan{}
		must.Nil(json.Unmarshal([]byte(tst.Stdout()), &have))
		wDir := filepath.Join(src, "ctx42", "a")
		want := plan{
			Module:   "github.com/ctx42/a",
			Map:      "ctx42",
			Rounds:   [][]planMod{{{Path: "github.com/ctx42/a", Dir: wDir}}},
			Excluded: []planMod{},
		}
		assert.Equal(t, want, have)
		assert.Contain(t, `"excluded": []`, tst.Stdout())

		assert.Contain(t, "graph has 4 modules", tst.Stderr())
	})

	t.Run("a dependent from the module cache is excluded", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, "module example.com/t\n", src, "t")
		writeMod(t, ""+
			"module example.com/a\n"+
			"require example.com/c v1.0.0\n", src, "a")
		cnf := oskit.Create(t, ""+
			"out: all.svg\n"+
			"maps:\n"+
			"  - name: one\n"+
			"    dirs: ["+src+"]\n", t.TempDir(), "modmap.yaml")
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "example.com/t", names: []string{"one"}}

		env := proxyEnv(t, map[string]string{
			"example.com/c": "" +
				"module example.com/c\n" +
				"require example.com/t v1.0.0\n",
		})
		tst := ringtest.New(t, ring.WithEnv(env)).WetStdout().WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.NoError(t, err)
		have := plan{}
		must.Nil(json.Unmarshal([]byte(tst.Stdout()), &have))
		want := [][]planMod{
			{{Path: "example.com/t", Dir: filepath.Join(src, "t")}},
			{{Path: "example.com/a", Dir: filepath.Join(src, "a")}},
		}
		assert.Equal(t, want, have.Rounds)
		assert.Equal(t, []planMod{{Path: "example.com/c"}}, have.Excluded)

		assert.Contain(t, "resolving example.com/c@v1.0.0", tst.Stderr())
	})

	t.Run("error - unknown map name", func(t *testing.T) {
		// --- Given ---
		cnf, _ := planMaps(t)
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "github.com/ctx42/a", names: []string{"three"}}
		rng := ringtest.New(t).Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.ErrorIs(t, conf.ErrUnkMap, err)
	})

	t.Run("error - a map directory cannot be scanned", func(t *testing.T) {
		// --- Given ---
		gone := filepath.Join(t.TempDir(), "gone")
		cnf := oskit.Create(t, ""+
			"out: all.svg\n"+
			"maps:\n"+
			"  - name: one\n"+
			"    dirs: ["+gone+"]\n", t.TempDir(), "modmap.yaml")
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "example.com/a", names: []string{"one"}}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.ErrorContain(t, "scan "+gone, err)
		assert.Contain(t, "scanning "+gone, tst.Stderr())
	})

	t.Run("error - module not found", func(t *testing.T) {
		// --- Given ---
		cnf, _ := planMaps(t)
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "github.com/ctx42/nope", names: []string{"ctx42"}}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.ErrorIs(t, errModNotFound, err)
		assert.ErrorContain(t, "module not found: github.com/ctx42/nope", err)
		assert.Contain(t, "graph has 4 modules", tst.Stderr())
	})

	t.Run("error - module outside the map", func(t *testing.T) {
		// --- Given ---
		cnf, _ := planMaps(t)
		fil := must.Value(conf.Load(cnf))
		names := []string{"work"}
		cfg := &config{plan: "github.com/ctx42/testing", names: names}
		rng := ringtest.New(t).Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		want := "module not in the map: github.com/ctx42/testing (map: work)"
		assert.ErrorEqual(t, want, err)
	})

	t.Run("error - module from the module cache", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, ""+
			"module example.com/a\n"+
			"require example.com/c v1.0.0\n", src, "a")
		cnf := oskit.Create(t, ""+
			"out: all.svg\n"+
			"maps:\n"+
			"  - name: one\n"+
			"    dirs: ["+src+"]\n", t.TempDir(), "modmap.yaml")
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "example.com/c", names: []string{"one"}}

		env := proxyEnv(t, map[string]string{
			"example.com/c": "module example.com/c\n",
		})
		tst := ringtest.New(t, ring.WithEnv(env)).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.ErrorIs(t, errModNotOnDisk, err)
		assert.ErrorContain(t, "on disk: example.com/c", err)
		assert.Contain(t, "resolving example.com/c@v1.0.0", tst.Stderr())
	})

	t.Run("error - plan cannot be written", func(t *testing.T) {
		// --- Given ---
		cnf, _ := planMaps(t)
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "github.com/ctx42/a", names: []string{"ctx42"}}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.SetStdout(iokit.ErrWriter(&bytes.Buffer{}, 0))

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.ErrorContain(t, "write plan: ", err)
		assert.Contain(t, "graph has 4 modules", tst.Stderr())
	})
}
