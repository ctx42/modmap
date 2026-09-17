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
