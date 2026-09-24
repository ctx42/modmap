// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package cli implements the modmap command line interface: option parsing,
// run configuration, and the top level dispatch returning the exit code. A
// run either writes every map it is asked for or opens a single one in a
// browser.
package cli

import "errors"

// binName is the command name used in usage and error messages.
const binName = "modmap"

// Configuration errors.
var (
	// errNoDir is returned when the directory to scan is not given.
	errNoDir = errors.New("directory to scan is required")

	// errManyDirs is returned when more than one directory is given.
	errManyDirs = errors.New("only one directory can be scanned at a time")

	// errNoOut is returned when the output file is not given.
	errNoOut = errors.New("either -o or --web is required")

	// errConfOnly is returned when options which the configuration file
	// provides are used alongside it.
	errConfOnly = errors.New("-c cannot be used with other options")

	// errWebOut is returned when the map is opened in a browser and
	// written to a file at once.
	errWebOut = errors.New("--web cannot be used with -o")

	// errWebMap is returned when the map to open in a browser is not
	// named exactly once.
	errWebMap = errors.New("--web with -c needs exactly one map name")
)
