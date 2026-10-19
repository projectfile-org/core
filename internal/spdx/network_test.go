// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package spdx_test

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/internal/netfetch"
	"kiota.ch/projectfile/core/v2/internal/spdx"
)

// roundTripFunc adapts a function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper.
func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// recorder counts the requests a stub saw so a retry assertion names the attempts.
type recorder struct {
	mu    sync.Mutex
	urls  []string
	users []string
}

func (r *recorder) record(req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.urls = append(r.urls, req.URL.String())
	r.users = append(r.users, req.Header.Get("User-Agent"))
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.urls)
}

func (r *recorder) userAgents() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.users...)
}

// serve installs fn as the process-wide transport for one test.
func serve(t *testing.T, fn roundTripFunc) *recorder {
	t.Helper()
	rec := &recorder{}
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		rec.record(req)
		return fn(req)
	})
	t.Cleanup(func() { http.DefaultTransport = prev })
	return rec
}

func respond(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
}

func TestMain(m *testing.M) {
	restore := netfetch.NoBackoff()
	code := m.Run()
	restore()
	os.Exit(code)
}

// isolateCache gives the test its own XDG slot so the developer's real cache is never read.
func isolateCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	return dir
}

func seedCache(t *testing.T, ids ...string) {
	t.Helper()
	dir, err := spdx.CacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	for _, id := range ids {
		require.NoError(t, os.WriteFile(filepath.Join(dir, id+".txt"), []byte("cached "+id), 0o644))
	}
}

// ── Text: the online tier ────────────────────────────────────────────────────

// TestTextFetchesAndCaches pins the 3-tier contract: a miss on both tiers goes to the network and is cached.
func TestTextFetchesAndCaches(t *testing.T) {
	isolateCache(t)
	spdx.SetEmbedded(nil)

	rec := serve(t, func(*http.Request) (*http.Response, error) {
		return respond(http.StatusOK, "fetched MIT body"), nil
	})

	got, err := spdx.Text(testMIT, spdx.Options{})
	require.NoError(t, err)
	assert.Equal(t, "fetched MIT body", got)
	require.Equal(t, 1, rec.count())
	assert.Equal(t, []string{"projectfile"}, rec.userAgents(), "upstream requires a User-Agent")

	dir, err := spdx.CacheDir()
	require.NoError(t, err)
	body, err := os.ReadFile(filepath.Join(dir, testMIT+".txt"))
	require.NoError(t, err)
	assert.Equal(t, "fetched MIT body", string(body))

	_, err = spdx.Text(testMIT, spdx.Options{})
	require.NoError(t, err)
	assert.Equal(t, 1, rec.count(), "a cache hit must not reach the network")
}

func TestTextEmbeddedBeatsNetwork(t *testing.T) {
	isolateCache(t)
	withCorpus(t, map[string]string{fileMIT: "embedded MIT body"})
	rec := serve(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("tier 1 must answer before the network is consulted")
		return nil, nil
	})

	got, err := spdx.Text(testMIT, spdx.Options{})
	require.NoError(t, err)
	assert.Equal(t, "embedded MIT body", got)
	assert.Equal(t, 0, rec.count())
}

func TestTextCacheBeatsNetwork(t *testing.T) {
	isolateCache(t)
	spdx.SetEmbedded(nil)
	seedCache(t, testMIT)
	serve(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("tier 2 must answer before the network is consulted")
		return nil, nil
	})

	got, err := spdx.Text(testMIT, spdx.Options{})
	require.NoError(t, err)
	assert.Equal(t, "cached MIT", got)
}

func TestTextUpstream404IsErrUnknown(t *testing.T) {
	isolateCache(t)
	spdx.SetEmbedded(nil)
	serve(t, func(*http.Request) (*http.Response, error) { return respond(http.StatusNotFound, ""), nil })

	_, err := spdx.Text("Nope-1.0", spdx.Options{})
	require.Error(t, err)
	assert.ErrorIs(t, err, spdx.ErrUnknown)
}

