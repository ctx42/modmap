// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package svg

import (
	"encoding/base64"
	"fmt"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// FontFamily is the font family name the map labels are set in.
const FontFamily = "Modmap Sans"

// Font measures label text in the font embedded in the map, so a box is as
// wide as the browser will draw its label.
type Font struct {
	data []byte      // The font file embedded in the map.
	sft  *sfnt.Font  // The parsed font file.
	buf  sfnt.Buffer // Scratch buffer reused by the measurements.
}

// NewFont returns the font the map labels are set in.
func NewFont() (*Font, error) {
	sft, err := sfnt.Parse(goregular.TTF)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	return &Font{data: goregular.TTF, sft: sft}, nil
}

// Width returns the width of the text set at the given font size. Runes the
// font has no glyph for are skipped.
func (fnt *Font) Width(text string, size float64) float64 {
	ppem := fixed.Int26_6(size * 64)
	var total fixed.Int26_6
	for _, rne := range text {
		idx, err := fnt.sft.GlyphIndex(&fnt.buf, rne)
		if err != nil || idx == 0 {
			continue
		}
		adv, err := fnt.sft.GlyphAdvance(&fnt.buf, idx, ppem, 0)
		if err != nil {
			continue
		}
		total += adv
	}
	return float64(total) / 64
}

// CapHeight returns the height of a capital letter set at the given font size.
func (fnt *Font) CapHeight(size float64) float64 {
	ppem := fixed.Int26_6(size * 64)
	met, err := fnt.sft.Metrics(&fnt.buf, ppem, 0)
	if err != nil {
		return size / 2
	}
	return float64(met.CapHeight) / 64
}

// DataURI returns the font as a data URI for a CSS font-face rule.
func (fnt *Font) DataURI() string {
	enc := base64.StdEncoding.EncodeToString(fnt.data)
	return "data:font/ttf;base64," + enc
}
