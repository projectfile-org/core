// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serveInclude starts a server for one include URL; handler sees the attempt number.
func serveInclude(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, attempt int)) string {
	t.Helper()
	attempt := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, attempt)
		attempt++
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/inc.yaml"
}

// okBody answers a plain fetch with a well-formed include document.
func okBody(w http.ResponseWriter, name string) {
	w.Header().Set("Content-Type", "application/yaml")
	_, _ = fmt.Fprintf(w, "identity:\n  name: %s\n", name)
}

// nameOf reads back the identity the include contributed.
func nameOf(t *testing.T, m map[string]any) string {
	t.Helper()
	id, ok := m["identity"].(map[string]any)
	require.True(t, ok, "the include must contribute an identity block")
	name, ok := id["name"].(string)
	require.True(t, ok)
	return name
}

// resolveIn fetches one ref through the network tier against dir.
func resolveIn(t *testing.T, dir, ref string, ttl time.Duration) (map[string]any, error) {
	t.Helper()
	return ResolveIncludesOnly(
		map[string]any{"includes": []any{ref}},
		dir,
		ReadOptions{CacheTTL: ttl},
	)
}

// TestConditionalRequestCarriesTheValidators pins that a stale entry revalidates with the stored validators.
func TestConditionalRequestCarriesTheValidators(t *testing.T) {
	isolateCache(t)
	var sent []*http.Request

	url := serveInclude(t, func(w http.ResponseWriter, r *http.Request, attempt int) {
		sent = append(sent, r.Clone(r.Context()))
		w.Header().Set("ETag", `W/"v1"`)
		w.Header().Set("Last-Modified", "Wed, 01 Jan 2026 00:00:00 GMT")
		if attempt > 0 {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		okBody(w, "served")
	})

	first, err := resolveIn(t, t.TempDir(), url, time.Nanosecond)
	require.NoError(t, err)
	require.Equal(t, "served", nameOf(t, first))
	require.Len(t, sent, 1)
	assert.Empty(t, sent[0].Header.Get("If-None-Match"), "the first fetch is unconditional")

	dir := t.TempDir()
	second, err := resolveIn(t, dir, url, time.Nanosecond)
	require.NoError(t, err)
	require.Equal(t, "served", nameOf(t, second), "a 304 reuses the cached body")
	require.Len(t, sent, 2, "a stale entry is revalidated")
	assert.Equal(t, `W/"v1"`, sent[1].Header.Get("If-None-Match"))
	assert.Equal(t, "Wed, 01 Jan 2026 00:00:00 GMT", sent[1].Header.Get("If-Modified-Since"))
}

// TestBareNotModifiedKeepsEveryStoredValidator pins the carry-forward at its source.
func TestBareNotModifiedKeepsEveryStoredValidator(t *testing.T) {
	isolateCache(t)

	const (
		etag  = `W/"v1"`
		lastM = "Wed, 01 Jan 2026 00:00:00 GMT"
	)
	attempt := 0
	url := serveInclude(t, func(w http.ResponseWriter, _ *http.Request, n int) {
		attempt = n
		if n == 0 {
			w.Header().Set("ETag", etag)
			w.Header().Set("Last-Modified", lastM)
			// max-age=0 is stored verbatim but floored at the fallback TTL.
			w.Header().Set("Cache-Control", "max-age=0")
			okBody(w, "served")
			return
		}
		w.WriteHeader(http.StatusNotModified)
	})

	dir := t.TempDir()
	resolve := func() {
		_, err := resolveIn(t, dir, url, time.Nanosecond)
		require.NoError(t, err)
	}

	resolve()
	meta, err := loadCacheMeta(url)
	require.NoError(t, err)
	require.Equal(t, etag, meta.ETag)
	require.Equal(t, lastM, meta.LastModified)
	require.Equal(t, "max-age=0", meta.CacheControl)
	require.False(t, meta.ExpiresAt.IsZero())

	resolve()
	require.Equal(t, 1, attempt, "the second read revalidated")

	got, err := loadCacheMeta(url)
	require.NoError(t, err)
	assert.Equal(t, etag, got.ETag, "the ETag must survive a 304 that omitted it")
	assert.Equal(t, lastM, got.LastModified, "Last-Modified must survive too")
	assert.Equal(t, "max-age=0", got.CacheControl, "Cache-Control must survive too")
	assert.False(t, got.ExpiresAt.IsZero(), "the expiry must survive too")
}

// TestNotModifiedAdoptsFreshValidators is the complement.
func TestNotModifiedAdoptsFreshValidators(t *testing.T) {
	isolateCache(t)

	attempt := 0
	url := serveInclude(t, func(w http.ResponseWriter, _ *http.Request, n int) {
		attempt = n
		switch n {
		case 0:
			w.Header().Set("ETag", `W/"v1"`)
			okBody(w, "served")
		default:
			w.Header().Set("ETag", `W/"v2"`)
			w.Header().Set("Last-Modified", "Thu, 02 Jan 2026 00:00:00 GMT")
			w.WriteHeader(http.StatusNotModified)
		}
	})

	dir := t.TempDir()
	for range 2 {
		_, err := resolveIn(t, dir, url, time.Nanosecond)
		require.NoError(t, err)
	}

	require.Equal(t, 1, attempt)
	got, err := loadCacheMeta(url)
	require.NoError(t, err)
	assert.Equal(t, `W/"v2"`, got.ETag, "the 304's validator replaces the stored one")
	assert.Equal(t, "Thu, 02 Jan 2026 00:00:00 GMT", got.LastModified)
}

// TestFreshCacheSkipsTheNetwork pins that an entry inside its window costs no request.
func TestFreshCacheSkipsTheNetwork(t *testing.T) {
	isolateCache(t)
	attempts := 0

	url := serveInclude(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		attempts++
		okBody(w, "served")
	})

	dir := t.TempDir()
	for range 5 {
		got, err := resolveIn(t, dir, url, 0)
		require.NoError(t, err)
		require.Equal(t, "served", nameOf(t, got))
	}
	assert.Equal(t, 1, attempts, "a fresh entry is read from disk, never refetched")
}

