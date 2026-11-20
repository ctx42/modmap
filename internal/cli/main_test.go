// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"path/filepath"
	"runtime/debug"
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

	t.Run("version is written to stdout", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStdout()
		rng := tst.Ring("--version")

		// --- When ---
		have := Main(t.Context(), rng)

		// --- Then ---
		assert.Equal(t, 0, have)
		inf, _ := debug.ReadBuildInfo()
		assert.Equal(t, "modmap "+version(inf)+"\n", tst.Stdout())
	})

	t.Run("the diagram is written to the output file", func(t *testing.T) {
		// --- Given ---
		src := t.TempDir()
		writeMod(t, "module example.com/a\n", src, "a")
		out := filepath.Join(t.TempDir(), "diagram.svg")
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
		out := filepath.Join(t.TempDir(), "diagram.svg")
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
		want := "modmap: either -o or --web is required\n"
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

func Test_version_tabular(t *testing.T) {
	tt := []struct {
		testN string

		inf  *debug.BuildInfo
		want string
	}{
		{"no build info", nil, "(devel)"},
		{"no version", &debug.BuildInfo{}, "(devel)"},
		{
			"devel",
			&debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			"(devel)",
		},
		{
			"release",
			&debug.BuildInfo{Main: debug.Module{Version: "v0.6.0"}},
			"v0.6.0",
		},
		{
			"pseudo-version",
			&debug.BuildInfo{Main: debug.Module{
				Version: "v0.6.1-0.20261120174235-76f1e0d2d5c8+dirty",
			}},
			"v0.6.1-0.20261120174235-76f1e0d2d5c8+dirty",
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := version(tc.inf)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
