// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build !linux

package cli

import (
	"os"

	"github.com/ctx42/testing/pkg/tester"
)

// openPTY skips the test, pseudo-terminals are opened on Linux only.
func openPTY(t tester.T) (*os.File, *os.File) {
	t.Helper()
	t.Skip("pseudo-terminals are opened on Linux only")
	return nil, nil
}