// TestPoisonedCacheEntrySelfHeals pins the defence against a cached auth/login page.
func TestPoisonedCacheEntrySelfHeals(t *testing.T) {
	isolateCache(t)

	url := serveInclude(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		okBody(w, "healed")
	})

	cp, err := includeCachePath(url)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(cp), 0o755))
	require.NoError(t, os.WriteFile(cp, []byte("<!DOCTYPE html><html><body>login</body></html>"), 0o644))

	got, err := resolveIn(t, t.TempDir(), url, 0)
	require.NoError(t, err)
	assert.Equal(t, "healed", nameOf(t, got), "a poisoned entry is replaced, not parsed")
}

// TestEmptyCacheEntrySelfHeals is the empty-file twin.
func TestEmptyCacheEntrySelfHeals(t *testing.T) {
	isolateCache(t)

	url := serveInclude(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		okBody(w, "refilled")
	})

	cp, err := includeCachePath(url)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(cp), 0o755))
	require.NoError(t, os.WriteFile(cp, nil, 0o644))

	got, err := resolveIn(t, t.TempDir(), url, 0)
	require.NoError(t, err)
	assert.Equal(t, "refilled", nameOf(t, got), "an empty entry is discarded, not returned blank")
}

// TestHTMLResponseIsRefused pins the content-type gate.
func TestHTMLResponseIsRefused(t *testing.T) {
	isolateCache(t)

	url := serveInclude(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body>Sign in</body></html>"))
	})

	_, err := resolveIn(t, t.TempDir(), url, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTML",
		"the real cause must reach the user instead of a YAML parse error")
}

// TestStaleServedWhenTheOriginFails pins graceful degradation.
func TestStaleServedWhenTheOriginFails(t *testing.T) {
	isolateCache(t)
	attempt := 0

	url := serveInclude(t, func(w http.ResponseWriter, _ *http.Request, n int) {
		attempt++
		switch n {
		case 0:
			okBody(w, "cached-body")
		case 1:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			panic(http.ErrAbortHandler)
		}
	})

	dir := t.TempDir()
	first, err := resolveIn(t, dir, url, time.Nanosecond)
	require.NoError(t, err)
	require.Equal(t, "cached-body", nameOf(t, first))

	again, err := resolveIn(t, dir, url, time.Nanosecond)
	require.NoError(t, err, "a 5xx against a cached entry serves stale rather than failing")
	assert.Equal(t, "cached-body", nameOf(t, again))

	aborted, err := resolveIn(t, dir, url, time.Nanosecond)
	require.NoError(t, err, "an unreachable origin serves stale too")
	assert.Equal(t, "cached-body", nameOf(t, aborted))
	assert.GreaterOrEqual(t, attempt, 3)
}

