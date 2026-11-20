// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package conf

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testing/pkg/must"
	"github.com/ctx42/testkit/pkg/oskit"

	"github.com/ctx42/modmap/pkg/mod"
)

func Test_Load(t *testing.T) {
	t.Run("groups are read", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		pth := writeConf(t, ""+
			"out: /out/ctx42.svg\n"+
			"groups:\n"+
			"  - name: ctx42\n"+
			"    dirs: [/src/ctx42]\n"+
			"    include: ['github.com/ctx42/*']\n"+
			"    exclude: ['github.com/ctx42/tst-*']\n", dir)

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "/out/ctx42.svg", have.Out)
		assert.Len(t, 1, have.Groups)
		assert.Equal(t, "ctx42", have.Groups[0].Name)
		assert.Equal(t, []string{"/src/ctx42"}, have.Groups[0].Dirs)
		wInc := []string{"github.com/ctx42/*"}
		assert.Equal(t, wInc, have.Groups[0].Include)
		wExc := []string{"github.com/ctx42/tst-*"}
		assert.Equal(t, wExc, have.Groups[0].Exclude)
	})

	t.Run("relative paths resolve against the file", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		pth := writeConf(t, ""+
			"out: tmp/here.svg\n"+
			"groups:\n"+
			"  - name: here\n"+
			"    dirs: [..]\n", dir)

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "tmp", "here.svg"), have.Out)
		assert.Equal(t, []string{filepath.Dir(dir)}, have.Groups[0].Dirs)
	})

	t.Run("error - file does not exist", func(t *testing.T) {
		// --- Given ---
		pth := filepath.Join(t.TempDir(), "nope.yaml")

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorIs(t, fs.ErrNotExist, err)
		assert.ErrorContain(t, "read configuration", err)
		assert.Nil(t, have)
	})

	t.Run("error - malformed file", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(t, "groups: [", t.TempDir())

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorContain(t, "parse "+pth+": ", err)
		assert.Nil(t, have)
	})

	t.Run("error - no output", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(t, ""+
			"groups:\n"+
			"  - name: a\n"+
			"    dirs: [src]\n",
			t.TempDir(),
		)

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorIs(t, ErrNoOut, err)
		assert.Nil(t, have)
	})

	t.Run("error - output on a group", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(t, ""+
			"out: all.svg\n"+
			"groups:\n"+
			"  - name: a\n"+
			"    dirs: [src]\n"+
			"    out: a.svg\n",
			t.TempDir(),
		)

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorRegexp(t, "^parse "+pth+": .*out", err)
		assert.Nil(t, have)
	})

	t.Run("error - maps key", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(t, ""+
			"out: a.svg\n"+
			"maps:\n"+
			"  - name: a\n"+
			"    dirs: [src]\n",
			t.TempDir(),
		)

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorIs(t, ErrMapsKey, err)
		want := "parse " + pth + ": maps was renamed to groups"
		assert.ErrorEqual(t, want, err)
		assert.Nil(t, have)
	})

	t.Run("error - maps key beside groups", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(t, ""+
			"out: a.svg\n"+
			"maps: []\n"+
			"groups:\n"+
			"  - name: a\n"+
			"    dirs: [src]\n",
			t.TempDir(),
		)

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorIs(t, ErrMapsKey, err)
		assert.Nil(t, have)
	})

	t.Run("error - unknown key", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(t, ""+
			"out: a.svg\n"+
			"groups:\n"+
			"  - name: a\n"+
			"    dirs: [src]\n"+
			"    exlude: [example.com/x]\n",
			t.TempDir(),
		)

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorRegexp(t, "^parse "+pth+": .*exlude", err)
		assert.Nil(t, have)
	})

	t.Run("error - invalid configuration", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(t, "out: a.svg\ngroups: []\n", t.TempDir())

		// --- When ---
		have, err := Load(pth)

		// --- Then ---
		assert.ErrorIs(t, ErrNoGroups, err)
		assert.ErrorEqual(t, "validate "+pth+": no groups declared", err)
		assert.Nil(t, have)
	})

	t.Run("files merge in the order given", func(t *testing.T) {
		// --- Given ---
		pa := writeConf(t, ""+
			"out: a.svg\n"+
			"groups:\n"+
			"  - {name: a1, dirs: [src]}\n"+
			"  - {name: a2, dirs: [/a2]}\n", t.TempDir())
		pb := writeConf(t, ""+
			"groups:\n"+
			"  - {name: b1, dirs: [src]}\n", t.TempDir())

		// --- When ---
		have, err := Load(pa, pb)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, []string{"a1", "a2", "b1"}, have.Names())
		wDirs := []string{filepath.Join(filepath.Dir(pa), "src")}
		assert.Equal(t, wDirs, have.Groups[0].Dirs)
		wDirs = []string{filepath.Join(filepath.Dir(pb), "src")}
		assert.Equal(t, wDirs, have.Groups[2].Dirs)
		assert.Equal(t, filepath.Join(filepath.Dir(pa), "a.svg"), have.Out)
		assert.Equal(t, pa, have.OutFrom)
		assert.Empty(t, have.OutIgnored)
	})

	t.Run("the last output wins", func(t *testing.T) {
		// --- Given ---
		pa := writeConf(
			t,
			"out: a.svg\ngroups: [{name: a, dirs: [/a]}]\n",
			t.TempDir(),
		)
		pb := writeConf(
			t,
			"out: b.svg\ngroups: [{name: b, dirs: [/b]}]\n",
			t.TempDir(),
		)
		pc := writeConf(t, "groups: [{name: c, dirs: [/c]}]\n", t.TempDir())

		// --- When ---
		have, err := Load(pa, pb, pc)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(filepath.Dir(pb), "b.svg"), have.Out)
		assert.Equal(t, pb, have.OutFrom)
		assert.Equal(t, []string{pa}, have.OutIgnored)
	})

	t.Run("error - no file", func(t *testing.T) {
		// --- When ---
		have, err := Load()

		// --- Then ---
		assert.ErrorIs(t, ErrNoFiles, err)
		assert.Nil(t, have)
	})

	t.Run("error - no output in any file", func(t *testing.T) {
		// --- Given ---
		pa := writeConf(t, "groups: [{name: a, dirs: [/a]}]\n", t.TempDir())
		pb := writeConf(t, "groups: [{name: b, dirs: [/b]}]\n", t.TempDir())

		// --- When ---
		have, err := Load(pa, pb)

		// --- Then ---
		assert.ErrorIs(t, ErrNoOut, err)
		assert.ErrorEqual(t, "no output file ("+pa+", "+pb+")", err)
		assert.Nil(t, have)
	})

	t.Run("error - group name in two files", func(t *testing.T) {
		// --- Given ---
		pa := writeConf(
			t,
			"out: a.svg\ngroups: [{name: x, dirs: [/a]}]\n",
			t.TempDir(),
		)
		pb := writeConf(t, "groups: [{name: x, dirs: [/b]}]\n", t.TempDir())

		// --- When ---
		have, err := Load(pa, pb)

		// --- Then ---
		assert.ErrorIs(t, ErrDupName, err)
		want := "duplicate group name: x (" + pa + ", " + pb + ")"
		assert.ErrorEqual(t, want, err)
		assert.Nil(t, have)
	})

	t.Run("error - file with no groups beside others", func(t *testing.T) {
		// --- Given ---
		pa := writeConf(
			t,
			"out: a.svg\ngroups: [{name: a, dirs: [/a]}]\n",
			t.TempDir(),
		)
		pb := writeConf(t, "groups: []\n", t.TempDir())

		// --- When ---
		have, err := Load(pa, pb)

		// --- Then ---
		assert.ErrorIs(t, ErrNoGroups, err)
		assert.ErrorEqual(t, "validate "+pb+": no groups declared", err)
		assert.Nil(t, have)
	})

	t.Run("error - same file twice", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(
			t,
			"out: a.svg\ngroups: [{name: a, dirs: [/a]}]\n",
			t.TempDir(),
		)

		// --- When ---
		have, err := Load(pth, pth)

		// --- Then ---
		assert.ErrorIs(t, ErrTwice, err)
		assert.ErrorEqual(t, "configuration "+pth+" given twice", err)
		assert.Nil(t, have)
	})

	t.Run("error - same file through a symbolic link", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(
			t,
			"out: a.svg\ngroups: [{name: a, dirs: [/a]}]\n",
			t.TempDir(),
		)
		lnk := filepath.Join(t.TempDir(), "link.yaml")
		must.Nil(os.Symlink(pth, lnk))

		// --- When ---
		have, err := Load(pth, lnk)

		// --- Then ---
		assert.ErrorIs(t, ErrTwice, err)
		assert.ErrorEqual(t, "configuration "+lnk+" given twice", err)
		assert.Nil(t, have)
	})
}

