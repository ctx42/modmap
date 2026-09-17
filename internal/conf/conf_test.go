// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package conf

import (
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
			"maps:\n"+
			"  - name: ctx42\n"+
			"    dirs: [/src/ctx42]\n"+
			"    include: ['github.com/ctx42/*']\n"+
			"    exclude: ['github.com/ctx42/tst-*']\n"+
			"    out: /out/ctx42.svg\n", dir)

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 1, have.Maps)
		assert.Equal(t, "ctx42", have.Maps[0].Name)
		assert.Equal(t, []string{"/src/ctx42"}, have.Maps[0].Dirs)
		wInc := []string{"github.com/ctx42/*"}
		assert.Equal(t, wInc, have.Maps[0].Include)
		wExc := []string{"github.com/ctx42/tst-*"}
		assert.Equal(t, wExc, have.Maps[0].Exclude)
		assert.Equal(t, "/out/ctx42.svg", have.Maps[0].Out)
	})

	t.Run("relative paths resolve against the file", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		pth := writeConf(t, ""+
			"maps:\n"+
			"  - name: here\n"+
			"    dirs: [..]\n"+
			"    out: tmp/here.svg\n", dir)

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.NoError(t, err)
		wDir := filepath.Dir(dir)
		assert.Equal(t, []string{wDir}, have.Maps[0].Dirs)
		wOut := filepath.Join(dir, "tmp", "here.svg")
		assert.Equal(t, wOut, have.Maps[0].Out)
	})

	t.Run("error - file does not exist", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "nope.yaml")

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
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

	t.Run("error - invalid configuration", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(t, "maps: []\n", t.TempDir())

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorIs(t, ErrNoMaps, err)
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
	// --- Given ---
	valid := Map{Name: "a", Dirs: []string{"/src"}, Out: "/out.svg"}
	cfg := &Config{Maps: []Map{valid}}

	// --- When ---
	err := cfg.validate()

	// --- Then ---
	assert.NoError(t, err)
}

func Test_Config_validate_tabular(t *testing.T) {
	valid := Map{Name: "a", Dirs: []string{"/src"}, Out: "/out.svg"}

	tt := []struct {
		testN string

		maps []Map
		want error
	}{
		{"no maps", nil, ErrNoMaps},
		{
			"no name",
			[]Map{{Dirs: []string{"/src"}, Out: "/out.svg"}},
			ErrNoName,
		},
		{"duplicate name", []Map{valid, valid}, ErrDupName},
		{"no dirs", []Map{{Name: "a", Out: "/out.svg"}}, ErrNoDirs},
		{
			"no output",
			[]Map{{Name: "a", Dirs: []string{"/src"}}},
			ErrNoOut,
		},
		{
			"malformed include glob",
			[]Map{{
				Name:    "a",
				Dirs:    []string{"/src"},
				Include: []string{"["},
				Out:     "/out.svg",
			}},
			mod.ErrInvGlob,
		},
		{
			"malformed exclude glob",
			[]Map{{
				Name:    "a",
				Dirs:    []string{"/src"},
				Exclude: []string{"["},
				Out:     "/out.svg",
			}},
			mod.ErrInvGlob,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			cfg := &Config{Maps: tc.maps}

			// --- When ---
			err := cfg.validate()

			// --- Then ---
			assert.ErrorIs(t, tc.want, err)
		})
	}
}
