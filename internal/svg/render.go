// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package svg

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/ctx42/modmap/pkg/graph"
	"github.com/ctx42/modmap/pkg/mod"
)

// Templates of the rendered SVG elements.
const (
	tplHead = "" +
		`<svg xmlns="http://www.w3.org/2000/svg" version="1.1"` +
		` viewBox="0 0 %[1]s %[2]s"` +
		` width="%[1]s" height="%[2]s">` + "\n" +
		`<defs><style>` + "\n" +
		`@font-face{font-family:"%[3]s";` +
		`src:url(%[4]s) format("truetype");}` +
		"\n" +
		`text{font-family:"%[3]s",sans-serif;}` + "\n" +
		`%[6]s` +
		`</style></defs>` + "\n" +
		`<rect x="0" y="0" width="%[1]s" height="%[2]s"` +
		` fill="%[5]s"/>` + "\n"

	tplSepar = "" +
		`<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s"` +
		` stroke-width="%s" stroke-dasharray="%s"/>` + "\n"

	tplLevel = "" +
		`<text x="%[1]s" y="%[2]s"` +
		` transform="rotate(-90 %[1]s %[2]s)"` +
		` fill="%[3]s" font-size="%[4]s"` +
		` text-anchor="middle">LEVEL %[5]d</text>` + "\n"

	tplModule = "" +
		`<g id="%s" class="%s" tabindex="0" data-module="%s"` +
		` data-level="%d" data-dependents="%s">` + "\n"

	// tplHover lights one module and every module related to it while that
	// module is hovered or pinned. Hovering is ignored while a module is
	// pinned, so the pinned set stays the one on show.
	tplHover = "" +
		`svg:not(:has(.module:focus)):has(#%[1]s:hover) #%[1]s,` +
		`svg:not(:has(.module:focus)):has(#%[1]s:hover) .%[2]s,` +
		`svg:has(#%[1]s:focus) #%[1]s,` +
		`svg:has(#%[1]s:focus) .%[2]s{opacity:1;}` + "\n"

	// tplOrder reveals every order badge belonging to one module while
	// that module is hovered or pinned, mirroring tplHover.
	tplOrder = "" +
		`svg:not(:has(.module:focus)):has(#%[1]s:hover) .%[2]s,` +
		`svg:has(#%[1]s:focus) .%[2]s{visibility:visible;}` + "\n"

	tplBadge = "" +
		`<text x="%s" y="%s" class="%s" fill="%s" font-size="%s"` +
		` text-anchor="end">%d</text>` + "\n"

	tplBox = "" +
		`<rect x="%s" y="%s" width="%s" height="%s"` +
		` rx="%s" fill="none"` +
		` stroke="%s" stroke-width="%s" stroke-dasharray="%s"/>` + "\n"

	tplLabel = "" +
		`<text x="%s" y="%s" fill="%s" font-size="%s"` +
		` text-anchor="middle">%s</text>` + "\n"
)

// Column is one map drawn beside the others in a combined image.
type Column struct {
	// Name is the map name drawn above the column.
	Name string

	// Filter decides which modules the column claims.
	Filter mod.Filter
}

// column is the geometry of one map column.
type column struct {
	name  string  // Map name drawn above the column.
	left  float64 // The x coordinate of the left side of the column.
	width float64 // Width of the column.
}

// cell is the place of one module box: its column and its position on the
// level within that column.
type cell struct {
	col int // Index of the column.
	pos int // Position on the level, counted from the left.
}

// layout holds the geometry computed for one graph.
type layout struct {
	width  float64            // Canvas width.
	height float64            // Canvas height.
	boxW   float64            // Width of every module box.
	baseY  float64            // Label baseline offset from the box top.
	badgeY float64            // Badge baseline offset from the box top.
	levels int                // Number of levels.
	cols   []column           // Columns from left to right.
	lefts  map[string]float64 // Box left side keyed by module path.
}

// boxTop returns the y coordinate of the top of the boxes on the level.
func (lay layout) boxTop(level int) float64 {
	band := boxHeight + gapY
	return lay.height - marginY - boxHeight - float64(level)*band
}

// named reports whether the columns are drawn under their map names, which
// takes two or more of them.
func (lay layout) named() bool { return len(lay.cols) > 1 }

// Renderer draws the layered module graph as an SVG document.
type Renderer struct {
	fnt *Font // Font the labels are measured and set in.
}

// NewRenderer returns a renderer using the embedded map font.
func NewRenderer() (*Renderer, error) {
	fnt, err := NewFont()
	if err != nil {
		return nil, err
	}
	return &Renderer{fnt: fnt}, nil
}

