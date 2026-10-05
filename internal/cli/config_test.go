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
	t.Run("arguments parsed", func(t *testing.T) {
		// --- When ---
		have, err := newConfig([]string{"-o", "/out.svg", "/src"})

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/out.svg", have.out)
		assert.Equal(t, []string{"/src"}, have.roots)
	})

	t.Run("error - undefined option", func(t *testing.T) {
		// --- When ---
		have, err := newConfig([]string{"--nope", "/src"})

		// --- Then ---
		assert.ErrorContain(t, "flag provided but not defined: -nope", err)
		assert.Nil(t, have)
	})
}

func Test_config_flags(t *testing.T) {
	t.Run("options registered", func(t *testing.T) {
		// --- Given ---
		cfg := &config{}

		// --- When ---
		have := cfg.flags()

		// --- Then ---
		assert.NotNil(t, have)
		for _, name := range []string{
			"include", "i", "exclude", "e", "out", "o", "config", "c",
			"web", "yes", "y", "help", "h",
		} {
			assert.NotNil(t, cfg.fs.Lookup(name))
		}
	})

	t.Run("values applied", func(t *testing.T) {
		// --- Given ---
		cfg := &config{}
		apply := cfg.flags()
		must.Nil(cfg.fs.Parse([]string{
			"-i", "a/*", "-e", "b/*", "-o", "out.svg", "-c", "m.yaml",
			"--web", "-y", "-h",
		}))

		// --- When ---
		apply()

		// --- Then ---
		assert.Equal(t, []string{"a/*"}, cfg.include)
		assert.Equal(t, []string{"b/*"}, cfg.exclude)
		assert.Equal(t, "out.svg", cfg.out)
		assert.Equal(t, "m.yaml", cfg.conf)
		assert.True(t, cfg.web)
		assert.True(t, cfg.yes)
		assert.True(t, cfg.showHelp)
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

	t.Run("web option leaves the directory to be scanned", func(t *testing.T) {
		// --- Given ---
		args := []string{"--web", "/src"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, cfg.web)
		assert.Equal(t, []string{"/src"}, cfg.roots)
		assert.Equal(t, "", cfg.out)
	})

	t.Run("web option names one configured map", func(t *testing.T) {
		// --- Given ---
		args := []string{"--web", "-c", "/etc/modmap.yaml", "ctx42"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.NoError(t, err)
		assert.True(t, cfg.web)
		assert.Equal(t, []string{"ctx42"}, cfg.names)
	})

	t.Run("error - web with an output file", func(t *testing.T) {
		// --- Given ---
		args := []string{"--web", "-o", "/tmp/out.svg", "/src"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.ErrorIs(t, errWebOut, err)
	})

	t.Run("error - web with no configured map named", func(t *testing.T) {
		// --- Given ---
		args := []string{"--web", "-c", "/etc/modmap.yaml"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.ErrorIs(t, errWebMap, err)
	})

	t.Run("error - web with several configured maps named", func(t *testing.T) {
		// --- Given ---
		args := []string{"--web", "-c", "/etc/modmap.yaml", "one", "two"}
		cfg := &config{}

		// --- When ---
		err := cfg.parse(args)

		// --- Then ---
		assert.ErrorIs(t, errWebMap, err)
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

func Test_config_parseConf(t *testing.T) {
	t.Run("relative path resolved", func(t *testing.T) {
		// --- Given ---
		cfg := parsedConfig(t, "-c", "modmap.yaml", "a", "b")

		// --- When ---
		err := cfg.parseConf("/wd")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/wd/modmap.yaml", cfg.conf)
		assert.Equal(t, []string{"a", "b"}, cfg.names)
	})

	t.Run("web with one map", func(t *testing.T) {
		// --- Given ---
		cfg := parsedConfig(t, "--web", "-c", "/m.yaml", "a")

		// --- When ---
		err := cfg.parseConf("/wd")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/m.yaml", cfg.conf)
		assert.Equal(t, []string{"a"}, cfg.names)
	})
}

func Test_config_parseConf_tabular(t *testing.T) {
	tt := []struct {
		testN string

		args []string
		want error
	}{
		{
			"error - with output",
			[]string{"-c", "m.yaml", "-o", "x.svg"},
			errConfOnly,
		},
		{
			"error - with include",
			[]string{"-c", "m.yaml", "-i", "x/*"},
			errConfOnly,
		},
		{
			"error - with exclude",
			[]string{"-c", "m.yaml", "-e", "x/*"},
			errConfOnly,
		},
		{
			"error - web without map",
			[]string{"--web", "-c", "m.yaml"},
			errWebMap,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			cfg := parsedConfig(t, tc.args...)

			// --- When ---
			err := cfg.parseConf("/wd")

			// --- Then ---
			assert.ErrorIs(t, tc.want, err)
		})
	}
}

func Test_config_parseDir(t *testing.T) {
	t.Run("output given", func(t *testing.T) {
		// --- Given ---
		cfg := parsedConfig(t, "-o", "out.svg", "src")

		// --- When ---
		err := cfg.parseDir("/wd")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"/wd/src"}, cfg.roots)
		assert.Equal(t, "/wd/out.svg", cfg.out)
	})

	t.Run("web given", func(t *testing.T) {
		// --- Given ---
		cfg := parsedConfig(t, "--web", "/src")

		// --- When ---
		err := cfg.parseDir("/wd")

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"/src"}, cfg.roots)
		assert.Equal(t, "", cfg.out)
	})
}

func Test_config_parseDir_tabular(t *testing.T) {
	tt := []struct {
		testN string

		args []string
		want error
	}{
		{"error - no directory", []string{"-o", "x.svg"}, errNoDir},
		{
			"error - two directories",
			[]string{"-o", "x.svg", "a", "b"},
			errManyDirs,
		},
		{"error - no output", []string{"src"}, errNoOut},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			cfg := parsedConfig(t, tc.args...)

			// --- When ---
			err := cfg.parseDir("/wd")

			// --- Then ---
			assert.ErrorIs(t, tc.want, err)
		})
	}
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
