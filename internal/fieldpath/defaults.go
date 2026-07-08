// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fieldpath

import (
	"time"

	"kiota.ch/projectfile/core/internal/projectfile"
)

const defaultKind = "library"

// Default is one entry in the spec-defaults registry. Value is a function
// so per-document conditional values (e.g. the current year for
// copyright.year) compute lazily; When optionally gates the entry on
// document state — applied only when no static value would be correct on
// its own.
type Default struct {
	Path  string
	Value func(*projectfile.Document) any
	When  func(*projectfile.Document) bool
}

// defaults is the curated table of spec-defined fallbacks. Extension
// namespaces deliberately don't get entries — extensions supply their own
// defaults at call site via --default, keeping pf-cli core unaware of
// every reverse-DNS namespace's conventions.
var defaults = []Default{
	{
		// spec §5.4a: a missing copyright.year falls back to the current year
		// so generated LICENSE / NOTICE lines never carry a 0.
		Path: "copyright.year",
		Value: func(*projectfile.Document) any {
			return time.Now().Year()
		},
	},
	{
		// spec §5.1: a missing kind defaults to "library" per spec defaults.
		Path: "kind",
		Value: func(*projectfile.Document) any {
			return defaultKind
		},
	},
}

// LookupDefault returns the spec default for an address against a given
// document, plus ok=true when one applies. The (doc, path) signature lets
// conditional When-functions inspect the document for per-document defaults.
func LookupDefault(doc *projectfile.Document, path string) (any, bool) {
	for _, d := range defaults {
		if d.Path != path {
			continue
		}
		if d.When != nil && !d.When(doc) {
			continue
		}
		return d.Value(doc), true
	}
	return nil, false
}
