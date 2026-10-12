// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ctx42/gitaid/pkg/gitaid"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"

	"github.com/ctx42/modmap/pkg/mod"
)

// breakingRx matches the subject of a Conventional Commit marked breaking by
// a "!" before the colon.
var breakingRx = regexp.MustCompile(`^[a-zA-Z]+(\([^)]*\))?!:`)

// lag represents the requirements of a planned module on other planned
// modules which the update lifts.
type lag struct {
	// Path is the module path of the dependent.
	Path string `json:"path"`

	// Requires holds the requirements behind, sorted by module path.
	Requires []lagReq `json:"requires"`
}

// lagReq represents how far a requirement is behind the code the update
// releases: the HEAD of the required module's local clone.
type lagReq struct {
	// Path is the module path of the required module.
	Path string `json:"path"`

	// Pinned is the version the dependent requires.
	Pinned string `json:"pinned"`

	// Latest is the highest release tag of the module's major version in
	// its local clone; empty when there is none.
	Latest string `json:"latest,omitempty"`

	// Tags is the number of release tags of that major above Pinned.
	Tags int `json:"tags"`

	// Commits is the number of commits from Pinned to HEAD.
	Commits int `json:"commits"`

	// Breaking holds the subjects of the commits from Pinned to HEAD marked
	// breaking, oldest first.
	Breaking []string `json:"breaking,omitempty"`

	// Missing reports that the local clone does not hold the revision
	// Pinned names, so Commits and Breaking stay empty.
	Missing bool `json:"missing,omitempty"`
}

// lags returns the lag of every module in the rounds requiring other modules
// in the rounds, sorted by module path; nil when none lags. A requirement is
// listed when HEAD of the required module's local clone is ahead of it, or
// when the clone does not hold the revision it pins.
func lags(
	ctx context.Context,
	mods map[string]*mod.Module,
	rounds [][]string,
) ([]lag, error) {

	planned := make(map[string]bool)
	for _, rnd := range rounds {
		for _, pth := range rnd {
			planned[pth] = true
		}
	}

	var lgs []lag
	for _, rnd := range rounds {
		for _, pth := range rnd {
			var reqs []lagReq
			for _, req := range mods[pth].Requires {
				if !planned[req.Path] {
					continue
				}
				lrq, err := behind(ctx, mods[req.Path], req.Ver)
				if err != nil {
					return nil, err
				}
				if lrq.Commits > 0 || lrq.Missing {
					reqs = append(reqs, lrq)
				}
			}
			if len(reqs) > 0 {
				lgs = append(lgs, lag{Path: pth, Requires: reqs})
			}
		}
	}
	slices.SortFunc(lgs, func(a, b lag) int {
		return strings.Compare(a.Path, b.Path)
	})
	return lgs, nil
}

// behind returns how far the version ver of the module req is behind HEAD of
// the module's local clone.
func behind(ctx context.Context, req *mod.Module, ver string) (lagReq, error) {
	lrq := lagReq{Path: req.Path, Pinned: ver}
	prefix, err := tagPrefix(ctx, req.Dir)
	if err != nil {
		return lrq, fmt.Errorf("lag of %s: %w", req.Path, err)
	}

	tags, err := gitaid.Tags(ctx, req.Dir)
	if err != nil {
		return lrq, fmt.Errorf("lag of %s: %w", req.Path, err)
	}
	pinned := strings.TrimSuffix(ver, "+incompatible")
	major := majorOf(req.Path)
	for _, tag := range tags {
		v, ok := strings.CutPrefix(tag, prefix)
		if !ok || !isRelease(v, major) {
			continue
		}
		if semver.Compare(v, pinned) > 0 {
			lrq.Tags++
		}
		if lrq.Latest == "" || semver.Compare(v, lrq.Latest) > 0 {
			lrq.Latest = v
		}
	}

	rev := prefix + pinned
	if module.IsPseudoVersion(pinned) {
		// A pseudo-version cannot fail to parse once it is recognized.
		rev, _ = module.PseudoVersionRev(pinned)
	}
	rng := rev + "..HEAD"
	cnt, err := gitaid.CountCommits(ctx, req.Dir, rng)
	if errors.Is(err, gitaid.ErrUnkRev) {
		lrq.Missing = true
		return lrq, nil
	}
	if err != nil {
		return lrq, fmt.Errorf("lag of %s: %w", req.Path, err)
	}
	lrq.Commits = cnt
	if cnt == 0 {
		return lrq, nil
	}

	msgs, err := gitaid.Messages(ctx, req.Dir, rng)
	if err != nil {
		return lrq, fmt.Errorf("lag of %s: %w", req.Path, err)
	}
	for _, msg := range msgs {
		if isBreaking(msg) {
			subject, _, _ := strings.Cut(msg, "\n")
			lrq.Breaking = append(lrq.Breaking, subject)
		}
	}
	return lrq, nil
}

// tagPrefix returns the prefix the version tags of the module in dir carry:
// the path of dir relative to the top of its working tree, with a trailing
// slash, or the empty string for a module at the top.
func tagPrefix(ctx context.Context, dir string) (string, error) {
	top, err := gitaid.TopLevel(ctx, dir)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(top, dir)
	if err != nil {
		return "", fmt.Errorf("tag prefix: %w", err)
	}
	if rel == "." {
		return "", nil
	}
	return filepath.ToSlash(rel) + "/", nil
}

// majorOf returns the major version the module path names, such as "v2" for
// "example.com/m/v2"; the empty string for a path naming none.
func majorOf(pth string) string {
	_, pathMajor, _ := module.SplitPathVersion(pth)
	return strings.TrimLeft(pathMajor, "/.")
}

// isRelease reports whether v is a canonical release version, not a
// pre-release, of the major version major. The empty major stands for the
// versions v0 and v1, the ones a module path without a major suffix holds.
func isRelease(v, major string) bool {
	if semver.Canonical(v) != v || semver.Prerelease(v) != "" {
		return false
	}
	if major == "" {
		return semver.Major(v) == "v0" || semver.Major(v) == "v1"
	}
	return semver.Major(v) == major
}

// isBreaking reports whether the commit message msg marks a breaking change:
// a "!" before the colon of its Conventional Commit subject, or a
// "BREAKING CHANGE:" or "BREAKING-CHANGE:" footer.
func isBreaking(msg string) bool {
	if breakingRx.MatchString(msg) {
		return true
	}
	for line := range strings.Lines(msg) {
		if strings.HasPrefix(line, "BREAKING CHANGE:") ||
			strings.HasPrefix(line, "BREAKING-CHANGE:") {

			return true
		}
	}
	return false
}
