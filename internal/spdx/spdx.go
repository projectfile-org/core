// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package spdx resolves SPDX license boilerplate. Lookup order is
// embedded set → XDG_CACHE_HOME → upstream raw text → cache. Substitute
// fills the common placeholders ([year], [fullname], ...) so the rendered
// body is ready to drop into a LICENSE file.
//
// The embedded set is DATA the consumer registers with SetEmbedded, not an asset
// core carries: core is a library, and a licence corpus is a build artifact of
// whoever renders LICENSE files. A consumer that registers nothing simply starts
// at the cache tier. Exotic ids fall through to the network on first use.
package spdx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	mrand "math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kiota.ch/projectfile/core/v2/internal/genlog"
)

// embedded holds the caller-supplied boilerplate corpus — tier 1 of the lookup.
// Core owns the ALGORITHM, not the DATA: a licence corpus is a build artifact of
// whoever renders LICENSE files, and core is a consumed library whose consumers
// compile it from the immutable module cache, where no build step could ever fetch
// one. Registering it instead of embedding it keeps ~2000 lines of third-party text
// out of a repository that never reads them, and lets each consumer ship exactly
// the ids it needs (or none — the tier is then simply skipped).
//
// Guarded because a long-lived process may register on one goroutine and resolve on
// another; registration is expected once at wiring time, so an RWMutex costs nothing
// on the read path that matters.
var (
	embeddedMu sync.RWMutex
	embeddedFS fs.FS
)

// SetEmbedded registers the embedded boilerplate corpus. fsys is read at
// `<id>.txt` relative to its root. Pass nil to clear (tier 1 is then skipped and
// lookup starts at the XDG cache) — that is the graceful default for a consumer
// like the CLI, which introspects the cache but renders no LICENSE text.
func SetEmbedded(fsys fs.FS) {
	embeddedMu.Lock()
	defer embeddedMu.Unlock()
	embeddedFS = fsys
	genlog.Info("spdx embedded corpus registered", "present", fsys != nil)
}

// Error sentinels — callers check with errors.Is.
var (
	// ErrOffline: the id is not in the embedded set, not in the XDG cache,
	// and the caller passed Options.Offline so no network fetch was attempted.
	ErrOffline = errors.New("spdx: id not available offline")
	// ErrCompound: caller passed a compound expression (X OR Y / X AND Y / X WITH Z)
	// to a single-term API. Split with SplitCompound before calling.
	ErrCompound = errors.New("spdx: compound expression, split before calling Text")
	// ErrUnknown: upstream returned 404 for the id.
	ErrUnknown = errors.New("spdx: id not found upstream")
)

// Vars carries the substitution payload. Holders are joined with ", " when
// the placeholder is the single-name form ([fullname], [name of author]).
type Vars struct {
	Year    int
	Holders []string
}

// Options threads CLI flags into Text. Today only Offline is exposed.
type Options struct {
	Offline bool
}

// IsCompound returns true when id contains OR / AND / WITH as standalone
// tokens, the SPDX spec's compound expression operators.
func IsCompound(id string) bool {
	for _, tok := range strings.Fields(id) {
		switch strings.ToUpper(tok) {
		case "OR", "AND", "WITH":
			return true
		}
	}
	return false
}

// SplitCompound splits a compound SPDX expression on its top-level
// conjunction operator (OR or AND). WITH is NOT a split point — it binds a
// license to its exception clause and has no standalone boilerplate text.
// Returns the constituent SPDX ids and the conjunction that joined them
// ("OR", "AND", or "" for a single id). Callers use the conjunction to
// render the correct legal wording (disjunctive vs conjunctive).
//
// When both OR and AND are present, OR takes precedence (it is the looser
// SPDX operator) so a split on OR is attempted first; surviving AND-clauses
// inside a term are left intact for a recursive call if needed.
func SplitCompound(id string) ([]string, string) {
	id = strings.TrimSpace(id)
	for _, op := range []string{" OR ", " AND "} {
		if !strings.Contains(id, op) {
			continue
		}
		parts := strings.Split(id, op)
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		conj := strings.TrimSpace(op)
		return out, conj
	}
	return []string{id}, ""
}

// StripException removes a trailing "WITH <exception-id>" from a single SPDX
// id. The exception clause has no standalone boilerplate text, so the LICENSE
// bridge calls this before Text to obtain the base license body. A project
// using "GPL-2.0-only WITH Classpath-exception-2.0" gets the GPL-2.0-only
// text; the exception clause is a short rider the project SHOULD append by
// hand (spdx.Substitute does not synthesise exception text).
func StripException(id string) string {
	id = strings.TrimSpace(id)
	upper := strings.ToUpper(id)
	if idx := strings.Index(upper, " WITH "); idx >= 0 {
		return strings.TrimSpace(id[:idx])
	}
	return id
}

