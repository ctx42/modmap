// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mod

import (
	"path"
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
		assert.ErrorIs(t, path.ErrBadPattern, err)
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

func Test_AnyOf(t *testing.T) {
	// --- Given ---
	one := NewFilter([]string{"example.com/*"}, nil)
	two := NewFilter(nil, []string{"other.com/x"})

	// --- When ---
	have := AnyOf(one, two)

	// --- Then ---
	assert.Equal(t, []Filter{one, two}, have.anyOf)
	assert.Nil(t, have.include)
	assert.Nil(t, have.exclude)
}

func Test_Filter_Match_anyOf_tabular(t *testing.T) {
	ctx42 := NewFilter([]string{"github.com/ctx42/*"}, nil)
	work := NewFilter(
		[]string{"github.com/ctx42/*", "github.com/customer/*"},
		[]string{"github.com/ctx42/*"},
	)

	tt := []struct {
		testN string

		flt  Filter
		pth  string
		want bool
	}{
		{"first passes", AnyOf(ctx42, work), "github.com/ctx42/a", true},
		{"second passes", AnyOf(ctx42, work), "github.com/customer/a", true},
		{"none passes", AnyOf(ctx42, work), "golang.org/x/mod", false},
		{"exclude of one", AnyOf(work), "github.com/ctx42/a", false},
		{"no filters pass all", AnyOf(), "golang.org/x/mod", true},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := tc.flt.Match(tc.pth)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
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
