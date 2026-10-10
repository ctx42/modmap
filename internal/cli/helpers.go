// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"io/fs"
	"os"
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

// replaceFile writes data to the file at pth through a temporary file renamed
// over it, so a failed write never leaves pth truncated. An existing file keeps
// its mode, a new one is created with mode 0644. When pth is a symbolic link,
// its target is replaced.
func replaceFile(pth string, data []byte) error {
	if dst, err := filepath.EvalSymlinks(pth); err == nil {
		pth = dst
	}
	perm := fs.FileMode(0o644)
	if inf, err := os.Stat(pth); err == nil {
		perm = inf.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(pth), filepath.Base(pth)+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(tmp.Name(), perm); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), pth)
}

// fail writes err to stderr decorated for the user. It is the single place
// controlling how command errors are presented.
func fail(rng *ring.Ring, err error) {
	_, _ = fmt.Fprintf(rng.Stderr(), "%s: %s\n", binName, err)
}

// logger returns the progress reporter writing every message as a line to
// stderr.
func logger(rng *ring.Ring) func(format string, args ...any) {
	return func(format string, args ...any) {
		_, _ = fmt.Fprintf(rng.Stderr(), format+"\n", args...)
	}
}
