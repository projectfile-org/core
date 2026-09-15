// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"kiota.ch/projectfile/core/v2/internal/genlog"
)

// rawLookupNS resolves a reverse-DNS namespace in a raw map[string]any,
// handling both flat key (YAML/JSON: "org.projectfile.cli") and dotted-table form
// (TOML: [org.projectfile.cli] exploded into nested maps by go-toml/v2).
func rawLookupNS(raw map[string]any, ns string) (map[string]any, bool) {
	if v, ok := raw[ns]; ok {
		m, ok := v.(map[string]any)
		return m, ok
	}
	segments := strings.Split(ns, ".")
	cur, ok := raw[segments[0]]
	if !ok {
		return nil, false
	}
	for _, seg := range segments[1:] {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[seg]
		if !ok {
			return nil, false
		}
	}
	m, ok := cur.(map[string]any)
	return m, ok
}

func rawLookupIncludes(raw map[string]any) []string {
	if inc := strListVal(raw, "includes"); len(inc) > 0 {
		return inc
	}
	ns, ok := rawLookupNS(raw, CLIExtensionNS)
	if !ok {
		return nil
	}
	return strListVal(ns, "includes")
}

// deepMerge returns a new map where winner's values take precedence over loser's.
// Maps are merged recursively; slices are concatenated (loser first, then winner)
// and DEDUPLICATED by deep equality (reflect.DeepEqual), so a value present on
// both sides — or the same document reached through two include branches
// (a diamond) — yields one entry, not two; all other types (scalars) use winner
// verbatim.
//
// The reserved entity-list keys `people` and `organizations` are exempt from
// plain concatenation: their entries describe identities (persons, orgs) that
// may appear in both the base document and an include with complementary
// fields (base carries project-scoped `from`/`to`; include carries identity
// and contact fields). Concatenating would duplicate the identity and trip
// the schema's required-field checks on the sparse entry. These keys merge
// by identity (orcid → email → canonical name) via MergePeopleRaw /
// MergeOrganizationsRaw, which already de-duplicates; other slices keep the
// concatenate-then-dedup contract.
func deepMerge(winner, loser map[string]any) map[string]any {
	out := make(map[string]any, len(loser)+len(winner))
	for k, v := range loser {
		out[k] = v
	}
	for k, wv := range winner {
		if lv, exists := out[k]; exists {
			wMap, wIsMap := wv.(map[string]any)
			lMap, lIsMap := lv.(map[string]any)
			if wIsMap && lIsMap {
				out[k] = deepMerge(wMap, lMap)
				continue
			}
			wSlice, wIsSlice := wv.([]any)
			lSlice, lIsSlice := lv.([]any)
			if wIsSlice && lIsSlice {
				if merged, ok := mergeEntitySliceKey(k, wSlice, lSlice); ok {
					out[k] = merged
					continue
				}
				out[k] = dedupSlice(lSlice, wSlice)
				continue
			}
		}
		out[k] = wv
	}
	return out
}

// dedupSlice concatenates the given slices IN ORDER (loser first, then winner)
// and drops any item that is deep-equal to one already added. This is the
// generic-slice merge contract: a base and an include (or two include branches
// of a diamond) declaring the same value collapse to a single entry. Equality
// is reflect.DeepEqual, so scalars, maps, and nested slices all dedup by
// content. Order is preserved on first occurrence. Entity lists (people,
// organizations) do NOT come through here — they merge by identity in
// mergeEntitySliceKey.
func dedupSlice(slices ...[]any) []any {
	out := make([]any, 0)
	for _, s := range slices {
		for _, v := range s {
			if !sliceContainsDeep(out, v) {
				out = append(out, v)
			}
		}
	}
	return out
}

// sliceContainsDeep reports whether needle is deep-equal to any element of
// haystack. Used by dedupSlice.
func sliceContainsDeep(haystack []any, needle any) bool {
	for _, h := range haystack {
		if reflect.DeepEqual(h, needle) {
			return true
		}
	}
	return false
}

// mergeEntitySliceKey dispatches reserved entity-list keys to identity-aware
// merging. Returns (nil, false) for any other key so deepMerge falls through
// to plain concatenation. Adding a new identity-merge list is one case here.
func mergeEntitySliceKey(key string, winner, loser []any) ([]any, bool) {
	switch key {
	case keyPeople:
		return MergePeopleRaw(winner, loser), true
	case keyOrganizations:
		return MergeOrganizationsRaw(winner, loser), true
	}
	return nil, false
}

func isHTTPInclude(ref string) bool {
	return strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "http://")
}

