// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package spdx_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/internal/spdx"
)

const (
	testMIT              = "MIT"
	testApache           = "Apache-2.0"
	testGPLWithClasspath = "GPL-2.0-only WITH Classpath-exception-2.0"
)

// ── IsCompound ───────────────────────────────────────────────────────────────

func TestIsCompound(t *testing.T) {
	cases := []struct {
		id       string
		compound bool
	}{
		{testMIT, false},
		{testApache, false},
		{"GPL-2.0-or-later", false},
		{testMIT + " OR " + testApache, true},
		{"GPL-2.0-only AND Classpath-exception-2.0", true},
		{testGPLWithClasspath, true},
		{testMIT + " OR " + testApache + " OR GPL-2.0-or-later", true},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			assert.Equal(t, tc.compound, spdx.IsCompound(tc.id))
		})
	}
}

// ── SplitCompound ────────────────────────────────────────────────────────────

func TestSplitCompound(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantIDs  []string
		wantConj string
	}{
		{"single", testMIT, []string{testMIT}, ""},
		{"or-two-terms", testMIT + " OR " + testApache, []string{testMIT, testApache}, "OR"},
		{"or-three-terms", testMIT + " OR " + testApache + " OR GPL-2.0-or-later", []string{testMIT, testApache, "GPL-2.0-or-later"}, "OR"},
		{"and-two-terms", testMIT + " AND CC-BY-4.0", []string{testMIT, "CC-BY-4.0"}, "AND"},
		{"and-three-terms", testMIT + " AND " + testApache + " AND ISC", []string{testMIT, testApache, "ISC"}, "AND"},
		// WITH is NOT a split point — it stays attached to its license.
		{"with-is-opaque", testGPLWithClasspath, []string{testGPLWithClasspath}, ""},
		// Mixed OR+AND: OR (looser operator) is split first; AND stays inside the term.
		{"or-takes-precedence-over-and", testMIT + " OR " + testApache + " AND GPL-3.0", []string{testMIT, testApache + " AND GPL-3.0"}, "OR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotIDs, gotConj := spdx.SplitCompound(tc.in)
			assert.Equal(t, tc.wantIDs, gotIDs)
			assert.Equal(t, tc.wantConj, gotConj)
		})
	}
}

// ── StripException ───────────────────────────────────────────────────────────

func TestStripException(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain-id", testMIT, testMIT},
		{"with-exception", testGPLWithClasspath, "GPL-2.0-only"},
		{"case-insensitive-with", "GPL-2.0-only with Classpath-exception-2.0", "GPL-2.0-only"},
		{"no-exception", testApache, testApache},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, spdx.StripException(tc.in))
		})
	}
}

// ── Substitute ───────────────────────────────────────────────────────────────

func TestSubstituteBracketYear(t *testing.T) {
	result := spdx.Substitute("Copyright [year] [fullname]", spdx.Vars{Year: 2024, Holders: []string{"Alice"}})
	assert.Contains(t, result, "2024")
	assert.Contains(t, result, "Alice")
}

func TestSubstituteAngleBrackets(t *testing.T) {
	result := spdx.Substitute("Copyright <year> <copyright holders>", spdx.Vars{Year: 2025, Holders: []string{"Bob"}})
	assert.Contains(t, result, "2025")
	assert.Contains(t, result, "Bob")
}

func TestSubstituteMultipleHoldersJoined(t *testing.T) {
	result := spdx.Substitute("[fullname]", spdx.Vars{Year: 2024, Holders: []string{"Alice", "Bob"}})
	assert.Contains(t, result, "Alice")
	assert.Contains(t, result, "Bob")
}

func TestSubstituteNameOfCopyrightOwner(t *testing.T) {
	result := spdx.Substitute("[name of copyright owner]", spdx.Vars{Year: 2024, Holders: []string{"Corp Inc"}})
	assert.Contains(t, result, "Corp Inc")
}

func TestSubstituteNoYearWhenZero(t *testing.T) {
	// Year=0 means "don't substitute year placeholders" — keeps them as-is.
	result := spdx.Substitute("[year]", spdx.Vars{Year: 0, Holders: nil})
	assert.Contains(t, result, "[year]", "zero year must leave the placeholder untouched")
}

func TestSubstituteEmptyHolders(t *testing.T) {
	// Empty holders means placeholder is NOT replaced (to avoids "Copyright 2024 " trailing space).
	result := spdx.Substitute("[fullname]", spdx.Vars{Year: 2024, Holders: nil})
	assert.Contains(t, result, "[fullname]", "empty holders list must not replace placeholder")
}

// ── Text (offline) ───────────────────────────────────────────────────────────

func TestTextEmbeddedMIT(t *testing.T) {
	text, err := spdx.Text(testMIT, spdx.Options{Offline: true})
	require.NoError(t, err)
	assert.NotEmpty(t, text)
}

func TestTextEmbeddedApache(t *testing.T) {
	text, err := spdx.Text(testApache, spdx.Options{Offline: true})
	require.NoError(t, err)
	assert.NotEmpty(t, text)
	assert.Contains(t, text, "Apache")
}

func TestTextUnknownReturnsErrOffline(t *testing.T) {
	_, err := spdx.Text("NonExistentLicense-99.9", spdx.Options{Offline: true})
	require.Error(t, err)
	assert.True(t, errors.Is(err, spdx.ErrOffline))
}

func TestTextCompoundReturnsErrCompound(t *testing.T) {
	_, err := spdx.Text(testMIT+" OR "+testApache, spdx.Options{Offline: true})
	require.Error(t, err)
	assert.True(t, errors.Is(err, spdx.ErrCompound))
}

func TestTextEmptyIDReturnsError(t *testing.T) {
	_, err := spdx.Text("", spdx.Options{Offline: true})
	assert.Error(t, err)
}

// TestAllEmbeddedIDsResolvable guards the invariant "every shipped SPDX id
// resolves offline". The embedded set ships empty in some builds (license
// texts are fetched via `make fetch-spdx` and committed separately), so an
// empty id list is a successful no-op rather than a failure — when IDs ARE
// present, each must resolve without network.
func TestAllEmbeddedIDsResolvable(t *testing.T) {
	ids := spdx.EmbeddedIDs()
	if len(ids) == 0 {
		t.Skip("embedded SPDX set is empty; run `make fetch-spdx` to populate")
	}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			text, err := spdx.Text(id, spdx.Options{Offline: true})
			require.NoError(t, err)
			assert.NotEmpty(t, text)
		})
	}
}
