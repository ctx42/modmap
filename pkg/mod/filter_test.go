// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mod

import (
	"testing"

	"github.com/ctx42/testing/pkg/assert"
)

func Test_ValidateGlob(t *testing.T) {
	t.Run("valid glob", func(t *testing.T) {
		// --- When ---
		err := ValidateGlob("github.com/ctx42/*")

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error - malformed glob", func(t *testing.T) {
		// --- When ---
		err := ValidateGlob("[")

		// --- Then ---
		assert.ErrorIs(t, ErrInvGlob, err)
		assert.ErrorContain(t, "[", err)
	})
}

func Test_NewFilter(t *testing.T) {
	// --- Given ---
	include := []string{"example.com/*"}
	exclude := []string{"example.com/x"}

	// --- When ---
	have := NewFilter(include, exclude)

	// --- Then ---
	assert.Equal(t, include, have.include)
	assert.Equal(t, exclude, have.exclude)
}

func Test_Filter_Match_tabular(t *testing.T) {
	tt := []struct {
		testN string

		include []string
		exclude []string
		pth     string
		want    bool
	}{
		{"zero value matches", nil, nil, "example.com/a", true},
		{
			"include match",
			[]string{"example.com/*"},
			nil,
			"example.com/a",
			true,
		},
		{
			"include miss",
			[]string{"example.com/*"},
			nil,
			"other.com/a",
			false,
		},
		{
			"star does not cross separator",
			[]string{"example.com/*"},
			nil,
			"example.com/a/b",
			false,
		},
		{
			"exclude match",
			nil,
			[]string{"example.com/a"},
			"example.com/a",
			false,
		},
		{
			"exclude wins over include",
			[]string{"example.com/*"},
			[]string{"example.com/a"},
			"example.com/a",
			false,
		},
		{"malformed glob", []string{"["}, nil, "[", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			flt := NewFilter(tc.include, tc.exclude)

			// --- When ---
			have := flt.Match(tc.pth)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
