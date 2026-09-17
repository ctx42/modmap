// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mod

import (
	"errors"
	"fmt"
	"path"
)

// ErrInvGlob is returned for a malformed module path glob.
var ErrInvGlob = errors.New("invalid module path glob")

// ValidateGlob returns an error when glob is not a valid module path glob.
func ValidateGlob(glob string) error {
	if _, err := path.Match(glob, ""); err != nil {
		return fmt.Errorf("%w: %s", ErrInvGlob, glob)
	}
	return nil
}

// Filter decides which module paths take part in the map. The globs use the
// [path.Match] syntax where the "*" wildcard does not cross a path separator.
type Filter struct {
	include []string // Globs a module path must match, any when empty.
	exclude []string // Globs removing a module path from the map.
}

// NewFilter returns a filter matching module paths against the include and
// exclude globs. The globs must be validated with [ValidateGlob] beforehand;
// a malformed glob never matches. The zero value matches every module path.
func NewFilter(include, exclude []string) Filter {
	return Filter{include: include, exclude: exclude}
}

// Match reports whether the module path passes the filter. A path matching
// the exclude globs never passes, even when it also matches the include ones.
func (flt Filter) Match(pth string) bool {
	for _, glob := range flt.exclude {
		if ok, _ := path.Match(glob, pth); ok {
			return false
		}
	}
	if len(flt.include) == 0 {
		return true
	}
	for _, glob := range flt.include {
		if ok, _ := path.Match(glob, pth); ok {
			return true
		}
	}
	return false
}