func Test_Config_Select(t *testing.T) {
	t.Run("no names select every group", func(t *testing.T) {
		// --- Given ---
		cfg := &Config{Groups: []Group{{Name: "a"}, {Name: "b"}}}

		// --- When ---
		have, err := cfg.Select(nil)

		// --- Then ---
		assert.NoError(t, err)
		assert.Len(t, 2, have)
	})

	t.Run("names select in the order they are given", func(t *testing.T) {
		// --- Given ---
		cfg := &Config{Groups: []Group{{Name: "a"}, {Name: "b"}}}

		// --- When ---
		have, err := cfg.Select([]string{"b", "a"})

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, "b", have[0].Name)
		assert.Equal(t, "a", have[1].Name)
	})

	t.Run("error - unknown name lists the known ones", func(t *testing.T) {
		// --- Given ---
		cfg := &Config{Groups: []Group{{Name: "a"}, {Name: "b"}}}

		// --- When ---
		have, err := cfg.Select([]string{"c"})

		// --- Then ---
		assert.ErrorIs(t, ErrUnkGroup, err)
		assert.ErrorContain(t, "have: a, b", err)
		assert.Nil(t, have)
	})
}

func Test_Config_Names(t *testing.T) {
	// --- Given ---
	cfg := &Config{Groups: []Group{{Name: "a"}, {Name: "b"}}}

	// --- When ---
	have := cfg.Names()

	// --- Then ---
	assert.Equal(t, []string{"a", "b"}, have)
}