// statusHint returns an actionable hint for common HTTP error codes so the
// user sees WHY a fetch failed, not just the bare number. Empty for codes
// we have nothing useful to say about.
func statusHint(code int) string {
	switch {
	case code == http.StatusUnauthorized:
		return " (authentication required)"
	case code == http.StatusForbidden:
		return " (forbidden — repo may be private)"
	case code == http.StatusNotFound:
		return " (not found)"
	case code/100 == 5:
		return " (server error, retry later)"
	default:
		return ""
	}
}

// isHTMLResponse reports whether resp declares itself as HTML. Used to
// reject auth / login pages that return HTTP 200 with an HTML body when a
// repo is private or a path is wrong — without this gate pf-cli would
// parse the HTML as YAML and surface a confusing parse error instead.
func isHTMLResponse(resp *http.Response) bool {
	ct := resp.Header.Get("Content-Type")
	// Content-Type may carry params: "text/html; charset=utf-8".
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.EqualFold(strings.TrimSpace(ct), "text/html")
}

// looksLikeHTML reports whether data begins with an HTML document marker.
// Used to self-heal poisoned cache entries: an older pf-cli build may have
// cached an auth/login page (HTML) as a .yaml include before content-type
// validation existed. Such entries are discarded on read so a fresh fetch
// replaces them.
func looksLikeHTML(data []byte) bool {
	i := 0
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	rest := data[i:]
	if len(rest) > 32 {
		rest = rest[:32]
	}
	lower := strings.ToLower(string(rest))
	return strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html")
}

// includeCachePath returns the XDG cache path for a given HTTP include URL.
// Uses SHA-256 of the URL as the filename with the original extension preserved.
// One shared slot (~/.cache/pf/includes) for every pf-* binary, so a purge in
// one clears the cache the others also read.
func includeCachePath(ref string) (string, error) {
	dir, err := includesCacheDir()
	if err != nil {
		return "", err
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	ext := filepath.Ext(u.Path)
	h := sha256.Sum256([]byte(ref))
	return filepath.Join(dir, fmt.Sprintf("%x%s", h, ext)), nil
}

// includeCacheMetaPath returns the sidecar metadata path for a cached URL.
func includeCacheMetaPath(ref string) (string, error) {
	cp, err := includeCachePath(ref)
	if err != nil {
		return "", err
	}
	return cp + ".meta.json", nil
}

// includesCacheDir resolves the shared includes cache directory (~/.cache/pf/
// includes). Every pf-* binary shares one slot so a purge in one clears the
// cache the others also read.
func includesCacheDir() (string, error) {
	base, err := xdgCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "pf", "includes"), nil
}

// xdgCacheDir resolves ${XDG_CACHE_HOME:-~/.cache}.
func xdgCacheDir() (string, error) {
	if base := os.Getenv("XDG_CACHE_HOME"); base != "" {
		return base, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache"), nil
}

// includeCacheMeta is the sidecar JSON stored beside each cached include.
type includeCacheMeta struct {
	URL          string    `json:"url"`
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"last_modified,omitempty"`
	CacheControl string    `json:"cache_control,omitempty"`
	Expires      string    `json:"expires,omitempty"`
	FetchedAt    time.Time `json:"fetched_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// defaultIncludeTTL is the fallback TTL when origin sends no cache directives.
const defaultIncludeTTL = time.Hour

const maxIncludeBodyBytes = 5 << 20

// effectiveIncludeTTL resolves the TTL to use for ref, honoring ReadOptions
// and $PF_CACHE_TTL / $PF_INCLUDE_CACHE_TTL. Zero means default; negative
// means never expire (used by tests to freeze cache).
func effectiveIncludeTTL(opts ReadOptions) time.Duration {
	if opts.CacheTTL < 0 {
		return -1
	}
	if opts.CacheTTL != 0 {
		return opts.CacheTTL
	}
	for _, key := range []string{"PF_CACHE_TTL", "PF_INCLUDE_CACHE_TTL"} {
		if v := os.Getenv(key); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				return d
			}
		}
	}
	return defaultIncludeTTL
}

// computeExpiresAt derives the expiry from response headers, floored at fallback — an origin may raise the TTL, never lower it.
func computeExpiresAt(fetchedAt time.Time, h http.Header, fallback time.Duration) time.Time {
	cc := h.Get("Cache-Control")
	lower := strings.ToLower(cc)
	// no-store is the sole opt-out — no-cache, must-revalidate and private are freshness hints and get floored
	if strings.Contains(lower, "no-store") {
		return fetchedAt
	}
	if fallback < 0 {
		return time.Time{}
	}
	if fallback == 0 {
		fallback = defaultIncludeTTL
	}
	if cc != "" {
		for _, part := range strings.Split(cc, ",") {
			p := strings.TrimSpace(strings.ToLower(part))
			if strings.HasPrefix(p, "max-age=") {
				v := strings.TrimPrefix(p, "max-age=")
				if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs >= 0 {
					if ttl := time.Duration(secs) * time.Second; ttl > fallback {
						return fetchedAt.Add(ttl)
					}
					return fetchedAt.Add(fallback)
				}
			}
		}
	}
	if exp := h.Get("Expires"); exp != "" {
		if t, err := http.ParseTime(exp); err == nil && t.After(fetchedAt) {
			if floor := fetchedAt.Add(fallback); t.Before(floor) {
				return floor
			}
			return t
		}
	}
	return fetchedAt.Add(fallback)
}

