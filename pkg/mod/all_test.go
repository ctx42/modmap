// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mod

import (
	"context"
	"fmt"

	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/oskit"
)

// writeMod creates the directory and writes content to its go.mod file. It
// returns the path of the created go.mod file.
func writeMod(t tester.T, content, dir string, elems ...string) string {
	t.Helper()
	dir = oskit.MkdirAll(t, dir, elems...)
	return oskit.Create(t, content, dir, "go.mod")
}

// fakeFetcher serves go.mod file contents keyed by "path@version".
type fakeFetcher struct {
	mods   map[string]string // Module file contents.
	calls  []string          // Module versions fetched, in order.
	closed bool              // Set by the close method.
}

func (fkf *fakeFetcher) fetch(
	_ context.Context,
	pth, ver string,
) ([]byte, error) {

	key := pth + "@" + ver
	fkf.calls = append(fkf.calls, key)
	data, ok := fkf.mods[key]
	if !ok {
		return nil, fmt.Errorf("module not served: %s", key)
	}
	return []byte(data), nil
}

func (fkf *fakeFetcher) close() error {
	fkf.closed = true
	return nil
}

// newTestResolver returns a resolver serving go.mod files from fkf.
func newTestResolver(flt Filter, fkf *fakeFetcher) *Resolver {
	return &Resolver{flt: flt, fch: fkf, logf: func(string, ...any) {}}
}

// newModule returns a module found on disk requiring the given module paths,
// each at version v1.0.0.
func newModule(pth, dir string, reqs ...string) *Module {
	mod := &Module{Path: pth, Dir: dir}
	for _, req := range reqs {
		hav := Require{Path: req, Ver: "v1.0.0"}
		mod.Requires = append(mod.Requires, hav)
	}
	return mod
}
