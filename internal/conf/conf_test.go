// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package conf

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"

	"github.com/ctx42/modmap/pkg/mod"
)

func Test_Load(t *testing.T) {
	t.Run("maps are read", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		pth := writeConf(t, ""+
			"out: /out/ctx42.svg\n"+
			"maps:\n"+
			"  - name: ctx42\n"+
			"    dirs: [/src/ctx42]\n"+
			"    include: ['github.com/ctx42/*']\n"+
			"    exclude: ['github.com/ctx42/tst-*']\n", dir)

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/out/ctx42.svg", have.Out)
		assert.Len(t, 1, have.Maps)
		assert.Equal(t, "ctx42", have.Maps[0].Name)
		assert.Equal(t, []string{"/src/ctx42"}, have.Maps[0].Dirs)
		wInc := []string{"github.com/ctx42/*"}
		assert.Equal(t, wInc, have.Maps[0].Include)
		wExc := []string{"github.com/ctx42/tst-*"}
		assert.Equal(t, wExc, have.Maps[0].Exclude)
	})

	t.Run("relative paths resolve against the file", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		pth := writeConf(t, ""+
			"out: tmp/here.svg\n"+
			"maps:\n"+
			"  - name: here\n"+
			"    dirs: [..]\n", dir)

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "tmp", "here.svg"), have.Out)
		assert.Equal(t, []string{filepath.Dir(dir)}, have.Maps[0].Dirs)
	})

	t.Run("error - file does not exist", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "nope.yaml")

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.ErrorContain(t, "read configuration", err)
		assert.Nil(t, have)
	})

	t.Run("error - malformed file", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(t, "maps: [", t.TempDir())

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorContain(t, "parse configuration", err)
		assert.Nil(t, have)
	})

	t.Run("error - no output", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(t, ""+
			"maps:\n"+
			"  - name: a\n"+
			"    dirs: [src]\n",
			t.TempDir(),
		)

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorIs(t, ErrNoOut, err)
		assert.Nil(t, have)
	})

	t.Run("error - output on a map", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(t, ""+
			"out: all.svg\n"+
			"maps:\n"+
			"  - name: a\n"+
			"    dirs: [src]\n"+
			"    out: a.svg\n",
			t.TempDir(),
		)

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorRegexp(t, "^parse configuration: .*out", err)
		assert.Nil(t, have)
	})

	t.Run("error - unknown key", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(t, ""+
			"out: a.svg\n"+
			"maps:\n"+
			"  - name: a\n"+
			"    dirs: [src]\n"+
			"    exlude: [example.com/x]\n",
			t.TempDir(),
		)

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorRegexp(t, "^parse configuration: .*exlude", err)
		assert.Nil(t, have)
	})

	t.Run("error - invalid configuration", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(t, "out: a.svg\nmaps: []\n", t.TempDir())

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorIs(t, ErrNoMaps, err)
		assert.ErrorEqual(t, "validate "+pth+": no maps declared", err)
		assert.Nil(t, have)
	})
}

func Test_Config_Select(t *testing.T) {
	t.Run("no names select every map", func(t *testing.T) {
		// --- Given ---
		cfg := &Config{Maps: []Map{{Name: "a"}, {Name: "b"}}}

		// --- When ---
		have, err := cfg.Select(nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 2, have)
	})

	t.Run("names select in the order they are given", func(t *testing.T) {
		// --- Given ---
		cfg := &Config{Maps: []Map{{Name: "a"}, {Name: "b"}}}

		// --- When ---
		have, err := cfg.Select([]string{"b", "a"})

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "b", have[0].Name)
		assert.Equal(t, "a", have[1].Name)
	})

	t.Run("error - unknown name lists the known ones", func(t *testing.T) {
		// --- Given ---
		cfg := &Config{Maps: []Map{{Name: "a"}, {Name: "b"}}}

		// --- When ---
		have, err := cfg.Select([]string{"c"})

		// --- Then ---
		assert.ErrorIs(t, ErrUnkMap, err)
		assert.ErrorContain(t, "have: a, b", err)
		assert.Nil(t, have)
	})
}

