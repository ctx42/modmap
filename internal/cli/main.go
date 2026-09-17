// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"

	"github.com/ctx42/ring/pkg/ring"
)

// Main is the modmap entrypoint: it parses the command line arguments carried
// by rng, renders the requested maps, and returns the process exit code. The
// usage text and every error are written to stderr; the SVG is always written
// to a file.
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
	if err = run(ctx, rng, cfg); err != nil {
		fail(rng, err)
		return 1
	}
	return 0
}
