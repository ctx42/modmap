// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/oskit"
)

// writeMod creates the directory and writes content to its go.mod file.
func writeMod(t tester.T, content, dir string, elems ...string) {
	t.Helper()
	dir = oskit.MkdirAll(t, dir, elems...)
	oskit.Create(t, content, dir, "go.mod")
}

// parsedConfig returns a configuration whose flag set parsed args, with the
// option values copied into it.
func parsedConfig(t tester.T, args ...string) *config {
	t.Helper()
	cfg := &config{}
	apply := cfg.flags()
	if err := cfg.fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	apply()
	return cfg
}
