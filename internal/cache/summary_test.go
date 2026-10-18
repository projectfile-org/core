// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
	"kiota.ch/projectfile/core/v2/internal/spdx"
)

// TestSPDXEntriesAreAlwaysFresh pins the SPDX area's own rule: a licence text is immutable.
func TestSPDXEntriesAreAlwaysFresh(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)

	dir, err := spdx.CacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o755))

	old := time.Now().Add(-72 * time.Hour)
	for _, id := range []string{"MIT", "Apache-2.0"} {
		p := filepath.Join(dir, id+".txt")
		require.NoError(t, os.WriteFile(p, []byte("body"), 0o644))
		require.NoError(t, os.Chtimes(p, old, old))
	}

	got, err := Status()
	require.NoError(t, err)

	require.Len(t, got.SPDX.Entries, 2)
	assert.Equal(t, 2, got.SPDX.Total)
	assert.Equal(t, 2, got.SPDX.Fresh)
	assert.Equal(t, 0, got.SPDX.Stale, "a licence text is immutable, so it never goes stale")

	for _, e := range got.SPDX.Entries {
		assert.Equal(t, AreaSPDX, e.Area)
		assert.True(t, e.Fresh)
		assert.InDelta(t, 72*time.Hour, e.Age, float64(time.Hour), "the age comes from the mtime")
		assert.False(t, e.FetchedAt.IsZero())
	}
}

// TestSPDXEntryAgeUnknownWhenTheBodyIsUnreadable pins the fallback for an unstattable body.
func TestSPDXEntryAgeUnknownWhenTheBodyIsUnreadable(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	dir, err := spdx.CacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	// A dangling link is listed by the scan but cannot be stat'd.
	require.NoError(t, os.Symlink(filepath.Join(dir, "gone"), filepath.Join(dir, "MIT.txt")))

	got, err := Status()
	require.NoError(t, err)
	require.Len(t, got.SPDX.Entries, 1)
	assert.Equal(t, "MIT", got.SPDX.Entries[0].Name)
	assert.True(t, got.SPDX.Entries[0].FetchedAt.IsZero(),
		"an unreadable entry reports no fetch date rather than inventing one")
	assert.True(t, got.SPDX.Entries[0].Fresh, "an immutable text is fresh whatever its age")
}

// TestIncludesOldestAgeTracksTheStalest pins the aggregation a purge reads.
func TestIncludesOldestAgeTracksTheStalest(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)

	dir, err := projectfile.IncludesCacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o755))

	for i, age := range []time.Duration{time.Hour, 48 * time.Hour, 6 * time.Hour} {
		p := filepath.Join(dir, "url"+string(rune('a'+i)))
		require.NoError(t, os.WriteFile(p, []byte("a: 1\n"), 0o644))
		when := time.Now().Add(-age)
		require.NoError(t, os.Chtimes(p, when, when))
	}

	got, err := Status()
	require.NoError(t, err)
	assert.Positive(t, got.Includes.Total)
	assert.GreaterOrEqual(t, got.Includes.OldestAge, 47*time.Hour,
		"the oldest stale entry sets the aggregate, not the most recent one")
}

// TestStatusWithAnEmptySlotIsACleanZero pins that a first run reads as zeroes.
func TestStatusWithAnEmptySlotIsACleanZero(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	got, err := Status()
	require.NoError(t, err)
	assert.Zero(t, got.SPDX.Total)
	assert.Zero(t, got.SPDX.Fresh)
	assert.Zero(t, got.Includes.Total)
	assert.Zero(t, got.SPDXEmbedded)
	assert.NotEmpty(t, got.Root)
}

// TestRootAndStatusFailTogetherOnAnUnresolvableSlot pins one failure mode, both surfaces.
func TestRootAndStatusFailTogetherOnAnUnresolvableSlot(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")

	_, err := Root()
	require.Error(t, err)

	_, err = Status()
	require.Error(t, err, "status must not silently report an empty slot it cannot find")
}

// TestRootNamesTheSharedSlot pins the one path every pf-* binary reads.
func TestRootNamesTheSharedSlot(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)

	got, err := Root()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(base, "pf"), got)
}

// TestPurgeAllEmptiesTheSlot pins that a purge leaves the slot in place for a warm.
func TestPurgeAllEmptiesTheSlot(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", base)

	dir, err := projectfile.IncludesCacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "url-a"), []byte("a: 1\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "url-b"), []byte("b: 1\n"), 0o644))

	spdxRemoved, includesRemoved, root, err := PurgeAll()
	require.NoError(t, err)
	assert.Equal(t, 0, spdxRemoved)
	assert.Equal(t, 2, includesRemoved)
	assert.NotEmpty(t, root)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.DirExists(t, dir, "the slot itself survives a purge")
}
