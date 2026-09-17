// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package svg

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ctx42/goldkit/pkg/goldkit"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
)

func Test_layout_boxTop(t *testing.T) {
	// --- Given ---
	lay := layout{height: 2919, levels: 5}

	// --- When ---
	have := lay.boxTop(0)

	// --- Then ---
	assert.Equal(t, 2919.0-marginY-boxHeight, have)
	assert.Equal(t, have-(boxHeight+gapY), lay.boxTop(1))
}

func Test_layout_boxLeft(t *testing.T) {
	// --- Given ---
	lay := layout{boxW: 500}

	// --- When ---
	have := lay.boxLeft(0)

	// --- Then ---
	assert.Equal(t, marginX, have)
	assert.Equal(t, marginX+500+gapX, lay.boxLeft(1))
}

func Test_NewRenderer(t *testing.T) {
	// --- When ---
	have, err := NewRenderer()

	// --- Then ---
	assert.NoError(t, err)
	assert.NotNil(t, have.fnt)
}

func Test_Renderer_Render(t *testing.T) {
	t.Run("three levels", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"example.com/app":  {"example.com/lib"},
			"example.com/lib":  {"example.com/core"},
			"example.com/core": nil,
			"example.com/tool": nil,
		})
		rnd := must.Value(NewRenderer())
		buf := &bytes.Buffer{}

		// --- When ---
		err := rnd.Render(grp, buf)

		// --- Then ---
		assert.NoError(t, err)
		gld := goldkit.Create(t, "testdata/three_levels.yml", nil)
		gld.Assert(trimFont(buf.Bytes()))
	})

	t.Run("ctx42 reference", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(loadDeps(t, "testdata/ctx42_modules.json"))
		rnd := must.Value(NewRenderer())
		buf := &bytes.Buffer{}

		// --- When ---
		err := rnd.Render(grp, buf)

		// --- Then ---
		assert.NoError(t, err)
		gld := goldkit.Create(t, "testdata/ctx42_reference.yml", nil)
		gld.Assert(trimFont(buf.Bytes()))
	})

	t.Run("no modules", func(t *testing.T) {
		// --- Given ---
		rnd := must.Value(NewRenderer())
		buf := &bytes.Buffer{}

		// --- When ---
		err := rnd.Render(newGraph(nil), buf)

		// --- Then ---
		assert.NoError(t, err)
		assert.Contain(t, `viewBox="0 0 432 400"`, buf.String())
		assert.NotContain(t, `class="module"`, buf.String())
		assert.NotContain(t, ":hover", buf.String())
	})

	t.Run("every module can be focused and pinned", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{"example.com/a": nil})
		rnd := must.Value(NewRenderer())
		buf := &bytes.Buffer{}

		// --- When ---
		err := rnd.Render(grp, buf)

		// --- Then ---
		assert.NoError(t, err)
		want := `<g id="m0" class="module" tabindex="0"`
		assert.Contain(t, want, buf.String())

		wPin := ".module:focus rect{stroke-dasharray:none;}"
		assert.Contain(t, wPin, buf.String())
	})

	t.Run("labels and dependents are escaped", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			`example.com/a&"b`: nil,
		})
		rnd := must.Value(NewRenderer())
		buf := &bytes.Buffer{}

		// --- When ---
		err := rnd.Render(grp, buf)

		// --- Then ---
		assert.NoError(t, err)
		assert.NotContain(t, `a&"b`, buf.String())
		assert.Contain(t, `a&amp;&#34;b`, buf.String())
	})

	t.Run("error - writer fails", func(t *testing.T) {
		// --- Given ---
		rnd := must.Value(NewRenderer())
		dst := &failWriter{}

		// --- When ---
		err := rnd.Render(newGraph(nil), dst)

		// --- Then ---
		assert.ErrorContain(t, "write map", err)
	})
}