// Render writes the map of the graph as a standalone SVG document. Every box
// carries the module path, its level, the modules which have to be updated
// when it changes, and the order to update them in.
//
// Every module is drawn once, in the first of the columns whose filter keeps
// it, or in the first column when none does. Two or more columns are drawn
// side by side, each under its name and split from the next by a divider;
// a single column is drawn without its name, and no columns at all draw as
// one column holding every module. The levels, the lighting, and the update
// order span the whole image, so a chain crossing columns lights up and is
// numbered as one.
func (rnd *Renderer) Render(
	grp *graph.Graph,
	cols []Column,
	dst io.Writer,
) error {

	lay := rnd.layout(grp, cols)
	ids := moduleIDs(grp)
	buf := &bytes.Buffer{}
	rnd.head(buf, lay, interactCSS(grp, ids))
	rnd.levels(buf, lay)
	rnd.columns(buf, lay)
	cls, bdg := relClasses(grp, ids), badges(grp, ids)
	rnd.modules(buf, grp, lay, ids, cls, bdg)
	buf.WriteString("</svg>\n")
	if _, err := dst.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("write map: %w", err)
	}
	return nil
}

// layout computes the geometry of the map. Every box is as wide as the widest
// label needs, so the levels keep an even rhythm, and every column is as wide
// as its own widest level. Named columns add one row above the top level.
func (rnd *Renderer) layout(grp *graph.Graph, cols []Column) layout {
	if len(cols) == 0 {
		cols = []Column{{}}
	}
	lay := layout{
		levels: len(grp.Levels),
		lefts:  make(map[string]float64, grp.Len()),
	}
	var widest float64
	for _, lvl := range grp.Levels {
		for _, nod := range lvl {
			widest = max(widest, rnd.fnt.Width(nod.Path, textSize))
		}
	}
	lay.boxW = widest + 2*boxPad
	lay.baseY = (boxHeight + rnd.fnt.CapHeight(textSize)) / 2
	lay.badgeY = badgePad + rnd.fnt.CapHeight(badgeSize)

	cells, slots := place(grp, cols)
	left := marginX
	for idx, col := range cols {
		cnt := float64(slots[idx])
		width := cnt*lay.boxW + max(cnt-1, 0)*gapX
		clm := column{name: col.Name, left: left, width: width}
		lay.cols = append(lay.cols, clm)
		left += width + gapX
	}
	for pth, cel := range cells {
		col := lay.cols[cel.col]
		lay.lefts[pth] = col.left + float64(cel.pos)*(lay.boxW+gapX)
	}

	lay.width = left - gapX + marginX
	rows := float64(lay.levels)
	lay.height = 2*marginY + rows*boxHeight + max(rows-1, 0)*gapY
	if lay.named() {
		lay.height += boxHeight + gapY
	}
	return lay
}

// head writes the document opening, the embedded font, the hover rules, and
// the background.
func (rnd *Renderer) head(buf *bytes.Buffer, lay layout, css string) {
	_, _ = fmt.Fprintf(
		buf,
		tplHead,
		num(lay.width),
		num(lay.height),
		FontFamily,
		rnd.fnt.DataURI(),
		colorBg,
		css,
	)
}

// levels writes the level separators and the level labels. Every level is
// closed by a separator drawn below it, the lowest one closing the map.
func (rnd *Renderer) levels(buf *bytes.Buffer, lay layout) {
	x1, x2 := marginX/2, lay.width-marginX/2
	for lvl := range lay.levels {
		top := lay.boxTop(lvl)
		y := top + boxHeight + gapY/2
		rnd.line(buf, x1, y, x2, y)
		_, _ = fmt.Fprintf(
			buf,
			tplLevel,
			num(marginX/2),
			num(top+boxHeight/2),
			colorLevelText,
			num(levelSize),
			lvl,
		)
	}
}

// columns writes the map names above the named columns and the dividers
// between them. A divider runs down the middle of the gap between two
// columns, from the top of the name row to the separator closing the map.
func (rnd *Renderer) columns(buf *bytes.Buffer, lay layout) {
	if !lay.named() {
		return
	}
	nameY := marginY + (boxHeight+rnd.fnt.CapHeight(levelSize))/2
	bottom := lay.boxTop(0) + boxHeight + gapY/2
	for idx, col := range lay.cols {
		_, _ = fmt.Fprintf(
			buf,
			tplLabel,
			num(col.left+col.width/2),
			num(nameY),
			colorLevelText,
			num(levelSize),
			html.EscapeString(col.name),
		)
		if idx > 0 {
			x := col.left - gapX/2
			rnd.line(buf, x, marginY, x, bottom)
		}
	}
}

// line writes one line styled as a level separator.
func (rnd *Renderer) line(buf *bytes.Buffer, x1, y1, x2, y2 float64) {
	_, _ = fmt.Fprintf(
		buf,
		tplSepar,
		num(x1),
		num(y1),
		num(x2),
		num(y2),
		colorLevel,
		num(strokeW),
		dashes,
	)
}

