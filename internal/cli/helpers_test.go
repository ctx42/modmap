// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"

	"github.com/ctx42/modmap/pkg/mod"
)

func Test_appendGlob(t *testing.T) {
	t.Run("appends to an empty list", func(t *testing.T) {
		// --- Given ---
		var dst []string

		// --- When ---
		err := appendGlob(&dst, "github.com/ctx42/*")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"github.com/ctx42/*"}, dst)
	})

	t.Run("keeps the order of the globs", func(t *testing.T) {
		// --- Given ---
		dst := []string{"a/*"}

		// --- When ---
		err := appendGlob(&dst, "b/*")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"a/*", "b/*"}, dst)
	})

	t.Run("error - malformed glob", func(t *testing.T) {
		// --- Given ---
		var dst []string

		// --- When ---
		err := appendGlob(&dst, "[")

		// --- Then ---
		assert.ErrorIs(t, mod.ErrInvGlob, err)
		assert.Nil(t, dst)
	})
}

func Test_abs_tabular(t *testing.T) {
	tt := []struct {
		testN string

		wd   string
		pth  string
		want string
	}{
		{"absolute path", "/home/u", "/etc/x", "/etc/x"},
		{"relative path", "/home/u", "a/b", "/home/u/a/b"},
		{"dot path", "/home/u", ".", "/home/u"},
		{"parent path", "/home/u", "../a", "/home/a"},
		{"empty path", "/home/u", "", "/home/u"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			assert.Equal(t, tc.want, abs(tc.wd, tc.pth))
		})
	}
}

func Test_replaceFile(t *testing.T) {
	t.Run("new file", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "map.svg")

		// --- When ---
		err := replaceFile(pth, []byte("<svg/>"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "<svg/>", oskit.ReadFileStr(t, pth))
		inf := must.Value(os.Stat(pth))
		assert.Equal(t, os.FileMode(0o644), inf.Mode().Perm())
	})

	t.Run("existing file keeps its mode", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		pth := oskit.Create(t, "<svg id=\"old\"/>", dir, "map.svg")
		must.Nil(os.Chmod(pth, 0o640))

		// --- When ---
		err := replaceFile(pth, []byte("<svg/>"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "<svg/>", oskit.ReadFileStr(t, pth))
		inf := must.Value(os.Stat(pth))
		assert.Equal(t, os.FileMode(0o640), inf.Mode().Perm())
		assert.Len(t, 1, oskit.List(t, dir))
	})

	t.Run("link target is replaced", func(t *testing.T) {
		// --- Given ---
		tgt := oskit.Create(t, "<svg id=\"old\"/>", t.TempDir(), "map.svg")

		lnk := filepath.Join(t.TempDir(), "link.svg")
		must.Nil(os.Symlink(tgt, lnk))

		// --- When ---
		err := replaceFile(lnk, []byte("<svg/>"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "<svg/>", oskit.ReadFileStr(t, tgt))
		inf := must.Value(os.Lstat(lnk))
		assert.True(t, inf.Mode()&os.ModeSymlink != 0)
	})

	t.Run("error - directory does not exist", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "gone", "map.svg")

		// --- When ---
		err := replaceFile(pth, []byte("<svg/>"))

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.False(t, oskit.PathExists(t, pth))
	})

	t.Run("error - name taken by a directory", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		pth := oskit.MkdirAll(t, dir, "map.svg")

		// --- When ---
		err := replaceFile(pth, []byte("<svg/>"))

		// --- Then ---
		var lne *os.LinkError
		assert.ErrorAs(t, &lne, err)
		assert.Len(t, 1, oskit.List(t, dir))
	})
}

func Test_fail(t *testing.T) {
	// --- Given ---
	tst := ringtest.New(t).WetStderr()
	rng := tst.Ring()
	err := errors.New("boom")

	// --- When ---
	fail(rng, err)

	// --- Then ---
	assert.Equal(t, "modmap: boom\n", tst.Stderr())
}

func Test_logger(t *testing.T) {
	// --- Given ---
	tst := ringtest.New(t).WetStderr()
	logf := logger(tst.Ring())

	// --- When ---
	logf("found %s", "example.com/a")

	// --- Then ---
	assert.Equal(t, "found example.com/a\n", tst.Stderr())
}
