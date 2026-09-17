// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package mod_test

import (
	"fmt"

	"github.com/ctx42/modmap/pkg/mod"
)

func ExampleNewFilter() {
	flt := mod.NewFilter(
		[]string{"github.com/ctx42/*"},
		[]string{"github.com/ctx42/tst-*"},
	)

	fmt.Println(flt.Match("github.com/ctx42/testing"))
	fmt.Println(flt.Match("github.com/ctx42/tst-a"))
	fmt.Println(flt.Match("golang.org/x/mod"))
	// Output:
	// true
	// false
	// false
}

func ExampleModule_Deps() {
	module := &mod.Module{
		Path: "github.com/ctx42/gomake",
		Requires: []mod.Require{
			{Path: "github.com/ctx42/ring", Ver: "v0.7.1"},
			{Path: "github.com/ctx42/ring", Ver: "v0.6.0"},
			{Path: "github.com/ctx42/xdef", Ver: "v0.5.0"},
		},
	}

	fmt.Println(module.Deps())
	// Output:
	// [github.com/ctx42/ring github.com/ctx42/xdef]
}