// modules writes one group per module, level by level, starting at the
// bottom one.
func (rnd *Renderer) modules(
	buf *bytes.Buffer,
	grp *graph.Graph,
	lay layout,
	ids map[string]string,
	cls map[string][]string,
	bdg map[string][]badge,
) {

	for lvl, nodes := range grp.Levels {
		top := lay.boxTop(lvl)
		for _, nod := range nodes {
			left := lay.lefts[nod.Path]
			deps := strings.Join(nod.Dependents, " ")
			names := append([]string{classModule}, cls[nod.Path]...)
			_, _ = fmt.Fprintf(
				buf,
				tplModule,
				ids[nod.Path],
				strings.Join(names, " "),
				html.EscapeString(nod.Path),
				nod.Level,
				html.EscapeString(deps),
			)
			_, _ = fmt.Fprintf(
				buf,
				tplBox,
				num(left),
				num(top),
				num(lay.boxW),
				num(boxHeight),
				num(boxRadius),
				colorBox,
				num(strokeW),
				dashes,
			)
			_, _ = fmt.Fprintf(
				buf,
				tplLabel,
				num(left+lay.boxW/2),
				num(top+lay.baseY),
				colorLabel,
				num(textSize),
				html.EscapeString(nod.Path),
			)
			rnd.orders(buf, lay, left, top, bdg[nod.Path])
			buf.WriteString("</g>\n")
		}
	}
}

// orders writes the update order badges of one module box. Every badge is
// hidden until the module whose change it belongs to is hovered or pinned,
// so a box shows one digit at a time.
func (rnd *Renderer) orders(
	buf *bytes.Buffer,
	lay layout,
	left, top float64,
	bdgs []badge,
) {

	for _, bad := range bdgs {
		_, _ = fmt.Fprintf(
			buf,
			tplBadge,
			num(left+lay.boxW-badgePad),
			num(top+lay.badgeY),
			classBadge+" "+classOrder+bad.pin,
			colorBadge,
			num(badgeSize),
			bad.rank,
		)
	}
}

// place returns the cell of every module keyed by module path, and the number
// of boxes on the widest level of every column. A module goes to the first
// column whose filter keeps it, or to the first column when none does, and
// keeps the order the level sorts it in. The cols must not be empty.
func place(grp *graph.Graph, cols []Column) (map[string]cell, []int) {
	cells := make(map[string]cell, grp.Len())
	slots := make([]int, len(cols))
	for _, nodes := range grp.Levels {
		used := make([]int, len(cols))
		for _, nod := range nodes {
			keep := func(col Column) bool { return col.Filter.Match(nod.Path) }
			idx := max(slices.IndexFunc(cols, keep), 0)
			cells[nod.Path] = cell{col: idx, pos: used[idx]}
			used[idx]++
		}
		for idx, cnt := range used {
			slots[idx] = max(slots[idx], cnt)
		}
	}
	return cells, slots
}

// moduleIDs returns the element id of every module keyed by module path. The
// ids are assigned level by level, so they change only when the graph does.
func moduleIDs(grp *graph.Graph) map[string]string {
	ids := make(map[string]string, grp.Len())
	var idx int
	for _, nodes := range grp.Levels {
		for _, nod := range nodes {
			ids[nod.Path] = fmt.Sprintf("%s%d", idPrefix, idx)
			idx++
		}
	}
	return ids
}

// relClasses returns the classes of every module keyed by module path. A
// module carries one class for every module related to it: the modules it
// relies on and the modules relying on it, directly or through other
// modules. Lighting is therefore mutual — whichever end of a chain is
// hovered, the whole chain lights up.
func relClasses(grp *graph.Graph, ids map[string]string) map[string][]string {
	cls := make(map[string][]string, grp.Len())
	for _, nodes := range grp.Levels {
		for _, nod := range nodes {
			pth := nod.Path
			name := classRel + ids[pth]
			for _, dep := range nod.Dependents {
				cls[dep] = append(cls[dep], name)
				cls[pth] = append(cls[pth], classRel+ids[dep])
			}
		}
	}
	for pth := range cls {
		slices.Sort(cls[pth])
	}
	return cls
}

// badge is one update order digit drawn on a module box.
type badge struct {
	pin  string // Element id of the module the order belongs to.
	rank int    // Round the module is updated in for that change.
}

// badges returns the order digits of every module keyed by module path. A
// module carries one digit for every module whose change reaches it, its
// own change included, so pinning any module shows the whole update order
// of that change at once.
func badges(grp *graph.Graph, ids map[string]string) map[string][]badge {
	bdg := make(map[string][]badge, grp.Len())
	for _, nodes := range grp.Levels {
		for _, nod := range nodes {
			ord := grp.Order(nod.Path)
			pin := ids[nod.Path]
			for _, pth := range slices.Sorted(maps.Keys(ord)) {
				bdg[pth] = append(bdg[pth], badge{pin, ord[pth]})
			}
		}
	}
	return bdg
}

// interactCSS returns the style rules dimming every module but the hovered
// one and the modules related to it, and revealing the order badges of the
// hovered module.
func interactCSS(grp *graph.Graph, ids map[string]string) string {
	if grp.Len() == 0 {
		return ""
	}
	buf := &strings.Builder{}
	buf.WriteString(cssBase)
	for _, nodes := range grp.Levels {
		for _, nod := range nodes {
			id := ids[nod.Path]
			_, _ = fmt.Fprintf(buf, tplHover, id, classRel+id)
			_, _ = fmt.Fprintf(buf, tplOrder, id, classOrder+id)
		}
	}
	return buf.String()
}

// num formats the coordinate for the SVG document.
func num(val float64) string {
	return strconv.FormatFloat(val, 'f', -1, 64)
}
