// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"

	"github.com/ctx42/modmap/internal/conf"
	"github.com/ctx42/modmap/internal/view"
	"github.com/ctx42/modmap/pkg/graph"
	"github.com/ctx42/modmap/pkg/mod"
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
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, "example.com/a", oskit.ReadFileStr(t, out))
		assert.Contain(t, "found example.com/a", tst.Stderr())
	})

	t.Run("the options filter the map", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, "module example.com/a\n", src, "a")
		writeMod(t, "module example.com/b\n", src, "b")
		out := filepath.Join(t.TempDir(), "map.svg")
		cfg := &config{
			roots:   []string{src},
			include: []string{"example.com/*"},
			exclude: []string{"example.com/b"},
			out:     out,
		}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		doc := oskit.ReadFileStr(t, out)
		assert.Contain(t, `data-module="example.com/a"`, doc)
		assert.NotContain(t, `data-module="example.com/b"`, doc)
		assert.Contain(t, "graph has 1 modules", tst.Stderr())
	})

	t.Run("every configured map in one image", func(t *testing.T) {
		// --- Given ---
		cnf := crossMaps(t)
		cfg := &config{conf: cnf}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		dst := filepath.Dir(cnf)
		ents := must.Value(os.ReadDir(dst))
		assert.Len(t, 2, ents)
		doc := oskit.ReadFileStr(t, dst, "all.svg")
		assert.Contain(t, `data-module="github.com/ctx42/testing"`, doc)
		assert.Contain(t, `data-module="github.com/customer/a"`, doc)
		assert.Contain(t, ">ctx42</text>", doc)
		assert.Contain(t, ">work</text>", doc)

		assert.Contain(t, "graph has 2 modules", tst.Stderr())
	})

	t.Run("only the named map is drawn", func(t *testing.T) {
		// --- Given ---
		cnf := crossMaps(t)
		cfg := &config{conf: cnf, names: []string{"ctx42"}}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		doc := oskit.ReadFileStr(t, filepath.Dir(cnf), "all.svg")
		assert.Contain(t, `data-module="github.com/ctx42/testing"`, doc)
		assert.NotContain(t, "github.com/customer/a", doc)
		assert.NotContain(t, ">ctx42</text>", doc)

		assert.NotContain(t, "found github.com/customer/a", tst.Stderr())
	})

	t.Run("a requirement crossing maps is an edge", func(t *testing.T) {
		// --- Given ---
		cnf := crossMaps(t)
		cfg := &config{conf: cnf, names: []string{"ctx42", "work"}}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		doc := oskit.ReadFileStr(t, filepath.Dir(cnf), "all.svg")
		wTst := `<g id="m0" class="module rm1" tabindex="0"` +
			` data-module="github.com/ctx42/testing" data-level="0"` +
			` data-dependents="github.com/customer/a">`
		assert.Contain(t, wTst, doc)
		wApp := `<g id="m1" class="module rm0" tabindex="0"` +
			` data-module="github.com/customer/a" data-level="1"`
		assert.Contain(t, wApp, doc)
		assert.Equal(t, 1, strings.Count(doc, `data-level="0"`))

		lib := group(doc, "github.com/ctx42/testing")
		assert.Contain(t, `<rect x="216" `, lib)
		app := group(doc, "github.com/customer/a")
		assert.NotContain(t, `<rect x="216" `, app)
		assert.Contain(t, `class="badge bm0"`, app)

		assert.Contain(t, "graph has 2 modules, widest level 1", tst.Stderr())
	})

	t.Run("columns follow the named order", func(t *testing.T) {
		// --- Given ---
		cnf := crossMaps(t)
		cfg := &config{conf: cnf, names: []string{"work", "ctx42"}}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		doc := oskit.ReadFileStr(t, filepath.Dir(cnf), "all.svg")
		lib := group(doc, "github.com/ctx42/testing")
		assert.NotContain(t, `<rect x="216" `, lib)
		app := group(doc, "github.com/customer/a")
		assert.Contain(t, `<rect x="216" `, app)
		work := strings.Index(doc, ">work</text>")
		assert.True(t, work < strings.Index(doc, ">ctx42</text>"))

		assert.Contain(t, "graph has 2 modules", tst.Stderr())
	})

	t.Run("a wide row across maps is confirmed", func(t *testing.T) {
		// --- Given ---
		cnf := wideMaps(t)

		ctl, trm := openPTY(t)
		must.Value(ctl.WriteString("n\n"))

		cfg := &config{conf: cnf}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.SetStdin(trm)

		// --- When ---
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, oskit.PathExists(t, filepath.Dir(cnf), "all.svg"))
		want := "the widest level holds 24 modules"
		assert.Contain(t, want, tst.Stderr())
		assert.Contain(t, "render it anyway? [y/N]: ", tst.Stderr())
	})

	t.Run("a wide row across maps with yes", func(t *testing.T) {
		// --- Given ---
		cnf := wideMaps(t)
		_, trm := openPTY(t)
		cfg := &config{conf: cnf, yes: true}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.SetStdin(trm)

		// --- When ---
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, oskit.PathExists(t, filepath.Dir(cnf), "all.svg"))
		assert.NotContain(t, "render it anyway?", tst.Stderr())
	})

	t.Run("web opens every configured map", func(t *testing.T) {
		// --- Given ---
		tmp := t.TempDir()
		cnf := crossMaps(t)
		// os.TempDir reads the process environment, which no ring reaches.
		t.Setenv("TMPDIR", tmp)
		var opened string
		cfg := &config{
			conf:   cnf,
			web:    true,
			opener: func(url string) error { opened = url; return nil },
		}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		pth := filepath.Join(tmp, "modmap", "ctx42-work.html")
		assert.Equal(t, "file://"+pth, opened)
		page := oskit.ReadFileStr(t, pth)
		assert.Contain(t, `data-module="github.com/ctx42/testing"`, page)
		assert.Contain(t, `data-module="github.com/customer/a"`, page)
		assert.False(t, oskit.PathExists(t, filepath.Dir(cnf), "all.svg"))

		assert.Contain(t, "map written to "+pth, tst.Stderr())
	})

	t.Run("web opens the named maps", func(t *testing.T) {
		// --- Given ---
		tmp := t.TempDir()
		cnf := crossMaps(t)
		t.Setenv("TMPDIR", tmp)
		cfg := &config{
			conf:   cnf,
			names:  []string{"work", "ctx42"},
			web:    true,
			opener: func(string) error { return nil },
		}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		pth := filepath.Join(tmp, "modmap", "work-ctx42.html")
		page := oskit.ReadFileStr(t, pth)
		assert.Contain(t, ">work</text>", page)
		assert.Contain(t, ">ctx42</text>", page)
		assert.False(t, oskit.PathExists(t, filepath.Dir(cnf), "all.svg"))

		assert.Contain(t, "map written to "+pth, tst.Stderr())
	})

	t.Run("web opens one named map", func(t *testing.T) {
		// --- Given ---
		tmp := t.TempDir()
		cnf := crossMaps(t)
		t.Setenv("TMPDIR", tmp)
		cfg := &config{
			conf:   cnf,
			names:  []string{"ctx42"},
			web:    true,
			opener: func(string) error { return nil },
		}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		page := oskit.ReadFileStr(t, tmp, "modmap", "ctx42.html")
		assert.Contain(t, `data-module="github.com/ctx42/testing"`, page)
		assert.NotContain(t, ">ctx42</text>", page)
		assert.NotContain(t, `y1="200"`, page)
		assert.False(t, oskit.PathExists(t, filepath.Dir(cnf), "all.svg"))

		assert.Contain(t, "graph has 1 modules", tst.Stderr())
	})

	t.Run("plan printed instead of a map", func(t *testing.T) {
		// --- Given ---
		cnf, _ := planMaps(t)
		names := []string{"ctx42"}
		cfg := &config{conf: cnf, plan: "github.com/ctx42/d", names: names}
		tst := ringtest.New(t).WetStdout().WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, `"module": "github.com/ctx42/d"`, tst.Stdout())
		assert.Len(t, 1, must.Value(os.ReadDir(filepath.Dir(cnf))))

		assert.Contain(t, "graph has 4 modules", tst.Stderr())
	})

	t.Run("error - unknown map name", func(t *testing.T) {
		// --- Given ---
		cfg := &config{conf: crossMaps(t), names: []string{"three"}}
		rng := ringtest.New(t).Ring()

		// --- When ---
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.ErrorContain(t, "unknown map: three", err)
	})

	t.Run("error - configuration cannot be read", func(t *testing.T) {
		// --- Given ---
		cfg := &config{conf: filepath.Join(t.TempDir(), "nope.yaml")}
		rng := ringtest.New(t).Ring()

		// --- When ---
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.ErrorContain(t, "read configuration", err)
	})

	t.Run("error - a map directory cannot be scanned", func(t *testing.T) {
		// --- Given ---
		gone := filepath.Join(t.TempDir(), "gone")
		cnf := oskit.Create(t, ""+
			"out: all.svg\n"+
			"maps:\n"+
			"  - name: one\n"+
			"    dirs: ["+gone+"]\n", t.TempDir(), "modmap.yaml")
		cfg := &config{conf: cnf}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := run(t.Context(), rng, cfg)

		// --- Then ---
		assert.ErrorContain(t, "scan "+gone, err)
		assert.Contain(t, "scanning "+gone, tst.Stderr())
	})
}

