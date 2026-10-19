// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Response-header names and network failure spellings the cache tests share.
const (
	testErrConnRefused     = "connection refused"
	testErrNetUnreachable  = "network is unreachable"
	testErrNoSuchHost      = "no such host"
	testErrTimeout         = "timeout"
	testHeaderCacheControl = "Cache-Control"
	testHeaderExpires      = "Expires"
)

// TestFormatStaleAgeEdges pins every branch at its boundary.
func TestFormatStaleAgeEdges(t *testing.T) {
	cases := []struct {
		name string
		in   time.Duration
		want string
	}{
		{"unknown is negative", -time.Nanosecond, "unknown age"},
		{"exactly zero", 0, "just now"},
		{"just under a minute", 59 * time.Second, "just now"},
		{"exactly a minute", time.Minute, "1m ago"},
		{"just under an hour", 59*time.Minute + 59*time.Second, "59m ago"},
		{"exactly an hour", time.Hour, "1h ago"},
		{"just under a day", 23*time.Hour + 59*time.Minute, "23h ago"},
		{"exactly a day", 24 * time.Hour, "1d ago"},
		{"many days", 72*time.Hour + time.Hour, "3d ago"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, formatStaleAge(tc.in))
		})
	}
}

// TestComputeExpiresAtOptOutAndFloor pins that an origin may bend the TTL but not break it.
func TestComputeExpiresAtOptOutAndFloor(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	const fallback = time.Hour

	t.Run("no-store opts out", func(t *testing.T) {
		h := http.Header{testHeaderCacheControl: {"private, no-store"}}
		assert.Equal(t, now, computeExpiresAt(now, h, fallback))
	})

	t.Run("negative fallback never expires", func(t *testing.T) {
		h := http.Header{testHeaderCacheControl: {"max-age=60"}}
		assert.True(t, computeExpiresAt(now, h, -1).IsZero())
	})

	t.Run("zero fallback becomes the default", func(t *testing.T) {
		assert.Equal(t, now.Add(defaultIncludeTTL), computeExpiresAt(now, http.Header{}, 0))
	})

	t.Run("no directives uses the fallback", func(t *testing.T) {
		assert.Equal(t, now.Add(fallback), computeExpiresAt(now, http.Header{}, fallback))
	})

	t.Run("a short max-age is raised to the floor", func(t *testing.T) {
		h := http.Header{testHeaderCacheControl: {"private, max-age=300"}}
		assert.Equal(t, now.Add(fallback), computeExpiresAt(now, h, fallback),
			"Forgejo sends max-age=300 on every raw response")
	})

	t.Run("a long max-age wins", func(t *testing.T) {
		h := http.Header{testHeaderCacheControl: {"max-age=7200"}}
		assert.Equal(t, now.Add(2*time.Hour), computeExpiresAt(now, h, fallback))
	})

	t.Run("an explicit zero still forces revalidation", func(t *testing.T) {
		h := http.Header{testHeaderCacheControl: {"max-age=0"}}
		assert.Equal(t, now.Add(fallback), computeExpiresAt(now, h, fallback))
	})

	t.Run("no-cache is a hint, not an opt-out", func(t *testing.T) {
		h := http.Header{testHeaderCacheControl: {"no-cache"}}
		assert.Equal(t, now.Add(fallback), computeExpiresAt(now, h, fallback))
	})

	t.Run("an unparseable max-age falls through", func(t *testing.T) {
		h := http.Header{testHeaderCacheControl: {"max-age=soon"}}
		assert.Equal(t, now.Add(fallback), computeExpiresAt(now, h, fallback))
	})

	t.Run("a negative max-age falls through", func(t *testing.T) {
		h := http.Header{testHeaderCacheControl: {"max-age=-5"}}
		assert.Equal(t, now.Add(fallback), computeExpiresAt(now, h, fallback))
	})

	t.Run("Expires inside the floor is raised", func(t *testing.T) {
		h := http.Header{testHeaderExpires: {now.Add(60 * time.Second).Format(http.TimeFormat)}}
		assert.Equal(t, now.Add(fallback), computeExpiresAt(now, h, fallback))
	})

	t.Run("Expires beyond the floor wins", func(t *testing.T) {
		h := http.Header{testHeaderExpires: {now.Add(4 * time.Hour).Format(http.TimeFormat)}}
		assert.Equal(t, now.Add(4*time.Hour), computeExpiresAt(now, h, fallback))
	})

	t.Run("an Expires in the past is ignored", func(t *testing.T) {
		h := http.Header{testHeaderExpires: {now.Add(-time.Hour).Format(http.TimeFormat)}}
		assert.Equal(t, now.Add(fallback), computeExpiresAt(now, h, fallback))
	})

	t.Run("an unparseable Expires is ignored", func(t *testing.T) {
		h := http.Header{testHeaderExpires: {"not a date"}}
		assert.Equal(t, now.Add(fallback), computeExpiresAt(now, h, fallback))
	})
}

