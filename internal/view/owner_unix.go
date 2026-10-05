// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build unix

package view

import (
	"io/fs"
	"os"
	"syscall"
)

// ownedByUser reports whether the file belongs to the current user. A file
// whose owner cannot be told is taken for the user's own.
func ownedByUser(inf fs.FileInfo) bool {
	sta, ok := inf.Sys().(*syscall.Stat_t)
	return !ok || int(sta.Uid) == os.Geteuid()
}
