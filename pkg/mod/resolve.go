// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mod

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
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
//
// The replace directives in the go.mod files of the modules on disk are
// honoured: a requirement replaced by a directory is read from that
// directory, one replaced by another module version is read from that
// version. When the modules disagree, the module with the lowest module path
// wins.
func (rsv *Resolver) Resolve(
	ctx context.Context,
	mods map[string]*Module,
) error {

	rps, err := readReplaces(mods)
	if err != nil {
		return err
	}
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
		reqs, err := rsv.expand(ctx, req, rps, mods)
		if err != nil {
			return err
		}
		queue = append(queue, reqs...)
	}
	return nil
}

// expand reads the go.mod file of the required module version, or of its
// replacement, merges its requirements into the module in mods, and returns
// the requirements to follow next.
func (rsv *Resolver) expand(
	ctx context.Context,
	req Require,
	rps replaces,
	mods map[string]*Module,
) ([]Require, error) {

	data, err := rsv.read(ctx, req, rps)
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

// read returns the content of the go.mod file of the required module version
// or of its replacement.
func (rsv *Resolver) read(
	ctx context.Context,
	req Require,
	rps replaces,
) ([]byte, error) {

	rpl, ok := rps.lookup(req)
	switch {
	case !ok:
		rsv.logf("resolving %s@%s", req.Path, req.Ver)
		return rsv.fch.fetch(ctx, req.Path, req.Ver)

	case rpl.dir != "":
		rsv.logf("resolving %s@%s => %s", req.Path, req.Ver, rpl.dir)
		pth := filepath.Join(rpl.dir, "go.mod")
		data, err := os.ReadFile(pth) //nolint:gosec
		if err != nil {
			return nil, fmt.Errorf("read module file: %w", err)
		}
		return data, nil

	default:
		format := "resolving %s@%s => %s@%s"
		rsv.logf(format, req.Path, req.Ver, rpl.pth, rpl.ver)
		return rsv.fch.fetch(ctx, rpl.pth, rpl.ver)
	}
}

// Close removes the temporary resources the resolver created.
func (rsv *Resolver) Close() error { return rsv.fch.close() }

// replacement is what a replace directive substitutes for a requirement:
// either a directory or another module version.
type replacement struct {
	dir string // Directory of a local replacement, empty otherwise.
	pth string // Module path of a version replacement.
	ver string // Module version of a version replacement.
}

// replaces maps the replaced requirements to their replacements. A key with
// an empty version replaces every version of the module path.
type replaces map[Require]replacement

// readReplaces returns the replace directives of the go.mod files of the
// modules on disk. A module whose directory holds no go.mod file replaces
// nothing.
func readReplaces(mods map[string]*Module) (replaces, error) {
	rps := make(replaces)
	for _, name := range slices.Sorted(maps.Keys(mods)) {
		mod := mods[name]
		if mod.Dir == "" {
			continue
		}
		pth := filepath.Join(mod.Dir, "go.mod")
		data, err := os.ReadFile(pth) //nolint:gosec
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read module file: %w", err)
		}
		fil, err := modfile.Parse(pth, data, nil)
		if err != nil {
			return nil, fmt.Errorf("parse module file: %w", err)
		}
		for _, rep := range fil.Replace {
			key := Require{Path: rep.Old.Path, Ver: rep.Old.Version}
			if _, ok := rps[key]; ok {
				continue
			}
			rps[key] = newReplacement(mod.Dir, rep.New.Path, rep.New.Version)
		}
	}
	return rps, nil
}

// newReplacement returns the replacement a replace directive in the go.mod
// file in dir names. A relative replacement directory is relative to dir.
func newReplacement(dir, pth, ver string) replacement {
	if ver != "" {
		return replacement{pth: pth, ver: ver}
	}
	if !filepath.IsAbs(pth) {
		pth = filepath.Join(dir, pth)
	}
	return replacement{dir: pth}
}

// lookup returns the replacement of the required module version: the one
// naming that version, else the one replacing every version.
func (rps replaces) lookup(req Require) (replacement, bool) {
	if rpl, ok := rps[req]; ok {
		return rpl, true
	}
	rpl, ok := rps[Require{Path: req.Path}]
	return rpl, ok
}

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
		if ctx.Err() != nil {
			return nil, fmt.Errorf("query %s: %w", arg, ctx.Err())
		}
		var exe *exec.ExitError
		if errors.As(err, &exe) && len(exe.Stderr) > 0 {
			msg := strings.TrimSpace(string(exe.Stderr))
			return nil, fmt.Errorf("query %s: %s: %w", arg, msg, err)
		}
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
