// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cache

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
	"kiota.ch/projectfile/core/v2/internal/spdx"
)

// isolateSlot points XDG_CACHE_HOME at a fresh temp dir so cache tests never
// touch the developer's real slot.
func isolateSlot(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
}

// seedInclude writes one include body plus an optional sidecar into the
// isolated slot. Empty url skips the sidecar (legacy entry).
func seedInclude(t *testing.T, file, url string, fetchedAt, expiresAt time.Time) {
	t.Helper()
	dir, err := projectfile.IncludesCacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte("keywords:\n  - x\n"), 0o644))
	if url == "" {
		return
	}
	meta := fmt.Sprintf(`{"url":%q,"fetched_at":%q,"expires_at":%q}`,
		url, fetchedAt.Format(time.RFC3339), expiresAt.Format(time.RFC3339))
	require.NoError(t, os.WriteFile(filepath.Join(dir, file+".meta.json"), []byte(meta), 0o644))
}

func TestRootSharedSlot(t *testing.T) {
	isolateSlot(t)
	root, err := Root()
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(root, string(filepath.Separator)+"pf"), "root must be the shared pf slot, got %s", root)
}

func TestStatusEmpty(t *testing.T) {
	isolateSlot(t)
	st, err := Status()
	require.NoError(t, err)
	assert.Equal(t, 0, st.Includes.Total)
	assert.Equal(t, 0, st.SPDX.Total)
	assert.Empty(t, st.Includes.Entries)
	assert.NotEmpty(t, st.Root)
}

func TestStatusIncludesEntries(t *testing.T) {
	isolateSlot(t)
	now := time.Now()
	seedInclude(t, "aaa.yaml", "https://x.example/fresh.yaml", now.Add(-time.Minute), now.Add(time.Hour))
	seedInclude(t, "bbb.yaml", "https://x.example/stale.yaml", now.Add(-2*time.Hour), now.Add(-time.Hour))
	seedInclude(t, "ccc.yaml", "", time.Time{}, time.Time{})

	st, err := Status()
	require.NoError(t, err)
	require.Len(t, st.Includes.Entries, 3, "sidecars and temp files must never inflate the count")
	assert.Equal(t, 3, st.Includes.Total)
	assert.Equal(t, 1, st.Includes.Fresh)
	assert.Equal(t, 2, st.Includes.Stale)
	assert.Equal(t, "ccc.yaml", st.Includes.Entries[0].Name, "legacy entry falls back to filename")
	assert.Equal(t, "https://x.example/fresh.yaml", st.Includes.Entries[1].Name)
	assert.True(t, st.Includes.Entries[1].Fresh)
	assert.Equal(t, "https://x.example/stale.yaml", st.Includes.Entries[2].Name)
	assert.False(t, st.Includes.Entries[2].Fresh)
	assert.True(t, st.Includes.OldestAge >= 2*time.Hour)
}

func TestStatusSPDXEntries(t *testing.T) {
	isolateSlot(t)
	dir, err := spdx.CacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "MIT.txt"), []byte("mit"), 0o644))

	st, err := Status()
	require.NoError(t, err)
	require.Len(t, st.SPDX.Entries, 1)
	assert.Equal(t, "MIT", st.SPDX.Entries[0].Name)
	assert.True(t, st.SPDX.Entries[0].Fresh, "immutable texts never go stale")
}

func TestPurgeAll(t *testing.T) {
	isolateSlot(t)
	now := time.Now()
	seedInclude(t, "aaa.yaml", "https://x.example/a.yaml", now, now.Add(time.Hour))
	dir, err := spdx.CacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "MIT.txt"), []byte("mit"), 0o644))

	s, inc, root, err := PurgeAll()
	require.NoError(t, err)
	assert.Equal(t, 1, s)
	assert.Equal(t, 1, inc, "sidecars must not inflate the purged count")
	assert.NotEmpty(t, root)
	st, err := Status()
	require.NoError(t, err)
	assert.Equal(t, 0, st.Includes.Total)
	assert.Equal(t, 0, st.SPDX.Total)
}

func TestWarmIncludesDirMissing(t *testing.T) {
	isolateSlot(t)
	warmed, total, err := WarmIncludesDir(t.TempDir(), false)
	require.NoError(t, err, "a directory with no projectfile is a benign no-op")
	assert.Equal(t, 0, warmed)
	assert.Equal(t, 0, total)
}

func TestWarmIncludesDir(t *testing.T) {
	isolateSlot(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("keywords:\n  - remote\n"))
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile.yaml"),
		[]byte("includes:\n  - "+srv.URL+"/remote.yaml\n"), 0o644))

	warmed, total, err := WarmIncludesDir(dir, false)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, 1, warmed)
	entries, err := projectfile.IncludeCacheEntries()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, srv.URL+"/remote.yaml", entries[0].URL)
}

func TestWarmIncludesDirUnreachable(t *testing.T) {
	isolateSlot(t)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	dead := srv.URL + "/gone.yaml"
	srv.Close()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile.yaml"),
		[]byte("includes:\n  - "+dead+"\n"), 0o644))

	warmed, total, err := WarmIncludesDir(dir, false)
	require.NoError(t, err, "one dead remote must not abort the run")
	assert.Equal(t, 1, total, "unreachable URL is still reported")
	assert.Equal(t, 0, warmed)
}
