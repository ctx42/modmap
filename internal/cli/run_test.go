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

	"github.com/ctx42/modmap/internal/view"
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
		err := generate(t.Context(), rng, &config{}, spc)

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
		err := generate(t.Context(), rng, &config{}, spc)

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
		err := generate(t.Context(), rng, &config{}, spc)

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
		err := generate(t.Context(), rng, &config{}, spc)

		// --- Then ---
		assert.ErrorContain(t, "create map file", err)

		assert.Contain(t, "graph has 1 modules", tst.Stderr())
	})
}

func Test_spec_title(t *testing.T) {
	t.Run("the map name wins", func(t *testing.T) {
		// --- Given ---
		spc := spec{Name: "ctx42", Dirs: []string{"/src/work"}}

		// --- When ---
		have := spc.title()

		// --- Then ---
		assert.Equal(t, "ctx42", have)
	})

	t.Run("the directory names an unnamed map", func(t *testing.T) {
		// --- Given ---
		spc := spec{Dirs: []string{"/src/work"}}

		// --- When ---
		have := spc.title()

		// --- Then ---
		assert.Equal(t, "work", have)
	})

	t.Run("nothing to name it after", func(t *testing.T) {
		// --- Given ---
		spc := spec{}

		// --- When ---
		have := spc.title()

		// --- Then ---
		assert.Equal(t, binName, have)
	})
}

func Test_show(t *testing.T) {
	t.Run("the map is written and opened in a browser", func(t *testing.T) {
		// --- Given ---
		tmp := t.TempDir()
		src := t.TempDir()
		writeMod(t, "module example.com/a\n", src, "a")
		// os.TempDir reads the process environment, which no ring reaches.
		t.Setenv("TMPDIR", tmp)
		var opened string
		cfg := &config{
			web:    true,
			opener: func(url string) error { opened = url; return nil },
		}
		spc := spec{Dirs: []string{src}}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := generate(t.Context(), rng, cfg, spc)

		// --- Then ---
		assert.NoError(t, err)

		pth := filepath.Join(tmp, "modmap", filepath.Base(src)+".html")
		assert.Equal(t, "file://"+pth, opened)
		assert.Contain(t, "example.com/a", oskit.ReadFileStr(t, pth))
		assert.Contain(t, "map written to "+pth, tst.Stderr())
	})

	t.Run("error - no browser to open the map in", func(t *testing.T) {
		// --- Given ---
		tmp := t.TempDir()
		src := t.TempDir()
		writeMod(t, "module example.com/a\n", src, "a")
		t.Setenv("TMPDIR", tmp)
		cfg := &config{
			web:    true,
			opener: func(string) error { return view.ErrNoBrowser },
		}
		spc := spec{Dirs: []string{src}}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := generate(t.Context(), rng, cfg, spc)

		// --- Then ---
		assert.ErrorIs(t, view.ErrNoBrowser, err)

		pth := filepath.Join(tmp, "modmap", filepath.Base(src)+".html")
		assert.Contain(t, "map written to "+pth, tst.Stderr())
	})
}