func Test_Renderer_layout(t *testing.T) {
	t.Run("the widest label sets the box width", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"example.com/a":              nil,
			"example.com/very/long/path": nil,
		})
		rnd := must.Value(NewRenderer())

		// --- When ---
		have := rnd.layout(grp)

		// --- Then ---
		widest := rnd.fnt.Width("example.com/very/long/path", textSize)
		assert.Equal(t, widest+2*boxPad, have.boxW)
		assert.Equal(t, 1, have.levels)
		assert.Equal(t, 2*marginX+2*have.boxW+gapX, have.width)
		assert.Equal(t, 2*marginY+boxHeight, have.height)
	})

	t.Run("levels add height", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"example.com/a": {"example.com/b"},
			"example.com/b": nil,
		})
		rnd := must.Value(NewRenderer())

		// --- When ---
		have := rnd.layout(grp)

		// --- Then ---
		assert.Equal(t, 2, have.levels)
		assert.Equal(t, 2*marginY+2*boxHeight+gapY, have.height)
	})
}

func Test_moduleIDs(t *testing.T) {
	// --- Given ---
	grp := newGraph(map[string][]string{
		"example.com/b": {"example.com/a"},
		"example.com/a": nil,
		"example.com/c": nil,
	})

	// --- When ---
	have := moduleIDs(grp)

	// --- Then ---
	want := map[string]string{
		"example.com/a": "m0",
		"example.com/c": "m1",
		"example.com/b": "m2",
	}
	assert.Equal(t, want, have)
}

func Test_relClasses(t *testing.T) {
	t.Run("a chain lights from either end", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"example.com/app":  {"example.com/lib"},
			"example.com/lib":  {"example.com/core"},
			"example.com/core": nil,
		})
		ids := moduleIDs(grp)

		// --- When ---
		have := relClasses(grp, ids)

		// --- Then ---
		wCore := []string{"rm1", "rm2"}
		assert.Equal(t, wCore, have["example.com/core"])
		wLib := []string{"rm0", "rm2"}
		assert.Equal(t, wLib, have["example.com/lib"])
		wApp := []string{"rm0", "rm1"}
		assert.Equal(t, wApp, have["example.com/app"])
	})

	t.Run("modules on separate chains stay unrelated", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"example.com/app":  {"example.com/core"},
			"example.com/tool": nil,
			"example.com/core": nil,
		})
		ids := moduleIDs(grp)

		// --- When ---
		have := relClasses(grp, ids)

		// --- Then ---
		assert.Equal(t, []string{"rm0"}, have["example.com/app"])
		assert.Equal(t, []string{"rm2"}, have["example.com/core"])
		assert.Empty(t, have["example.com/tool"])
	})

	t.Run("a module on its own is related to nothing", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{"example.com/a": nil})
		ids := moduleIDs(grp)

		// --- When ---
		have := relClasses(grp, ids)

		// --- Then ---
		assert.Empty(t, have)
	})
}

func Test_hoverCSS(t *testing.T) {
	t.Run("one rule per module and the base rules", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"example.com/app": {"example.com/lib"},
			"example.com/lib": nil,
		})
		ids := moduleIDs(grp)

		// --- When ---
		have := hoverCSS(grp, ids)

		// --- Then ---
		assert.Contain(t, cssBase, have)
		wHover := "svg:not(:has(.module:focus)):has(#m0:hover) #m0," +
			"svg:not(:has(.module:focus)):has(#m0:hover) .rm0,"
		assert.Contain(t, wHover, have)

		wPin := "svg:has(#m0:focus) #m0," +
			"svg:has(#m0:focus) .rm0{opacity:1;}"
		assert.Contain(t, wPin, have)

		wApp := "svg:has(#m1:focus) #m1," +
			"svg:has(#m1:focus) .rm1{opacity:1;}"
		assert.Contain(t, wApp, have)
	})

	t.Run("no modules need no rules", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(nil)

		// --- When ---
		have := hoverCSS(grp, moduleIDs(grp))

		// --- Then ---
		assert.Equal(t, "", have)
	})
}

func Test_num_tabular(t *testing.T) {
	tt := []struct {
		testN string

		val  float64
		want string
	}{
		{"integer", 231, "231"},
		{"fraction", 2.5, "2.5"},
		{"negative", -50.25, "-50.25"},
		{"zero", 0, "0"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := num(tc.val)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

// failWriter is a writer failing every write.
type failWriter struct{}

func (fwr *failWriter) Write([]byte) (int, error) {
	return 0, errors.New("write refused")
}