func Test_Config_validate(t *testing.T) {
	valid := Group{Name: "a", Dirs: []string{"/src"}}

	t.Run("valid", func(t *testing.T) {
		// --- Given ---
		cfg := &Config{Out: "/out.svg", Groups: []Group{valid}}

		// --- When ---
		err := cfg.validate()

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error - no name", func(t *testing.T) {
		// --- Given ---
		groups := []Group{valid, {Dirs: []string{"/src"}}}
		cfg := &Config{Out: "/out.svg", Groups: groups}

		// --- When ---
		err := cfg.validate()

		// --- Then ---
		assert.ErrorEqual(t, "group without a name: groups[1]", err)
	})
}

func Test_Config_validate_tabular(t *testing.T) {
	valid := Group{Name: "a", Dirs: []string{"/src"}}

	tt := []struct {
		testN string

		out    string
		groups []Group
		want   error
	}{
		{"no groups", "/out.svg", nil, ErrNoGroups},
		{"no name", "/out.svg", []Group{{Dirs: []string{"/src"}}}, ErrNoName},
		{"duplicate name", "/out.svg", []Group{valid, valid}, ErrDupName},
		{"no dirs", "/out.svg", []Group{{Name: "a"}}, ErrNoDirs},
		{
			"empty dir",
			"/out.svg",
			[]Group{{Name: "a", Dirs: []string{""}}},
			ErrEmptyDir,
		},
		{
			"malformed include glob",
			"/out.svg",
			[]Group{
				{Name: "a", Dirs: []string{"/src"}, Include: []string{"["}},
			},
			mod.ErrInvGlob,
		},
		{
			"malformed exclude glob",
			"/out.svg",
			[]Group{
				{Name: "a", Dirs: []string{"/src"}, Exclude: []string{"["}},
			},
			mod.ErrInvGlob,
		},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- Given ---
			cfg := &Config{Out: tc.out, Groups: tc.groups}

			// --- When ---
			err := cfg.validate()

			// --- Then ---
			assert.ErrorIs(t, tc.want, err)
		})
	}
}

