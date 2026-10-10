// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package cli implements the modmap command line interface: option parsing,
// run configuration, and the top level dispatch returning the exit code. A
// run draws one image, holding every map it is asked for side by side, and
// either writes it or opens it in a browser; or it prints the update plan of
// a changed module as JSON.
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

	// errPlanWeb is returned when the plan is asked for together with
	// opening the map in a browser.
	errPlanWeb = errors.New("--plan cannot be used with --web")

	// errPlanConf is returned when the plan is asked for without a
	// configuration file.
	errPlanConf = errors.New("--plan requires -c")

	// errPlanMap is returned when the plan is not limited to exactly one
	// map.
	errPlanMap = errors.New("--plan requires exactly one map name")
)

// Plan errors.
var (
	// errModNotFound is returned when the changed module is not in the
	// scanned modules.
	errModNotFound = errors.New("module not found")

	// errModNotInMap is returned when the changed module is not kept by
	// the map the plan is limited to.
	errModNotInMap = errors.New("module not in the map")

	// errModNotOnDisk is returned when the changed module was read from the
	// module cache instead of a scanned directory.
	errModNotOnDisk = errors.New("module not found on disk")
)
