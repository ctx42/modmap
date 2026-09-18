// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/ctx42/xflag/pkg/xflag"
)

// Option usage strings.
const (
	usgInclude = "render only modules matching the glob (repeatable)"
	usgExclude = "never render modules matching the glob (repeatable)"
	usgOut     = "path of the SVG file to write"
	usgConf    = "path of the YAML configuration file"
	usgWeb     = "serve the map in a browser (--web=host:port)"
	usgYes     = "do not ask for confirmation on wide levels"
	usgHelp    = "show this help"
)

// defAddr is the address the "--web" server listens on when the option
// carries no value: a free port on the loopback interface.
const defAddr = "127.0.0.1:0"

// webAddr is the value of the "--web" option. The option stands on its
// own, keeping the default address, or carries a "host:port" value
// pinning where the server listens. The value is kept as it was typed;
// [config.resolveAddr] turns it into an address, so a malformed one is
// reported as itself rather than through the flag package.
type webAddr struct {
	set bool   // The option was given.
	val string // The option value, empty when it stood on its own.
}

// IsBoolFlag lets "--web" stand on its own, so it never swallows the
// directory following it. A value is given as "--web=host:port".
func (wad *webAddr) IsBoolFlag() bool { return true }

func (wad *webAddr) String() string { return wad.val }

func (wad *webAddr) Set(val string) error {
	wad.set = true
	if val != "true" {
		wad.val = val
	}
	return nil
}

// listenAddr returns the "--web" option value as a host:port address. A
// bare port number is taken for that port on the loopback interface.
func listenAddr(val string) (string, error) {
	if !strings.Contains(val, ":") {
		if _, err := strconv.Atoi(val); err != nil {
			return "", fmt.Errorf("%w: %s", errWebAddr, val)
		}
		return net.JoinHostPort("127.0.0.1", val), nil
	}
	_, port, err := net.SplitHostPort(val)
	if err != nil || port == "" {
		return "", fmt.Errorf("%w: %s", errWebAddr, val)
	}
	return val, nil
}

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

	// Serve the map in a browser instead of writing it to a file. Set
	// by the "--web" option.
	web bool

	// Address the served map listens on, as a host:port pair. A zero
	// port picks a free one. Set by the "--web" option value.
	webAddr string

	// Render wide levels without asking for confirmation. Set by the
	// "--yes" option.
	yes bool

	// Show the command usage. Set by the "--help" option.
	showHelp bool

	// Opens the served map in a browser. It is nil outside the tests,
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
	fWeb := &webAddr{}
	cfg.fs.Var(fWeb, "web", usgWeb)
	fYes := cfg.fs.BoolSL("yes", "y", false, usgYes)
	fHelp := cfg.fs.BoolSL("help", "h", false, usgHelp)

	return func() {
		cfg.out = *fOut
		cfg.conf = *fConf
		cfg.web = fWeb.set
		cfg.webAddr = fWeb.val
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
	if err = cfg.resolveAddr(); err != nil {
		return err
	}
	if cfg.conf != "" {
		return cfg.parseConf(wd)
	}
	return cfg.parseDir(wd)
}

// resolveAddr turns the "--web" option value into the address the server
// listens on. The option standing on its own keeps the default address.
func (cfg *config) resolveAddr() error {
	if !cfg.web {
		return nil
	}
	if cfg.webAddr == "" {
		cfg.webAddr = defAddr
		return nil
	}
	addr, err := listenAddr(cfg.webAddr)
	if err != nil {
		return err
	}
	cfg.webAddr = addr
	return nil
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
		"  %[1]s [options] --web[=host:port] <dir>\n" +
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
