// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_run(t *testing.T) {
	t.Run("the map from the options is generated", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, "module example.com/a\n", src, "a")
		out := filepath.Join(t.TempDir(), "map.svg")
		cfg := &config{roots: []string{src}, out: out}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := run(context.Background(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "example.com/a", oskit.ReadFileStr(t, out))
		assert.Contain(t, "found example.com/a", tst.Stderr())
	})

	t.Run("every configured map is generated", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, "module example.com/a\n", src, "a")
		dst := t.TempDir()
		conf := oskit.Create(t, ""+
			"maps:\n"+
			"  - name: one\n"+
			"    dirs: ["+src+"]\n"+
			"    out: one.svg\n"+
			"  - name: two\n"+
			"    dirs: ["+src+"]\n"+
			"    out: two.svg\n", dst, "modmap.yaml")
		cfg := &config{conf: conf}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := run(context.Background(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, oskit.PathExists(t, dst, "one.svg"))
		assert.True(t, oskit.PathExists(t, dst, "two.svg"))
		assert.Contain(t, "found example.com/a", tst.Stderr())
	})

	t.Run("only the named map is generated", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, "module example.com/a\n", src, "a")
		dst := t.TempDir()
		conf := oskit.Create(t, ""+
			"maps:\n"+
			"  - name: one\n"+
			"    dirs: ["+src+"]\n"+
			"    out: one.svg\n"+
			"  - name: two\n"+
			"    dirs: ["+src+"]\n"+
			"    out: two.svg\n", dst, "modmap.yaml")
		cfg := &config{conf: conf, names: []string{"two"}}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := run(context.Background(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, oskit.PathExists(t, dst, "one.svg"))
		assert.True(t, oskit.PathExists(t, dst, "two.svg"))
		assert.Contain(t, "found example.com/a", tst.Stderr())
	})

	t.Run("error - unknown map name", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		dst := t.TempDir()
		conf := oskit.Create(t, ""+
			"maps:\n"+
			"  - name: one\n"+
			"    dirs: ["+src+"]\n"+
			"    out: one.svg\n", dst, "modmap.yaml")
		cfg := &config{conf: conf, names: []string{"three"}}
		rng := ringtest.New(t).Ring()

		// --- When ---
		err := run(context.Background(), rng, cfg)

		// --- Then ---
		assert.ErrorContain(t, "unknown map: three", err)
	})

	t.Run("error - configuration cannot be read", func(t *testing.T) {
		// --- Given ---
		cfg := &config{conf: filepath.Join(t.TempDir(), "nope.yaml")}
		rng := ringtest.New(t).Ring()

		// --- When ---
		err := run(context.Background(), rng, cfg)

		// --- Then ---
		assert.ErrorContain(t, "read configuration", err)
	})

	t.Run("error - a map fails to generate", func(t *testing.T) {
		// --- Given ---
		dst := t.TempDir()
		gone := filepath.Join(t.TempDir(), "gone")
		conf := oskit.Create(t, ""+
			"maps:\n"+
			"  - name: one\n"+
			"    dirs: ["+gone+"]\n"+
			"    out: one.svg\n", dst, "modmap.yaml")
		cfg := &config{conf: conf}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := run(context.Background(), rng, cfg)

		// --- Then ---
		assert.ErrorContain(t, "map one: scan "+gone, err)
		assert.Contain(t, "scanning "+gone, tst.Stderr())
	})
}

func Test_generate(t *testing.T) {
	t.Run("levels follow the dependencies", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, ""+
			"module example.com/a\n"+
			"require example.com/b v1.0.0\n", src, "a")
		writeMod(t, "module example.com/b\n", src, "b")
		out := filepath.Join(t.TempDir(), "map.svg")
		spc := spec{Dirs: []string{src}, Out: out}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := generate(context.Background(), rng, spc, false)

		// --- Then ---
		assert.NoError(t, err)
		doc := oskit.ReadFileStr(t, out)
		wDep := `data-module="example.com/b" data-level="0"` +
			` data-dependents="example.com/a"`
		assert.Contain(t, wDep, doc)
		wTop := `data-module="example.com/a" data-level="1"`
		assert.Contain(t, wTop, doc)

		want := "graph has 2 modules, widest level 1"
		assert.Contain(t, want, tst.Stderr())
	})

	t.Run("filtered modules are not drawn", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, "module example.com/a\n", src, "a")
		writeMod(t, "module other.com/b\n", src, "b")
		out := filepath.Join(t.TempDir(), "map.svg")
		spc := spec{
			Dirs:    []string{src},
			Include: []string{"example.com/*"},
			Out:     out,
		}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := generate(context.Background(), rng, spc, false)

		// --- Then ---
		assert.NoError(t, err)
		doc := oskit.ReadFileStr(t, out)
		assert.Contain(t, "example.com/a", doc)
		assert.NotContain(t, "other.com/b", doc)

		assert.Contain(t, "graph has 1 modules", tst.Stderr())
	})

	t.Run("error - dependency cycle", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, ""+
			"module example.com/a\n"+
			"require example.com/b v1.0.0\n", src, "a")
		writeMod(t, ""+
			"module example.com/b\n"+
			"require example.com/a v1.0.0\n", src, "b")
		out := filepath.Join(t.TempDir(), "map.svg")
		spc := spec{Dirs: []string{src}, Out: out}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := generate(context.Background(), rng, spc, false)

		// --- Then ---
		want := "dependency cycle: example.com/a -> example.com/b"
		assert.ErrorContain(t, want, err)
		assert.False(t, oskit.PathExists(t, out))

		assert.Contain(t, "scanning "+src, tst.Stderr())
	})

	t.Run("error - output cannot be written", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, "module example.com/a\n", src, "a")
		out := filepath.Join(t.TempDir(), "gone", "map.svg")
		spc := spec{Dirs: []string{src}, Out: out}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := generate(context.Background(), rng, spc, false)

		// --- Then ---
		assert.ErrorContain(t, "create map file", err)

		assert.Contain(t, "graph has 1 modules", tst.Stderr())
	})
}