// Substitute fills the common SPDX placeholders best-effort. Documented as
// "exotic templates may need post-edits" — pf-cli does not pretend to be a
// general SPDX rendering engine.
func Substitute(text string, vars Vars) string {
	year := ""
	if vars.Year > 0 {
		year = fmt.Sprintf("%d", vars.Year)
	}
	holders := strings.Join(vars.Holders, ", ")
	for _, r := range []struct{ from, to string }{
		{"[year]", year},
		{"<year>", year},
		{"[yyyy]", year},
		{"[fullname]", holders},
		{"[name of copyright owner]", holders},
		{"[name of author]", holders},
		// SPDX's standard MIT placeholders use angle brackets; the plan's
		// list missed these. Adding them keeps `pf-cli generate LICENSE`
		// "best-effort for the common SPDX texts" honest on the single
		// most-used licence.
		{"<copyright holders>", holders},
		{"<copyright holder>", holders},
		{"<owner>", holders},
	} {
		if r.to == "" {
			continue
		}
		text = strings.ReplaceAll(text, r.from, r.to)
	}
	return text
}

// Text resolves a single SPDX id (not a compound expression). Lookup order:
//
//  1. embedded set (no I/O)
//  2. XDG_CACHE_HOME/projectfile-cli/spdx/<id>.txt
//  3. network fetch → cache → return (skipped when opts.Offline is true)
//
// Returns ErrCompound when id is a compound expression — the LICENSE
// generator splits with SplitCompound and calls Text per term.
func Text(id string, opts Options) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errors.New("spdx: empty id")
	}
	if IsCompound(id) {
		return "", fmt.Errorf("%w: %q", ErrCompound, id)
	}
	if b, ok := embeddedText(id); ok {
		genlog.Info("spdx text from embedded set", "id", id)
		return string(b), nil
	}
	cp, cpErr := cachePath(id)
	if cpErr == nil {
		if b, err := os.ReadFile(cp); err == nil { // #nosec G304 -- path derived from XDG + id, no user-controlled traversal vector
			genlog.Info("spdx text from cache", "id", id)
			return string(b), nil
		}
	}
	if opts.Offline {
		return "", fmt.Errorf("%w: %s (not in embedded set or cache)", ErrOffline, id)
	}
	body, err := fetch(id)
	if err != nil {
		return "", err
	}
	if cpErr == nil {
		_ = os.MkdirAll(filepath.Dir(cp), 0o755)  // #nosec G301 -- best-effort cache directory under XDG
		_ = os.WriteFile(cp, []byte(body), 0o644) // #nosec G306 -- license text, world-readable by intent
	}
	return body, nil
}

// EmbeddedIDs returns the sorted list of SPDX ids available from the registered
// corpus. Empty when no consumer called SetEmbedded — an honest report, not a
// failure: that consumer genuinely ships no boilerplate.
func EmbeddedIDs() []string {
	embeddedMu.RLock()
	fsys := embeddedFS
	embeddedMu.RUnlock()
	if fsys == nil {
		return nil
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil
	}
	out := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".txt") {
			continue
		}
		out = append(out, strings.TrimSuffix(name, ".txt"))
	}
	return out
}

// CacheStatus reports how many SPDX texts are available from each tier.
type CacheStatus struct {
	Embedded int
	Cached   int
}

// Status returns the count of embedded and cached SPDX texts.
func Status() CacheStatus {
	embedded := EmbeddedIDs()
	cached := 0
	embSet := make(map[string]bool, len(embedded))
	for _, id := range embedded {
		embSet[id] = true
	}
	cd := cacheDir()
	entries, err := os.ReadDir(cd)
	if err != nil {
		return CacheStatus{Embedded: len(embedded), Cached: 0}
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		cached++
	}
	return CacheStatus{Embedded: len(embedded), Cached: cached}
}

func cacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "pf", "spdx")
}

func cachePath(id string) (string, error) {
	dir := cacheDir()
	if dir == "" {
		return "", errors.New("cannot resolve XDG cache directory")
	}
	return filepath.Join(dir, id+".txt"), nil
}

// CacheDir returns the resolved SPDX cache directory ($XDG_CACHE_HOME/pf/spdx),
// or an error when XDG/home cannot be resolved. Used by the `cache status` and
// `cache purge` commands to report and clear the real on-disk path.
func CacheDir() (string, error) {
	if dir := cacheDir(); dir != "" {
		return dir, nil
	}
	return "", errors.New("cannot resolve XDG cache directory")
}

