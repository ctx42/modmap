// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package conf

import (
	"github.com/ctx42/testing/pkg/tester"
	"github.com/ctx42/testkit/pkg/oskit"
)

// writeConf writes content to a configuration file in dir and returns its
// path.
func writeConf(t tester.T, content, dir string) string {
	t.Helper()
	return oskit.Create(t, content, dir, "modmap.yaml")
}
