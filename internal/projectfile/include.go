// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"kiota.ch/projectfile/core/internal/genlog"
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
func includeCachePath(ref string) (string, error) {
	base, err := xdgCacheDir()
	if err != nil {
		return "", err
	}
	u, err := url.Parse(ref)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	ext := filepath.Ext(u.Path)
	h := sha256.Sum256([]byte(ref))
	return filepath.Join(base, "projectfile-cli", "includes", fmt.Sprintf("%x%s", h, ext)), nil
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

// fetchInclude retrieves a single include reference. HTTP(S) URLs go through
// the 3-tier resolver (cache → network → cache); everything else is a local
// path relative to baseDir. When offline, HTTP includes use cache only and
// are skipped with a warning when not cached.
func fetchInclude(ref, baseDir string, opts ReadOptions) (data []byte, pathHint string, err error) {
	if isHTTPInclude(ref) {
		return fetchHTTPInclude(ref, opts.Offline)
	}
	return fetchLocalInclude(ref, baseDir, opts.FailOn)
}

func fetchHTTPInclude(ref string, offline bool) ([]byte, string, error) {
	cp, _ := includeCachePath(ref)
	// Tier 1: XDG cache.
	if cp != "" {
		if cached, err := os.ReadFile(cp); err == nil { // #nosec G304 -- path derived from XDG + SHA-256 hash
			// Self-heal: an older pf-cli build may have cached an auth/login
			// HTML page before content-type validation existed. Drop it and
			// fall through to a fresh fetch instead of serving poison.
			if looksLikeHTML(cached) {
				genlog.Warn("include cache entry looks like HTML; discarding and refetching", "url", ref)
				_ = os.Remove(cp)
			} else {
				genlog.Info("include loaded from cache", "url", ref, "bytes", len(cached))
				ext := filepath.Ext(cp)
				return cached, "include" + ext, nil
			}
		}
	}
	if offline {
		genlog.Warn("include skipped (offline, not cached)", "url", ref)
		return nil, "", nil
	}
	// Tier 2: network fetch.
	u, err := url.Parse(ref)
	if err != nil {
		return nil, "", fmt.Errorf("invalid URL: %w", err)
	}
	ext := filepath.Ext(u.Path)
	if ext == "" {
		return nil, "", fmt.Errorf("URL must carry a file extension (.yaml, .toml, or .json) for format detection")
	}
	// Refuse cross-host redirects: a forge should never bounce raw content
	// to a different host. When it does, it's almost always an SSO / auth
	// gateway; following it would land us on an HTML login page that we'd
	// then try (and fail) to parse as YAML — masking the real cause.
	client := &http.Client{
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
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ref, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "projectfile")
	resp, err := client.Do(req) // #nosec G107 -- user-authored include URL
	if err != nil {
		return nil, "", fmt.Errorf("fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return nil, "", fmt.Errorf("HTTP %d%s for %s", resp.StatusCode, statusHint(resp.StatusCode), ref)
	}
	// An include document is never HTML. A 200 OK with an HTML body is the
	// signature of an auth / login wall reached after a same-host redirect
	// (Forgejo/Gitea redirect private-repo raw URLs to /user/login), which
	// the host check above can't catch.
	if isHTMLResponse(resp) {
		return nil, "", fmt.Errorf("received HTML from %s (HTTP %d) — likely an auth or login page; make the repo public or fix the URL", ref, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read response: %w", err)
	}
	genlog.Info("include fetched", "url", ref, "bytes", len(data))
	// Write to cache (best-effort).
	if cp != "" {
		_ = os.MkdirAll(filepath.Dir(cp), 0o755) // #nosec G301 -- cache dir under XDG
		_ = os.WriteFile(cp, data, 0o644)        // #nosec G306 -- include text, world-readable by intent
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
	genlog.Info("include loaded", "path", abs, "bytes", len(data))
	return data, abs, nil
}

// WarmInclude forces a network fetch for a single HTTP include URL and writes
// the result to the XDG cache. Returns an error if the URL is not HTTP or the
// fetch fails. Used by the cache warm command.
func WarmInclude(ref string) error {
	if !isHTTPInclude(ref) {
		return fmt.Errorf("not an HTTP include: %s", ref)
	}
	_, _, err := fetchHTTPInclude(ref, false)
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

// XDGCacheDir returns the base XDG cache directory for pf-cli.
func XDGCacheDir() (string, error) {
	return xdgCacheDir()
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
	chain, err := resolveIncludesChain(raw, baseDir, opts, ancestors)
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
func resolveIncludesChain(raw map[string]any, baseDir string, opts ReadOptions, ancestors map[string]struct{}) (map[string]any, error) {
	includes := rawLookupIncludes(raw)
	if len(includes) == 0 {
		return map[string]any{}, nil
	}
	genlog.Info("resolving includes", "count", len(includes), "offline", opts.Offline)
	acc := map[string]any{}
	for _, ref := range includes {
		data, pathHint, err := fetchInclude(ref, baseDir, opts)
		if err != nil {
			return nil, fmt.Errorf("include %q: %w", ref, err)
		}
		// Skipped offline include returns nil data — skip merge.
		if data == nil {
			continue
		}
		inc, err := ReadRawFromBytes(pathHint, data)
		if err != nil {
			return nil, fmt.Errorf("include %q: parse: %w", ref, err)
		}
		// Cycle detection. Identity is the URL (HTTP) or the absolute resolved
		// path (local, as returned by fetchLocalInclude). ancestors is a path
		// stack, so a diamond (D reached via two branches) is NOT flagged — D
		// was removed from the stack when the first branch returned.
		key := includeCycleKey(ref, pathHint)
		if _, onPath := ancestors[key]; onPath {
			return nil, fmt.Errorf("include %q: cycle detected — document includes itself transitively (spec §4.9a)", ref)
		}
		// Nested includes resolve relative to THIS include's location. A local
		// include contributes the directory of the file just read; an HTTP
		// include has no on-disk directory, so nested local paths fall back to
		// the parent baseDir (best-effort) while nested HTTP includes resolve
		// normally. This matches spec §4.9a: relative paths resolve against the
		// directory containing the including document.
		nestedBaseDir := baseDir
		if !isHTTPInclude(ref) && filepath.IsAbs(pathHint) {
			nestedBaseDir = filepath.Dir(pathHint)
		}
		// Fully resolve the include: overlay inc on top of inc's own chain so
		// the include wins over its includes (base-wins semantics, per level).
		ancestors[key] = struct{}{}
		incChain, err := resolveIncludesChain(inc, nestedBaseDir, opts, ancestors)
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
	var walk func(m map[string]any, dir string)
	walk = func(m map[string]any, dir string) {
		for _, ref := range rawLookupIncludes(m) {
			httpRef := isHTTPInclude(ref)
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
			walk(nested, nestedDir)
			delete(ancestors, key)
		}
	}
	walk(raw, baseDir)
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
	genlog.Info("checking include redundancy", "entries", len(refs), "baseDir", baseDir)

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
			genlog.Info("redundant include (duplicate)", "ref", ref, "via", refs[j])
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
		collectReachable([]string{sib}, baseDir, opts, ancestors, closure)
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
				genlog.Info("redundant include (transitive)", "ref", ref, "via", sib)
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
func collectReachable(refs []string, dir string, opts ReadOptions, ancestors, out map[string]struct{}) {
	for _, ref := range refs {
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
		collectReachable(rawLookupIncludes(doc), nestedDir, opts, ancestors, out)
		delete(ancestors, key)
	}
}
