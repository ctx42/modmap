// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/oskit"
)

// writeMod creates the directory and writes content to its go.mod file.
func writeMod(t tester.T, content, dir string, elems ...string) {
	t.Helper()
	dir = oskit.MkdirAll(t, dir, elems...)
	oskit.Create(t, content, dir, "go.mod")
}

// crossMaps writes the modules of two maps, "ctx42" holding
// github.com/ctx42/testing and "work" holding github.com/customer/a, which
// requires it, and the configuration declaring them. It returns the path of
// the configuration, which writes "all.svg" beside it.
func crossMaps(t tester.T) string {
	t.Helper()
	src := t.TempDir()
	writeMod(t, "module github.com/ctx42/testing\n", src, "ctx42", "testing")
	writeMod(t, ""+
		"module github.com/customer/a\n"+
		"require github.com/ctx42/testing v0.1.0\n",
		src, "customer", "a",
	)
	return oskit.Create(t, ""+
		"out: all.svg\n"+
		"maps:\n"+
		"  - name: ctx42\n"+
		"    dirs: ["+filepath.Join(src, "ctx42")+"]\n"+
		"    include: ['github.com/ctx42/*']\n"+
		"  - name: work\n"+
		"    dirs: ["+filepath.Join(src, "customer")+"]\n"+
		"    include: ['github.com/customer/*']\n",
		t.TempDir(), "modmap.yaml",
	)
}

// planMaps writes the modules of two maps and the configuration declaring
// them. Map "ctx42" holds github.com/ctx42/testing, github.com/ctx42/a, which
// requires github.com/customer/b, and github.com/ctx42/d, which requires
// github.com/ctx42/testing. Map "work" holds github.com/customer/b, which
// requires github.com/ctx42/testing. It returns the path of the configuration
// and the directory holding the modules.
func planMaps(t tester.T) (string, string) {
	t.Helper()
	src := t.TempDir()
	writeMod(t, "module github.com/ctx42/testing\n", src, "ctx42", "testing")
	writeMod(t, ""+
		"module github.com/ctx42/a\n"+
		"require github.com/customer/b v0.1.0\n",
		src, "ctx42", "a",
	)
	writeMod(t, ""+
		"module github.com/ctx42/d\n"+
		"require github.com/ctx42/testing v0.1.0\n",
		src, "ctx42", "d",
	)
	writeMod(t, ""+
		"module github.com/customer/b\n"+
		"require github.com/ctx42/testing v0.1.0\n",
		src, "customer", "b",
	)
	cnf := oskit.Create(t, ""+
		"out: all.svg\n"+
		"maps:\n"+
		"  - name: ctx42\n"+
		"    dirs: ["+filepath.Join(src, "ctx42")+"]\n"+
		"    include: ['github.com/ctx42/*']\n"+
		"  - name: work\n"+
		"    dirs: ["+filepath.Join(src, "customer")+"]\n"+
		"    include: ['github.com/customer/*']\n",
		t.TempDir(), "modmap.yaml",
	)
	return cnf, src
}

// proxyEnv returns the process environment with the go command reading
// modules from a proxy on disk serving the go.mod files, keyed by module
// path, each at version v1.0.0.
func proxyEnv(t tester.T, mods map[string]string) []string {
	t.Helper()
	dir := t.TempDir()
	for pth, content := range mods {
		ver := oskit.MkdirAll(t, dir, pth, "@v")
		oskit.Create(t, "v1.0.0\n", ver, "list")
		oskit.Create(t, content, ver, "v1.0.0.mod")
		oskit.Create(t, `{"Version":"v1.0.0"}`, ver, "v1.0.0.info")
	}
	return append(
		os.Environ(),
		"GOPROXY=file://"+filepath.ToSlash(dir),
		"GOSUMDB=off",
		"GOFLAGS=-mod=mod",
		"GOMODCACHE="+t.TempDir(),
	)
}

// wideMaps writes two maps holding 12 modules each, all on level zero, and
// the configuration declaring them. It returns the path of the
// configuration, which writes "all.svg" beside it.
func wideMaps(t tester.T) string {
	t.Helper()
	src := t.TempDir()
	for idx := range 12 {
		name := fmt.Sprintf("m%d", idx)
		writeMod(t, "module one.com/"+name+"\n", src, "one", name)
		writeMod(t, "module two.com/"+name+"\n", src, "two", name)
	}
	return oskit.Create(t, ""+
		"out: all.svg\n"+
		"maps:\n"+
		"  - name: one\n"+
		"    dirs: ["+filepath.Join(src, "one")+"]\n"+
		"  - name: two\n"+
		"    dirs: ["+filepath.Join(src, "two")+"]\n",
		t.TempDir(), "modmap.yaml",
	)
}

// group returns the markup of the module group drawing the module path in
// the SVG document, or an empty string when there is none.
func group(doc, pth string) string {
	_, grp, ok := strings.Cut(doc, `data-module="`+pth+`"`)
	if !ok {
		return ""
	}
	grp, _, _ = strings.Cut(grp, "</g>")
	return grp
}

// parsedConfig returns a configuration whose flag set parsed args, with the
// option values copied into it.
func parsedConfig(t tester.T, args ...string) *config {
	t.Helper()
	cfg := &config{}
	apply := cfg.flags()
	if err := cfg.fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	apply()
	return cfg
}