// TestUncachedOfflineIncludeIsSkipped pins that partial data beats a hard stop.
func TestUncachedOfflineIncludeIsSkipped(t *testing.T) {
	isolateCache(t)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "local.yaml"),
		[]byte("keywords:\n  - local\n"), 0o644))

	got, err := ResolveIncludesOnly(map[string]any{
		"includes": []any{"local.yaml", "https://example.com/never.yaml"},
	}, dir, ReadOptions{Offline: true})
	require.NoError(t, err)
	assert.Equal(t, []any{"local"}, got["keywords"], "the resolvable include still lands")
}

// TestOfflineReadsAFreshCachedEntry pins that offline tolerates a stale cached entry.
func TestOfflineReadsAFreshCachedEntry(t *testing.T) {
	isolateCache(t)

	url := serveInclude(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		okBody(w, "from-cache")
	})

	_, err := resolveIn(t, t.TempDir(), url, time.Hour)
	require.NoError(t, err)

	got, err := ResolveIncludesOnly(map[string]any{"includes": []any{url}},
		t.TempDir(), ReadOptions{Offline: true, CacheTTL: time.Nanosecond})
	require.NoError(t, err, "offline must tolerate a stale cached entry")
	assert.Equal(t, "from-cache", nameOf(t, got))
}

// TestCrossHostRedirectRefused pins the SSO-gateway defence.
func TestCrossHostRedirectRefused(t *testing.T) {
	isolateCache(t)

	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		okBody(w, "elsewhere")
	}))
	t.Cleanup(elsewhere.Close)

	url := serveInclude(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		http.Redirect(w, r, elsewhere.URL+"/login", http.StatusFound)
	})

	_, err := resolveIn(t, t.TempDir(), url, 0)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "elsewhere",
		"a cross-host bounce must never deliver another host's body")
}

// TestRedirectWithinTheSameHostIsFollowed pins that the guard is about the HOST.
func TestRedirectWithinTheSameHostIsFollowed(t *testing.T) {
	isolateCache(t)

	url := serveInclude(t, func(w http.ResponseWriter, r *http.Request, n int) {
		if n == 0 {
			http.Redirect(w, r, "/moved/inc.yaml", http.StatusMovedPermanently)
			return
		}
		okBody(w, "after-redirect")
	})

	got, err := resolveIn(t, t.TempDir(), url, 0)
	require.NoError(t, err, "a same-host redirect is not an auth gateway")
	assert.Equal(t, "after-redirect", nameOf(t, got))
}

// TestRedirectLoopIsRefused pins the hop ceiling.
func TestRedirectLoopIsRefused(t *testing.T) {
	isolateCache(t)

	url := serveInclude(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		http.Redirect(w, r, "/loop/inc.yaml", http.StatusFound)
	})

	_, err := resolveIn(t, t.TempDir(), url, 0)
	require.Error(t, err, "a redirect loop must stop rather than spin")
}

// TestCrossHostRedirectNamesTheHandoff pins that the refusal explains itself.
func TestCrossHostRedirectNamesTheHandoff(t *testing.T) {
	isolateCache(t)

	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		okBody(w, "elsewhere")
	}))
	t.Cleanup(elsewhere.Close)

	url := serveInclude(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		http.Redirect(w, r, elsewhere.URL+"/login", http.StatusFound)
	})

	_, err := resolveIn(t, t.TempDir(), url, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cross-host")
	assert.Contains(t, err.Error(), "auth gateway",
		"the message must name the cause a user can act on")
}

// TestStatusCodesCarryTheirCause pins the hint table reaching the user.
func TestStatusCodesCarryTheirCause(t *testing.T) {
	isolateCache(t)

	for _, tc := range []struct {
		code int
		want string
	}{
		{http.StatusUnauthorized, "authentication"},
		{http.StatusForbidden, "forbidden"},
		{http.StatusNotFound, "not found"},
		{http.StatusInternalServerError, "HTTP 500"},
	} {
		t.Run(http.StatusText(tc.code), func(t *testing.T) {
			url := serveInclude(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
				w.WriteHeader(tc.code)
			})

			_, err := resolveIn(t, t.TempDir(), url, 0)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.Contains(t, err.Error(), fmt.Sprint(tc.code), "the status code itself is named")
		})
	}
}

