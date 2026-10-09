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

const (
	pinBodyA = "keywords: [a]\n"
	pinBodyB = "keywords: [b]\n"
	pinLocal = "local.yaml"
	pinKey   = "sha256"
)

// pinServer serves *body, or a 500 while *fail is set, and counts requests.
func pinServer(t *testing.T, body *string, fail *bool) (url string, calls *int) {
	t.Helper()
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n++
		if *fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(*body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/inc.yaml", &n
}

// pinnedBase writes a base document including url pinned to the digest of pinned.
func pinnedBase(t *testing.T, url, pinned string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	body := fmt.Sprintf("includes:\n  - url: %s\n    sha256: %s\n", url, IncludeDigest([]byte(pinned)))
	return dir, writeInc(t, dir, "base.yaml", body)
}

func TestIncludePin_Match(t *testing.T) {
	isolateCache(t)
	body, fail := pinBodyA, false
	url, _ := pinServer(t, &body, &fail)
	dir, base := pinnedBase(t, url, pinBodyA)
	resolved, err := resolveIncludes(readInc(t, base), dir, base, ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, []any{"a"}, kwSlice(t, resolved))
}

func TestIncludePin_MismatchNamesURLAndDigests(t *testing.T) {
	isolateCache(t)
	body, fail := pinBodyB, false
	url, _ := pinServer(t, &body, &fail)
	dir, base := pinnedBase(t, url, pinBodyA)
	_, err := resolveIncludes(readInc(t, base), dir, base, ReadOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), url)
	assert.Contains(t, err.Error(), IncludeDigest([]byte(pinBodyA)))
	assert.Contains(t, err.Error(), IncludeDigest([]byte(pinBodyB)))
}

func TestIncludePin_FreshCacheMismatchRefetches(t *testing.T) {
	isolateCache(t)
	body, fail := pinBodyA, false
	url, calls := pinServer(t, &body, &fail)
	_, _, err := fetchHTTPInclude(url, ReadOptions{})
	require.NoError(t, err)
	body = pinBodyB
	dir, base := pinnedBase(t, url, pinBodyB)
	resolved, err := resolveIncludes(readInc(t, base), dir, base, ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, []any{"b"}, kwSlice(t, resolved))
	assert.Equal(t, 2, *calls, "a cached copy that misses the pin must be refetched")
}

func TestIncludePin_OfflineCacheMismatchFails(t *testing.T) {
	isolateCache(t)
	body, fail := pinBodyA, false
	url, _ := pinServer(t, &body, &fail)
	_, _, err := fetchHTTPInclude(url, ReadOptions{})
	require.NoError(t, err)
	dir, base := pinnedBase(t, url, pinBodyB)
	_, err = resolveIncludes(readInc(t, base), dir, base, ReadOptions{Offline: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cached copy")
}

func TestIncludePin_StaleFallbackStillVerified(t *testing.T) {
	isolateCache(t)
	body, fail := pinBodyA, false
	url, _ := pinServer(t, &body, &fail)
	_, _, err := fetchHTTPInclude(url, ReadOptions{CacheTTL: time.Nanosecond})
	require.NoError(t, err)
	fail = true
	dir, base := pinnedBase(t, url, pinBodyB)
	_, err = resolveIncludes(readInc(t, base), dir, base, ReadOptions{CacheTTL: time.Nanosecond})
	require.Error(t, err, "a stale copy that misses the pin must not be served on a 5xx")
	dir, base = pinnedBase(t, url, pinBodyA)
	resolved, err := resolveIncludes(readInc(t, base), dir, base, ReadOptions{CacheTTL: time.Nanosecond})
	require.NoError(t, err)
	assert.Equal(t, []any{"a"}, kwSlice(t, resolved))
}

func TestIncludePin_LocalFileVerified(t *testing.T) {
	dir := t.TempDir()
	writeInc(t, dir, "a.yaml", pinBodyA)
	base := writeInc(t, dir, "base.yaml", fmt.Sprintf("includes:\n  - url: a.yaml\n    sha256: %s\n", IncludeDigest([]byte(pinBodyB))))
	_, err := resolveIncludes(readInc(t, base), dir, base, ReadOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "local file")
}

func TestIncludePin_TypedRoundTripKeepsMapping(t *testing.T) {
	for _, ext := range []string{"yaml", "toml", "json"} {
		t.Run(ext, func(t *testing.T) {
			dir := t.TempDir()
			pin := IncludeDigest([]byte(pinBodyA))
			src := &Document{
				Identity:    Identity{Namespace: "org.example", Name: "demo"},
				Includes:    []string{"https://example.org/a.yaml", pinLocal},
				IncludePins: map[string]string{"https://example.org/a.yaml": pin},
			}
			path := filepath.Join(dir, "projectfile."+ext)
			require.NoError(t, Write(src, path))
			got, err := ReadBaseFromPath(path)
			require.NoError(t, err)
			assert.Equal(t, src.Includes, got.Includes)
			assert.Equal(t, src.IncludePins, got.IncludePins)
			raw, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Contains(t, string(raw), pin)
		})
	}
}

func TestSortIncludes_SortsPinnedEntriesByURL(t *testing.T) {
	pinned := map[string]any{keyURL: "https://example.org/b.yaml", pinKey: "00"}
	raw := map[string]any{keyIncludes: []any{pinned, testIncludeA}}
	SortIncludes(raw)
	assert.Equal(t, []any{testIncludeA, pinned}, raw[keyIncludes])
}

func TestIncludePin_CanvasWriteKeepsMapping(t *testing.T) {
	dir := t.TempDir()
	pin := IncludeDigest([]byte(pinBodyA))
	path := writeInc(t, dir, "projectfile.yaml", fmt.Sprintf("# keep\nidentity:\n  namespace: org.example\n  name: demo\nincludes:\n  - local.yaml\n  - url: https://example.org/a.yaml\n    sha256: %s\n", pin))
	doc, err := ReadBaseFromPath(path)
	require.NoError(t, err)
	doc.Keywords = []string{"edited"}
	require.NoError(t, Write(doc, path))
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "sha256: "+pin)
	assert.Contains(t, string(raw), "# keep")
}

func TestPinIncludes_StampsHTTPEntriesOnly(t *testing.T) {
	isolateCache(t)
	body, fail := pinBodyA, false
	url, _ := pinServer(t, &body, &fail)
	raw := map[string]any{keyIncludes: []any{pinLocal, url}}
	pinned, err := PinIncludes(raw, nil, ReadOptions{})
	require.NoError(t, err)
	want := IncludeDigest([]byte(pinBodyA))
	assert.Equal(t, []IncludeEntry{{Ref: url, SHA256: want}}, pinned)
	assert.Equal(t, []any{pinLocal, map[string]any{keyURL: url, pinKey: want}}, raw[keyIncludes])
}

func TestPinIncludes_RepinsAndRejectsUnknown(t *testing.T) {
	isolateCache(t)
	body, fail := pinBodyB, false
	url, _ := pinServer(t, &body, &fail)
	raw := map[string]any{keyIncludes: []any{map[string]any{keyURL: url, pinKey: IncludeDigest([]byte(pinBodyA))}}}
	_, err := PinIncludes(raw, []string{url}, ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, IncludeDigest([]byte(pinBodyB)), raw[keyIncludes].([]any)[0].(map[string]any)[pinKey])
	_, err = PinIncludes(raw, []string{"https://example.invalid/x.yaml"}, ReadOptions{})
	require.Error(t, err)
}
