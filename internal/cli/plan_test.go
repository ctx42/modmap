// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring"
	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/iokit"
	"github.com/ctx42/testkit/pkg/oskit"
	"github.com/ctx42/testkit/pkg/prjkit"

	"github.com/ctx42/modmap/internal/conf"
)

func Test_finder_holds_tabular(t *testing.T) {
	tt := []struct {
		testN string

		dir  string
		want bool
	}{
		{"first directory", "/src/ctx42/ring", true},
		{"second directory", "/opt/work/app", true},
		{"directory itself", "/opt/work", true},
		{"outside every directory", "/src/other/ring", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			fnd := finder{dirs: []string{"/src/ctx42", "/opt/work"}}

			// --- When ---
			have := fnd.holds(tc.dir)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_newFinders(t *testing.T) {
	t.Run("directories and globs", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		maps := []conf.Map{
			{
				Name:    "ctx42",
				Dirs:    []string{dir},
				Include: []string{"github.com/ctx42/*"},
				Exclude: []string{"github.com/ctx42/blog"},
			},
			{Name: "all", Dirs: []string{dir}},
		}

		// --- When ---
		have := newFinders(maps)

		// --- Then ---
		assert.Len(t, 2, have)
		assert.Equal(t, "ctx42", have[0].name)
		assert.Equal(t, []string{dir}, have[0].dirs)
		assert.True(t, have[0].incl.Match("github.com/ctx42/blog"))
		assert.False(t, have[0].flt.Match("github.com/ctx42/blog"))
		assert.True(t, have[0].flt.Match("github.com/ctx42/ring"))
		assert.False(t, have[0].flt.Match("github.com/other/ring"))
		assert.Equal(t, "all", have[1].name)
		assert.True(t, have[1].flt.Match("github.com/other/ring"))
	})

	t.Run("symbolic link directory is resolved", func(t *testing.T) {
		// --- Given ---
		dst := t.TempDir()
		lnk := filepath.Join(t.TempDir(), "link")
		must.Nil(os.Symlink(dst, lnk))
		maps := []conf.Map{{Name: "one", Dirs: []string{lnk}}}

		// --- When ---
		have := newFinders(maps)

		// --- Then ---
		want := []string{must.Value(filepath.EvalSymlinks(dst))}
		assert.Equal(t, want, have[0].dirs)
	})

	t.Run("missing directory is kept as it is", func(t *testing.T) {
		// --- Given ---
		gone := filepath.Join(t.TempDir(), "gone")
		maps := []conf.Map{{Name: "one", Dirs: []string{gone}}}

		// --- When ---
		have := newFinders(maps)

		// --- Then ---
		assert.Equal(t, []string{gone}, have[0].dirs)
	})
}

func Test_finders_filter(t *testing.T) {
	// --- Given ---
	fnds := newFinders([]conf.Map{
		{
			Name:    "ctx42",
			Include: []string{"github.com/ctx42/*"},
			Exclude: []string{"github.com/ctx42/blog"},
		},
		{Name: "work", Include: []string{"github.com/customer/*"}},
	})

	// --- When ---
	have := fnds.filter()

	// --- Then ---
	assert.True(t, have.Match("github.com/ctx42/ring"))
	assert.True(t, have.Match("github.com/ctx42/blog"))
	assert.True(t, have.Match("github.com/customer/b"))
	assert.False(t, have.Match("github.com/other/c"))
}

func Test_finders_owner_tabular(t *testing.T) {
	fnds := newFinders([]conf.Map{
		{
			Name:    "ctx42",
			Dirs:    []string{"/src/ctx42"},
			Include: []string{"github.com/ctx42/*"},
			Exclude: []string{"github.com/ctx42/blog"},
		},
		{
			Name:    "work",
			Dirs:    []string{"/src/work"},
			Include: []string{"github.com/customer/*"},
		},
		{
			Name:    "wide",
			Dirs:    []string{"/src"},
			Include: []string{"github.com/ctx42/*", "github.com/other/*"},
		},
	})

	tt := []struct {
		testN string

		pth   string
		dir   string
		wName string
		wKeep bool
	}{
		{
			"found by one map",
			"github.com/customer/b",
			"/src/work/b",
			"work",
			true,
		},
		{
			"found by two maps",
			"github.com/ctx42/ring",
			"/src/ctx42/ring",
			"ctx42",
			true,
		},
		{
			"excluded by one of two maps",
			"github.com/ctx42/blog",
			"/src/ctx42/blog",
			"ctx42",
			false,
		},
		{
			"found by a map not holding the first",
			"github.com/other/c",
			"/src/work/c",
			"wide",
			true,
		},
		{
			"held but included by no map",
			"github.com/nobody/c",
			"/src/work/c",
			"work",
			false,
		},
		{"held by no map", "github.com/ctx42/x", "/opt/x", "", false},
		{"not on disk", "github.com/ctx42/x", "", "", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			hName, hKeep := fnds.owner(tc.pth, tc.dir)

			// --- Then ---
			assert.Equal(t, tc.wName, hName)
			assert.Equal(t, tc.wKeep, hKeep)
		})
	}
}

func Test_runPlan(t *testing.T) {
	t.Run("plan spans every map", func(t *testing.T) {
		// --- Given ---
		cnf, src := planMaps(t)
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "github.com/ctx42/testing"}
		tst := ringtest.New(t).WetStdout().WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.NoError(t, err)
		want := plan{
			Module: "github.com/ctx42/testing",
			Rounds: [][]planMod{
				{
					{
						Path: "github.com/ctx42/testing",
						Map:  "ctx42",
						Dir:  filepath.Join(src, "ctx42", "testing"),
					},
				},
				{
					{
						Path: "github.com/ctx42/d",
						Map:  "ctx42",
						Dir:  filepath.Join(src, "ctx42", "d"),
					},
					{
						Path: "github.com/customer/b",
						Map:  "work",
						Dir:  filepath.Join(src, "customer", "b"),
					},
				},
				{
					{
						Path: "github.com/ctx42/a",
						Map:  "ctx42",
						Dir:  filepath.Join(src, "ctx42", "a"),
					},
				},
			},
			Excluded: []planMod{
				{
					Path: "github.com/ctx42/blog",
					Map:  "ctx42",
					Dir:  filepath.Join(src, "ctx42", "blog"),
				},
			},
		}
		wJSON := must.Value(json.MarshalIndent(want, "", "  "))
		assert.Equal(t, string(wJSON)+"\n", tst.Stdout())
		assert.NotContain(t, "github.com/customer/z", tst.Stdout())

		assert.Contain(t, "graph has 6 modules", tst.Stderr())
	})

	t.Run("two runs print the same plan", func(t *testing.T) {
		// --- Given ---
		cnf, _ := planMaps(t)
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "github.com/ctx42/testing"}
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

		assert.Contain(t, "graph has 6 modules", tst.Stderr())
	})

	t.Run("a module without dependents is alone", func(t *testing.T) {
		// --- Given ---
		cnf, src := planMaps(t)
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "github.com/ctx42/a"}
		tst := ringtest.New(t).WetStdout().WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.NoError(t, err)
		have := plan{}
		must.Nil(json.Unmarshal([]byte(tst.Stdout()), &have))
		wMod := planMod{
			Path: "github.com/ctx42/a",
			Map:  "ctx42",
			Dir:  filepath.Join(src, "ctx42", "a"),
		}
		want := plan{
			Module:   "github.com/ctx42/a",
			Rounds:   [][]planMod{{wMod}},
			Excluded: []planMod{},
		}
		assert.Equal(t, want, have)
		assert.Contain(t, `"excluded": []`, tst.Stdout())

		assert.Contain(t, "graph has 6 modules", tst.Stderr())
	})

	t.Run("exclusion wins over a map keeping the module", func(t *testing.T) {
		// --- Given ---
		_, src := planMaps(t)
		cnf := oskit.Create(t, ""+
			"out: all.svg\n"+
			"maps:\n"+
			"  - name: ctx42\n"+
			"    dirs: ["+filepath.Join(src, "ctx42")+"]\n"+
			"    include: ['github.com/ctx42/*']\n"+
			"    exclude: ['github.com/ctx42/blog']\n"+
			"  - name: wide\n"+
			"    dirs: ["+src+"]\n"+
			"    include: ['github.com/*/*']\n",
			t.TempDir(), "modmap.yaml",
		)
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "github.com/ctx42/testing"}
		tst := ringtest.New(t).WetStdout().WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.NoError(t, err)
		have := plan{}
		must.Nil(json.Unmarshal([]byte(tst.Stdout()), &have))
		want := []planMod{
			{
				Path: "github.com/ctx42/blog",
				Map:  "ctx42",
				Dir:  filepath.Join(src, "ctx42", "blog"),
			},
		}
		assert.Equal(t, want, have.Excluded)
		wMod := planMod{
			Path: "github.com/customer/b",
			Map:  "wide",
			Dir:  filepath.Join(src, "customer", "b"),
		}
		assert.Equal(t, wMod, have.Rounds[1][1])

		assert.Contain(t, "graph has 6 modules", tst.Stderr())
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
		cfg := &config{plan: "example.com/t"}

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
			{{Path: "example.com/t", Map: "one", Dir: filepath.Join(src, "t")}},
			{{Path: "example.com/a", Map: "one", Dir: filepath.Join(src, "a")}},
		}
		assert.Equal(t, want, have.Rounds)
		assert.Equal(t, []planMod{{Path: "example.com/c"}}, have.Excluded)

		assert.Contain(t, "resolving example.com/c@v1.0.0", tst.Stderr())
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
		cfg := &config{plan: "example.com/a"}
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
		cfg := &config{plan: "github.com/ctx42/nope"}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.ErrorIs(t, errModNotFound, err)
		assert.ErrorContain(t, "module not found: github.com/ctx42/nope", err)
		assert.Contain(t, "graph has 6 modules", tst.Stderr())
	})

	t.Run("error - module excluded by its map", func(t *testing.T) {
		// --- Given ---
		cnf, _ := planMaps(t)
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "github.com/ctx42/blog"}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		want := "module excluded: github.com/ctx42/blog (map: ctx42)"
		assert.ErrorEqual(t, want, err)
		assert.ErrorIs(t, errModExcluded, err)
		assert.Contain(t, "graph has 6 modules", tst.Stderr())
	})

	t.Run("error - module held by no map", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, "module example.com/a\n", src, "one", "a")
		writeMod(t, "module other.com/b\n", src, "one", "b")
		cnf := oskit.Create(t, ""+
			"out: all.svg\n"+
			"maps:\n"+
			"  - name: one\n"+
			"    dirs: ["+filepath.Join(src, "one")+"]\n"+
			"    include: ['example.com/*']\n"+
			"  - name: two\n"+
			"    dirs: ["+filepath.Join(src, "two")+"]\n"+
			"    include: ['other.com/*']\n", t.TempDir(), "modmap.yaml")
		oskit.MkdirAll(t, src, "two")
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "other.com/b"}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.ErrorEqual(t, "module excluded: other.com/b (map: one)", err)
		assert.Contain(t, "graph has 2 modules", tst.Stderr())
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
		cfg := &config{plan: "example.com/c"}

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

	t.Run("lag of the requirements the plan lifts", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		prj := prjkit.New(t, oskit.MkdirAll(t, src, "ctx42", "testing"))
		prj.CreateFileWith("module github.com/ctx42/testing\n", "go.mod")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("1", "file.txt")
		prj.GitCommit("v0.2.0", "feat!: drop Old")
		prj.Close()

		writeMod(t, ""+
			"module github.com/customer/a\n"+
			"require github.com/ctx42/testing v0.1.0\n",
			src, "customer", "a",
		)
		cnf := oskit.Create(t, ""+
			"out: all.svg\n"+
			"maps:\n"+
			"  - name: ctx42\n"+
			"    dirs: ["+filepath.Join(src, "ctx42")+"]\n"+
			"    include: ['github.com/ctx42/*']\n"+
			"  - name: work\n"+
			"    dirs: ["+filepath.Join(src, "customer")+"]\n"+
			"    include: ['github.com/customer/*']\n",
			t.TempDir(), "modmap.yaml",
		)
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "github.com/ctx42/testing", lag: true}
		tst := ringtest.New(t).WetStdout().WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.NoError(t, err)
		have := plan{}
		must.Nil(json.Unmarshal([]byte(tst.Stdout()), &have))
		want := []lag{
			{
				Path: "github.com/customer/a",
				Requires: []lagReq{
					{
						Path:     "github.com/ctx42/testing",
						Pinned:   "v0.1.0",
						Latest:   "v0.2.0",
						Tags:     1,
						Commits:  1,
						Breaking: []string{"feat!: drop Old"},
					},
				},
			},
		}
		assert.Equal(t, want, have.Lag)

		assert.Contain(t, "graph has 2 modules", tst.Stderr())
	})

	t.Run("no lag without the option", func(t *testing.T) {
		// --- Given ---
		cnf, _ := planMaps(t)
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "github.com/ctx42/testing"}
		tst := ringtest.New(t).WetStdout().WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.NoError(t, err)
		assert.NotContain(t, `"lag"`, tst.Stdout())

		assert.Contain(t, "graph has 6 modules", tst.Stderr())
	})

	t.Run("error - lag of a module outside git", func(t *testing.T) {
		// --- Given ---
		cnf, _ := planMaps(t)
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "github.com/ctx42/testing", lag: true}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.ErrorContain(t, "lag of github.com/ctx42/testing", err)
		assert.Contain(t, "graph has 6 modules", tst.Stderr())
	})

	t.Run("error - plan cannot be written", func(t *testing.T) {
		// --- Given ---
		cnf, _ := planMaps(t)
		fil := must.Value(conf.Load(cnf))
		cfg := &config{plan: "github.com/ctx42/a"}
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()
		rng.SetStdout(iokit.ErrWriter(&bytes.Buffer{}, 0))

		// --- When ---
		err := runPlan(t.Context(), rng, cfg, fil)

		// --- Then ---
		assert.ErrorContain(t, "write plan: ", err)
		assert.Contain(t, "graph has 6 modules", tst.Stderr())
	})
}
