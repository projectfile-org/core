// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fieldpath_test

import (
	"testing"

	"kiota.ch/projectfile/core/internal/fieldpath"
)

// FuzzParse checks that the parser never panics on arbitrary input.
// Seeds are drawn from the full grammar: dotted keys, index, selector,
// projection, map-project, and combinations thereof.
func FuzzParse(f *testing.F) {
	f.Add("identity.name")
	f.Add("authors[0].email")
	f.Add("authors[-1].name")
	f.Add("authors[email=alice@example.com].name")
	f.Add("authors[email=a,roles=author].name")
	f.Add("keywords[]")
	f.Add("extensions{}")
	f.Add("extensions{}.keys")
	f.Add("extensions{}.values")
	f.Add("")
	f.Add("a[b=c,d=e].f[0]")
	f.Add(".")
	f.Add("[")
	f.Add("{}")
	f.Add("key[")
	f.Add("key[=]")
	f.Add("key[0][1]")

	f.Fuzz(func(_ *testing.T, s string) {
		// Must not panic; errors are fine.
		p, err := fieldpath.Parse(s)
		if err == nil {
			// If parse succeeds, String() must also not panic.
			_ = p.String()
		}
	})
}