func TestTextUpstream500SurfacesAfterRetries(t *testing.T) {
	isolateCache(t)
	spdx.SetEmbedded(nil)
	rec := serve(t, func(*http.Request) (*http.Response, error) {
		return respond(http.StatusInternalServerError, "boom"), nil
	})

	_, err := spdx.Text(testMIT, spdx.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
	assert.Equal(t, netfetch.Attempts(), rec.count(), "a 5xx is retried to the attempt budget")
}

func TestTextUpstreamOversizedBody(t *testing.T) {
	isolateCache(t)
	spdx.SetEmbedded(nil)
	serve(t, func(*http.Request) (*http.Response, error) {
		return respond(http.StatusOK, strings.Repeat("x", 1<<20+1)), nil
	})

	_, err := spdx.Text(testMIT, spdx.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

// TestTextFetchWritesCacheOnlyWhenResolvable pins that an unresolvable cache dir still returns the body.
func TestTextFetchWritesCacheOnlyWhenResolvable(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	spdx.SetEmbedded(nil)
	serve(t, func(*http.Request) (*http.Response, error) { return respond(http.StatusOK, "body"), nil })

	got, err := spdx.Text(testMIT, spdx.Options{})
	require.NoError(t, err)
	assert.Equal(t, "body", got)
}

// TestTextOfflineWithUnresolvableCache proves offline refuses rather than reaching the network.
func TestTextOfflineWithUnresolvableCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	spdx.SetEmbedded(nil)
	serve(t, func(*http.Request) (*http.Response, error) {
		t.Fatal("offline must never reach the network")
		return nil, nil
	})

	_, err := spdx.Text(testMIT, spdx.Options{Offline: true})
	require.Error(t, err)
	assert.ErrorIs(t, err, spdx.ErrOffline)
}

func TestTextRejectsInvalidID(t *testing.T) {
	isolateCache(t)
	spdx.SetEmbedded(nil)
	for _, id := range []string{"has space", "sl/ash", "../escape", "under_score"} {
		_, err := spdx.Text(id, spdx.Options{})
		require.Error(t, err, "id %q", id)
		assert.Contains(t, err.Error(), "invalid id", "id %q", id)
	}
}

// TestTextAcceptsEveryIDCharacter pins the alphabet validSPDXID allows, at its edges.
func TestTextAcceptsEveryIDCharacter(t *testing.T) {
	isolateCache(t)
	for _, id := range []string{testMIT, "Zlib", "0BSD", "a", "9", testApache, "CC-BY-4.0", "GPL-2.0+", "x:y", "x.y"} {
		withCorpus(t, map[string]string{id + ".txt": "body " + id})
		got, err := spdx.Text(id, spdx.Options{Offline: true})
		require.NoError(t, err, "id %q", id)
		assert.Equal(t, "body "+id, got, "id %q", id)
	}
}

// TestTextTrimsSurroundingSpace pins that stray whitespace never turns a legal id into an invalid one.
func TestTextTrimsSurroundingSpace(t *testing.T) {
	isolateCache(t)
	withCorpus(t, map[string]string{fileMIT: "trimmed"})
	got, err := spdx.Text("  MIT  ", spdx.Options{Offline: true})
	require.NoError(t, err)
	assert.Equal(t, "trimmed", got)
}

// ── cache directory resolution ───────────────────────────────────────────────

func TestCacheDirHomeFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", home)

	dir, err := spdx.CacheDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".cache", "pf", "spdx"), dir)
}

// TestCacheDirUnresolvable pins the only failure: neither XDG nor HOME names a root.
func TestCacheDirUnresolvable(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")

	dir, err := spdx.CacheDir()
	require.Error(t, err)
	assert.Empty(t, dir)

	_, err = spdx.Purge()
	require.Error(t, err, "Purge reports the unresolved slot instead of claiming success")
}

// ── CachedIDs / Status / EmbeddedIDs ─────────────────────────────────────────

func TestCachedIDsMissingDir(t *testing.T) {
	isolateCache(t)
	assert.Nil(t, spdx.CachedIDs(), "a missing cache is a clean state")
}

func TestCachedIDsSortedAndFiltered(t *testing.T) {
	isolateCache(t)
	seedCache(t, testMIT, testApache, testISC)
	dir, err := spdx.CacheDir()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.md"), []byte("x"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))

	assert.Equal(t, []string{testApache, testISC, testMIT}, spdx.CachedIDs())
}

