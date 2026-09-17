// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mod

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
)

// probeMod is the go.mod file of the throwaway module the go command runs in
// when it is asked about a module version.
const probeMod = "module modmap.local/probe\n\ngo 1.21\n"

// fetcher returns the content of a go.mod file of the given module version.
type fetcher interface {
	fetch(ctx context.Context, pth, ver string) ([]byte, error)
	close() error
}

var _ fetcher = (*goFetcher)(nil) // Compile time check.

// Resolver expands the requirements of the scanned modules into the full
// dependency closure. Modules found on disk are read from disk; the rest are
// read from the Go module cache, reaching the network only when the cache
// does not hold the go.mod file yet.
type Resolver struct {
	flt  Filter                           // Module path filter.
	fch  fetcher                          // Source of the go.mod files.
	logf func(format string, args ...any) // Progress reporter.
}

// NewResolver returns a resolver keeping only the modules passing flt and
// reporting its progress with logf. A nil logf discards progress messages.
// The env carries the environment the go command runs with.
func NewResolver(
	flt Filter,
	env []string,
	logf func(format string, args ...any),
) (*Resolver, error) {

	fch, err := newGoFetcher(env)
	if err != nil {
		return nil, err
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Resolver{flt: flt, fch: fch, logf: logf}, nil
}

// Resolve adds to mods every module reachable from the modules already in it,
// following direct requirements only. A module already in mods with a
// directory on disk keeps the requirements read from that disk copy; for a
// module which is not on disk, the requirements are the union of the
// requirements of every version the closure asked for.
func (rsv *Resolver) Resolve(
	ctx context.Context,
	mods map[string]*Module,
) error {

	var queue []Require
	for _, mod := range mods {
		queue = append(queue, mod.Requires...)
	}
	slices.SortFunc(queue, cmpRequire)

	seen := make(map[Require]bool, len(queue))
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		req := queue[0]
		queue = queue[1:]
		if seen[req] {
			continue
		}
		seen[req] = true

		if dst, ok := mods[req.Path]; ok && dst.Dir != "" {
			continue
		}
		reqs, err := rsv.expand(ctx, req, mods)
		if err != nil {
			return err
		}
		queue = append(queue, reqs...)
	}
	return nil
}

// expand reads the go.mod file of the required module version, merges its
// requirements into the module in mods, and returns the requirements to
// follow next.
func (rsv *Resolver) expand(
	ctx context.Context,
	req Require,
	mods map[string]*Module,
) ([]Require, error) {

	rsv.logf("resolving %s@%s", req.Path, req.Ver)
	data, err := rsv.fch.fetch(ctx, req.Path, req.Ver)
	if err != nil {
		return nil, err
	}
	src, err := parseModData(req.Path, data, rsv.flt)
	if err != nil {
		return nil, err
	}

	dst, ok := mods[req.Path]
	if !ok {
		dst = &Module{Path: req.Path}
		mods[req.Path] = dst
	}
	dst.Requires = sortRequires(append(dst.Requires, src.Requires...))
	return src.Requires, nil
}

// Close removes the temporary resources the resolver created.
func (rsv *Resolver) Close() error { return rsv.fch.close() }

// goFetcher reads go.mod files with the go command, which uses the module
// cache first and the module proxy only when the cache misses.
type goFetcher struct {
	dir string   // Directory with the probe module the go command runs in.
	env []string // Environment the go command runs with.
}

// newGoFetcher returns a fetcher running the go command with env in a
// throwaway module directory created in the operating system temporary
// directory.
func newGoFetcher(env []string) (*goFetcher, error) {
	dir, err := os.MkdirTemp("", "modmap-")
	if err != nil {
		return nil, fmt.Errorf("probe module: %w", err)
	}
	pth := filepath.Join(dir, "go.mod")
	if err = os.WriteFile(pth, []byte(probeMod), 0600); err != nil {
		return nil, fmt.Errorf("probe module: %w", err)
	}
	return &goFetcher{dir: dir, env: append(env, "GOWORK=off")}, nil
}

// close removes the probe module directory.
func (gof *goFetcher) close() error { return os.RemoveAll(gof.dir) }

func (gof *goFetcher) fetch(
	ctx context.Context,
	pth, ver string,
) ([]byte, error) {

	arg := pth + "@" + ver
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", arg)
	cmd.Dir = gof.dir
	cmd.Env = gof.env
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", arg, err)
	}
	var info struct {
		GoMod string `json:"GoMod"`
	}
	if err = json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("query %s: %w", arg, err)
	}
	if info.GoMod == "" {
		return nil, fmt.Errorf("%w: %s", ErrNoModFile, arg)
	}
	data, err := os.ReadFile(info.GoMod)
	if err != nil {
		return nil, fmt.Errorf("read module file: %w", err)
	}
	return data, nil
}
