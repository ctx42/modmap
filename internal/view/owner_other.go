// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

//go:build !unix

package view

import (
	"io/fs"
)

// ownedByUser reports whether the file belongs to the current user, which
// is always taken to be the case where file owners are not told apart.
func ownedByUser(fs.FileInfo) bool { return true }
