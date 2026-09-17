// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package svg

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/ctx42/modmap/pkg/graph"
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

	tplBox = "" +
		`<rect x="%s" y="%s" width="%s" height="%s"` +
		` rx="%s" fill="none"` +
		` stroke="%s" stroke-width="%s" stroke-dasharray="%s"/>` + "\n"

	tplLabel = "" +
		`<text x="%s" y="%s" fill="%s" font-size="%s"` +
		` text-anchor="middle">%s</text>` + "\n"
)

// layout holds the geometry computed for one graph.
type layout struct {
	width  float64 // Canvas width.
	height float64 // Canvas height.
	boxW   float64 // Width of every module box.
	baseY  float64 // Label baseline offset from the box top.
	levels int     // Number of levels.
}

// boxTop returns the y coordinate of the top of the boxes on the level.
func (lay layout) boxTop(level int) float64 {
	band := boxHeight + gapY
	return lay.height - marginY - boxHeight - float64(level)*band
}

// boxLeft returns the x coordinate of the left side of the column.
func (lay layout) boxLeft(col int) float64 {
	return marginX + float64(col)*(lay.boxW+gapX)
}

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
// carries the module path, its level, and the modules which have to be
// updated when it changes.
func (rnd *Renderer) Render(grp *graph.Graph, dst io.Writer) error {
	lay := rnd.layout(grp)
	ids := moduleIDs(grp)
	buf := &bytes.Buffer{}
	rnd.head(buf, lay, hoverCSS(grp, ids))
	rnd.levels(buf, lay)
	rnd.modules(buf, grp, lay, ids, relClasses(grp, ids))
	buf.WriteString("</svg>\n")
	if _, err := dst.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("write map: %w", err)
	}
	return nil
}

// layout computes the geometry of the map. Every box is as wide as the widest
// label needs, so the levels keep an even rhythm.
func (rnd *Renderer) layout(grp *graph.Graph) layout {
	lay := layout{levels: len(grp.Levels)}
	var widest float64
	for _, lvl := range grp.Levels {
		for _, nod := range lvl {
			widest = max(widest, rnd.fnt.Width(nod.Path, textSize))
		}
	}
	lay.boxW = widest + 2*boxPad
	lay.baseY = (boxHeight + rnd.fnt.CapHeight(textSize)) / 2

	cols := float64(grp.Widest())
	lay.width = 2*marginX + cols*lay.boxW + max(cols-1, 0)*gapX
	rows := float64(lay.levels)
	lay.height = 2*marginY + rows*boxHeight + max(rows-1, 0)*gapY
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
	x1, x2 := num(marginX/2), num(lay.width-marginX/2)
	for lvl := range lay.levels {
		top := lay.boxTop(lvl)
		rnd.separator(buf, x1, x2, top+boxHeight+gapY/2)
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

// separator writes one level separator line.
func (rnd *Renderer) separator(buf *bytes.Buffer, x1, x2 string, y float64) {
	_, _ = fmt.Fprintf(
		buf,
		tplSepar,
		x1,
		num(y),
		x2,
		num(y),
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
) {

	for lvl, nodes := range grp.Levels {
		top := lay.boxTop(lvl)
		for col, nod := range nodes {
			left := lay.boxLeft(col)
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
			buf.WriteString("</g>\n")
		}
	}
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

// hoverCSS returns the style rules dimming every module but the hovered one
// and the modules related to it.
func hoverCSS(grp *graph.Graph, ids map[string]string) string {
	if grp.Len() == 0 {
		return ""
	}
	buf := &strings.Builder{}
	buf.WriteString(cssBase)
	for _, nodes := range grp.Levels {
		for _, nod := range nodes {
			id := ids[nod.Path]
			_, _ = fmt.Fprintf(buf, tplHover, id, classRel+id)
		}
	}
	return buf.String()
}

// num formats the coordinate for the SVG document.
func num(val float64) string {
	return strconv.FormatFloat(val, 'f', -1, 64)
}