// TestOversizedBodyRefused pins the size ceiling.
func TestOversizedBodyRefused(t *testing.T) {
	isolateCache(t)

	url := serveInclude(t, func(w http.ResponseWriter, _ *http.Request, _ int) {
		w.Header().Set("Content-Type", "application/yaml")
		chunk := make([]byte, 1<<20)
		for i := range chunk {
			chunk[i] = 'a'
		}
		for range maxIncludeBodyBytes/(1<<20) + 2 {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	})

	_, err := resolveIn(t, t.TempDir(), url, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

// ── slice dedup + merge ──────────────────────────────────────────────────────

// TestDedupSliceKeepsUncomparableValuesOnce pins the comparator guard.
func TestDedupSliceKeepsUncomparableValuesOnce(t *testing.T) {
	m1 := map[string]any{"a": 1}
	got := dedupSlice([]any{"x", "y"}, []any{"y", "z"})
	assert.Equal(t, []any{"x", "y", "z"}, got, "a repeated comparable value appears once")

	assert.NotPanics(t, func() {
		out := dedupSlice([]any{m1, []any{1, 2}, "s"})
		assert.Len(t, out, 3, "uncomparable entries are kept, not dropped and not fatal")
	})

	assert.Equal(t, []any{1, 2}, dedupSlice([]any{1, 2}), "a single slice deduplicates against itself")
	assert.Empty(t, dedupSlice(), "no slices means no values")
	assert.Empty(t, dedupSlice(nil, nil))
}

// TestDedupSliceCollapsesRepeatedNils pins that a nil is deduplicated like any repeated value.
func TestDedupSliceCollapsesRepeatedNils(t *testing.T) {
	assert.Equal(t, []any{nil, "a"}, dedupSlice([]any{nil, "a", nil}))
	assert.Equal(t, []any{nil}, dedupSlice([]any{nil, nil}))
}

// TestDeepMergeDirection pins that the winner wins and the loser fills.
func TestDeepMergeDirection(t *testing.T) {
	winner := map[string]any{"name": "from-base", "only-base": 1}
	loser := map[string]any{"name": "from-include", "only-include": 2}

	got := deepMerge(winner, loser)
	assert.Equal(t, "from-base", got["name"])
	assert.Equal(t, 1, got["only-base"])
	assert.Equal(t, 2, got["only-include"], "a field the winner omits is filled")
}

func TestDeepMergeEmptySides(t *testing.T) {
	only := map[string]any{"a": 1}
	assert.Equal(t, only, deepMerge(only, nil))
	assert.Equal(t, only, deepMerge(nil, only))
	assert.Equal(t, map[string]any{}, deepMerge(nil, nil))
}

// ── ancestor seeding and cycles ──────────────────────────────────────────────

// TestSeedIncludeAncestors pins the cycle seed for every entry point.
func TestSeedIncludeAncestors(t *testing.T) {
	assert.Empty(t, seedIncludeAncestors(""), "an empty selfPath seeds nothing")

	p := filepath.Join(t.TempDir(), "projectfile.yaml")
	assert.Len(t, seedIncludeAncestors(p), 1, "a real path seeds itself")
}

// TestResolveIncludesWithAndWithoutSelfPath pins that the seed is optional on the read path.
func TestResolveIncludesWithAndWithoutSelfPath(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "inc.yaml"), []byte("keywords:\n  - from-inc\n"), 0o644))

	for _, selfPath := range []string{"", filepath.Join(dir, "base.yaml")} {
		t.Run("selfPath="+selfPath, func(t *testing.T) {
			got, err := resolveIncludes(map[string]any{"includes": []any{"inc.yaml"}}, dir, selfPath, ReadOptions{})
			require.NoError(t, err)
			assert.Equal(t, []any{"from-inc"}, got["keywords"])
		})
	}
}

