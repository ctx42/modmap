// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package web

import (
	"os/exec"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
)

func Test_open(t *testing.T) {
	// --- Given ---
	// Nothing on the PATH can open a browser, so nothing is launched.
	// exec.LookPath reads the process environment, which no ring
	// reaches.
	t.Setenv("PATH", t.TempDir())

	// --- When ---
	err := open("http://127.0.0.1:1/")

	// --- Then ---
	assert.ErrorIs(t, ErrNoBrowser, err)
}

func Test_openWith(t *testing.T) {
	t.Run("the first command present is used", func(t *testing.T) {
		// --- Given ---
		if _, err := exec.LookPath("true"); err != nil {
			t.Skip("no true command on this machine")
		}
		cmds := [][]string{{"modmap-no-such-opener"}, {"true"}}

		// --- When ---
		err := openWith(cmds, "http://127.0.0.1:1/")

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error - no command is present", func(t *testing.T) {
		// --- Given ---
		cmds := [][]string{{"modmap-no-such-opener"}}

		// --- When ---
		err := openWith(cmds, "http://127.0.0.1:1/")

		// --- Then ---
		assert.ErrorIs(t, ErrNoBrowser, err)
	})

	t.Run("error - no commands at all", func(t *testing.T) {
		// --- When ---
		err := openWith(nil, "http://127.0.0.1:1/")

		// --- Then ---
		assert.ErrorIs(t, ErrNoBrowser, err)
	})
}

func Test_openers_tabular(t *testing.T) {
	tt := []struct {
		testN string

		goos string
		want string
	}{
		{"macos", "darwin", "open"},
		{"windows", "windows", "rundll32"},
		{"linux", "linux", "xdg-open"},
		{"unknown", "plan9", "xdg-open"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := openers(tc.goos)

			// --- Then ---
			assert.Equal(t, tc.want, have[0][0])
		})
	}
}