func Test_Config_resolve(t *testing.T) {
	t.Run("relative paths", func(t *testing.T) {
		// --- Given ---
		cfg := &Config{
			Out: "out/all.svg",
			Groups: []Group{
				{Name: "a", Dirs: []string{"src", "/abs"}},
				{Name: "b", Dirs: []string{""}},
			},
		}

		// --- When ---
		cfg.resolve("/cfg")

		// --- Then ---
		assert.Equal(t, "/cfg/out/all.svg", cfg.Out)
		want := []Group{
			{Name: "a", Dirs: []string{"/cfg/src", "/abs"}},
			{Name: "b", Dirs: []string{""}},
		}
		assert.Equal(t, want, cfg.Groups)
	})

	t.Run("empty output stays empty", func(t *testing.T) {
		// --- Given ---
		cfg := &Config{Groups: []Group{{Name: "a", Dirs: []string{"src"}}}}

		// --- When ---
		cfg.resolve("/cfg")

		// --- Then ---
		assert.Equal(t, "", cfg.Out)
	})
}

func Test_distinct(t *testing.T) {
	t.Run("different files", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		pa := writeConf(t, "", oskit.MkdirAll(t, dir, "a"))
		pb := writeConf(t, "", oskit.MkdirAll(t, dir, "b"))

		// --- When ---
		err := distinct([]string{pa, pb})

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("missing files compare as given", func(t *testing.T) {
		// --- When ---
		err := distinct([]string{"/nope/a.yaml", "/nope/b.yaml"})

		// --- Then ---
		assert.NoError(t, err)
	})

	t.Run("error - uncleaned path to the same file", func(t *testing.T) {
		// --- Given ---
		pth := writeConf(t, "", t.TempDir())
		dup := filepath.Dir(pth) + "/./" + filepath.Base(pth)

		// --- When ---
		err := distinct([]string{pth, dup})

		// --- Then ---
		assert.ErrorIs(t, ErrTwice, err)
		assert.ErrorEqual(t, "configuration "+dup+" given twice", err)
	})

	t.Run("error - missing file twice", func(t *testing.T) {
		// --- When ---
		err := distinct([]string{"/nope/a.yaml", "/nope/a.yaml"})

		// --- Then ---
		assert.ErrorIs(t, ErrTwice, err)
	})
}

func Test_hasKey_tabular(t *testing.T) {
	tt := []struct {
		testN string

		data string
		want bool
	}{
		{"present", "a: 1\nb: 2\n", true},
		{"present without a value", "a:\n", true},
		{"absent", "b: 2\n", false},
		{"nested only", "b:\n  a: 1\n", false},
		{"list document", "- a\n", false},
		{"malformed", "a: [", false},
		{"empty", "", false},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := hasKey([]byte(tc.data), "a")

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_absPath_tabular(t *testing.T) {
	tt := []struct {
		testN string

		pth  string
		want string
	}{
		{"relative", "out/a.svg", "/cfg/out/a.svg"},
		{"absolute", "/out/a.svg", "/out/a.svg"},
		{"absolute uncleaned", "/out/../a.svg", "/a.svg"},
		{"empty", "", ""},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := absPath("/cfg", tc.pth)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
