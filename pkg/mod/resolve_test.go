// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mod

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_NewResolver(t *testing.T) {
	// --- Given ---
	flt := NewFilter([]string{"example.com/*"}, nil)

	// --- When ---
	have, err := NewResolver(flt, os.Environ(), nil)

	// --- Then ---
	assert.NoError(t, err)
	assert.Equal(t, flt, have.flt)
	assert.NotNil(t, have.logf)
	assert.NoError(t, have.Close())
}

func Test_Resolver_Resolve(t *testing.T) {
	t.Run("the closure is expanded", func(t *testing.T) {
		// --- Given ---
		fkf := &fakeFetcher{mods: map[string]string{
			"example.com/b@v1.0.0": "" +
				"module example.com/b\n" +
				"require example.com/c v1.0.0\n",
			"example.com/c@v1.0.0": "module example.com/c\n",
		}}
		mods := map[string]*Module{
			"example.com/a": newModule(
				"example.com/a",
				"/src/a",
				"example.com/b",
			),
		}
		rsv := newTestResolver(Filter{}, fkf)

		// --- When ---
		err := rsv.Resolve(context.Background(), mods)

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 3, mods)
		assert.Equal(t, "", mods["example.com/b"].Dir)
		want := []Require{{Path: "example.com/c", Ver: "v1.0.0"}}
		assert.Equal(t, want, mods["example.com/b"].Requires)
		assert.Empty(t, mods["example.com/c"].Requires)
	})

	t.Run("requirements are the union across versions", func(t *testing.T) {
		// --- Given ---
		fkf := &fakeFetcher{mods: map[string]string{
			"example.com/x@v1.0.0": "" +
				"module example.com/x\n" +
				"require example.com/p v1.0.0\n",
			"example.com/x@v2.0.0": "" +
				"module example.com/x\n" +
				"require example.com/q v1.0.0\n",
			"example.com/p@v1.0.0": "module example.com/p\n",
			"example.com/q@v1.0.0": "module example.com/q\n",
		}}
		modA := newModule("example.com/a", "/src/a", "example.com/x")
		modB := newModule("example.com/b", "/src/b")
		reqX := Require{Path: "example.com/x", Ver: "v2.0.0"}
		modB.Requires = []Require{reqX}
		mods := map[string]*Module{
			"example.com/a": modA,
			"example.com/b": modB,
		}
		rsv := newTestResolver(Filter{}, fkf)

		// --- When ---
		err := rsv.Resolve(context.Background(), mods)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{"example.com/p", "example.com/q"}
		assert.Equal(t, want, mods["example.com/x"].Deps())

		wCalls := []string{
			"example.com/x@v1.0.0",
			"example.com/x@v2.0.0",
			"example.com/p@v1.0.0",
			"example.com/q@v1.0.0",
		}
		assert.Equal(t, wCalls, fkf.calls)
	})

	t.Run("a module on disk is never fetched", func(t *testing.T) {
		// --- Given ---
		fkf := &fakeFetcher{mods: map[string]string{}}
		mods := map[string]*Module{
			"example.com/a": newModule(
				"example.com/a",
				"/src/a",
				"example.com/b",
			),
			"example.com/b": {Path: "example.com/b", Dir: "/src/b"},
		}
		rsv := newTestResolver(Filter{}, fkf)

		// --- When ---
		err := rsv.Resolve(context.Background(), mods)

		// --- Then ---
		assert.NoError(t, err)
		assert.Empty(t, fkf.calls)
		assert.Len(t, 2, mods)
	})

	t.Run("local replacement is read from disk", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		dirA := filepath.Dir(writeMod(t, ""+
			"module example.com/a\n"+
			"require example.com/b v1.0.0\n"+
			"replace example.com/b => ../b\n",
			root, "a",
		))
		writeMod(t, ""+
			"module example.com/b\n"+
			"require example.com/c v1.0.0\n",
			root, "b",
		)

		fkf := &fakeFetcher{mods: map[string]string{
			"example.com/c@v1.0.0": "module example.com/c\n",
		}}
		mods := map[string]*Module{
			"example.com/a": newModule("example.com/a", dirA, "example.com/b"),
		}
		rsv := newTestResolver(Filter{}, fkf)

		// --- When ---
		err := rsv.Resolve(t.Context(), mods)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"example.com/c"}, mods["example.com/b"].Deps())
		assert.Equal(t, []string{"example.com/c@v1.0.0"}, fkf.calls)
	})

	t.Run("version replacement is fetched", func(t *testing.T) {
		// --- Given ---
		dirA := filepath.Dir(writeMod(t, ""+
			"module example.com/a\n"+
			"require example.com/x v1.0.0\n"+
			"replace example.com/x => example.com/fork v1.2.3\n",
			t.TempDir(), "a",
		))

		fkf := &fakeFetcher{mods: map[string]string{
			"example.com/fork@v1.2.3": "" +
				"module example.com/fork\n" +
				"require example.com/p v1.0.0\n",
			"example.com/p@v1.0.0": "module example.com/p\n",
		}}
		mods := map[string]*Module{
			"example.com/a": newModule("example.com/a", dirA, "example.com/x"),
		}
		rsv := newTestResolver(Filter{}, fkf)

		// --- When ---
		err := rsv.Resolve(t.Context(), mods)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"example.com/p"}, mods["example.com/x"].Deps())
		want := []string{"example.com/fork@v1.2.3", "example.com/p@v1.0.0"}
		assert.Equal(t, want, fkf.calls)
	})

	t.Run("error - local replacement without module file", func(t *testing.T) {
		// --- Given ---
		dirA := filepath.Dir(writeMod(t, ""+
			"module example.com/a\n"+
			"require example.com/b v1.0.0\n"+
			"replace example.com/b => ../b\n",
			t.TempDir(), "a",
		))

		fkf := &fakeFetcher{mods: map[string]string{}}
		mods := map[string]*Module{
			"example.com/a": newModule("example.com/a", dirA, "example.com/b"),
		}
		rsv := newTestResolver(Filter{}, fkf)

		// --- When ---
		err := rsv.Resolve(t.Context(), mods)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.Empty(t, fkf.calls)
	})

	t.Run("filtered requirements are not fetched", func(t *testing.T) {
		// --- Given ---
		fkf := &fakeFetcher{mods: map[string]string{
			"example.com/b@v1.0.0": "" +
				"module example.com/b\n" +
				"require other.com/c v1.0.0\n",
		}}
		mods := map[string]*Module{
			"example.com/a": newModule(
				"example.com/a",
				"/src/a",
				"example.com/b",
			),
		}
		flt := NewFilter([]string{"example.com/*"}, nil)
		rsv := newTestResolver(flt, fkf)

		// --- When ---
		err := rsv.Resolve(context.Background(), mods)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"example.com/b@v1.0.0"}, fkf.calls)
		assert.Len(t, 2, mods)
	})

	t.Run("error - module file cannot be fetched", func(t *testing.T) {
		// --- Given ---
		fkf := &fakeFetcher{mods: map[string]string{}}
		mods := map[string]*Module{
			"example.com/a": newModule(
				"example.com/a",
				"/src/a",
				"example.com/b",
			),
		}
		rsv := newTestResolver(Filter{}, fkf)

		// --- When ---
		err := rsv.Resolve(context.Background(), mods)

		// --- Then ---
		assert.ErrorContain(t, "module not served: example.com/b", err)
	})

	t.Run("error - context is cancelled", func(t *testing.T) {
		// --- Given ---
		fkf := &fakeFetcher{mods: map[string]string{}}
		mods := map[string]*Module{
			"example.com/a": newModule(
				"example.com/a",
				"/src/a",
				"example.com/b",
			),
		}
		rsv := newTestResolver(Filter{}, fkf)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// --- When ---
		err := rsv.Resolve(ctx, mods)

		// --- Then ---
		assert.ErrorIs(t, context.Canceled, err)
		assert.Empty(t, fkf.calls)
	})
}

