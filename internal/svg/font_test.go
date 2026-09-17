// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package svg

import (
	"strings"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
)

func Test_NewFont(t *testing.T) {
	// --- When ---
	have, err := NewFont()

	// --- Then ---
	assert.NoError(t, err)
	assert.NotNil(t, have.sft)
	assert.True(t, len(have.data) > 0)
}

func Test_Font_Width(t *testing.T) {
	t.Run("width grows with the text", func(t *testing.T) {
		// --- Given ---
		fnt := must.Value(NewFont())

		// --- When ---
		have := fnt.Width("github.com/ctx42/testing", textSize)

		// --- Then ---
		short := fnt.Width("github.com/ctx42/xrr", textSize)
		assert.True(t, have > short)
	})

	t.Run("width scales with the font size", func(t *testing.T) {
		// --- Given ---
		fnt := must.Value(NewFont())

		// --- When ---
		have := fnt.Width("modmap", 72)

		// --- Then ---
		assert.Delta(t, 2*fnt.Width("modmap", 36), 1.0, have)
	})

	t.Run("empty text has no width", func(t *testing.T) {
		// --- Given ---
		fnt := must.Value(NewFont())

		// --- When ---
		have := fnt.Width("", textSize)

		// --- Then ---
		assert.Equal(t, 0.0, have)
	})

	t.Run("runes without a glyph are skipped", func(t *testing.T) {
		// --- Given ---
		fnt := must.Value(NewFont())

		// --- When ---
		have := fnt.Width("ab\U0010FFFF", textSize)

		// --- Then ---
		assert.Equal(t, fnt.Width("ab", textSize), have)
	})
}

func Test_Font_CapHeight(t *testing.T) {
	// --- Given ---
	fnt := must.Value(NewFont())

	// --- When ---
	have := fnt.CapHeight(textSize)

	// --- Then ---
	assert.True(t, have > 0 && have < textSize)
}

func Test_Font_DataURI(t *testing.T) {
	// --- Given ---
	fnt := must.Value(NewFont())

	// --- When ---
	have := fnt.DataURI()

	// --- Then ---
	assert.True(t, strings.HasPrefix(have, "data:font/ttf;base64,"))
	assert.True(t, len(have) > len(fnt.data))
}
