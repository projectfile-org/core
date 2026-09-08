// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCache_FreshHitAvoidsNetwork verifies a fresh cached entry is served
// without a second network round-trip.
func TestCache_FreshHitAvoidsNetwork(t *testing.T) {
	isolateCache(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Cache-Control", "max-age=3600")
		_, _ = w.Write([]byte("identity:\n  name: fresh\n"))
	}))
	t.Cleanup(srv.Close)
	url := srv.URL + "/include.yaml"
	data, _, err := fetchHTTPInclude(url, ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, []byte("identity:\n  name: fresh\n"), data)
	assert.Equal(t, 1, calls)
	// Second fetch should hit fresh cache, not network.
	data2, _, err := fetchHTTPInclude(url, ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, []byte("identity:\n  name: fresh\n"), data2)
	assert.Equal(t, 1, calls, "fresh cache must not trigger network")
}

// TestCache_StaleRevalidates304 verifies stale entry sends If-None-Match and
// handles 304 Not Modified by revalidating cache.
func TestCache_StaleRevalidates304(t *testing.T) {
	isolateCache(t)
	etag := `"abc123"`
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("If-None-Match") == etag {
			w.Header().Set("Cache-Control", "max-age=3600")
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "max-age=0")
		_, _ = w.Write([]byte("identity:\n  name: v1\n"))
	}))
	t.Cleanup(srv.Close)
	url := srv.URL + "/include.yaml"
	data, _, err := fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Nanosecond})
	require.NoError(t, err)
	assert.Equal(t, []byte("identity:\n  name: v1\n"), data)
	assert.Equal(t, 1, calls)
	// the 1ns TTL floor expires the entry immediately, forcing revalidation
	data2, _, err := fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Nanosecond})
	require.NoError(t, err)
	assert.Equal(t, []byte("identity:\n  name: v1\n"), data2, "304 must return cached body")
	assert.Equal(t, 2, calls, "stale must revalidate")
	// the 304 carried max-age=3600, which outranks the floor and makes the entry fresh
	data3, _, err := fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Nanosecond})
	require.NoError(t, err)
	assert.Equal(t, []byte("identity:\n  name: v1\n"), data3)
	assert.Equal(t, 2, calls, "304 carrying a longer max-age must leave the entry fresh")
}

// TestCache_StaleFetchesNewContent verifies stale with changed content fetches new body.
func TestCache_StaleFetchesNewContent(t *testing.T) {
	isolateCache(t)
	etag1 := `"v1"`
	etag2 := `"v2"`
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("ETag", etag1)
			w.Header().Set("Cache-Control", "max-age=0")
			_, _ = w.Write([]byte("identity:\n  name: v1\n"))
			return
		}
		if r.Header.Get("If-None-Match") == etag1 {
			w.Header().Set("ETag", etag2)
			w.Header().Set("Cache-Control", "max-age=3600")
			_, _ = w.Write([]byte("identity:\n  name: v2\n"))
			return
		}
		w.Header().Set("ETag", etag2)
		_, _ = w.Write([]byte("identity:\n  name: v2\n"))
	}))
	t.Cleanup(srv.Close)
	url := srv.URL + "/include.yaml"
	data, _, err := fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Nanosecond})
	require.NoError(t, err)
	assert.Contains(t, string(data), "v1")
	data2, _, err := fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Nanosecond})
	require.NoError(t, err)
	assert.Contains(t, string(data2), "v2", "stale should fetch new content when ETag mismatches")
}

// TestCache_TTL_EnvOverride verifies $PF_CACHE_TTL overrides default.
func TestCache_TTL_EnvOverride(t *testing.T) {
	isolateCache(t)
	t.Setenv("PF_CACHE_TTL", "50ms")
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte("identity:\n  name: test\n"))
	}))
	t.Cleanup(srv.Close)
	url := srv.URL + "/include.yaml"
	_, _, err := fetchHTTPInclude(url, ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	// Fresh within 50ms.
	_, _, err = fetchHTTPInclude(url, ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "within TTL should be fresh")
	time.Sleep(60 * time.Millisecond)
	_, _, err = fetchHTTPInclude(url, ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "after TTL should revalidate")
}

// TestCache_ShortOriginTTLFloored verifies an origin max-age shorter than the
// fallback TTL is floored, not honored literally — Forgejo's raw endpoint
// sends "Cache-Control: private, max-age=300" on every response, and a fresh
// fetch must still skip the network well past that window.
func TestCache_ShortOriginTTLFloored(t *testing.T) {
	isolateCache(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Cache-Control", "private, max-age=300")
		_, _ = w.Write([]byte("identity:\n  name: floored\n"))
	}))
	t.Cleanup(srv.Close)
	url := srv.URL + "/include.yaml"
	_, _, err := fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Hour})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	_, _, err = fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Hour})
	require.NoError(t, err)
	assert.Equal(t, 1, calls, "fallback TTL must floor a shorter origin max-age")
}

// TestCache_FreshnessDirectivesFloored verifies an origin cannot push the TTL below the floor, whatever it sends.
func TestCache_FreshnessDirectivesFloored(t *testing.T) {
	// the first case is the literal header Forgejo's raw endpoint returns
	for _, cc := range []string{"max-age=0, private, must-revalidate", "no-cache", "max-age=1", "private"} {
		t.Run(cc, func(t *testing.T) {
			isolateCache(t)
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.Header().Set("Cache-Control", cc)
				_, _ = w.Write([]byte("identity:\n  name: floored\n"))
			}))
			t.Cleanup(srv.Close)
			url := srv.URL + "/include.yaml"
			_, _, err := fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Hour})
			require.NoError(t, err)
			assert.Equal(t, 1, calls)
			_, _, err = fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Hour})
			require.NoError(t, err)
			assert.Equal(t, 1, calls, "origin must not lower the TTL below the floor")
		})
	}
}