func Test_Resolver_Close(t *testing.T) {
	// --- Given ---
	fkf := &fakeFetcher{}
	rsv := newTestResolver(Filter{}, fkf)

	// --- When ---
	err := rsv.Close()

	// --- Then ---
	assert.NoError(t, err)
	assert.True(t, fkf.closed)
}

func Test_readReplaces(t *testing.T) {
	t.Run("lowest module path wins", func(t *testing.T) {
		// --- Given ---
		root := t.TempDir()
		dirA := filepath.Dir(writeMod(t, ""+
			"module example.com/a\n"+
			"replace example.com/x => ../x\n",
			root, "a",
		))
		dirB := filepath.Dir(writeMod(t, ""+
			"module example.com/b\n"+
			"replace example.com/x v1.0.0 => example.com/y v1.1.0\n"+
			"replace example.com/x => ../other\n",
			root, "b",
		))
		mods := map[string]*Module{
			"example.com/a": {Path: "example.com/a", Dir: dirA},
			"example.com/b": {Path: "example.com/b", Dir: dirB},
			"example.com/c": {Path: "example.com/c"},
			"example.com/d": {Path: "example.com/d", Dir: root},
		}

		// --- When ---
		have, err := readReplaces(mods)

		// --- Then ---
		assert.NoError(t, err)
		want := replaces{
			{Path: "example.com/x"}: {
				dir: filepath.Join(root, "x"),
			},
			{Path: "example.com/x", Ver: "v1.0.0"}: {
				pth: "example.com/y",
				ver: "v1.1.0",
			},
		}
		assert.Equal(t, want, have)
	})

	t.Run("error - malformed module file", func(t *testing.T) {
		// --- Given ---
		dir := filepath.Dir(writeMod(t, "module\n", t.TempDir(), "a"))
		mods := map[string]*Module{
			"example.com/a": {Path: "example.com/a", Dir: dir},
		}

		// --- When ---
		have, err := readReplaces(mods)

		// --- Then ---
		assert.ErrorContain(t, "parse module file", err)
		assert.Nil(t, have)
	})
}