func Test_combine(t *testing.T) {
	t.Run("maps side by side", func(t *testing.T) {
		// --- Given ---
		maps := []conf.Map{
			{
				Name:    "ctx42",
				Dirs:    []string{"/src/ctx42", "/src/shared"},
				Include: []string{"github.com/ctx42/*"},
			},
			{
				Name:    "work",
				Dirs:    []string{"/src/work", "/src/shared"},
				Include: []string{"github.com/customer/*"},
				Exclude: []string{"github.com/customer/x"},
			},
		}

		// --- When ---
		have := combine("/out/all.svg", maps)

		// --- Then ---
		assert.Equal(t, "ctx42-work", have.Name)
		wDirs := []string{"/src/ctx42", "/src/shared", "/src/work"}
		assert.Equal(t, wDirs, have.Dirs)
		assert.Equal(t, "/out/all.svg", have.Out)

		assert.True(t, have.Filter.Match("github.com/ctx42/a"))
		assert.True(t, have.Filter.Match("github.com/customer/a"))
		assert.False(t, have.Filter.Match("github.com/customer/x"))
		assert.False(t, have.Filter.Match("golang.org/x/mod"))

		assert.Len(t, 2, have.Columns)
		assert.Equal(t, "ctx42", have.Columns[0].Name)
		assert.True(t, have.Columns[0].Filter.Match("github.com/ctx42/a"))
		assert.False(t, have.Columns[0].Filter.Match("github.com/customer/a"))
		assert.Equal(t, "work", have.Columns[1].Name)
		assert.True(t, have.Columns[1].Filter.Match("github.com/customer/a"))
	})

	t.Run("one map", func(t *testing.T) {
		// --- Given ---
		maps := []conf.Map{{Name: "ctx42", Dirs: []string{"/src"}}}

		// --- When ---
		have := combine("/out/all.svg", maps)

		// --- Then ---
		assert.Equal(t, "ctx42", have.Name)
		assert.Equal(t, []string{"/src"}, have.Dirs)
		assert.Len(t, 1, have.Columns)
		assert.True(t, have.Filter.Match("golang.org/x/mod"))
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
			Dirs:   []string{src},
			Filter: mod.NewFilter([]string{"example.com/*"}, nil),
			Out:    out,
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

	t.Run("wide map declined", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		for idx := range wideLevel + 1 {
			content := fmt.Sprintf("module example.com/m%d\n", idx)
			writeMod(t, content, src, fmt.Sprintf("m%d", idx))
		}

		ctl, trm := openPTY(t)
		must.Value(ctl.WriteString("n\n"))

		out := filepath.Join(t.TempDir(), "map.svg")
		spc := spec{Dirs: []string{src}, Out: out}

		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.SetStdin(trm)

		// --- When ---
		err := generate(t.Context(), rng, &config{}, spc)

		// --- Then ---
		assert.NoError(t, err)
		assert.False(t, oskit.PathExists(t, out))
		assert.Contain(t, "render it anyway? [y/N]: ", tst.Stderr())
	})

	t.Run("web map opened", func(t *testing.T) {
		// --- Given ---
		tmp := t.TempDir()
		src := t.TempDir()
		writeMod(t, "module example.com/a\n", src, "a")
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
		assert.Contain(t, "map written to "+pth, tst.Stderr())
	})

	t.Run("error - directory does not exist", func(t *testing.T) {
		// --- Given ---
		src := filepath.Join(t.TempDir(), "gone")
		spc := spec{Dirs: []string{src}, Out: "/out.svg"}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := generate(t.Context(), rng, &config{}, spc)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.Contain(t, "scanning "+src, tst.Stderr())
	})

	t.Run("error - resolver cannot start", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, "module example.com/a\n", src, "a")
		spc := spec{Dirs: []string{src}, Out: "/out.svg"}
		// os.MkdirTemp reads the process environment, which no ring reaches.
		t.Setenv("TMPDIR", oskit.Create(t, "", t.TempDir(), "file"))
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := generate(t.Context(), rng, &config{}, spc)

		// --- Then ---
		assert.ErrorIs(t, syscall.ENOTDIR, err)
		assert.ErrorContain(t, "probe module", err)
		assert.Contain(t, "found example.com/a", tst.Stderr())
	})

	t.Run("error - requirement cannot be resolved", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, ""+
			"module example.com/a\n"+
			"require example.com/nope v1.0.0\n", src, "a")
		spc := spec{Dirs: []string{src}, Out: "/out.svg"}

		env := append(os.Environ(), "GOPROXY=off", "GOFLAGS=-mod=mod")
		tst := ringtest.New(t, ring.WithEnv(env)).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := generate(t.Context(), rng, &config{}, spc)

		// --- Then ---
		want := "^query example.com/nope@v1.0.0: .*GOPROXY=off"
		assert.ErrorRegexp(t, want, err)
		assert.Contain(t, "resolving example.com/nope@v1.0.0", tst.Stderr())
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
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.ErrorContain(t, "write map file", err)

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

func Test_write(t *testing.T) {
	t.Run("map written", func(t *testing.T) {
		// --- Given ---
		mods := map[string]*mod.Module{
			"example.com/a": {Path: "example.com/a"},
		}
		grp := must.Value(graph.New(mods))
		out := filepath.Join(t.TempDir(), "map.svg")

		// --- When ---
		err := write(grp, nil, out)

		// --- Then ---
		assert.NoError(t, err)
		doc := oskit.ReadFileStr(t, out)
		assert.Contain(t, `data-module="example.com/a"`, doc)
	})

	t.Run("error - directory does not exist", func(t *testing.T) {
		// --- Given ---
		grp := must.Value(graph.New(nil))
		out := filepath.Join(t.TempDir(), "gone", "map.svg")

		// --- When ---
		err := write(grp, nil, out)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.ErrorContain(t, "write map file", err)
	})
}