// loadCacheMeta reads the sidecar meta for ref, if present.
func loadCacheMeta(ref string) (*includeCacheMeta, error) {
	mp, err := includeCacheMetaPath(ref)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(mp) // #nosec G304 -- path derived from XDG + SHA-256 hash
	if err != nil {
		return nil, err
	}
	var m includeCacheMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// saveCacheMeta writes meta atomically beside the cached content.
func saveCacheMeta(ref string, m *includeCacheMeta) {
	mp, err := includeCacheMetaPath(ref)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(mp), 0o755) // #nosec G301 -- cache dir under XDG
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	tmp := mp + ".tmp"
	_ = os.WriteFile(tmp, b, 0o644) // #nosec G306 -- cache metadata, world-readable
	_ = os.Rename(tmp, mp)
}

// cacheFresh reports whether the cached entry is still fresh.
func cacheFresh(m *includeCacheMeta, now time.Time) bool {
	if m == nil || m.ExpiresAt.IsZero() {
		return false
	}
	return now.Before(m.ExpiresAt)
}

// fetchInclude retrieves a single include reference. HTTP(S) URLs go through
// the 3-tier resolver (cache → network → cache); everything else is a local
// path relative to baseDir. When offline, HTTP includes use cache only and
// are skipped with a warning when not cached.
func fetchInclude(ref, baseDir string, opts ReadOptions) (data []byte, pathHint string, err error) {
	if isHTTPInclude(ref) {
		return fetchHTTPInclude(ref, opts)
	}
	return fetchLocalInclude(ref, baseDir, opts.FailOn)
}

// sharedHTTPClient serves every include fetch so connections pool across includes.
var sharedHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		if req.URL.Host != via[0].URL.Host {
			return fmt.Errorf("cross-host redirect %s -> %s (likely an auth gateway; make the repo public or fix the URL)",
				via[0].URL.Host, req.URL.Host)
		}
		return nil
	},
}

