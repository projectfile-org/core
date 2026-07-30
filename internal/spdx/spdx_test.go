// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package spdx_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/internal/spdx"
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

// withCorpus registers a fixture corpus for the duration of one test and clears
// it afterwards. Core ships NO licence texts of its own (a consumer registers
// them — see SetEmbedded), so a test that exercises tier 1 must supply its own;
// relying on a corpus that happens to be lying around is what let an empty
// embedded set reach a release unnoticed.
func withCorpus(t *testing.T, files map[string]string) {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}
	spdx.SetEmbedded(fsys)
	t.Cleanup(func() { spdx.SetEmbedded(nil) })
}

func TestTextEmbeddedMIT(t *testing.T) {
	withCorpus(t, map[string]string{"MIT.txt": "MIT License\n\nCopyright (c) [year] [fullname]\n"})
	text, err := spdx.Text(testMIT, spdx.Options{Offline: true})
	require.NoError(t, err)
	assert.NotEmpty(t, text)
}

func TestTextEmbeddedApache(t *testing.T) {
	withCorpus(t, map[string]string{"Apache-2.0.txt": "Apache License\nVersion 2.0\n"})
	text, err := spdx.Text(testApache, spdx.Options{Offline: true})
	require.NoError(t, err)
	assert.NotEmpty(t, text)
	assert.Contains(t, text, "Apache")
}

// TestTextNoCorpusRegisteredFallsThrough pins the graceful-degradation contract:
// with no corpus registered, tier 1 is SKIPPED rather than erroring, and an
// offline lookup reports ErrOffline. This is the CLI's normal state.
func TestTextNoCorpusRegisteredFallsThrough(t *testing.T) {
	spdx.SetEmbedded(nil)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	_, err := spdx.Text(testMIT, spdx.Options{Offline: true})
	require.Error(t, err)
	assert.True(t, errors.Is(err, spdx.ErrOffline))
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

// TestAllEmbeddedIDsResolvable guards the invariant "every id a registered
// corpus advertises resolves offline" — EmbeddedIDs and Text must agree on the
// same lookup root, so a corpus laid out wrongly fails loudly here instead of
// silently degrading to a network fetch at generation time.
func TestAllEmbeddedIDsResolvable(t *testing.T) {
	withCorpus(t, map[string]string{
		"MIT.txt":        "MIT License\n",
		"Apache-2.0.txt": "Apache License\n",
		"ISC.txt":        "ISC License\n",
	})
	ids := spdx.EmbeddedIDs()
	require.Len(t, ids, 3, "EmbeddedIDs must enumerate the registered corpus")
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			text, err := spdx.Text(id, spdx.Options{Offline: true})
			require.NoError(t, err)
			assert.NotEmpty(t, text)
		})
	}
}

// ── per-binary cache slot + purge ────────────────────────────────────────────

// TestCacheDirRoutesPerApp pins that SetCacheApp selects the on-disk slot, so
// each binary (cli, bridge, ci-resolver) keeps its own SPDX cache and a purge
// in one cannot clobber another.
func TestCacheDirRoutesPerApp(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Cleanup(func() { spdx.SetCacheApp("cli") }) // restore the default

	spdx.SetCacheApp("cli")
	cliDir, err := spdx.CacheDir()
	require.NoError(t, err)
	assert.Contains(t, cliDir, filepath.Join("projectfile", "cli", "spdx"))

	spdx.SetCacheApp("bridge")
	bridgeDir, err := spdx.CacheDir()
	require.NoError(t, err)
	assert.Contains(t, bridgeDir, filepath.Join("projectfile", "bridge", "spdx"))

	assert.NotEqual(t, cliDir, bridgeDir, "cli and bridge must own separate slots")
}

// TestPurgeRemovesCachedTexts verifies Purge empties the spdx slot and reports
// the count, while a missing slot is a clean no-op (not an error).
func TestPurgeRemovesCachedTexts(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	spdx.SetCacheApp("cli")
	t.Cleanup(func() { spdx.SetCacheApp("cli") })

	dir, err := spdx.CacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "MIT.txt"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ISC.txt"), []byte("y"), 0o644))

	removed, err := spdx.Purge()
	require.NoError(t, err)
	assert.Equal(t, 2, removed)

	// Second purge on the now-empty slot is a no-op success.
	removed, err = spdx.Purge()
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
}
