// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mod

import (
	"errors"
	"fmt"
	"path"
	"slices"
)

// ErrInvGlob is returned for a malformed module path glob.
var ErrInvGlob = errors.New("invalid module path glob")

// ValidateGlob returns an error when glob is not a valid module path glob.
func ValidateGlob(glob string) error {
	if _, err := path.Match(glob, ""); err != nil {
		return fmt.Errorf("%w: %s: %w", ErrInvGlob, glob, err)
	}
	return nil
}

// Filter decides which module paths take part in the map. The globs use the
// [path.Match] syntax where the "*" wildcard does not cross a path separator.
type Filter struct {
	include []string // Globs a module path must match, any when empty.
	exclude []string // Globs removing a module path from the map.
	anyOf   []Filter // Filters one of which must pass a module path.
}

// NewFilter returns a filter matching module paths against the include and
// exclude globs. The globs must be validated with [ValidateGlob] beforehand;
// a malformed glob never matches. The zero value matches every module path.
func NewFilter(include, exclude []string) Filter {
	return Filter{include: include, exclude: exclude}
}

// AnyOf returns a filter passing a module path when any of flts passes it,
// so the modules of several maps are discovered in one go. Without filters it
// passes every module path, as the zero value does.
func AnyOf(flts ...Filter) Filter { return Filter{anyOf: flts} }

// Match reports whether the module path passes the filter. A path matching
// the exclude globs never passes, even when it also matches the include ones.
func (flt Filter) Match(pth string) bool {
	if len(flt.anyOf) > 0 {
		pass := func(alt Filter) bool { return alt.Match(pth) }
		return slices.ContainsFunc(flt.anyOf, pass)
	}
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
