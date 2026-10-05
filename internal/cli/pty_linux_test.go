// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"

	"github.com/ctx42/testing/pkg/tester"
	"golang.org/x/sys/unix"
)

// openPTY opens a new pseudo-terminal pair and returns its controller and
// its terminal end. What is written to the controller is read from the
// terminal end. Both are closed when the test ends.
func openPTY(t tester.T) (*os.File, *os.File) {
	t.Helper()
	ctl, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		t.Skip("no pseudo-terminal support:", err)
	}
	t.Cleanup(func() { _ = ctl.Close() })

	fd := int(ctl.Fd())
	if err = unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	num, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	pth := fmt.Sprintf("/dev/pts/%d", num)
	trm, err := os.OpenFile(pth, os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = trm.Close() })
	return ctl, trm
}
