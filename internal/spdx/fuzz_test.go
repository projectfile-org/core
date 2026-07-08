// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package spdx_test

import (
	"strings"
	"testing"

	"kiota.ch/projectfile/core/internal/spdx"
)

// FuzzSplitCompound checks that the license-expression splitter never panics.
func FuzzSplitCompound(f *testing.F) {
	f.Add("MIT")
	f.Add("MIT OR Apache-2.0")
	f.Add("MIT AND CC-BY-4.0")
	f.Add("GPL-2.0-only OR (MIT AND BSD-2-Clause)")
	f.Add("MIT OR Apache-2.0 OR GPL-2.0-or-later")
	f.Add("GPL-2.0-only WITH Classpath-exception-2.0")
	f.Add("")
	f.Add(" OR ")
	f.Add("OR OR OR")
	f.Add(" AND ")
	f.Add("WITH WITH WITH")

	f.Fuzz(func(t *testing.T, s string) {
		result, _ := spdx.SplitCompound(s)
		// For non-empty input every element must be non-empty — SplitCompound
		// filters blanks. Empty input → single-element [""] is the documented
		// passthrough behaviour.
		if strings.TrimSpace(s) == "" {
			return
		}
		for _, part := range result {
			if part == "" {
				t.Errorf("SplitCompound(%q) returned an empty element", s)
			}
		}
	})
}

// FuzzSubstitute checks that the placeholder replacer never panics.
func FuzzSubstitute(f *testing.F) {
	f.Add("Copyright [year] [fullname]", 2024, "Alice Smith")
	f.Add("<year> <copyright holders>", 0, "")
	f.Add("[name of copyright owner]", 2023, "Corp Inc")
	f.Add("", 2024, "Bob")
	f.Add("[year][year][year]", 2024, "X")

	f.Fuzz(func(_ *testing.T, text string, year int, holder string) {
		holders := []string{}
		if holder != "" {
			holders = []string{holder}
		}
		_ = spdx.Substitute(text, spdx.Vars{Year: year, Holders: holders})
	})
}
