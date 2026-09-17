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

func Test_Main(t *testing.T) {
	t.Run("help is written to stderr", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("--help")

		// --- When ---
		have := Main(context.Background(), rng)

		// --- Then ---
		assert.Equal(t, 0, have)
		assert.Contain(t, "Usage:", tst.Stderr())
	})

	t.Run("the map is written to the output file", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, "module example.com/a\n", src, "a")
		out := filepath.Join(t.TempDir(), "map.svg")
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("-o", out, src)

		// --- When ---
		have := Main(context.Background(), rng)

		// --- Then ---
		assert.Equal(t, 0, have)
		assert.Contain(t, "example.com/a", oskit.ReadFileStr(t, out))
		assert.Contain(t, "found example.com/a", tst.Stderr())
	})

	t.Run("error - the run fails", func(t *testing.T) {
		// --- Given ---
		gone := filepath.Join(t.TempDir(), "gone")
		out := filepath.Join(t.TempDir(), "map.svg")
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring("-o", out, gone)

		// --- When ---
		have := Main(context.Background(), rng)

		// --- Then ---
		assert.Equal(t, 1, have)
		assert.Contain(t, "modmap: scan "+gone, tst.Stderr())
	})

	t.Run("error - missing output option", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring(t.TempDir())

		// --- When ---
		have := Main(context.Background(), rng)

		// --- Then ---
		assert.Equal(t, 1, have)
		want := "modmap: the -o option is required\n"
		assert.Contain(t, want, tst.Stderr())
		assert.Contain(t, "Usage:", tst.Stderr())
	})

	t.Run("error - no directory", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		out := filepath.Join(t.TempDir(), "out.svg")
		rng := tst.Ring("-o", out)

		// --- When ---
		have := Main(context.Background(), rng)

		// --- Then ---
		want := "modmap: directory to scan is required\n"
		assert.Equal(t, 1, have)
		assert.Contain(t, want, tst.Stderr())
		assert.Contain(t, "Usage:", tst.Stderr())
	})
}
