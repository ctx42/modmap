// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"path/filepath"

	"github.com/ctx42/ring/pkg/ring"

	"github.com/ctx42/modmap/pkg/mod"
)

// appendGlob appends the module path glob to dst. It returns an error when
// the glob is malformed.
func appendGlob(dst *[]string, glob string) error {
	if err := mod.ValidateGlob(glob); err != nil {
		return err
	}
	*dst = append(*dst, glob)
	return nil
}

// abs returns pth as an absolute path, resolving a relative one against wd.
func abs(wd, pth string) string {
	if filepath.IsAbs(pth) {
		return pth
	}
	return filepath.Join(wd, pth)
}

// fail writes err to stderr decorated for the user. It is the single place
// controlling how command errors are presented.
func fail(rng *ring.Ring, err error) {
	_, _ = fmt.Fprintf(rng.Stderr(), "%s: %s\n", binName, err)
}