func TestCachedIDsSkipsEveryNonTextEntry(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	dir, err := spdx.CacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	for name, body := range map[string]string{
		fileMIT: "a", fileApache: "b", "README.md": "c",
		"notes": "d", "MIT.txt.meta.json": "{}",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
	}

	assert.Equal(t, []string{testApache, testMIT}, spdx.CachedIDs())
}

func TestStatusCountsBothTiers(t *testing.T) {
	isolateCache(t)
	withCorpus(t, map[string]string{fileMIT: "m", fileISC: "i", "README.md": "ignored"})
	seedCache(t, testMIT, testApache)
	dir, err := spdx.CacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))

	got := spdx.Status()
	assert.Equal(t, 2, got.Embedded, "a non-.txt corpus entry is not a licence")
	assert.Equal(t, 2, got.Cached)
}

func TestStatusMissingCacheDir(t *testing.T) {
	isolateCache(t)
	withCorpus(t, map[string]string{fileMIT: "m"})
	assert.Equal(t, spdx.CacheStatus{Embedded: 1, Cached: 0}, spdx.Status())
}

// TestEmbeddedIDsNoCorpus is the honest-report contract: a consumer that registers nothing says zero.
func TestEmbeddedIDsNoCorpus(t *testing.T) {
	spdx.SetEmbedded(nil)
	assert.Nil(t, spdx.EmbeddedIDs())
}

// TestEmbeddedIDsUnreadableRoot covers a corpus whose root cannot be listed.
func TestEmbeddedIDsUnreadableRoot(t *testing.T) {
	spdx.SetEmbedded(failingFS{})
	t.Cleanup(func() { spdx.SetEmbedded(nil) })
	assert.Nil(t, spdx.EmbeddedIDs())
}

// failingFS is an fs.FS that refuses to open anything.
type failingFS struct{}

// Open implements fs.FS.
func (failingFS) Open(_ string) (fs.File, error) { return nil, fs.ErrPermission }

func TestEmbeddedIDsSkipsDirectories(t *testing.T) {
	spdx.SetEmbedded(fstest.MapFS{
		fileMIT:              &fstest.MapFile{Data: []byte("m")},
		fileISC:              &fstest.MapFile{Data: []byte("i")},
		"notes.md":           &fstest.MapFile{Data: []byte("x")},
		"nested/MIT-dup.txt": &fstest.MapFile{Data: []byte("n")},
	})
	t.Cleanup(func() { spdx.SetEmbedded(nil) })
	assert.ElementsMatch(t, []string{testMIT, testISC}, spdx.EmbeddedIDs())
}

// ── Purge ────────────────────────────────────────────────────────────────────

func TestPurgeMissingDirIsNoOp(t *testing.T) {
	isolateCache(t)
	removed, err := spdx.Purge()
	require.NoError(t, err)
	assert.Equal(t, 0, removed)
}

func TestPurgeSkipsDirectories(t *testing.T) {
	isolateCache(t)
	seedCache(t, testMIT, testISC)
	dir, err := spdx.CacheDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))

	removed, err := spdx.Purge()
	require.NoError(t, err)
	assert.Equal(t, 2, removed, "a subdirectory is not a cached licence")
	assert.DirExists(t, filepath.Join(dir, "sub"), "Purge must not recurse")
}

// ── WarmAll ──────────────────────────────────────────────────────────────────

// serveLicenseList answers the index URL from list and every text/ request from texts.
func serveLicenseList(t *testing.T, list string, texts map[string]string) *recorder {
	t.Helper()
	return serve(t, func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/json/licenses.json") {
			return respond(http.StatusOK, list), nil
		}
		id := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/spdx/license-list-data/main/text/"), ".txt")
		body, ok := texts[id]
		if !ok {
			return respond(http.StatusNotFound, ""), nil
		}
		return respond(http.StatusOK, body), nil
	})
}

func licenseListJSON(entries ...string) string {
	return `{"licenses":[` + strings.Join(entries, ",") + `]}`
}

