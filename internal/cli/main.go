// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/ctx42/ring/pkg/ring"
)

// Main is the modmap entrypoint: it parses the command line arguments carried
// by rng, renders the requested groups into one image, and returns the process
// exit code. The usage text and every error are written to stderr; the image
// goes to the output file or, with the "--web" option, to a browser, and
// never to stdout. With the "--plan" or "--modules" option no image is
// rendered: the update plan, or the modules the groups keep, is printed to
// stdout as JSON instead. The "--version" option prints the version line to
// stdout.
//
// The exit code is 0 on success and 1 on a usage or rendering failure.
func Main(ctx context.Context, rng *ring.Ring) int {
	cfg, err := newConfig(rng.Args())
	if err != nil {
		fail(rng, err)
		_, _ = fmt.Fprint(rng.Stderr(), usage())
		return 1
	}
	if cfg.showHelp {
		_, _ = fmt.Fprint(rng.Stderr(), cfg.help())
		return 0
	}
	if cfg.showVersion {
		// The build info is nil when it cannot be read.
		inf, _ := debug.ReadBuildInfo()
		_, _ = fmt.Fprintln(rng.Stdout(), binName, version(inf))
		return 0
	}
	if err = run(ctx, rng, cfg); err != nil {
		fail(rng, err)
		return 1
	}
	return 0
}

// version returns the module version the Go toolchain recorded in the build
// info inf: the release for a binary installed with "go install" at a version,
// and the version worked out from the tags and the commit for one built in a
// clone, with "+dirty" for uncommitted changes. It returns "(devel)" for a nil
// inf or one with no version, as for a build from a copy of the source.
func version(inf *debug.BuildInfo) string {
	if inf == nil || inf.Main.Version == "" {
		return "(devel)"
	}
	return inf.Main.Version
}
