// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
)

func Test_newConfig(t *testing.T) {
	t.Run("one off mode", func(t *testing.T) {
		// --- Given ---
		args := []string{"-o", "/tmp/out.svg", "/src"}

		// --- When ---
		have, err := newConfig(args)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"/src"}, have.roots)
		assert.Equal(t, "/tmp/out.svg", have.out)
		assert.False(t, have.yes)
	})

	t.Run("error - unknown option", func(t *testing.T) {
		// --- Given ---
		args := []string{"--nope", "/src"}

		// --- When ---
		have, err := newConfig(args)

		// --- Then ---
		assert.ErrorContain(t, "nope", err)
		assert.Nil(t, have)
	})
}

func Test_config_parse(t *testing.T) {
	t.Run("relative paths are made absolute", func(t *testing.T) {
		// --- Given ---
		wd := must.Value(os.Getwd())
		args := []string{"-o", "out.svg", "src"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{filepath.Join(wd, "src")}, cfg.roots)
		assert.Equal(t, filepath.Join(wd, "out.svg"), cfg.out)
	})

	t.Run("globs are collected in order", func(t *testing.T) {
		// --- Given ---
		args := []string{
			"-i", "github.com/ctx42/*",
			"--include", "example.com/*",
			"-e", "*/internal",
			"-o", "/tmp/out.svg",
			"/src",
		}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.NoError(t, err)
		want := []string{"github.com/ctx42/*", "example.com/*"}
		assert.Equal(t, want, cfg.include)
		assert.Equal(t, []string{"*/internal"}, cfg.exclude)
	})

	t.Run("config mode collects map names", func(t *testing.T) {
		// --- Given ---
		args := []string{"-c", "/etc/modmap.yaml", "ctx42", "all"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/etc/modmap.yaml", cfg.conf)
		assert.Equal(t, []string{"ctx42", "all"}, cfg.names)
		assert.Nil(t, cfg.roots)
	})

	t.Run("help skips validation", func(t *testing.T) {
		// --- Given ---
		args := []string{"-h"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, cfg.showHelp)
	})

	t.Run("yes option is recognized", func(t *testing.T) {
		// --- Given ---
		args := []string{"--yes", "-o", "/tmp/out.svg", "/src"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, cfg.yes)
	})

	t.Run("error - no directory", func(t *testing.T) {
		// --- Given ---
		args := []string{"-o", "/tmp/out.svg"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.ErrorIs(t, errNoDir, err)
	})

	t.Run("error - more than one directory", func(t *testing.T) {
		// --- Given ---
		args := []string{"-o", "/tmp/out.svg", "/src", "/other"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.ErrorIs(t, errManyDirs, err)
	})

	t.Run("error - missing output", func(t *testing.T) {
		// --- Given ---
		args := []string{"/src"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.ErrorIs(t, errNoOut, err)
	})

	t.Run("error - config with output", func(t *testing.T) {
		// --- Given ---
		args := []string{"-c", "/etc/modmap.yaml", "-o", "/tmp/out.svg"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.ErrorIs(t, errConfOnly, err)
	})

	t.Run("error - config with include", func(t *testing.T) {
		// --- Given ---
		args := []string{"-c", "/etc/conf.yaml", "-i", "example.com/*"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.ErrorIs(t, errConfOnly, err)
	})

	t.Run("error - config with exclude", func(t *testing.T) {
		// --- Given ---
		args := []string{"-c", "/etc/conf.yaml", "-e", "example.com/*"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.ErrorIs(t, errConfOnly, err)
	})

	t.Run("error - malformed glob", func(t *testing.T) {
		// --- Given ---
		args := []string{"-i", "[", "-o", "/tmp/out.svg", "/src"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.ErrorContain(t, "invalid module path glob", err)
	})
}

func Test_config_help(t *testing.T) {
	// --- Given ---
	cfg := must.Value(newConfig([]string{"-h"}))

	// --- When ---
	have := cfg.help()

	// --- Then ---
	assert.Contain(t, "modmap [options] -o <file.svg> <dir>", have)
	assert.Contain(t, "modmap -c <config.yaml> [map...]", have)
	assert.Contain(t, "-i, --include", have)
	assert.Contain(t, "-o, --out", have)
}

func Test_usage(t *testing.T) {
	// --- When ---
	have := usage()

	// --- Then ---
	assert.Contain(t, "Usage:", have)
	assert.Contain(t, "-c, --config", have)
	assert.Contain(t, "-y, --yes", have)
}