// TestEffectiveIncludeTTLPrecedence pins where the TTL comes from, in order.
func TestEffectiveIncludeTTLPrecedence(t *testing.T) {
	assert.Equal(t, time.Duration(-1), effectiveIncludeTTL(ReadOptions{CacheTTL: -1}),
		"negative is the never-expire escape hatch")

	t.Setenv("PF_CACHE_TTL", "30m")
	t.Setenv("PF_INCLUDE_CACHE_TTL", "10m")
	assert.Equal(t, 30*time.Minute, effectiveIncludeTTL(ReadOptions{}))
	assert.Equal(t, 2*time.Hour, effectiveIncludeTTL(ReadOptions{CacheTTL: 2 * time.Hour}),
		"an explicit option outranks both env vars")

	t.Setenv("PF_CACHE_TTL", "nonsense")
	assert.Equal(t, 10*time.Minute, effectiveIncludeTTL(ReadOptions{}),
		"an unparseable value falls through to the next source")

	t.Setenv("PF_INCLUDE_CACHE_TTL", "")
	assert.Equal(t, defaultIncludeTTL, effectiveIncludeTTL(ReadOptions{}))
}

// TestIsHTMLResponse pins the auth-wall gate.
func TestIsHTMLResponse(t *testing.T) {
	cases := []struct {
		ct   string
		want bool
	}{
		{"text/html", true},
		{"TEXT/HTML", true},
		{"text/html; charset=utf-8", true},
		{" text/html ", true},
		{"application/json", false},
		{"text/plain", false},
		{"", false},
		{"text/html-ish", false},
	}
	for _, tc := range cases {
		t.Run(tc.ct, func(t *testing.T) {
			resp := &http.Response{Header: http.Header{}}
			if tc.ct != "" {
				resp.Header.Set("Content-Type", tc.ct)
			}
			assert.Equal(t, tc.want, isHTMLResponse(resp))
		})
	}
}

// TestLooksLikeHTML pins that the marker must be first after leading whitespace.
func TestLooksLikeHTML(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"doctype", "<!DOCTYPE html><html></html>", true},
		{"lowercase doctype", "<!doctype html><html></html>", true},
		{"html tag", "<html><body>login</body></html>", true},
		{"leading whitespace", "\n\t  <!DOCTYPE html>", true},
		{"long html preamble still caught", strings.Repeat(" ", 100) + "<!DOCTYPE html>" + strings.Repeat("y", 500), true},
		{"marker preceded by content", "<!-- x -->\n<!DOCTYPE html>", false},
		{"marker at the window edge", "<html" + strings.Repeat("z", 40), true},
		{"yaml body", "identity:\n  name: sample\n", false},
		{"empty", "", false},
		{"whitespace only", "  \n\t", false},
		{"comment", "<!-- a comment -->", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, looksLikeHTML([]byte(tc.in)))
		})
	}
}

// TestShortNetReasonUnwrapsToTheRootCause pins the one-line warning contract.
func TestShortNetReasonUnwrapsToTheRootCause(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{testErrNoSuchHost, `Get "https://x.test/a": dial tcp: lookup x.test: no such host`, testErrNoSuchHost},
		{testErrConnRefused, "dial tcp 1.2.3.4:443: connect: connection refused", testErrConnRefused},
		{"unreachable", testErrNetUnreachable, testErrNetUnreachable},
		{"reset", "read: connection reset by peer", "connection reset by peer"},
		{"redirect loop", "too many redirects", "too many redirects"},
		{testErrTimeout, "context deadline exceeded", testErrTimeout},
		{"timeout word", "i/o timeout", testErrTimeout},
		{"unmapped reason passes through", "some other failure", "some other failure"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, shortNetReason(fmt.Errorf("%s", tc.in)))
		})
	}
}

// TestShortNetReasonTrunc pins the length guard at its edge.
func TestShortNetReasonTrunc(t *testing.T) {
	short := strings.Repeat("a", 120)
	assert.Equal(t, short, shortNetReason(fmt.Errorf("%s", short)), "120 bytes is not truncated")

	long := strings.Repeat("b", 121)
	assert.Equal(t, long[:120]+"…", shortNetReason(fmt.Errorf("%s", long)))
}

// TestShortNetReasonUnwrapsNestedWrappers pins that the unwalk covers the whole chain.
func TestShortNetReasonUnwrapsNestedWrappers(t *testing.T) {
	inner := fmt.Errorf("dial tcp: lookup x.test: no such host")
	mid := fmt.Errorf("outer: %w", inner)
	outer := &url.Error{Op: "Get", URL: "https://x.test", Err: mid}
	assert.Equal(t, testErrNoSuchHost, shortNetReason(outer))
}

// TestStaleCacheAgeFromEverySource pins the three outcomes.
func TestStaleCacheAgeFromEverySource(t *testing.T) {
	dir := t.TempDir()
	cp := filepath.Join(dir, "inc.yaml")

	assert.Negative(t, staleCacheAge(cp, nil), "no sidecar and no file means an unknown age")

	require.NoError(t, os.WriteFile(cp, []byte("a: 1\n"), 0o644))
	assert.Positive(t, staleCacheAge(cp, nil), "a legacy entry falls back to its file mtime")

	sidecar := &includeCacheMeta{FetchedAt: time.Now().Add(-2 * time.Hour)}
	assert.InDelta(t, 2*time.Hour, staleCacheAge(cp, sidecar), float64(time.Minute))

	zero := &includeCacheMeta{}
	assert.Positive(t, staleCacheAge(cp, zero), "a zero timestamp falls back to the mtime")
}

