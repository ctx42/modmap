// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ctx42/gitaid/pkg/gitaid"
	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/prjkit"

	"github.com/ctx42/modmap/pkg/mod"
)

func Test_lags(t *testing.T) {
	t.Run("only requirements behind on planned modules", func(t *testing.T) {
		// --- Given ---
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("module example.com/r\n", "go.mod")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("1", "file.txt")
		prj.GitCommit("v0.2.0", "feat!: drop Old")
		prj.Close()

		mods := map[string]*mod.Module{
			"example.com/r": {Path: "example.com/r", Dir: prj.Root()},
			"example.com/b": {
				Path: "example.com/b",
				Requires: []mod.Require{
					{Path: "example.com/other", Ver: "v1.0.0"},
					{Path: "example.com/r", Ver: "v0.1.0"},
				},
			},
			"example.com/a": {
				Path:     "example.com/a",
				Requires: []mod.Require{{Path: "example.com/r", Ver: "v0.1.0"}},
			},
			"example.com/c": {
				Path:     "example.com/c",
				Requires: []mod.Require{{Path: "example.com/r", Ver: "v0.2.0"}},
			},
		}
		rounds := [][]string{
			{"example.com/r"},
			{"example.com/b", "example.com/c"},
			{"example.com/a"},
		}

		// --- When ---
		have, err := lags(t.Context(), mods, rounds)

		// --- Then ---
		assert.NoError(t, err)
		lrq := lagReq{
			Path:     "example.com/r",
			Pinned:   "v0.1.0",
			Latest:   "v0.2.0",
			Tags:     1,
			Commits:  1,
			Breaking: []string{"feat!: drop Old"},
		}
		want := []lag{
			{Path: "example.com/a", Requires: []lagReq{lrq}},
			{Path: "example.com/b", Requires: []lagReq{lrq}},
		}
		assert.Equal(t, want, have)
	})

	t.Run("nothing behind", func(t *testing.T) {
		// --- Given ---
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("module example.com/r\n", "go.mod")
		prj.GitInitAddAll("v0.1.0")
		prj.Close()

		mods := map[string]*mod.Module{
			"example.com/r": {Path: "example.com/r", Dir: prj.Root()},
			"example.com/a": {
				Path:     "example.com/a",
				Requires: []mod.Require{{Path: "example.com/r", Ver: "v0.1.0"}},
			},
		}
		rounds := [][]string{{"example.com/r"}, {"example.com/a"}}

		// --- When ---
		have, err := lags(t.Context(), mods, rounds)

		// --- Then ---
		assert.NoError(t, err)
		assert.Nil(t, have)
	})

	t.Run("error - required module outside git", func(t *testing.T) {
		// --- Given ---
		mods := map[string]*mod.Module{
			"example.com/r": {Path: "example.com/r", Dir: t.TempDir()},
			"example.com/a": {
				Path:     "example.com/a",
				Requires: []mod.Require{{Path: "example.com/r", Ver: "v0.1.0"}},
			},
		}
		rounds := [][]string{{"example.com/r"}, {"example.com/a"}}

		// --- When ---
		have, err := lags(t.Context(), mods, rounds)

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrNotRepo, err)
		assert.ErrorContain(t, "lag of example.com/r", err)
		assert.Nil(t, have)
	})
}

