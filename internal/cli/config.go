// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ctx42/xflag/pkg/xflag"
)

// Option usage strings.
const (
	usgInclude = "render only modules matching the glob (repeatable)"
	usgExclude = "never render modules matching the glob (repeatable)"
	usgOut     = "path of the SVG file to write"
	usgConf    = "path of the YAML configuration file"
	usgWeb     = "open the map in a browser instead of writing it"
	usgYes     = "do not ask for confirmation on wide levels"
	usgHelp    = "show this help"
)

// config represents modmap run configuration.
type config struct {
	// Absolute paths of the directories scanned for Go modules. In the
	// one-off mode it holds the single directory given as the argument.
	roots []string

	// Module path globs deciding which modules are rendered. An empty list
	// renders every module found. Set by the "--include" option.
	include []string

	// Module path globs removing modules from the render. A module matching
	// both lists is not rendered. Set by the "--exclude" option.
	exclude []string

	// Absolute path of the SVG file to write. Set by the "--out" option and
	// required unless the configuration file is used.
	out string

	// Absolute path of the YAML configuration file. Set by the "--config"
	// option. When set, it is the only source of directories and globs.
	conf string

	// Names of the configuration file maps to generate. An empty list
	// generates every map the file declares.
	names []string

	// Open the map in a browser instead of writing it to a file. Set
	// by the "--web" option.
	web bool

	// Render wide levels without asking for confirmation. Set by the
	// "--yes" option.
	yes bool

	// Show the command usage. Set by the "--help" option.
	showHelp bool

	// Opens the written map in a browser. It is nil outside the tests,
	// which replace it to keep a browser from being launched.
	opener func(url string) error

	// The command line flag set.
	fs *xflag.FlagSet
}

// newConfig returns the run configuration parsed from the command line
// arguments.
func newConfig(args []string) (*config, error) {
	cfg := &config{}
	if err := cfg.parse(args); err != nil {
		return nil, err
	}
	return cfg, nil
}

// flags registers the command line options on the configuration flag set. It
// returns the function copying the parsed option values into the
// configuration, which the caller runs once the arguments are parsed.
func (cfg *config) flags() func() {
	cfg.fs = xflag.NewFlagSet(binName, flag.ContinueOnError)
	cfg.fs.SetOutput(io.Discard)
	cfg.fs.Usage = func() {}

	cfg.fs.FuncSL("include", "i", usgInclude, func(glob string) error {
		return appendGlob(&cfg.include, glob)
	})
	cfg.fs.FuncSL("exclude", "e", usgExclude, func(glob string) error {
		return appendGlob(&cfg.exclude, glob)
	})
	fOut := cfg.fs.StringSL("out", "o", "", usgOut)
	fConf := cfg.fs.StringSL("config", "c", "", usgConf)
	fWeb := cfg.fs.Bool("web", false, usgWeb)
	fYes := cfg.fs.BoolSL("yes", "y", false, usgYes)
	fHelp := cfg.fs.BoolSL("help", "h", false, usgHelp)

	return func() {
		cfg.out = *fOut
		cfg.conf = *fConf
		cfg.web = *fWeb
		cfg.yes = *fYes
		cfg.showHelp = *fHelp
	}
}

// parse fills the configuration from the command line arguments and
// validates the combination of options they set.
func (cfg *config) parse(args []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	apply := cfg.flags()
	if err = cfg.fs.Parse(args); err != nil {
		return err
	}
	apply()

	if cfg.showHelp {
		return nil
	}
	if cfg.web && cfg.out != "" {
		return errWebOut
	}
	if cfg.conf != "" {
		return cfg.parseConf(wd)
	}
	return cfg.parseDir(wd)
}

// parseConf validates and resolves the configuration file mode, where
// the file is the only source of directories, globs, and outputs.
func (cfg *config) parseConf(wd string) error {
	if cfg.out != "" || len(cfg.include)+len(cfg.exclude) > 0 {
		return errConfOnly
	}
	left := cfg.fs.Args()
	if cfg.web && len(left) != 1 {
		return errWebMap
	}
	cfg.conf = abs(wd, cfg.conf)
	cfg.names = left
	return nil
}

// parseDir validates and resolves the one-off mode, where a single
// directory is given as the argument.
func (cfg *config) parseDir(wd string) error {
	left := cfg.fs.Args()
	switch {
	case len(left) == 0:
		return errNoDir

	case len(left) > 1:
		return errManyDirs
	}
	if cfg.out == "" && !cfg.web {
		return errNoOut
	}
	cfg.roots = []string{abs(wd, left[0])}
	if cfg.out != "" {
		cfg.out = abs(wd, cfg.out)
	}
	return nil
}

// help returns the command usage text.
func (cfg *config) help() string {
	const format = "" +
		"Usage:\n" +
		"  %[1]s [options] -o <file.svg> <dir>\n" +
		"  %[1]s [options] --web <dir>\n" +
		"  %[1]s -c <config.yaml> [map...]\n" +
		"  %[1]s --web -c <config.yaml> <map>\n" +
		"\n" +
		"Options:\n" +
		"%[2]s"
	return fmt.Sprintf(format, binName, xflag.HelpOptions(cfg.fs))
}

// usage returns the command usage text without a parsed configuration.
func usage() string {
	cfg := &config{}
	_ = cfg.flags()
	return cfg.help()
}
