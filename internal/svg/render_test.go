// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package svg

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ctx42/goldkit/pkg/goldkit"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"

	"github.com/ctx42/modmap/pkg/mod"
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

func Test_layout_named(t *testing.T) {
	t.Run("one column", func(t *testing.T) {
		// --- Given ---
		lay := layout{cols: []column{{}}}

		// --- When ---
		have := lay.named()

		// --- Then ---
		assert.False(t, have)
	})

	t.Run("two columns", func(t *testing.T) {
		// --- Given ---
		lay := layout{cols: []column{{}, {}}}

		// --- When ---
		have := lay.named()

		// --- Then ---
		assert.True(t, have)
	})
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
		err := rnd.Render(grp, nil, buf)

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
		err := rnd.Render(grp, nil, buf)

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
		err := rnd.Render(newGraph(nil), nil, buf)

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
		err := rnd.Render(grp, nil, buf)

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
		err := rnd.Render(grp, nil, buf)

		// --- Then ---
		assert.NoError(t, err)
		assert.NotContain(t, `a&"b`, buf.String())
		assert.Contain(t, `a&amp;&#34;b`, buf.String())
	})

	t.Run("one named column draws as no columns", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"example.com/app": {"example.com/lib"},
			"example.com/lib": nil,
		})
		cols := []Column{{Name: "ctx42"}}
		rnd := must.Value(NewRenderer())
		buf := &bytes.Buffer{}

		// --- When ---
		err := rnd.Render(grp, cols, buf)

		// --- Then ---
		assert.NoError(t, err)
		want := &bytes.Buffer{}
		must.Nil(rnd.Render(grp, nil, want))
		assert.Equal(t, want.String(), buf.String())
		assert.NotContain(t, ">ctx42</text>", buf.String())
	})

	t.Run("two maps side by side", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"github.com/ctx42/testing": nil,
			"github.com/ctx42/ring":    nil,
			"github.com/customer/a":    {"github.com/ctx42/testing"},
		})
		cols := []Column{
			{
				Name:   "ctx42",
				Filter: mod.NewFilter([]string{"github.com/ctx42/*"}, nil),
			},
			{
				Name:   "work",
				Filter: mod.NewFilter([]string{"github.com/*/*"}, nil),
			},
		}
		rnd := must.Value(NewRenderer())
		buf := &bytes.Buffer{}

		// --- When ---
		err := rnd.Render(grp, cols, buf)

		// --- Then ---
		assert.NoError(t, err)
		have := buf.String()
		for _, pth := range []string{
			"github.com/ctx42/ring",
			"github.com/ctx42/testing",
			"github.com/customer/a",
		} {
			want := `data-module="` + pth + `"`
			assert.Equal(t, 1, strings.Count(have, want))
		}
		wApp := `<g id="m2" class="module rm1" tabindex="0"` +
			` data-module="github.com/customer/a" data-level="1"`
		assert.Contain(t, wApp, have)
		_, app, _ := strings.Cut(have, wApp)
		app, _, _ = strings.Cut(app, "</g>")
		assert.Contain(t, `class="badge bm1" fill`, app)
		wTst := `<g id="m1" class="module rm2" tabindex="0"` +
			` data-module="github.com/ctx42/testing" data-level="0"` +
			` data-dependents="github.com/customer/a">`
		assert.Contain(t, wTst, have)

		assert.Equal(t, 1, strings.Count(have, ">ctx42</text>"))
		assert.Equal(t, 1, strings.Count(have, ">work</text>"))

		lay := rnd.layout(grp, cols)
		x := num(lay.cols[1].left - gapX/2)
		wDiv := `<line x1="` + x + `" y1="200" x2="` + x + `" `
		assert.Contain(t, wDiv, have)
		wWidth := ` width="` + num(lay.boxW) + `" height="231"`
		assert.Equal(t, 3, strings.Count(have, wWidth))
	})

	t.Run("error - writer fails", func(t *testing.T) {
		// --- Given ---
		rnd := must.Value(NewRenderer())
		dst := &failWriter{}

		// --- When ---
		err := rnd.Render(newGraph(nil), nil, dst)

		// --- Then ---
		assert.ErrorIs(t, errWrite, err)
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
		have := rnd.layout(grp, nil)

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
		have := rnd.layout(grp, nil)

		// --- Then ---
		assert.Equal(t, 2, have.levels)
		assert.Equal(t, 2*marginY+2*boxHeight+gapY, have.height)
	})

	t.Run("one column holds every module", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{"a": nil, "b": nil})
		rnd := must.Value(NewRenderer())

		// --- When ---
		have := rnd.layout(grp, nil)

		// --- Then ---
		width := 2*have.boxW + gapX
		assert.Equal(t, []column{{left: marginX, width: width}}, have.cols)
		wLefts := map[string]float64{
			"a": marginX,
			"b": marginX + have.boxW + gapX,
		}
		assert.Equal(t, wLefts, have.lefts)
	})

	t.Run("columns are as wide as their widest level", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"one/a": nil,
			"two/a": nil,
			"two/b": nil,
			"two/c": {"one/a"},
		})
		cols := []Column{
			{Name: "one", Filter: mod.NewFilter([]string{"one/*"}, nil)},
			{Name: "two", Filter: mod.NewFilter([]string{"two/*"}, nil)},
		}
		rnd := must.Value(NewRenderer())

		// --- When ---
		have := rnd.layout(grp, cols)

		// --- Then ---
		step := have.boxW + gapX
		wTwo := marginX + have.boxW + gapX
		want := []column{
			{name: "one", left: marginX, width: have.boxW},
			{name: "two", left: wTwo, width: 2*have.boxW + gapX},
		}
		assert.Equal(t, want, have.cols)
		wLefts := map[string]float64{
			"one/a": marginX,
			"two/a": wTwo,
			"two/b": wTwo + step,
			"two/c": wTwo,
		}
		assert.Equal(t, wLefts, have.lefts)
		assert.Equal(t, 2*marginX+3*have.boxW+2*gapX, have.width)
		wHeight := 2*marginY + 3*boxHeight + 2*gapY
		assert.Equal(t, wHeight, have.height)
	})

	t.Run("an empty column has no width", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{"a": nil})
		cols := []Column{
			{Name: "one"},
			{Name: "two", Filter: mod.NewFilter([]string{"b"}, nil)},
		}
		rnd := must.Value(NewRenderer())

		// --- When ---
		have := rnd.layout(grp, cols)

		// --- Then ---
		wTwo := column{name: "two", left: marginX + have.boxW + gapX}
		assert.Equal(t, wTwo, have.cols[1])
		assert.Equal(t, 2*marginX+have.boxW+gapX, have.width)
	})
}

