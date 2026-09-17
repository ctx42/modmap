// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mod

import (
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
)

func Test_parseMod(t *testing.T) {
	t.Run("module with direct requirements", func(t *testing.T) {
		// --- Given ---
		content := "" +
			"module example.com/a\n" +
			"\n" +
			"go 1.26\n" +
			"\n" +
			"require (\n" +
			"\texample.com/c v1.0.0\n" +
			"\texample.com/b v1.0.0\n" +
			"\texample.com/d v1.0.0 // indirect\n" +
			")\n"
		pth := writeMod(t, content, t.TempDir())

		// --- When ---
		have, err := parseMod(pth, Filter{})

		// --- Then ---
		assert.NoError(t, err)
		want := []Require{
			{Path: "example.com/b", Ver: "v1.0.0"},
			{Path: "example.com/c", Ver: "v1.0.0"},
		}
		assert.Equal(t, "example.com/a", have.Path)
		assert.Equal(t, want, have.Requires)
		assert.Equal(t, "", have.Dir)
	})

	t.Run("error - file does not exist", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "go.mod")

		// --- When ---
		have, err := parseMod(pth, Filter{})

		// --- Then ---
		assert.ErrorContain(t, "read module file", err)
		assert.Nil(t, have)
	})
}

func Test_parseModData(t *testing.T) {
	t.Run("requirements are filtered", func(t *testing.T) {
		// --- Given ---
		data := []byte("" +
			"module example.com/a\n" +
			"\n" +
			"require (\n" +
			"\texample.com/b v1.0.0\n" +
			"\tother.com/c v1.0.0\n" +
			")\n")
		flt := NewFilter([]string{"example.com/*"}, nil)

		// --- When ---
		have, err := parseModData("go.mod", data, flt)

		// --- Then ---
		assert.NoError(t, err)
		want := []Require{{Path: "example.com/b", Ver: "v1.0.0"}}
		assert.Equal(t, want, have.Requires)
	})

	t.Run("module without requirements", func(t *testing.T) {
		// --- Given ---
		data := []byte("module example.com/a\n")

		// --- When ---
		have, err := parseModData("go.mod", data, Filter{})

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, have.Requires)
	})

	t.Run("error - malformed file", func(t *testing.T) {
		// --- Given ---
		data := []byte("module\n")

		// --- When ---
		have, err := parseModData("go.mod", data, Filter{})

		// --- Then ---
		assert.ErrorContain(t, "parse module file", err)
		assert.Nil(t, have)
	})

	t.Run("error - no module directive", func(t *testing.T) {
		// --- Given ---
		data := []byte("go 1.26\n")

		// --- When ---
		have, err := parseModData("go.mod", data, Filter{})

		// --- Then ---
		assert.ErrorIs(t, ErrNoModule, err)
		assert.Nil(t, have)
	})
}

func Test_Module_Deps(t *testing.T) {
	t.Run("versions of the same path collapse", func(t *testing.T) {
		// --- Given ---
		mod := &Module{
			Path: "example.com/a",
			Requires: []Require{
				{Path: "example.com/c", Ver: "v1.0.0"},
				{Path: "example.com/b", Ver: "v2.0.0"},
				{Path: "example.com/b", Ver: "v1.0.0"},
			},
		}

		// --- When ---
		have := mod.Deps()

		// --- Then ---
		want := []string{"example.com/b", "example.com/c"}
		assert.Equal(t, want, have)
	})

	t.Run("module without requirements", func(t *testing.T) {
		// --- Given ---
		mod := &Module{Path: "example.com/a"}

		// --- When ---
		have := mod.Deps()

		// --- Then ---
		assert.Empty(t, have)
	})
}

func Test_cmpRequire_tabular(t *testing.T) {
	tt := []struct {
		testN string

		a    Require
		b    Require
		want int
	}{
		{"path before", Require{Path: "a"}, Require{Path: "b"}, -1},
		{"path after", Require{Path: "b"}, Require{Path: "a"}, 1},
		{"version ties", Require{"a", "v1"}, Require{"a", "v2"}, -1},
		{"equal", Require{"a", "v1"}, Require{"a", "v1"}, 0},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := cmpRequire(tc.a, tc.b)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_sortRequires(t *testing.T) {
	// --- Given ---
	reqs := []Require{
		{Path: "example.com/b", Ver: "v2.0.0"},
		{Path: "example.com/a", Ver: "v1.0.0"},
		{Path: "example.com/b", Ver: "v2.0.0"},
		{Path: "example.com/b", Ver: "v1.0.0"},
	}

	// --- When ---
	have := sortRequires(reqs)

	// --- Then ---
	want := []Require{
		{Path: "example.com/a", Ver: "v1.0.0"},
		{Path: "example.com/b", Ver: "v1.0.0"},
		{Path: "example.com/b", Ver: "v2.0.0"},
	}
	assert.Equal(t, want, have)
}