// Purge removes every cached SPDX text for the current app slot. Best-effort:
// a missing dir is a no-op (success). Returns the count of files removed so the
// caller can report it. The directory itself is recreated empty so a subsequent
// warm has a target.
func Purge() (removed int, err error) {
	dir, err := CacheDir()
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
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			continue
		}
		removed++
	}
	return removed, nil
}

// spdxLicenseList is the subset of the SPDX license-list-data JSON we need.
// IsDeprecatedLicenseID matters for WarmAll: deprecated ids (AGPL-1.0,
// GPL-2.0, …) have NO upstream text/ file, so fetching them 404s. Skipping
// them by this flag is what keeps a warm run WARN-free.
type spdxLicenseList struct {
	Licenses []struct {
		LicenseID             string `json:"licenseId"`
		IsDeprecatedLicenseID bool   `json:"isDeprecatedLicenseId"`
	} `json:"licenses"`
}

// WarmAll fetches the SPDX license list index, then downloads every license
// text that is not already in the embedded set or cache. Returns counts:
// (embedded, alreadyCached, newlyFetched).
func WarmAll() (embeddedCount, cachedCount, fetchedCount int, err error) {
	embeddedIDs := EmbeddedIDs()
	embeddedCount = len(embeddedIDs)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet,
		"https://raw.githubusercontent.com/spdx/license-list-data/main/json/licenses.json", nil)
	if err != nil {
		return embeddedCount, 0, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "projectfile")
	resp, err := client.Do(req)
	if err != nil {
		return embeddedCount, 0, 0, fmt.Errorf("fetch license list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		return embeddedCount, 0, 0, fmt.Errorf("fetch license list: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return embeddedCount, 0, 0, fmt.Errorf("read license list: %w", err)
	}

	var list spdxLicenseList
	if err := json.Unmarshal(body, &list); err != nil {
		return embeddedCount, 0, 0, fmt.Errorf("parse license list: %w", err)
	}

	embSet := make(map[string]bool, len(embeddedIDs))
	for _, id := range embeddedIDs {
		embSet[id] = true
	}
	deprecatedSkipped := 0
	for _, lic := range list.Licenses {
		id := lic.LicenseID
		if id == "" || IsCompound(id) {
			continue
		}
		// Deprecated ids (AGPL-1.0, GPL-2.0, …) are kept in the index for
		// historical lookups but have NO upstream text/ file — fetching one
		// always 404s. Skip them up front rather than emitting a WARN per id.
		if lic.IsDeprecatedLicenseID {
			deprecatedSkipped++
			continue
		}
		if embSet[id] {
			continue
		}
		cp, cpErr := cachePath(id)
		if cpErr != nil {
			continue
		}
		if _, err := os.Stat(cp); err == nil {
			cachedCount++
			continue
		}
		text, ferr := fetch(id)
		if ferr != nil {
			genlog.Warn("spdx warm: skip", "id", id, "err", ferr.Error())
			continue
		}
		_ = os.MkdirAll(filepath.Dir(cp), 0o755)  // #nosec G301
		_ = os.WriteFile(cp, []byte(text), 0o644) // #nosec G306
		fetchedCount++
		genlog.Info("spdx warmed", "id", id)
	}
	if deprecatedSkipped > 0 {
		genlog.Info("spdx warm: skipped deprecated ids (no upstream text)", "count", deprecatedSkipped)
	}
	return embeddedCount, cachedCount, fetchedCount, nil
}

func embeddedText(id string) ([]byte, bool) {
	embeddedMu.RLock()
	fsys := embeddedFS
	embeddedMu.RUnlock()
	if fsys == nil {
		return nil, false
	}
	b, err := fs.ReadFile(fsys, id+".txt")
	if err != nil {
		return nil, false
	}
	return b, true
}

// fetch performs the network lookup with a 10s timeout and a single retry
// using bounded backoff + jitter. Matches AGENTS.md "timeouts, retries with
// backoff+jitter (any network/process crossing)".
func fetch(id string) (string, error) {
	url := "https://raw.githubusercontent.com/spdx/license-list-data/main/text/" + id + ".txt"
	client := &http.Client{Timeout: 10 * time.Second}
	var lastErr error
	for attempt := range 2 {
		if attempt > 0 {
			jitter := time.Duration(mrand.Int63n(int64(250 * time.Millisecond))) // #nosec G404 -- non-crypto retry jitter
			time.Sleep(250*time.Millisecond + jitter)
		}
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "projectfile")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusNotFound {
			_ = resp.Body.Close()
			return "", fmt.Errorf("%w: %s", ErrUnknown, id)
		}
		if resp.StatusCode/100 != 2 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("spdx fetch %s: HTTP %d", id, resp.StatusCode)
			continue
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return "", err
		}
		return string(body), nil
	}
	if lastErr == nil {
		lastErr = errors.New("spdx fetch: unknown failure")
	}
	return "", lastErr
}
