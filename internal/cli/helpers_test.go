// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"errors"
	"testing"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"

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