func Test_Renderer_head(t *testing.T) {
	// --- Given ---
	rnd := must.Value(NewRenderer())
	buf := &bytes.Buffer{}

	// --- When ---
	rnd.head(buf, layout{width: 100, height: 50}, ".x{}\n")

	// --- Then ---
	have := buf.String()
	assert.Contain(t, `viewBox="0 0 100 50" width="100" height="50">`, have)
	assert.Contain(t, `@font-face{font-family:"Modmap Sans";`, have)
	assert.Contain(t, ".x{}\n</style></defs>", have)
	assert.Contain(t, `<rect x="0" y="0" width="100" height="50"`, have)
}

func Test_Renderer_levels(t *testing.T) {
	// --- Given ---
	rnd := must.Value(NewRenderer())
	buf := &bytes.Buffer{}

	// --- When ---
	rnd.levels(buf, layout{width: 100, height: 300, levels: 2})

	// --- Then ---
	have := buf.String()
	assert.Equal(t, 2, strings.Count(have, "<line "))
	assert.Contain(t, ">LEVEL 0</text>", have)
	assert.Contain(t, ">LEVEL 1</text>", have)
}

func Test_Renderer_columns(t *testing.T) {
	t.Run("names and dividers", func(t *testing.T) {
		// --- Given ---
		rnd := must.Value(NewRenderer())
		buf := &bytes.Buffer{}
		lay := layout{
			height: 2000,
			cols: []column{
				{name: "one", left: 100, width: 200},
				{name: "a&b", left: 560, width: 400},
				{name: "three", left: 1220, width: 0},
			},
		}

		// --- When ---
		rnd.columns(buf, lay)

		// --- Then ---
		have := buf.String()
		nameY := num(marginY + (boxHeight+rnd.fnt.CapHeight(levelSize))/2)
		wOne := `<text x="200" y="` + nameY + `" fill="` + colorLevelText +
			`" font-size="96" text-anchor="middle">one</text>`
		assert.Contain(t, wOne, have)
		assert.Contain(t, `<text x="760" `, have)
		assert.Contain(t, `>a&amp;b</text>`, have)
		assert.Contain(t, `<text x="1220" `, have)

		bottom := num(lay.boxTop(0) + boxHeight + gapY/2)
		wDiv := `<line x1="430" y1="200" x2="430" y2="` + bottom + `"`
		assert.Contain(t, wDiv, have)
		assert.Contain(t, `<line x1="1090" y1="200" x2="1090" `, have)
		assert.Equal(t, 2, strings.Count(have, "<line "))
		assert.Equal(t, 3, strings.Count(have, "<text "))
	})

	t.Run("one column has no names and no dividers", func(t *testing.T) {
		// --- Given ---
		rnd := must.Value(NewRenderer())
		buf := &bytes.Buffer{}
		lay := layout{cols: []column{{name: "one", left: 100, width: 200}}}

		// --- When ---
		rnd.columns(buf, lay)

		// --- Then ---
		assert.Equal(t, "", buf.String())
	})
}