func Test_behind(t *testing.T) {
	t.Run("tags and commits since the pinned tag", func(t *testing.T) {
		// --- Given ---
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("module example.com/r\n", "go.mod")
		prj.GitInitAddAll("v0.1.0")
		prj.CreateFileWith("1", "file.txt")
		prj.GitCommit("v0.2.0", "fix(r)!: return an error")
		prj.CreateFileWith("2", "file.txt")
		prj.GitCommit("v0.3.0-rc.1", "feat: add New")
		prj.CreateFileWith("3", "file.txt")
		prj.GitCommit("", "refactor: tidy\n\nBREAKING CHANGE: Run is gone.")
		prj.CreateFileWith("4", "file.txt")
		prj.GitCommit("v1.0.0", "chore: release")
		prj.Close()

		req := &mod.Module{Path: "example.com/r", Dir: prj.Root()}

		// --- When ---
		have, err := behind(t.Context(), req, "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)
		want := lagReq{
			Path:     "example.com/r",
			Pinned:   "v0.1.0",
			Latest:   "v1.0.0",
			Tags:     2,
			Commits:  4,
			Breaking: []string{"fix(r)!: return an error", "refactor: tidy"},
		}
		assert.Equal(t, want, have)
	})

	t.Run("pinned at head", func(t *testing.T) {
		// --- Given ---
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("module example.com/r\n", "go.mod")
		prj.GitInitAddAll("v0.1.0")
		prj.Close()

		req := &mod.Module{Path: "example.com/r", Dir: prj.Root()}

		// --- When ---
		have, err := behind(t.Context(), req, "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)
		want := lagReq{
			Path:   "example.com/r",
			Pinned: "v0.1.0",
			Latest: "v0.1.0",
		}
		assert.Equal(t, want, have)
	})

	t.Run("pseudo-version", func(t *testing.T) {
		// --- Given ---
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("module example.com/r\n", "go.mod")
		prj.GitInitAddAll()
		rev, _ := prj.Exe("git", "rev-parse", "--short=12", "HEAD")
		prj.CreateFileWith("1", "file.txt")
		prj.GitCommit("", "feat: add New")
		prj.Close()

		req := &mod.Module{Path: "example.com/r", Dir: prj.Root()}
		ver := "v0.0.0-20260101000000-" + strings.TrimSpace(rev)

		// --- When ---
		have, err := behind(t.Context(), req, ver)

		// --- Then ---
		assert.NoError(t, err)
		want := lagReq{Path: "example.com/r", Pinned: ver, Commits: 1}
		assert.Equal(t, want, have)
	})

	t.Run("module below the top of its repository", func(t *testing.T) {
		// --- Given ---
		prj := prjkit.New(t, t.TempDir())
		prj.CreateDir("sub")
		prj.CreateFileWith("module example.com/r/sub\n", "sub/go.mod")
		prj.GitInitAddAll("v9.0.0")
		prj.Exe("git", "tag", "sub/v0.1.0")
		prj.CreateFileWith("1", "sub/file.txt")
		prj.GitCommit("sub/v0.2.0", "feat: add New")
		prj.Close()

		dir := filepath.Join(prj.Root(), "sub")
		req := &mod.Module{Path: "example.com/r/sub", Dir: dir}

		// --- When ---
		have, err := behind(t.Context(), req, "v0.1.0")

		// --- Then ---
		assert.NoError(t, err)
		want := lagReq{
			Path:    "example.com/r/sub",
			Pinned:  "v0.1.0",
			Latest:  "v0.2.0",
			Tags:    1,
			Commits: 1,
		}
		assert.Equal(t, want, have)
	})

	t.Run("pinned revision missing", func(t *testing.T) {
		// --- Given ---
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("module example.com/r\n", "go.mod")
		prj.GitInitAddAll("v0.2.0")
		prj.Close()

		req := &mod.Module{Path: "example.com/r", Dir: prj.Root()}
		ver := "v0.1.1-0.20260101000000-0123456789ab"

		// --- When ---
		have, err := behind(t.Context(), req, ver)

		// --- Then ---
		assert.NoError(t, err)
		want := lagReq{
			Path:    "example.com/r",
			Pinned:  ver,
			Latest:  "v0.2.0",
			Tags:    1,
			Missing: true,
		}
		assert.Equal(t, want, have)
	})

	t.Run("error - not a repository", func(t *testing.T) {
		// --- Given ---
		req := &mod.Module{Path: "example.com/r", Dir: t.TempDir()}

		// --- When ---
		have, err := behind(t.Context(), req, "v0.1.0")

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrNotRepo, err)
		assert.ErrorContain(t, "lag of example.com/r", err)
		assert.Equal(t, lagReq{Path: "example.com/r", Pinned: "v0.1.0"}, have)
	})
}

func Test_tagPrefix(t *testing.T) {
	t.Run("module at the top", func(t *testing.T) {
		// --- Given ---
		prj := prjkit.New(t, t.TempDir())
		prj.CreateFileWith("module example.com/r\n", "go.mod")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		have, err := tagPrefix(t.Context(), prj.Root())

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, have)
	})

	t.Run("nested module", func(t *testing.T) {
		// --- Given ---
		prj := prjkit.New(t, t.TempDir())
		prj.CreateDir("a", "b")
		prj.CreateFileWith("module example.com/r/a/b\n", "a/b/go.mod")
		prj.GitInitAddAll()
		prj.Close()

		// --- When ---
		have, err := tagPrefix(t.Context(), filepath.Join(prj.Root(), "a", "b"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "a/b/", have)
	})

	t.Run("error - not a repository", func(t *testing.T) {
		// --- When ---
		have, err := tagPrefix(t.Context(), t.TempDir())

		// --- Then ---
		assert.ErrorIs(t, gitaid.ErrNotRepo, err)
		assert.Empty(t, have)
	})
}

func Test_majorOf_tabular(t *testing.T) {
	tt := []struct {
		testN string

		pth  string
		want string
	}{
		{"no suffix", "example.com/m", ""},
		{"v2 suffix", "example.com/m/v2", "v2"},
		{"gopkg.in", "gopkg.in/yaml.v3", "v3"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := majorOf(tc.pth)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_isRelease_tabular(t *testing.T) {
	tt := []struct {
		testN string

		v     string
		major string
		want  bool
	}{
		{"v0 release", "v0.2.0", "", true},
		{"v1 release", "v1.2.0", "", true},
		{"v2 without suffix", "v2.0.0", "", false},
		{"v2 with suffix", "v2.0.0", "v2", true},
		{"v1 with v2 suffix", "v1.0.0", "v2", false},
		{"pre-release", "v1.0.0-rc.1", "", false},
		{"build metadata", "v1.0.0+b", "", false},
		{"short", "v1.2", "", false},
		{"not a version", "release-1", "", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := isRelease(tc.v, tc.major)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_isBreaking_tabular(t *testing.T) {
	tt := []struct {
		testN string

		msg  string
		want bool
	}{
		{"bang", "feat!: drop Old", true},
		{"bang with scope", "fix(xrr)!: return an error", true},
		{"footer", "refactor: tidy\n\nBREAKING CHANGE: Run is gone.", true},
		{"hyphen footer", "refactor: tidy\n\nBREAKING-CHANGE: gone.", true},
		{"plain", "feat: add New", false},
		{"bang in body", "feat: add New\n\nfix!: not a subject", false},
		{"footer text mid line", "fix: note the BREAKING CHANGE: text", false},
		{"no type", "SKW-2935 Refactor expiry handling", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := isBreaking(tc.msg)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
