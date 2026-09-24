// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package view

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
)

// ErrNoBrowser is returned when the system has no command able to open a URL.
var ErrNoBrowser = errors.New("no command to open a browser")

// open opens the URL in the desktop browser. It returns [ErrNoBrowser] when
// no command able to do that is on the PATH.
func open(url string) error {
	return openWith(openers(runtime.GOOS), url)
}

// openWith opens the URL with the first of the commands present on the
// machine. It returns [ErrNoBrowser] when none of them is.
func openWith(cmds [][]string, url string) error {
	for _, cmd := range cmds {
		pth, err := exec.LookPath(cmd[0])
		if err != nil {
			continue
		}
		args := append(append([]string{}, cmd[1:]...), url)
		// The browser has to outlive the run, so it is started
		// without a context.
		prc := exec.Command(pth, args...) //nolint:gosec,noctx
		if err = prc.Start(); err != nil {
			return fmt.Errorf("start %s: %w", cmd[0], err)
		}
		return nil
	}
	return ErrNoBrowser
}

// openers returns the commands able to open a URL on the operating system,
// the preferred one first. Only the first command present on the machine is
// used, so the list is a fallback chain, not a set of alternatives.
func openers(goos string) [][]string {
	switch goos {
	case "darwin":
		return [][]string{{"open"}}

	case "windows":
		return [][]string{{"rundll32", "url.dll,FileProtocolHandler"}}

	default:
		return [][]string{
			{"xdg-open"},
			{"gio", "open"},
			{"gnome-open"},
			{"x-www-browser"},
		}
	}
}