func fetchHTTPInclude(ref string, opts ReadOptions) ([]byte, string, error) {
	cp, _ := includeCachePath(ref)
	mp, _ := includeCacheMetaPath(ref)
	var cached []byte
	var meta *includeCacheMeta
	// Tier 1: XDG cache (with freshness check).
	if cp != "" {
		if b, err := os.ReadFile(cp); err == nil { // #nosec G304 -- path derived from XDG + SHA-256 hash
			if looksLikeHTML(b) {
				genlog.Warn("include cache entry looks like HTML; discarding and refetching", "url", ref)
				_ = os.Remove(cp)
				if mp != "" {
					_ = os.Remove(mp)
				}
			} else {
				cached = b
				if m, err := loadCacheMeta(ref); err == nil {
					meta = m
				} else if fi, err := os.Stat(cp); err == nil {
					fetchedAt := fi.ModTime()
					meta = &includeCacheMeta{URL: ref, FetchedAt: fetchedAt, ExpiresAt: fetchedAt.Add(effectiveIncludeTTL(opts))}
				}
				if !opts.ForceRefresh && cacheFresh(meta, time.Now()) {
					genlog.Debug("include loaded from cache", "url", ref, "bytes", len(cached), "fresh", true, "expires_at", meta.ExpiresAt.Format(time.RFC3339))
					ext := filepath.Ext(cp)
					return cached, "include" + ext, nil
				}
				if opts.Offline {
					genlog.Debug("include loaded from cache (offline, stale tolerated)", "url", ref, "bytes", len(cached), "stale", !cacheFresh(meta, time.Now()))
					ext := filepath.Ext(cp)
					return cached, "include" + ext, nil
				}
				// Fall through to conditional revalidation.
			}
		} else if opts.Offline {
			genlog.Warn("include skipped (offline, not cached)", "url", ref)
			return nil, "", nil
		}
	} else if opts.Offline {
		genlog.Warn("include skipped (offline, not cached)", "url", ref)
		return nil, "", nil
	}
	if opts.Offline {
		genlog.Warn("include skipped (offline, not cached)", "url", ref)
		return nil, "", nil
	}
	// Network fetch (conditional when we have validators).
	u, err := url.Parse(ref)
	if err != nil {
		return nil, "", fmt.Errorf("invalid URL: %w", err)
	}
	ext := filepath.Ext(u.Path)
	if ext == "" {
		return nil, "", fmt.Errorf("URL must carry a file extension (.yaml, .toml, or .json) for format detection")
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ref, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "projectfile")
	if meta != nil {
		if meta.ETag != "" {
			req.Header.Set("If-None-Match", meta.ETag)
		}
		if meta.LastModified != "" {
			req.Header.Set("If-Modified-Since", meta.LastModified)
		}
	}
	resp, err := sharedHTTPClient.Do(req) // #nosec G107 -- user-authored include URL
	if err != nil {
		if cached != nil {
			genlog.Warn("include fetch failed, serving stale cache", "url", ref, "err", err.Error())
			ext := filepath.Ext(cp)
			return cached, "include" + ext, nil
		}
		return nil, "", fmt.Errorf("fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotModified && cached != nil {
		now := time.Now()
		newExpires := computeExpiresAt(now, resp.Header, effectiveIncludeTTL(opts))
		updated := &includeCacheMeta{
			URL:          ref,
			ETag:         resp.Header.Get("ETag"),
			LastModified: resp.Header.Get("Last-Modified"),
			CacheControl: resp.Header.Get("Cache-Control"),
			Expires:      resp.Header.Get("Expires"),
			FetchedAt:    now,
			ExpiresAt:    newExpires,
		}
		if updated.ETag == "" && meta != nil {
			updated.ETag = meta.ETag
		}
		if updated.LastModified == "" && meta != nil {
			updated.LastModified = meta.LastModified
		}
		if updated.CacheControl == "" && meta != nil {
			updated.CacheControl = meta.CacheControl
		}
		if updated.Expires == "" && meta != nil {
			updated.Expires = meta.Expires
		}
		if updated.ExpiresAt.IsZero() && meta != nil {
			updated.ExpiresAt = meta.ExpiresAt
		}
		saveCacheMeta(ref, updated)
		genlog.Debug("include not modified, cache revalidated", "url", ref, "expires_at", newExpires.Format(time.RFC3339))
		ext := filepath.Ext(cp)
		return cached, "include" + ext, nil
	}
	if resp.StatusCode/100 != 2 {
		if cached != nil && resp.StatusCode/100 == 5 {
			genlog.Warn("include server error, serving stale cache", "url", ref, "status", resp.StatusCode)
			ext := filepath.Ext(cp)
			return cached, "include" + ext, nil
		}
		return nil, "", fmt.Errorf("HTTP %d%s for %s", resp.StatusCode, statusHint(resp.StatusCode), ref)
	}
	if isHTMLResponse(resp) {
		return nil, "", fmt.Errorf("received HTML from %s (HTTP %d) — likely an auth or login page; make the repo public or fix the URL", ref, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxIncludeBodyBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read response: %w", err)
	}
	if len(data) > maxIncludeBodyBytes {
		return nil, "", fmt.Errorf("response exceeds %d bytes", maxIncludeBodyBytes)
	}
	// Handle redirect cache semantics: 301/308 would have been followed;
	// log final URL when it differs for visibility.
	if resp.Request != nil && resp.Request.URL.String() != ref {
		genlog.Debug("include redirected", "from", ref, "to", resp.Request.URL.String(), "status", resp.StatusCode)
	}
	genlog.Debug("include fetched", "url", ref, "bytes", len(data))
	if cp != "" {
		_ = os.MkdirAll(filepath.Dir(cp), 0o755) // #nosec G301 -- cache dir under XDG
		_ = os.WriteFile(cp, data, 0o644)        // #nosec G306 -- include text, world-readable by intent
		now := time.Now()
		m := &includeCacheMeta{
			URL:          ref,
			ETag:         resp.Header.Get("ETag"),
			LastModified: resp.Header.Get("Last-Modified"),
			CacheControl: resp.Header.Get("Cache-Control"),
			Expires:      resp.Header.Get("Expires"),
			FetchedAt:    now,
			ExpiresAt:    computeExpiresAt(now, resp.Header, effectiveIncludeTTL(opts)),
		}
		saveCacheMeta(ref, m)
	}
	return data, "include" + ext, nil
}

func fetchLocalInclude(ref, baseDir string, failOn IncludeFailLevel) ([]byte, string, error) {
	abs, err := sanitizePath(baseDir, ref)
	if err != nil {
		return nil, "", fmt.Errorf("resolve path: %w", err)
	}
	data, err := os.ReadFile(abs) // #nosec G304 -- user-authored include path
	if err != nil {
		// A missing local include is a soft failure: the file may be
		// transiently absent (e.g. an m6e include being fixed in parallel).
		// Under FailOnError (default) we warn and skip so partial data still
		// resolves; FailOnWarning restores hard-fail strictness. Any other
		// read error (permission denied, ...) stays a hard failure.
		if os.IsNotExist(err) && failOn == FailOnError {
			genlog.Warn("include skipped (not found)", "path", abs)
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("read %s: %w", abs, err)
	}
	genlog.Debug("include loaded", "path", abs, "bytes", len(data))
	return data, abs, nil
}

// WarmInclude forces a network fetch for a single HTTP include URL and writes
// the result to the XDG cache. Returns an error if the URL is not HTTP or the
// fetch fails. Used by the cache warm command.
func WarmInclude(ref string) error {
	if !isHTTPInclude(ref) {
		return fmt.Errorf("not an HTTP include: %s", ref)
	}
	_, _, err := fetchHTTPInclude(ref, ReadOptions{})
	return err
}

// WarmIncludeWithOptions is like WarmInclude but honors the caller's cache opts.
func WarmIncludeWithOptions(ref string, opts ReadOptions) error {
	if !isHTTPInclude(ref) {
		return fmt.Errorf("not an HTTP include: %s", ref)
	}
	_, _, err := fetchHTTPInclude(ref, opts)
	return err
}

// HTTPIncludes extracts all HTTP(S) include URLs from the raw projectfile data.
// Used by the cache warm command to know what to prefetch.
func HTTPIncludes(raw map[string]any) []string {
	var out []string
	for _, ref := range rawLookupIncludes(raw) {
		if isHTTPInclude(ref) {
			out = append(out, ref)
		}
	}
	return out
}

// XDGCacheDir returns the base XDG cache directory (${XDG_CACHE_HOME:-~/.cache}).
func XDGCacheDir() (string, error) {
	return xdgCacheDir()
}

// IncludesCacheDir returns the shared includes cache directory
// ($XDG_CACHE_HOME/pf/includes). The `cache status`/`cache purge` commands use
// it to report and clear the real on-disk path.
func IncludesCacheDir() (string, error) {
	return includesCacheDir()
}

// IncludesCacheStatus summarises the on-disk includes cache for status output.
// Each entry pairs the include cache file with its sidecar meta (when present);
// legacy entries without a sidecar are counted as fresh=false (treated as
// infinitely stale per the migration contract). OldestAge is the maximum age
// across fresh=false entries; zero when none.
type IncludesCacheStatus struct {
	Total     int
	Fresh     int
	Stale     int
	OldestAge time.Duration
}

// IncludesCacheStatus reports counts of cached HTTP includes broken down by
// freshness, plus the age of the oldest stale entry. Sidecar metadata is the
// source of truth for freshness; a missing sidecar (legacy entry) is treated
// as stale, matching the migration contract.
func IncludesCacheStatusSummary() (IncludesCacheStatus, error) {
	dir, err := includesCacheDir()
	if err != nil {
		return IncludesCacheStatus{}, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return IncludesCacheStatus{}, nil
		}
		return IncludesCacheStatus{}, err
	}
	var s IncludesCacheStatus
	now := time.Now()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".meta.json") || strings.HasSuffix(name, ".tmp") {
			continue
		}
		s.Total++
		full := filepath.Join(dir, name)
		var fresh bool
		var age time.Duration
		mb, mErr := os.ReadFile(full + ".meta.json") // #nosec G304 -- sidecar beside cache file
		if mErr == nil {
			var m includeCacheMeta
			if json.Unmarshal(mb, &m) == nil {
				fresh = cacheFresh(&m, now)
				age = now.Sub(m.FetchedAt)
			}
		} else {
			fi, sErr := os.Stat(full)
			if sErr == nil {
				age = now.Sub(fi.ModTime())
			}
		}
		if fresh {
			s.Fresh++
			continue
		}
		s.Stale++
		if age > s.OldestAge {
			s.OldestAge = age
		}
	}
	return s, nil
}

