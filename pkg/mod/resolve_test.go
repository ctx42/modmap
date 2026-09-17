// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mod

import (
	"context"
	"os"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
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
		assert.ErrorContain(t, "query example.com/nope@v1.0.0", err)
		assert.Nil(t, have)
	})
}
