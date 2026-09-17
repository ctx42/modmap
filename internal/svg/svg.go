// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package svg renders the layered module graph as a standalone SVG document
// carrying the dependency data the hover interaction needs.
package svg

// Map geometry in user units, taken from the reference drawing.
const (
	boxHeight = 231.0  // Height of every module box.
	boxRadius = 32.0   // Corner radius of a module box.
	boxPad    = 60.0   // Space between a label and the box border.
	gapX      = 260.0  // Space between two boxes on a level.
	gapY      = 341.0  // Space between two levels.
	marginX   = 216.0  // Left and right canvas margin.
	marginY   = 200.0  // Top and bottom canvas margin.
	textSize  = 36.0   // Size of a module label.
	levelSize = 96.0   // Size of a level label.
	strokeW   = 2.5    // Width of every stroke.
	dashes    = "8 10" // Dash pattern of every stroke.
)

// Interaction markup and style. The rules use the CSS ":has" selector and no
// script, so the map stays interactive wherever style sheets are honored.
// Hovering a module lights it and every module related to it — the ones it
// relies on and the ones relying on it; clicking one pins that light by
// focusing the module, and clicking the canvas lets it go.
const (
	idPrefix    = "m"      // Prefix of a module element id.
	classModule = "module" // Class every module group carries.
	classRel    = "r"      // Prefix of a related-module class.

	// cssBase dims every module while one is hovered or pinned; the rules
	// generated per module light that module and the ones related to it
	// again. A pinned module drops its dashes, so it reads as held.
	cssBase = "" +
		".module{transition:opacity .15s ease-out;cursor:pointer;}\n" +
		".module rect{pointer-events:all;}\n" +
		".module:focus{outline:none;}\n" +
		".module:focus rect{stroke-dasharray:none;}\n" +
		"svg:has(.module:hover) .module," +
		"svg:has(.module:focus) .module{opacity:" + dimOpacity + ";}\n"

	// dimOpacity is the opacity of a module unrelated to the hovered or
	// pinned one.
	dimOpacity = "0.12"
)

// Map colors, taken from the reference drawing.
const (
	colorBg        = "#121212" // Canvas background.
	colorBox       = "#d3d3d3" // Module box border.
	colorLabel     = "#d3d3d3" // Module label.
	colorLevel     = "#154162" // Level separator.
	colorLevelText = "#b86200" // Level label.
)
