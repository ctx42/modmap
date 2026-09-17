// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Package cli implements the modmap command line interface: option parsing,
// run configuration, and the top level dispatch returning the exit code.
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
	errNoOut = errors.New("the -o option is required")

	// errConfOnly is returned when options which the configuration file
	// provides are used alongside it.
	errConfOnly = errors.New("-c cannot be used with other options")
)
