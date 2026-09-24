// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

// Command modmap is the modmap binary entry point. It scans directories for
// Go modules, builds the graph of their dependencies, and renders it as an
// SVG where every module sits above the modules it depends on. The map is
// written to a file, or opened in a browser with the "--web" option.
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/modmap/internal/cli"
)

func main() {
	// A run spends most of its time reading go.mod files, so Ctrl-C has
	// to reach it as a cancelled context rather than as a killed process.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := cli.Main(ctx, ring.New())
	stop()
	os.Exit(code)
}