// TestCache_NoStoreBypassesFloor verifies no-store is the one directive that still expires on arrival.
func TestCache_NoStoreBypassesFloor(t *testing.T) {
	isolateCache(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("identity:\n  name: nostore\n"))
	}))
	t.Cleanup(srv.Close)
	url := srv.URL + "/include.yaml"
	_, _, err := fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Hour})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	_, _, err = fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Hour})
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "no-store must keep forcing revalidation")
}

// TestCache_ServesStaleOn5xx verifies a 5xx with stale cache returns stale content.
func TestCache_ServesStaleOn5xx(t *testing.T) {
	isolateCache(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Cache-Control", "max-age=0")
			_, _ = w.Write([]byte("identity:\n  name: ok\n"))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	url := srv.URL + "/include.yaml"
	data, _, err := fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Nanosecond})
	require.NoError(t, err)
	assert.Contains(t, string(data), "ok")
	data2, _, err := fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Nanosecond})
	require.NoError(t, err, "5xx with stale cache should serve stale, not error")
	assert.Contains(t, string(data2), "ok")
}

// TestCache_OfflineServesStale verifies offline mode serves stale cache.
func TestCache_OfflineServesStale(t *testing.T) {
	isolateCache(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "max-age=0")
		_, _ = w.Write([]byte("identity:\n  name: offline\n"))
	}))
	t.Cleanup(srv.Close)
	url := srv.URL + "/include.yaml"
	_, _, err := fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Nanosecond})
	require.NoError(t, err)
	// Offline should serve even though stale.
	data, _, err := fetchHTTPInclude(url, ReadOptions{Offline: true})
	require.NoError(t, err)
	assert.Contains(t, string(data), "offline")
}

// TestCache_PurgeRemovesMeta verifies PurgeIncludes removes content and meta.
func TestCache_PurgeRemovesMeta(t *testing.T) {
	cacheDir := isolateCache(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", `"x"`)
		_, _ = w.Write([]byte("identity:\n  name: purge\n"))
	}))
	t.Cleanup(srv.Close)
	url := srv.URL + "/include.yaml"
	_, _, err := fetchHTTPInclude(url, ReadOptions{})
	require.NoError(t, err)
	entries, err := os.ReadDir(filepath.Join(cacheDir, "pf", "includes"))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(entries), 2, "should have content and meta")
	removed, err := PurgeIncludes()
	require.NoError(t, err)
	assert.Equal(t, 1, removed, "purge counts content files only")
	entries, err = os.ReadDir(filepath.Join(cacheDir, "pf", "includes"))
	if err == nil {
		for _, e := range entries {
			assert.NotContains(t, e.Name(), ".meta.json", "meta should be removed")
		}
	}
}

// TestCache_IfModifiedSince verifies Last-Modified conditional request.
func TestCache_IfModifiedSince(t *testing.T) {
	isolateCache(t)
	lm := "Wed, 21 Oct 2015 07:28:00 GMT"
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("If-Modified-Since") == lm {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Last-Modified", lm)
		w.Header().Set("Cache-Control", "max-age=0")
		_, _ = w.Write([]byte("identity:\n  name: lm\n"))
	}))
	t.Cleanup(srv.Close)
	url := srv.URL + "/include.yaml"
	_, _, err := fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Nanosecond})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	_, _, err = fetchHTTPInclude(url, ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
}

// TestCache_ForceRefresh verifies ForceRefresh bypasses freshness.
func TestCache_ForceRefresh(t *testing.T) {
	isolateCache(t)
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("If-None-Match") != "" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Cache-Control", "max-age=3600")
		_, _ = w.Write([]byte("identity:\n  name: force\n"))
	}))
	t.Cleanup(srv.Close)
	url := srv.URL + "/include.yaml"
	_, _, err := fetchHTTPInclude(url, ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	// Fresh without force should not hit network.
	_, _, err = fetchHTTPInclude(url, ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	// With ForceRefresh should revalidate (304).
	_, _, err = fetchHTTPInclude(url, ReadOptions{ForceRefresh: true})
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
}

// TestCache_StatusSummary verifies the on-disk includes cache is reported
// with fresh/stale counts and the oldest stale age. A fresh entry should
// count as fresh; a missing sidecar (legacy entry) should count as stale.
func TestCache_StatusSummary(t *testing.T) {
	isolateCache(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "max-age=3600")
		_, _ = w.Write([]byte("identity:\n  name: sum\n"))
	}))
	t.Cleanup(srv.Close)
	url := srv.URL + "/include.yaml"
	_, _, err := fetchHTTPInclude(url, ReadOptions{})
	require.NoError(t, err)

	st, err := IncludesCacheStatusSummary()
	require.NoError(t, err)
	assert.Equal(t, 1, st.Total)
	assert.Equal(t, 1, st.Fresh)
	assert.Equal(t, 0, st.Stale)

	// Drop sidecar to simulate legacy cache entry; should now read as stale.
	dir, err := IncludesCacheDir()
	require.NoError(t, err)
	matches, err := filepath.Glob(filepath.Join(dir, "*.meta.json"))
	require.NoError(t, err)
	require.NotEmpty(t, matches)
	require.NoError(t, os.Remove(matches[0]))

	st, err = IncludesCacheStatusSummary()
	require.NoError(t, err)
	assert.Equal(t, 1, st.Total)
	assert.Equal(t, 0, st.Fresh)
	assert.Equal(t, 1, st.Stale)
	assert.Greater(t, st.OldestAge, time.Duration(0))
}
