// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package userconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchPrivateHost(t *testing.T) {
	cases := []struct {
		pattern string
		host    string
		want    bool
	}{
		{"example.com", "example.com", true},
		{"example.org", "foo.example.org", true},
		{"example.net", "a.b.example.net", true},
		{"example.io", "bad-example.io", false},
		{"*.example.dev", "src.example.dev", true},
		{"*.example.app", "example.app", false},
		{"", "example.site", false},
		{"example.club", "", false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, matchPrivateHost(tc.pattern, tc.host), "pattern %q host %q", tc.pattern, tc.host)
	}
}

func TestLoadReturnsCopy(t *testing.T) {
	SetIgnored(true)
	t.Cleanup(func() { SetIgnored(false) })
	a := Load()
	require.NotNil(t, a)
	a.Scan.PrivateHosts = []string{"elsewhere.test"}
	a.Generate.Defaults = map[string]TargetDefault{"CONTRIBUTING.md": {Path: "docs/CONTRIBUTING.md"}}
	b := Load()
	assert.Empty(t, b.Scan.PrivateHosts, "mutation of a Load result must not leak into the cache")
	assert.Empty(t, b.Generate.Defaults, "mutation of a Load result must not leak into the cache")
}
