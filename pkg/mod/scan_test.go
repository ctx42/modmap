// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mod

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
)

func Test_NewScanner(t *testing.T) {
	t.Run("nil reporter is replaced", func(t *testing.T) {
		// --- Given ---
		flt := NewFilter([]string{"example.com/*"}, nil)

		// --- When ---
		have := NewScanner(flt, nil)

		// --- Then ---
		assert.Equal(t, flt, have.flt)
		assert.NotNil(t, have.logf)
	})
}

func Test_Scanner_Scan(t *testing.T) {
	t.Run("skipped directories are not descended into", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		writeMod(t, "module example.com/a\n", root, "a")
		skipped := filepath.Join("a", "testdata", "x")
		writeMod(t, "module example.com/x\n", root, skipped)
		writeMod(t, "module example.com/y\n", root, "vendor", "y")
		writeMod(t, "module example.com/z\n", root, ".git", "z")
		writeMod(t, "module example.com/h\n", root, ".hidden", "h")
		writeMod(t, "module example.com/u\n", root, "_old", "u")
		writeMod(t, "module example.com/c\n", root, "mod", "c@v1.0.0")
		scn := NewScanner(Filter{}, nil)

		// --- When ---
		have, err := scn.Scan(t.Context(), []string{root})

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 1, have)
		want := filepath.Join(root, "a")
		assert.Equal(t, want, have["example.com/a"].Dir)
	})

	t.Run("nested modules are found", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		writeMod(t, "module example.com/a\n", root, "a")
		writeMod(t, "module example.com/b\n", root, "a", "sub", "b")
		scn := NewScanner(Filter{}, nil)

		// --- When ---
		have, err := scn.Scan(t.Context(), []string{root})

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 2, have)
	})

	t.Run("modules not passing the filter are dropped", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		writeMod(t, "module example.com/a\n", root, "a")
		writeMod(t, "module other.com/b\n", root, "b")
		flt := NewFilter([]string{"example.com/*"}, nil)
		scn := NewScanner(flt, nil)

		// --- When ---
		have, err := scn.Scan(t.Context(), []string{root})

		// --- Then ---
		assert.Len(t, 1, have)
		assert.NoError(t, err)
		assert.NotNil(t, have["example.com/a"])
	})

	t.Run("more than one root is walked", func(t *testing.T) {
		// --- Given ---
		rootA := t.TempDir()
		rootB := t.TempDir()
		writeMod(t, "module example.com/a\n", rootA, "a")
		writeMod(t, "module example.com/b\n", rootB, "b")
		scn := NewScanner(Filter{}, nil)

		// --- When ---
		have, err := scn.Scan(t.Context(), []string{rootA, rootB})

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 2, have)
	})

	t.Run("symlinked root", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		writeMod(t, "module example.com/a\n", root, "a")

		lnk := filepath.Join(t.TempDir(), "link")
		must.Nil(os.Symlink(root, lnk))

		scn := NewScanner(Filter{}, nil)

		// --- When ---
		have, err := scn.Scan(t.Context(), []string{lnk})

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 1, have)
		want := filepath.Join(root, "a")
		assert.Equal(t, want, have["example.com/a"].Dir)
	})

	t.Run("the module found first wins", func(t *testing.T) {
		// --- Given ---
		rootA := t.TempDir()
		rootB := t.TempDir()
		writeMod(t, "module example.com/a\n", rootA, "a")
		writeMod(t, "module example.com/a\n", rootB, "dup")
		scn := NewScanner(Filter{}, nil)

		// --- When ---
		have, err := scn.Scan(t.Context(), []string{rootA, rootB})

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 1, have)
		want := filepath.Join(rootA, "a")
		assert.Equal(t, want, have["example.com/a"].Dir)
	})

	t.Run("progress is reported", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		writeMod(t, "module example.com/a\n", root, "a")
		var msgs []string
		logf := func(format string, args ...any) {
			msgs = append(msgs, fmt.Sprintf(format, args...))
		}
		scn := NewScanner(Filter{}, logf)

		// --- When ---
		_, err := scn.Scan(t.Context(), []string{root})

		// --- Then ---
		assert.NoError(t, err)
		want := []string{"scanning " + root, "found example.com/a"}
		assert.Equal(t, want, msgs)
	})

	t.Run("error - context cancelled", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		writeMod(t, "module example.com/a\n", root, "a")

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		scn := NewScanner(Filter{}, nil)

		// --- When ---
		have, err := scn.Scan(ctx, []string{root})

		// --- Then ---
		assert.ErrorIs(t, context.Canceled, err)
		assert.Nil(t, have)
	})

	t.Run("error - root does not exist", func(t *testing.T) {
		// --- Given ---
		root := filepath.Join(t.TempDir(), "nope")
		scn := NewScanner(Filter{}, nil)

		// --- When ---
		have, err := scn.Scan(t.Context(), []string{root})

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.ErrorContain(t, "scan "+root, err)
		assert.Nil(t, have)
	})

	t.Run("error - malformed module file", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		writeMod(t, "module\n", root, "a")
		scn := NewScanner(Filter{}, nil)

		// --- When ---
		have, err := scn.Scan(t.Context(), []string{root})

		// --- Then ---
		assert.ErrorContain(t, "parse module file", err)
		assert.Nil(t, have)
	})
}

func Test_skipDir_tabular(t *testing.T) {
	tt := []struct {
		testN string

		name string
		want bool
	}{
		{"plain", "src", false},
		{"dot", ".git", true},
		{"underscore", "_old", true},
		{"testdata", "testdata", true},
		{"vendor", "vendor", true},
		{"module cache entry", "ring@v0.8.0", true},
		{"inner dot", "a.b", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := skipDir(tc.name)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
