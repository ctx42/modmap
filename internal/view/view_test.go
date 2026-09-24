// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package view

import (
	"path/filepath"
	"testing"

	"github.com/ctx42/testing/pkg/assert"
	"github.com/ctx42/testkit/pkg/oskit"
)

func Test_Open(t *testing.T) {
	t.Run("the page is written and the browser opened", func(t *testing.T) {
		// --- Given ---
		// os.TempDir reads the process environment, which no ring reaches.
		tmp := t.TempDir()
		t.Setenv("TMPDIR", tmp)
		var opened string
		opener := func(url string) error { opened = url; return nil }

		// --- When ---
		have, err := Open("ctx42", []byte(`<svg id="m"/>`), opener)

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(tmp, dirName, "ctx42.html"), have)
		assert.Equal(t, "file://"+have, opened)

		page := oskit.ReadFileStr(t, have)
		assert.Contain(t, "<title>ctx42</title>", page)
		assert.Contain(t, `<svg id="m"/>`, page)
		assert.Contain(t, `href="ctx42.svg"`, page)

		doc := oskit.ReadFileStr(t, tmp, dirName, "ctx42.svg")
		assert.Equal(t, `<svg id="m"/>`, doc)
	})

	t.Run("the browser opener defaults to the desktop one", func(t *testing.T) {
		// --- Given ---
		// Nothing on the PATH can open a browser, so nothing is launched.
		tmp := t.TempDir()
		t.Setenv("TMPDIR", tmp)
		t.Setenv("PATH", tmp)

		// --- When ---
		have, err := Open("ctx42", []byte("<svg/>"), nil)

		// --- Then ---
		assert.ErrorIs(t, ErrNoBrowser, err)
		assert.Equal(t, filepath.Join(tmp, dirName, "ctx42.html"), have)
	})

	t.Run("error - the page cannot be written", func(t *testing.T) {
		// --- Given ---
		tmp := t.TempDir()
		t.Setenv("TMPDIR", oskit.Create(t, "", tmp, "file"))

		// --- When ---
		have, err := Open("ctx42", []byte("<svg/>"), nil)

		// --- Then ---
		assert.ErrorContain(t, "create map directory", err)
		assert.Equal(t, "", have)
	})
}

func Test_write(t *testing.T) {
	t.Run("a map with no usable title", func(t *testing.T) {
		// --- Given ---
		dir := filepath.Join(t.TempDir(), dirName)

		// --- When ---
		have, err := write(dir, "/ ", []byte("<svg/>"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, defName+".html"), have)
		assert.True(t, oskit.PathExists(t, dir, defName+".svg"))
	})

	t.Run("a rerun rewrites the same files", func(t *testing.T) {
		// --- Given ---
		dir := t.TempDir()
		_, err := write(dir, "ctx42", []byte("<svg id=\"old\"/>"))
		assert.NoError(t, err)

		// --- When ---
		have, err := write(dir, "ctx42", []byte("<svg id=\"new\"/>"))

		// --- Then ---
		assert.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, "ctx42.html"), have)
		assert.Contain(t, "<svg id=\"new\"/>", oskit.ReadFileStr(t, have))

		assert.Equal(t, 2, len(oskit.List(t, dir)))
	})

	t.Run("error - the directory cannot be created", func(t *testing.T) {
		// --- Given ---
		dir := oskit.Create(t, "", t.TempDir(), "file")

		// --- When ---
		have, err := write(filepath.Join(dir, "sub"), "ctx42", nil)

		// --- Then ---
		assert.ErrorContain(t, "create map directory", err)
		assert.Equal(t, "", have)
	})
}

func Test_html(t *testing.T) {
	// --- Given ---
	doc := []byte(`<svg id="m"><g/></svg>`)

	// --- When ---
	have, err := html("the map", "the-map.svg", doc)

	// --- Then ---
	assert.NoError(t, err)
	assert.Contain(t, "<title>the map</title>", string(have))
	assert.Contain(t, `<svg id="m"><g/></svg>`, string(have))
	assert.Contain(t, `href="the-map.svg"`, string(have))
	assert.Contain(t, "drag to pan", string(have))
}

func Test_slug_tabular(t *testing.T) {
	tt := []struct {
		testN string

		title string
		want  string
	}{
		{"a plain name", "ctx42", "ctx42"},
		{"a path", "github.com/ctx42/modmap", "github-com-ctx42-modmap"},
		{"spaces", "the map", "the-map"},
		{"underscores are kept", "the_map", "the_map"},
		{"trimmed to what is usable", "/src/", "src"},
		{"nothing usable", "//", defName},
		{"empty", "", defName},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := slug(tc.title)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}

func Test_fileURL_tabular(t *testing.T) {
	tt := []struct {
		testN string

		pth  string
		want string
	}{
		{"an absolute path", "/tmp/modmap/a.html", "file:///tmp/modmap/a.html"},
		{"a space", "/tmp/the map.html", "file:///tmp/the%20map.html"},
		{"a relative path", "a.html", "file:///a.html"},
	}

	for _, tc := range tt {
		t.Run(tc.testN, func(t *testing.T) {
			// --- When ---
			have := fileURL(tc.pth)

			// --- Then ---
			assert.Equal(t, tc.want, have)
		})
	}
}
