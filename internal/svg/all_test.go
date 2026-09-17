// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package svg

import (
	"encoding/json"
	"os"
	"regexp"

	"github.com/ctx42/testing/pkg/tester"

	"github.com/ctx42/modmap/pkg/graph"
	"github.com/ctx42/modmap/pkg/mod"
)

// fontURI matches the embedded font payload in a rendered map.
var fontURI = regexp.MustCompile(`data:font/ttf;base64,[A-Za-z0-9+/=]+`)

// newGraph returns the graph built from a map of a module path to the module
// paths it directly requires.
func newGraph(deps map[string][]string) *graph.Graph {
	mods := make(map[string]*mod.Module, len(deps))
	for pth, reqs := range deps {
		module := &mod.Module{Path: pth}
		for _, req := range reqs {
			hav := mod.Require{Path: req, Ver: "v1.0.0"}
			module.Requires = append(module.Requires, hav)
		}
		mods[pth] = module
	}
	grp, err := graph.New(mods)
	if err != nil {
		panic(err)
	}
	return grp
}

// loadDeps reads the module dependencies fixture at pth.
func loadDeps(t tester.T, pth string) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(pth)
	if err != nil {
		t.Fatal(err)
	}
	deps := make(map[string][]string)
	if err = json.Unmarshal(data, &deps); err != nil {
		t.Fatal(err)
	}
	return deps
}

// trimFont replaces the embedded font payload with a placeholder, so a golden
// file stays readable.
func trimFont(doc []byte) []byte {
	return fontURI.ReplaceAll(doc, []byte("data:font/ttf;base64,FONT"))
}