// TestAllHTTPIncludesWithAndWithoutSelfPath pins the cache-warm walker against both seed shapes.
func TestAllHTTPIncludesWithAndWithoutSelfPath(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "inc.yaml"),
		[]byte("includes:\n  - https://example.com/transitive.yaml\n"), 0o644))

	raw := map[string]any{"includes": []any{
		"https://example.com/direct.yaml",
		"inc.yaml",
	}}

	for _, selfPath := range []string{"", filepath.Join(dir, "base.yaml")} {
		t.Run("selfPath="+selfPath, func(t *testing.T) {
			got := AllHTTPIncludes(raw, dir, selfPath, ReadOptions{Offline: true})
			assert.Contains(t, got, "https://example.com/direct.yaml")
			assert.Contains(t, got, "https://example.com/transitive.yaml")
			assert.NotContains(t, got, "inc.yaml", "a local include is not warmable")
		})
	}
}

// TestPurgeIncludeUnresolvableCache pins that a purge never touches a path it cannot name.
func TestPurgeIncludeUnresolvableCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")

	removed, err := PurgeInclude("https://example.com/inc.yaml")
	require.Error(t, err)
	assert.False(t, removed)
}

// TestPurgeIncludeRemovesTheSidecarToo pins that a purge leaves no orphaned meta.
func TestPurgeIncludeRemovesTheSidecarToo(t *testing.T) {
	isolateCache(t)
	ref := "https://example.com/inc.yaml"

	cp, err := includeCachePath(ref)
	require.NoError(t, err)
	mp, err := includeCacheMetaPath(ref)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(cp), 0o755))
	require.NoError(t, os.WriteFile(cp, []byte("a: 1\n"), 0o644))
	require.NoError(t, os.WriteFile(mp, []byte("{}"), 0o644))

	removed, err := PurgeInclude(ref)
	require.NoError(t, err)
	assert.True(t, removed)
	assert.NoFileExists(t, cp)
	assert.NoFileExists(t, mp, "the sidecar must not outlive its body")
}

// TestIncludeCycleKey pins that a URL keeps its spelling and a local path uses its absolute form.
func TestIncludeCycleKey(t *testing.T) {
	abs, err := filepath.Abs("projectfile.yaml")
	require.NoError(t, err)

	url := includeCycleKey("https://example.com/a.yaml", "")
	assert.Contains(t, url, "https://example.com/a.yaml")

	local := includeCycleKey("projectfile.yaml", abs)
	assert.Equal(t, "path:"+filepath.Clean(abs), local,
		"a local include is keyed by its absolute path, namespaced apart from a URL")
}

// TestDirectSelfIncludeIsACycle pins the guard the seed exists for.
func TestDirectSelfIncludeIsACycle(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "projectfile.yaml")
	require.NoError(t, os.WriteFile(self, []byte("includes:\n  - projectfile.yaml\n"), 0o644))

	_, err := resolveIncludes(map[string]any{"includes": []any{"projectfile.yaml"}},
		dir, self, ReadOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

// TestIndirectCycleIsRejected walks a two-file loop the direct guard alone would miss.
func TestIndirectCycleIsRejected(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "base.yaml")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.yaml"),
		[]byte("includes:\n  - b.yaml\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.yaml"),
		[]byte("includes:\n  - a.yaml\n"), 0o644))

	_, err := resolveIncludes(map[string]any{"includes": []any{"a.yaml"}}, dir, self, ReadOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

// TestDiamondIsNotACycle pins that a shared document on two branches is not a cycle.
func TestDiamondIsNotACycle(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "base.yaml")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "shared.yaml"),
		[]byte("keywords:\n  - shared\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "left.yaml"),
		[]byte("includes:\n  - shared.yaml\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "right.yaml"),
		[]byte("includes:\n  - shared.yaml\n"), 0o644))

	got, err := resolveIncludes(map[string]any{
		"includes": []any{"left.yaml", "right.yaml"},
	}, dir, self, ReadOptions{})
	require.NoError(t, err)
	assert.Len(t, got["keywords"], 1, "the diamond's shared value is deduplicated, not listed twice")
}

// TestWriteCacheFileOverADirectoryTargetIsBestEffort pins that an unwritable cache write is dropped.
func TestWriteCacheFileOverADirectoryTargetIsBestEffort(t *testing.T) {
	isolateCache(t)

	dir, err := IncludesCacheDir()
	require.NoError(t, err)

	blocked := filepath.Join(dir, "blocked")
	require.NoError(t, os.MkdirAll(blocked, 0o755))

	assert.NotPanics(t, func() { writeCacheFile(blocked, []byte("data")) })
}