func Test_Renderer_line(t *testing.T) {
	// --- Given ---
	rnd := must.Value(NewRenderer())
	buf := &bytes.Buffer{}

	// --- When ---
	rnd.line(buf, 1, 3.5, 2, 4)

	// --- Then ---
	want := "" +
		`<line x1="1" y1="3.5" x2="2" y2="4" stroke="` + colorLevel +
		`" stroke-width="` + num(strokeW) +
		`" stroke-dasharray="8 10"/>` + "\n"
	assert.Equal(t, want, buf.String())
}

func Test_Renderer_modules(t *testing.T) {
	// --- Given ---
	rnd := must.Value(NewRenderer())
	buf := &bytes.Buffer{}
	grp := newGraph(map[string][]string{"a": {"b"}, "b": nil})
	lay := rnd.layout(grp, nil)
	ids := moduleIDs(grp)
	cls := relClasses(grp, ids)
	bdg := badges(grp, ids)

	// --- When ---
	rnd.modules(buf, grp, lay, ids, cls, bdg)

	// --- Then ---
	have := buf.String()
	assert.Equal(t, 2, strings.Count(have, "</g>\n"))
	wDep := `data-module="b" data-level="0" data-dependents="a">`
	assert.Contain(t, wDep, have)
	wTop := `data-module="a" data-level="1" data-dependents="">`
	assert.Contain(t, wTop, have)
}

func Test_Renderer_orders(t *testing.T) {
	t.Run("one badge per order", func(t *testing.T) {
		// --- Given ---
		rnd := must.Value(NewRenderer())
		buf := &bytes.Buffer{}
		bdgs := []badge{{pin: "m0", rank: 1}, {pin: "m1", rank: 2}}

		// --- When ---
		rnd.orders(buf, layout{boxW: 100}, 10, 20, bdgs)

		// --- Then ---
		have := buf.String()
		assert.Contain(t, `class="badge bm0"`, have)
		assert.Contain(t, ">1</text>", have)
		assert.Contain(t, `class="badge bm1"`, have)
		assert.Contain(t, ">2</text>", have)
	})

	t.Run("no badges", func(t *testing.T) {
		// --- Given ---
		rnd := must.Value(NewRenderer())
		buf := &bytes.Buffer{}

		// --- When ---
		rnd.orders(buf, layout{boxW: 100}, 10, 20, nil)

		// --- Then ---
		assert.Equal(t, "", buf.String())
	})
}