func Test_newReplacement_tabular(t *testing.T) {
	tt := []struct {
		testN string

		pth  string
		ver  string
		want replacement
	}{
		{
			"version",
			"example.com/y",
			"v1.0.0",
			replacement{pth: "example.com/y", ver: "v1.0.0"},
		},
		{"relative dir", "../y", "", replacement{dir: "/src/y"}},
		{"absolute dir", "/opt/y", "", replacement{dir: "/opt/y"}},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := newReplacement("/src/a", tc.pth, tc.ver)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_replaces_lookup_tabular(t *testing.T) {
	rps := replaces{
		{Path: "example.com/x"}:                {dir: "/src/x"},
		{Path: "example.com/x", Ver: "v1.0.0"}: {dir: "/src/x1"},
	}

	tt := []struct {
		testN string

		req    Require
		want   replacement
		wFound bool
	}{
		{
			"exact version",
			Require{Path: "example.com/x", Ver: "v1.0.0"},
			replacement{dir: "/src/x1"},
			true,
		},
		{
			"any version",
			Require{Path: "example.com/x", Ver: "v2.0.0"},
			replacement{dir: "/src/x"},
			true,
		},
		{
			"not replaced",
			Require{Path: "example.com/y", Ver: "v1.0.0"},
			replacement{},
			false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have, hFound := rps.lookup(tc.req)

			// --- Then ---
			assert.Equal(t, tc.want, have)
			assert.Equal(t, tc.wFound, hFound)
		})
	}
}

func Test_newGoFetcher(t *testing.T) {
	// --- When ---
	have, err := newGoFetcher(os.Environ())

	// --- Then ---
	assert.NoError(t, err)
	assert.Equal(t, probeMod, oskit.ReadFileStr(t, have.dir, "go.mod"))
	assert.NoError(t, have.close())
	assert.False(t, oskit.PathExists(t, have.dir))
}

func Test_goFetcher_fetch(t *testing.T) {
	t.Run("error - context done while querying", func(t *testing.T) {
		// --- Given ---
		if runtime.GOOS == "windows" {
			t.Skip("the fake go command is a shell script")
		}

		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		t.Cleanup(cancel)

		dir := t.TempDir()
		oskit.Create(t, "#!/bin/sh\nexec sleep 5\n", dir, "go")
		must.Nil(os.Chmod(filepath.Join(dir, "go"), 0o700))
		t.Setenv("PATH", dir)

		gof := must.Value(newGoFetcher(nil))
		t.Cleanup(func() { _ = gof.close() })

		// --- When ---
		have, err := gof.fetch(ctx, "example.com/a", "v1.0.0")

		// --- Then ---
		assert.ErrorIs(t, context.DeadlineExceeded, err)
		assert.Nil(t, have)
	})

	t.Run("error - module is not available offline", func(t *testing.T) {
		// --- Given ---
		env := append(os.Environ(), "GOPROXY=off", "GOFLAGS=-mod=mod")
		gof, err := newGoFetcher(env)
		assert.NoError(t, err)
		t.Cleanup(func() { _ = gof.close() })

		// --- When ---
		have, err := gof.fetch(
			context.Background(),
			"example.com/nope",
			"v1.0.0",
		)

		// --- Then ---
		var exe *exec.ExitError
		assert.ErrorAs(t, &exe, err)
		want := "^query example.com/nope@v1.0.0: .*GOPROXY=off"
		assert.ErrorRegexp(t, want, err)
		assert.Nil(t, have)
	})
}