// PurgeIncludes removes every cached HTTP include from the shared slot.
// Best-effort: a missing dir is a no-op (success). Returns the count of files
// removed so the caller can report it. The directory is recreated empty so a
// subsequent warm has a target.
func PurgeIncludes() (removed int, err error) {
	dir, err := includesCacheDir()
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil // nothing cached — a clean state, not an error
		}
		return 0, err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".meta.json") || strings.HasSuffix(name, ".tmp") {
			_ = os.Remove(filepath.Join(dir, name))
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			continue
		}
		removed++
		_ = os.Remove(filepath.Join(dir, name+".meta.json"))
	}
	return removed, nil
}

// PurgeInclude removes a single cached URL, if present. Returns true when removed.
func PurgeInclude(ref string) (bool, error) {
	cp, err := includeCachePath(ref)
	if err != nil {
		return false, err
	}
	mp, _ := includeCacheMetaPath(ref)
	removed := false
	if err := os.Remove(cp); err == nil {
		removed = true
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if mp != "" {
		_ = os.Remove(mp)
	}
	return removed, nil
}

// resolveIncludes applies the includes list: root-level `includes` is checked
// first (spec §4.9a); when absent, the legacy `org.projectfile.cli.includes`
// extension namespace is consulted for backward compatibility. Each entry is
// fetched and parsed, then merged in order (later entries win over earlier),
// and finally the base document is overlaid so its values always win.
//
// Includes are resolved RECURSIVELY: an include document MAY itself declare
// `includes` (spec §4.9a), which are resolved relative to that include's own
// location before the include is merged. selfPath seeds cycle detection so a
// direct self-include is caught; it may be empty when the caller has no path
// identity. Returns raw unchanged when no includes are declared.
func resolveIncludes(raw map[string]any, baseDir, selfPath string, opts ReadOptions) (map[string]any, error) {
	ancestors := make(map[string]struct{})
	if selfPath != "" {
		if abs, err := filepath.Abs(selfPath); err == nil {
			ancestors[includeCycleKey(selfPath, filepath.Clean(abs))] = struct{}{}
		}
	}
	chain, err := resolveIncludesChain(raw, baseDir, opts, ancestors, false)
	if err != nil {
		return nil, err
	}
	if len(chain) == 0 {
		return raw, nil
	}
	// Base document wins over all includes.
	return deepMerge(raw, chain), nil
}

// resolveIncludesChain fetches every include declared in raw, fully resolves
// each one (the include overlaid on ITS own transitive include chain — the
// include itself wins over its includes, base-wins semantics applied per
// level), and accumulates them with deepMerge (later entries win over
// earlier). raw is NOT overlaid here: resolveIncludes overlays it for the
// read path; ResolveIncludesOnly returns chain verbatim for the optimize
// path.
//
// ancestors is the set of document identity keys on the CURRENT resolution
// path (a stack, not a visited set): a key is added on descent and removed
// on return. This lets a diamond (the same document reached via two
// branches) resolve on each branch while a true cycle (a document on its
// own ancestor path) is rejected.
// maxIncludeFetchConcurrency caps concurrent include fetches per level.
const maxIncludeFetchConcurrency = 8

type fetchedInclude struct {
	data     []byte
	hint     string
	doc      map[string]any
	fetchErr error
	parseErr error
}

// fetchIncludesConcurrently fetches and parses every ref in order, returning one
// result per index so the caller merges and reports errors in declared order.
func fetchIncludesConcurrently(refs []string, baseDir string, opts ReadOptions) []fetchedInclude {
	results := make([]fetchedInclude, len(refs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxIncludeFetchConcurrency)
	for i, ref := range refs {
		wg.Add(1)
		go func(i int, ref string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			data, hint, ferr := fetchInclude(ref, baseDir, opts)
			if ferr != nil {
				results[i].fetchErr = ferr
				return
			}
			if data == nil {
				return
			}
			doc, perr := ReadRawFromBytes(hint, data)
			if perr != nil {
				results[i].parseErr = perr
				return
			}
			results[i].data = data
			results[i].hint = hint
			results[i].doc = doc
		}(i, ref)
	}
	wg.Wait()
	return results
}

func resolveIncludesChain(raw map[string]any, baseDir string, opts ReadOptions, ancestors map[string]struct{}, parentHTTP bool) (map[string]any, error) {
	includes := rawLookupIncludes(raw)
	if len(includes) == 0 {
		return map[string]any{}, nil
	}
	genlog.Debug("resolving includes", "count", len(includes), "offline", opts.Offline)
	fetchRefs := make([]string, 0, len(includes))
	for _, ref := range includes {
		if parentHTTP && !isHTTPInclude(ref) {
			genlog.Warn("include skipped (local nested under HTTP document)", "ref", ref)
			continue
		}
		fetchRefs = append(fetchRefs, ref)
	}
	results := fetchIncludesConcurrently(fetchRefs, baseDir, opts)
	acc := map[string]any{}
	for j, ref := range fetchRefs {
		r := results[j]
		if r.fetchErr != nil {
			return nil, fmt.Errorf("include %q: %w", ref, r.fetchErr)
		}
		// Skipped offline include returns nil data — skip merge.
		if r.data == nil {
			continue
		}
		if r.parseErr != nil {
			return nil, fmt.Errorf("include %q: parse: %w", ref, r.parseErr)
		}
		inc := r.doc
		pathHint := r.hint
		// Cycle detection. Identity is the URL (HTTP) or the absolute resolved
		// path (local, as returned by fetchLocalInclude). ancestors is a path
		// stack, so a diamond (D reached via two branches) is NOT flagged — D
		// was removed from the stack when the first branch returned.
		key := includeCycleKey(ref, pathHint)
		if _, onPath := ancestors[key]; onPath {
			return nil, fmt.Errorf("include %q: cycle detected — document includes itself transitively (spec §4.9a)", ref)
		}
		// Nested includes resolve against the including document's directory; an HTTP document resolves HTTP nested refs only.
		nestedBaseDir := baseDir
		if !isHTTPInclude(ref) && filepath.IsAbs(pathHint) {
			nestedBaseDir = filepath.Dir(pathHint)
		}
		// Fully resolve the include: overlay inc on top of inc's own chain so
		// the include wins over its includes (base-wins semantics, per level).
		ancestors[key] = struct{}{}
		incChain, err := resolveIncludesChain(inc, nestedBaseDir, opts, ancestors, isHTTPInclude(ref))
		delete(ancestors, key)
		if err != nil {
			return nil, err
		}
		resolved := inc
		if len(incChain) > 0 {
			resolved = deepMerge(inc, incChain)
		}
		// Later includes win over earlier ones.
		acc = deepMerge(resolved, acc)
	}
	return acc, nil
}

// includeCycleKey returns the stable identity of an include for cycle
// detection: the URL for HTTP includes, the absolute resolved path for local
// includes (pathHint from fetchLocalInclude is already cleaned+abs). The
// prefix namespaces the two spaces so a path that happens to share a textual
// form with a URL (or vice-versa) cannot collide.
func includeCycleKey(ref, pathHint string) string {
	if isHTTPInclude(ref) {
		return "url:" + ref
	}
	if filepath.IsAbs(pathHint) {
		return "path:" + filepath.Clean(pathHint)
	}
	return "ref:" + ref
}

// AllHTTPIncludes returns every HTTP(S) include URL reachable from raw,
// walking the include chain transitively — each include's own includes are
// fetched and inspected, so HTTP includes hidden behind a local fragment are
// discovered too. Relative paths inside an included document resolve against
// that document's location, matching resolveIncludes. Discovery is
// best-effort: a fetch or parse failure on one branch is skipped, not fatal.
// selfPath seeds cycle detection (same identity resolveIncludes uses). URLs
// are returned in discovery order with duplicates removed. Used by the
// cache-warm command so a deep HTTP include chain is prefetched end-to-end.
func AllHTTPIncludes(raw map[string]any, baseDir, selfPath string, opts ReadOptions) []string {
	ancestors := make(map[string]struct{})
	if selfPath != "" {
		if abs, err := filepath.Abs(selfPath); err == nil {
			ancestors[includeCycleKey(selfPath, filepath.Clean(abs))] = struct{}{}
		}
	}
	var out []string
	emitted := make(map[string]struct{})
	var walk func(m map[string]any, dir string, parentHTTP bool)
	walk = func(m map[string]any, dir string, parentHTTP bool) {
		for _, ref := range rawLookupIncludes(m) {
			httpRef := isHTTPInclude(ref)
			if parentHTTP && !httpRef {
				continue
			}
			data, pathHint, err := fetchInclude(ref, dir, opts)
			if err != nil || data == nil {
				continue
			}
			key := includeCycleKey(ref, pathHint)
			if _, onPath := ancestors[key]; onPath {
				continue
			}
			if httpRef {
				if _, dup := emitted[ref]; !dup {
					emitted[ref] = struct{}{}
					out = append(out, ref)
				}
			}
			nested, perr := ReadRawFromBytes(pathHint, data)
			if perr != nil {
				continue
			}
			nestedDir := dir
			if !httpRef && filepath.IsAbs(pathHint) {
				nestedDir = filepath.Dir(pathHint)
			}
			ancestors[key] = struct{}{}
			walk(nested, nestedDir, httpRef)
			delete(ancestors, key)
		}
	}
	walk(raw, baseDir, false)
	return out
}

// Redundancy reasons for a RedundantInclude (see RedundantIncludes).
const (
	// redundantTransitive: another sibling entry pulls this one in through its
	// own include chain, so listing it adds nothing.
	redundantTransitive = "transitive"
	// redundantDuplicate: the same target is listed more than once at this level.
	redundantDuplicate = "duplicate"
)

// RedundantInclude flags a direct include entry that is already provided by a
// sibling entry, so listing it changes nothing: deepMerge deduplicates and the
// resolver reaches the same document either way. Ref is the entry as written;
// Via is the sibling that already provides it (its transitive closure contains
// Ref, or — for Reason redundantDuplicate — it is the earlier identical entry).
type RedundantInclude struct {
	Ref    string
	Via    string
	Reason string
}

// RedundantIncludes reports the direct include entries in raw that a sibling
// entry already provides — either transitively (a sibling's include chain
// pulls it in) or verbatim (the same target listed twice). baseDir is the
// directory raw's relative includes resolve against; selfPath seeds cycle
// detection (may be empty); opts matches the resolver.
//
// This is the include-list twin of StripRedundant (which strips redundant
// FIELD VALUES): it needs the BASE document — passing a merged one would see
// the deduped union of every child's includes and find nothing. The walk is
// READ-ONLY and best-effort: an include that cannot be fetched or parsed
// (offline-uncached, transiently absent, malformed) contributes nothing and
// never aborts the check, matching the resolver's partial-data tolerance.
func RedundantIncludes(raw map[string]any, baseDir, selfPath string, opts ReadOptions) []RedundantInclude {
	refs := rawLookupIncludes(raw)
	if len(refs) < 2 {
		// A single entry has no sibling to be redundant against; a lone
		// self-include is a cycle caught by the resolver, not a redundancy.
		return nil
	}
	genlog.Debug("checking include redundancy", "entries", len(refs), "baseDir", baseDir)

	// Identity of each direct entry (URL for HTTP, absolute path for local).
	// fetchInclude yields an empty pathHint on any failure, so the key falls
	// back to the textual ref — enough to still catch verbatim duplicates.
	ids := make([]string, len(refs))
	for i, ref := range refs {
		_, pathHint, _ := fetchInclude(ref, baseDir, opts)
		ids[i] = includeCycleKey(ref, pathHint)
	}

	reported := make(map[int]struct{})
	var out []RedundantInclude

	// Pass 1: verbatim duplicates — the same target listed more than once.
	firstSeen := make(map[string]int, len(refs))
	for i, ref := range refs {
		if j, dup := firstSeen[ids[i]]; dup {
			genlog.Debug("redundant include (duplicate)", "ref", ref, "via", refs[j])
			out = append(out, RedundantInclude{Ref: ref, Via: refs[j], Reason: redundantDuplicate})
			reported[i] = struct{}{}
			continue
		}
		firstSeen[ids[i]] = i
	}

	// Pass 2: transitive redundancy — entry i is a proper descendant of the
	// closure sibling j already pulls in. Each sibling's closure is walked once.
	for j, sib := range refs {
		ancestors := seedIncludeAncestors(selfPath)
		closure := make(map[string]struct{})
		collectReachable([]string{sib}, baseDir, opts, ancestors, closure, false)
		for i, ref := range refs {
			if i == j {
				continue
			}
			if _, done := reported[i]; done {
				continue
			}
			if ids[i] == ids[j] {
				continue // verbatim dup — handled in pass 1
			}
			if _, ok := closure[ids[i]]; ok {
				genlog.Debug("redundant include (transitive)", "ref", ref, "via", sib)
				out = append(out, RedundantInclude{Ref: ref, Via: sib, Reason: redundantTransitive})
				reported[i] = struct{}{}
			}
		}
	}
	return out
}

// seedIncludeAncestors returns a fresh ancestor set seeded with selfPath's
// identity so a sibling that transitively re-includes the root document is
// treated as a cycle rather than walked forever — mirroring resolveIncludes'
// seeding. Empty selfPath yields an empty set.
func seedIncludeAncestors(selfPath string) map[string]struct{} {
	ancestors := make(map[string]struct{})
	if selfPath != "" {
		if abs, err := filepath.Abs(selfPath); err == nil {
			ancestors[includeCycleKey(selfPath, filepath.Clean(abs))] = struct{}{}
		}
	}
	return ancestors
}

// collectReachable resolves each ref at this level and records the include
// identity of every document reachable from them — themselves and, recursively,
// their own includes — into out. dir is the directory this level's relative
// refs resolve against. ancestors is the path stack that stops a cycle from
// looping (added on descent, removed on return); a diamond still records once
// because out is a set. Best-effort: an unresolvable branch is skipped.
func collectReachable(refs []string, dir string, opts ReadOptions, ancestors, out map[string]struct{}, parentHTTP bool) {
	for _, ref := range refs {
		if parentHTTP && !isHTTPInclude(ref) {
			continue
		}
		data, pathHint, err := fetchInclude(ref, dir, opts)
		if err != nil || data == nil {
			continue
		}
		key := includeCycleKey(ref, pathHint)
		out[key] = struct{}{}
		if _, onPath := ancestors[key]; onPath {
			continue // cycle guard — do not recurse through an ancestor
		}
		doc, perr := ReadRawFromBytes(pathHint, data)
		if perr != nil {
			continue
		}
		nestedDir := dir
		if !isHTTPInclude(ref) && filepath.IsAbs(pathHint) {
			nestedDir = filepath.Dir(pathHint)
		}
		ancestors[key] = struct{}{}
		collectReachable(rawLookupIncludes(doc), nestedDir, opts, ancestors, out, isHTTPInclude(ref))
		delete(ancestors, key)
	}
}
