// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/ring/pkg/ring/ringtest"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_confirm(t *testing.T) {
	t.Run("a narrow map needs no confirmation", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t)
		rng := tst.Ring()

		// --- When ---
		have, err := confirm(rng, wideLevel, false)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have)
	})

	t.Run("a wide map is confirmed by yes", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		rng := tst.Ring()

		// --- When ---
		have, err := confirm(rng, wideLevel+1, true)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have)
		want := "the widest level holds 21 modules"
		assert.Contain(t, want, tst.Stderr())
	})

	t.Run("a wide map renders with nobody to ask", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		tst.SetStdin(&bytes.Buffer{})
		rng := tst.Ring()

		// --- When ---
		have, err := confirm(rng, wideLevel+1, false)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have)
		assert.Contain(t, "the map will be very wide", tst.Stderr())
	})
}

func Test_prompt(t *testing.T) {
	t.Run("the question is asked", func(t *testing.T) {
		// --- Given ---
		tst := ringtest.New(t).WetStderr()
		tst.SetStdin(bytes.NewBufferString("y\n"))
		rng := tst.Ring()

		// --- When ---
		have, err := prompt(rng, "render it? ")

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, have)
		assert.Equal(t, "render it? ", tst.Stderr())
	})

}

func Test_prompt_tabular(t *testing.T) {
	tt := []struct {
		testN string

		answer string
		want   bool
	}{
		{"short yes", "y\n", true},
		{"upper case yes", "Y\n", true},
		{"full yes", "yes\n", true},
		{"padded yes", " y \n", true},
		{"short no", "n\n", false},
		{"empty line", "\n", false},
		{"other word", "nope\n", false},
		{"closed input", "", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			tst := ringtest.New(t).WetStderr()
			tst.SetStdin(bytes.NewBufferString(tc.answer))

			// --- When ---
			have, err := prompt(tst.Ring(), "render it? ")

			// --- Then ---
			assert.NoError(t, err)
			assert.Equal(t, tc.want, have)

			assert.Equal(t, "render it? ", tst.Stderr())
		})
	}
}

func Test_isTTY(t *testing.T) {
	t.Run("a buffer is not a terminal", func(t *testing.T) {
		// --- Given ---
		src := &bytes.Buffer{}

		// --- When ---
		have := isTTY(src)

		// --- Then ---
		assert.False(t, have)
	})

	t.Run("a regular file is not a terminal", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Create(t, "", filepath.Join(t.TempDir(), "file"))
		fil, err := os.Open(pth)
		assert.NoError(t, err)
		t.Cleanup(func() { _ = fil.Close() })

		// --- When ---
		have := isTTY(fil)

		// --- Then ---
		assert.False(t, have)
	})

	t.Run("a closed file is not a terminal", func(t *testing.T) {
		// --- Given ---
		pth := oskit.Create(t, "", filepath.Join(t.TempDir(), "file"))
		fil, err := os.Open(pth)
		assert.NoError(t, err)
		assert.NoError(t, fil.Close())

		// --- When ---
		have := isTTY(fil)

		// --- Then ---
		assert.False(t, have)
	})
}