func Test_place(t *testing.T) {
	t.Run("first keeping column wins", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"one/a": nil,
			"two/a": nil,
			"two/b": {"one/a"},
			"two/c": nil,
		})
		cols := []Column{
			{Filter: mod.NewFilter([]string{"one/*", "two/c"}, nil)},
			{Filter: mod.NewFilter([]string{"two/*"}, nil)},
		}

		// --- When ---
		cells, slots := place(grp, cols)

		// --- Then ---
		want := map[string]cell{
			"one/a": {col: 0, pos: 0},
			"two/a": {col: 1, pos: 0},
			"two/c": {col: 0, pos: 1},
			"two/b": {col: 1, pos: 0},
		}
		assert.Equal(t, want, cells)
		assert.Equal(t, []int{2, 1}, slots)
	})

	t.Run("a module no column keeps goes first", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{"x": nil})
		cols := []Column{
			{Filter: mod.NewFilter([]string{"one/*"}, nil)},
			{Filter: mod.NewFilter([]string{"two/*"}, nil)},
		}

		// --- When ---
		cells, slots := place(grp, cols)

		// --- Then ---
		assert.Equal(t, map[string]cell{"x": {col: 0, pos: 0}}, cells)
		assert.Equal(t, []int{1, 0}, slots)
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

func Test_badges(t *testing.T) {
	t.Run("a chain is numbered from every module", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"example.com/app":  {"example.com/lib"},
			"example.com/lib":  {"example.com/core"},
			"example.com/core": nil,
		})
		ids := moduleIDs(grp)

		// --- When ---
		have := badges(grp, ids)

		// --- Then ---
		wCore := []badge{{"m0", 1}}
		assert.Equal(t, wCore, have["example.com/core"])
		wLib := []badge{{"m0", 2}, {"m1", 1}}
		assert.Equal(t, wLib, have["example.com/lib"])
		wApp := []badge{{"m0", 3}, {"m1", 2}, {"m2", 1}}
		assert.Equal(t, wApp, have["example.com/app"])
	})

	t.Run("separate chains are numbered apart", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"example.com/app":  {"example.com/core"},
			"example.com/tool": nil,
			"example.com/core": nil,
		})
		ids := moduleIDs(grp)

		// --- When ---
		have := badges(grp, ids)

		// --- Then ---
		wApp := []badge{{"m0", 2}, {"m2", 1}}
		assert.Equal(t, wApp, have["example.com/app"])
		assert.Equal(t, []badge{{"m0", 1}}, have["example.com/core"])
		assert.Equal(t, []badge{{"m1", 1}}, have["example.com/tool"])
	})

	t.Run("no modules carry no digits", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(nil)

		// --- When ---
		have := badges(grp, moduleIDs(grp))

		// --- Then ---
		assert.Empty(t, have)
	})
}

func Test_interactCSS(t *testing.T) {
	t.Run("one rule per module and the base rules", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(map[string][]string{
			"example.com/app": {"example.com/lib"},
			"example.com/lib": nil,
		})
		ids := moduleIDs(grp)

		// --- When ---
		have := interactCSS(grp, ids)

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

		wBadge := "svg:not(:has(.module:focus)):has(#m0:hover) .bm0," +
			"svg:has(#m0:focus) .bm0{visibility:visible;}"
		assert.Contain(t, wBadge, have)
		shown := strings.Count(have, "{visibility:visible;}")
		assert.Equal(t, 2, shown)
	})

	t.Run("no modules need no rules", func(t *testing.T) {
		// --- Given ---
		grp := newGraph(nil)

		// --- When ---
		have := interactCSS(grp, moduleIDs(grp))

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

// errWrite is the error [failWriter] fails every write with.
var errWrite = errors.New("write refused")

// failWriter is a writer failing every write.
type failWriter struct{}

func (fwr *failWriter) Write([]byte) (int, error) { return 0, errWrite }
