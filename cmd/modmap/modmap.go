// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Command modmap is the modmap binary entry point. It scans directories for
// Go modules, builds the graph of their dependencies, and renders it as an
// SVG where every module sits above the modules it depends on.
package main

import (
	"context"
	"os"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/modmap/internal/cli"
)

func main() {
	ctx := context.Background()
	rng := ring.New()
	os.Exit(cli.Main(ctx, rng))
}