func Test_Config_Names(t *testing.T) {
	// --- Given ---
	cfg := &Config{Maps: []Map{{Name: "a"}, {Name: "b"}}}

	// --- When ---
	have := cfg.Names()

	// --- Then ---
	assert.Equal(t, []string{"a", "b"}, have)
}

func Test_Config_validate(t *testing.T) {
	valid := Map{Name: "a", Dirs: []string{"/src"}}

	t.Run("valid", func(t *testing.T) {
		// --- Given ---
		cfg := &Config{Out: "/out.svg", Maps: []Map{valid}}

		// --- When ---
		err := cfg.validate()

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error - no name", func(t *testing.T) {
		// --- Given ---
		maps := []Map{valid, {Dirs: []string{"/src"}}}
		cfg := &Config{Out: "/out.svg", Maps: maps}

		// --- When ---
		err := cfg.validate()

		// --- Then ---
		assert.ErrorEqual(t, "map without a name: maps[1]", err)
	})
}

func Test_Config_validate_tabular(t *testing.T) {
	valid := Map{Name: "a", Dirs: []string{"/src"}}

	tt := []struct {
		testN string

		out  string
		maps []Map
		want error
	}{
		{"no maps", "/out.svg", nil, ErrNoMaps},
		{"no output", "", []Map{valid}, ErrNoOut},
		{"no name", "/out.svg", []Map{{Dirs: []string{"/src"}}}, ErrNoName},
		{"duplicate name", "/out.svg", []Map{valid, valid}, ErrDupName},
		{"no dirs", "/out.svg", []Map{{Name: "a"}}, ErrNoDirs},
		{
			"empty dir",
			"/out.svg",
			[]Map{{Name: "a", Dirs: []string{""}}},
			ErrEmptyDir,
		},
		{
			"malformed include glob",
			"/out.svg",
			[]Map{{Name: "a", Dirs: []string{"/src"}, Include: []string{"["}}},
			mod.ErrInvGlob,
		},
		{
			"malformed exclude glob",
			"/out.svg",
			[]Map{{Name: "a", Dirs: []string{"/src"}, Exclude: []string{"["}}},
			mod.ErrInvGlob,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			cfg := &Config{Out: tc.out, Maps: tc.maps}

			// --- When ---
			err := cfg.validate()

			// --- Then ---
			assert.ErrorIs(t, tc.want, err)
		})
	}
}

func Test_Config_resolve(t *testing.T) {
	t.Run("relative paths", func(t *testing.T) {
		// --- Given ---
		cfg := &Config{
			Out: "out/all.svg",
			Maps: []Map{
				{Name: "a", Dirs: []string{"src", "/abs"}},
				{Name: "b", Dirs: []string{""}},
			},
		}

		// --- When ---
		cfg.resolve("/cfg")

		// --- Then ---
		assert.Equal(t, "/cfg/out/all.svg", cfg.Out)
		want := []Map{
			{Name: "a", Dirs: []string{"/cfg/src", "/abs"}},
			{Name: "b", Dirs: []string{""}},
		}
		assert.Equal(t, want, cfg.Maps)
	})

	t.Run("empty output stays empty", func(t *testing.T) {
		// --- Given ---
		cfg := &Config{Maps: []Map{{Name: "a", Dirs: []string{"src"}}}}

		// --- When ---
		cfg.resolve("/cfg")

		// --- Then ---
		assert.Equal(t, "", cfg.Out)
	})
}

func Test_absPath_tabular(t *testing.T) {
	tt := []struct {
		testN string

		pth  string
		want string
	}{
		{"relative", "out/a.svg", "/cfg/out/a.svg"},
		{"absolute", "/out/a.svg", "/out/a.svg"},
		{"absolute uncleaned", "/out/../a.svg", "/a.svg"},
		{"empty", "", ""},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := absPath("/cfg", tc.pth)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