func entry(id string, deprecated bool) string {
	return fmt.Sprintf(`{"licenseId":%q,"isDeprecatedLicenseId":%t}`, id, deprecated)
}

// TestWarmAllCountsEveryTier pins that embedded and cached ids are counted, never refetched.
func TestWarmAllCountsEveryTier(t *testing.T) {
	isolateCache(t)
	withCorpus(t, map[string]string{fileMIT: "embedded"})
	seedCache(t, testISC)
	serveLicenseList(t, licenseListJSON(
		entry(testMIT, false),
		entry(testISC, false),
		entry(testApache, false),
	), map[string]string{testApache: "apache body"})

	emb, cached, fetched, err := spdx.WarmAll()
	require.NoError(t, err)
	assert.Equal(t, 1, emb)
	assert.Equal(t, 1, cached)
	assert.Equal(t, 1, fetched)

	dir, err := spdx.CacheDir()
	require.NoError(t, err)
	body, err := os.ReadFile(filepath.Join(dir, fileApache))
	require.NoError(t, err)
	assert.Equal(t, "apache body", string(body))
}

// TestWarmAllSkipsDeprecated pins the rule that keeps a warm run WARN-free.
func TestWarmAllSkipsDeprecated(t *testing.T) {
	isolateCache(t)
	spdx.SetEmbedded(nil)
	rec := serveLicenseList(t, licenseListJSON(entry("GPL-2.0", true), entry(testMIT, false)),
		map[string]string{testMIT: "mit body"})

	_, _, fetched, err := spdx.WarmAll()
	require.NoError(t, err)
	assert.Equal(t, 1, fetched, "the deprecated id must not be attempted")
	assert.Equal(t, 2, rec.count(), "index plus the one live id, nothing for GPL-2.0")
}

func TestWarmAllSkipsCompoundAndEmptyIDs(t *testing.T) {
	isolateCache(t)
	spdx.SetEmbedded(nil)
	rec := serveLicenseList(t, licenseListJSON(
		entry("", false),
		entry("MIT OR Apache-2.0", false),
		entry("Foo Bar", false),
	), map[string]string{})

	_, cached, fetched, err := spdx.WarmAll()
	require.NoError(t, err)
	assert.Equal(t, 0, cached)
	assert.Equal(t, 0, fetched, "an id that cannot name a cache file is skipped, not fatal")
	assert.Equal(t, 1, rec.count(), "only the index was fetched")
}

func TestWarmAllContinuesPastAFailedID(t *testing.T) {
	isolateCache(t)
	spdx.SetEmbedded(nil)
	serveLicenseList(t, licenseListJSON(entry("Gone-1.0", false), entry(testMIT, false)),
		map[string]string{testMIT: "mit body"})

	_, _, fetched, err := spdx.WarmAll()
	require.NoError(t, err, "one unreachable id must not abort the whole warm")
	assert.Equal(t, 1, fetched)
}

func TestWarmAllIndexRejected(t *testing.T) {
	isolateCache(t)
	spdx.SetEmbedded(nil)
	serve(t, func(*http.Request) (*http.Response, error) { return respond(http.StatusForbidden, "no"), nil })

	emb, cached, fetched, err := spdx.WarmAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 403")
	assert.Equal(t, 0, emb)
	assert.Equal(t, 0, cached)
	assert.Equal(t, 0, fetched)
}

func TestWarmAllIndexUnparseable(t *testing.T) {
	isolateCache(t)
	spdx.SetEmbedded(nil)
	serve(t, func(*http.Request) (*http.Response, error) { return respond(http.StatusOK, "not json"), nil })

	_, _, _, err := spdx.WarmAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse license list")
}

func TestWarmAllIndexOversized(t *testing.T) {
	isolateCache(t)
	spdx.SetEmbedded(nil)
	serve(t, func(*http.Request) (*http.Response, error) {
		return respond(http.StatusOK, strings.Repeat("x", 5<<20+1)), nil
	})

	_, _, _, err := spdx.WarmAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestWarmAllIndexUnreachable(t *testing.T) {
	isolateCache(t)
	spdx.SetEmbedded(nil)
	serve(t, func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("dial tcp: connection refused")
	})

	_, _, _, err := spdx.WarmAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetch license list")
}