// TestCacheFresh pins the freshness predicate a revalidation decision rests on.
func TestCacheFresh(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	assert.False(t, cacheFresh(nil, now), "no sidecar is never fresh")
	assert.False(t, cacheFresh(&includeCacheMeta{}, now), "a zero expiry is never fresh")
	assert.True(t, cacheFresh(&includeCacheMeta{ExpiresAt: now.Add(time.Nanosecond)}, now))
	assert.False(t, cacheFresh(&includeCacheMeta{ExpiresAt: now}, now), "the expiry instant is stale")
	assert.False(t, cacheFresh(&includeCacheMeta{ExpiresAt: now.Add(-time.Nanosecond)}, now))
}

// TestWriteCacheFileIsAtomic pins that a concurrent reader never sees a truncated file.
func TestWriteCacheFileIsAtomic(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "nested", "inc.yaml")

	writeCacheFile(dst, []byte("first"))
	body, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "first", string(body))

	writeCacheFile(dst, []byte("second"))
	body, err = os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "second", string(body))

	entries, err := filepath.Glob(filepath.Join(dir, "nested", "*.tmp"))
	require.NoError(t, err)
	assert.Empty(t, entries, "a successful write leaves no temp file")

	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm())
}

// TestWriteCacheFileUnwritableTarget is the best-effort contract.
func TestWriteCacheFileUnwritableTarget(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o644))

	target := filepath.Join(blocker, "inc.yaml")
	assert.NotPanics(t, func() { writeCacheFile(target, []byte("data")) })
	assert.NoFileExists(t, target)
}

// TestSaveAndLoadCacheMetaRoundTrip pins the sidecar contract.
func TestSaveAndLoadCacheMetaRoundTrip(t *testing.T) {
	isolateCache(t)
	ref := testIncURL

	want := &includeCacheMeta{
		URL:          ref,
		ETag:         "W/\"abc\"",
		LastModified: "Wed, 01 Jan 2026 00:00:00 GMT",
		CacheControl: "private, max-age=300",
		Expires:      "Wed, 01 Jan 2026 01:00:00 GMT",
		FetchedAt:    time.Now().UTC().Truncate(time.Second),
		ExpiresAt:    time.Now().UTC().Add(time.Hour).Truncate(time.Second),
	}
	saveCacheMeta(ref, want)

	got, err := loadCacheMeta(ref)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want.URL, got.URL)
	assert.Equal(t, want.ETag, got.ETag)
	assert.Equal(t, want.LastModified, got.LastModified)
	assert.Equal(t, want.CacheControl, got.CacheControl)
	assert.Equal(t, want.Expires, got.Expires)
	assert.True(t, want.FetchedAt.Equal(got.FetchedAt))
	assert.True(t, want.ExpiresAt.Equal(got.ExpiresAt))
}

func TestLoadCacheMetaAbsent(t *testing.T) {
	isolateCache(t)
	got, err := loadCacheMeta("https://example.com/never-cached.yaml")
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestLoadCacheMetaCorrupt(t *testing.T) {
	isolateCache(t)
	ref := testIncURL
	mp, err := includeCacheMetaPath(ref)
	require.NoError(t, err)
	writeCacheFile(mp, []byte("not json"))

	got, err := loadCacheMeta(ref)
	require.Error(t, err)
	assert.Nil(t, got)
}

// TestSaveCacheMetaSkipsAnUnpathedRef pins that a meta with no resolvable path is dropped.
func TestSaveCacheMetaSkipsAnUnpathedRef(t *testing.T) {
	isolateCache(t)
	ref := "https://example.com/bad.yaml"
	mp, err := includeCacheMetaPath(ref)
	require.NoError(t, err)

	saveCacheMeta(ref, &includeCacheMeta{URL: ref})
	before, err := os.ReadFile(mp)
	require.NoError(t, err)

	assert.NotPanics(t, func() { saveCacheMeta("", &includeCacheMeta{}) })
	after, err := os.ReadFile(mp)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))
}

// TestStatusHint pins the table that turns a bare HTTP code into an actionable cause.
func TestStatusHint(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{http.StatusUnauthorized, "authentication"},
		{http.StatusForbidden, "forbidden"},
		{http.StatusNotFound, "not found"},
		{http.StatusInternalServerError, "server"},
		{http.StatusBadGateway, "server"},
		{http.StatusTeapot, ""},
	}
	for _, tc := range cases {
		t.Run(http.StatusText(tc.code), func(t *testing.T) {
			got := statusHint(tc.code)
			if tc.want == "" {
				assert.NotContains(t, got, "(")
				return
			}
			assert.Contains(t, got, tc.want)
		})
	}
}
