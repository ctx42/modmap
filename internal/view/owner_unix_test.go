// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build unix

package view

import (
	"os"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
)

func Test_ownedByUser(t *testing.T) {
	t.Run("own directory", func(t *testing.T) {
		// --- Given ---
		inf := must.Value(os.Lstat(t.TempDir()))

		// --- When ---
		have := ownedByUser(inf)

		// --- Then ---
		assert.True(t, have)
	})

	t.Run("root directory", func(t *testing.T) {
		// --- Given ---
		if os.Geteuid() == 0 {
			t.Skip("the root directory is the root user's own")
		}
		inf := must.Value(os.Lstat("/"))

		// --- When ---
		have := ownedByUser(inf)

		// --- Then ---
		assert.False(t, have)
	})
}
