// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package cache is the shared-slot view every pf-* cache command renders:
// one status shape, one purge path, one include-warm loop over the
// $XDG_CACHE_HOME/pf slot, so pf-cli, pf-bridge and pf-ci stop diverging in
// locations, counts and messages.
package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"kiota.ch/projectfile/core/v2/internal/genlog"
	"kiota.ch/projectfile/core/v2/internal/projectfile"
	"kiota.ch/projectfile/core/v2/internal/spdx"
)

// Areas of the shared slot.
const (
	AreaSPDX     = "spdx"
	AreaIncludes = "includes"
)

// Entry is one cached item for status output. Age is -1 when unknown.
type Entry struct {
	Area      string
	Name      string
	Fresh     bool
	FetchedAt time.Time
	Age       time.Duration
}

// AreaStatus is one area of the slot: counts plus the entries behind them,
// so status output can name what is cached instead of a bare number.
type AreaStatus struct {
	Total     int
	Fresh     int
	Stale     int
	OldestAge time.Duration
	Entries   []Entry
}

// Summary is the whole shared slot every cache status command renders.
type Summary struct {
	Root         string
	SPDXEmbedded int
	SPDX         AreaStatus
	Includes     AreaStatus
}

// Root resolves the shared cache slot ($XDG_CACHE_HOME/pf) every pf-* binary
// reads and writes. One source of truth for every status/purge line.
func Root() (string, error) {
	base, err := projectfile.XDGCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "pf"), nil
}

// Status reads the shared slot: SPDX corpus counts plus cached ids, includes
// counts plus per-URL freshness. SPDX texts are immutable, so every cached id
// reads as fresh with its mtime as the fetch date.
func Status() (Summary, error) {
	root, err := Root()
	if err != nil {
		return Summary{}, err
	}
	var st Summary
	st.Root = root
	st.SPDXEmbedded = len(spdx.EmbeddedIDs())
	spdxDir, derr := spdx.CacheDir()
	for _, id := range spdx.CachedIDs() {
		e := Entry{Area: AreaSPDX, Name: id, Fresh: true, Age: -1}
		if derr == nil {
			if fi, serr := os.Stat(filepath.Join(spdxDir, id+".txt")); serr == nil { // #nosec G304 -- own cache slot
				e.FetchedAt = fi.ModTime()
				e.Age = time.Since(fi.ModTime())
			}
		}
		st.SPDX.Entries = append(st.SPDX.Entries, e)
	}
	st.SPDX.Total = len(st.SPDX.Entries)
	st.SPDX.Fresh = st.SPDX.Total
	inc, ierr := projectfile.IncludeCacheEntries()
	if ierr != nil {
		return Summary{}, ierr
	}
	for _, ce := range inc {
		e := Entry{Area: AreaIncludes, Name: ce.URL, Fresh: ce.Fresh, FetchedAt: ce.FetchedAt, Age: ce.Age}
		st.Includes.Entries = append(st.Includes.Entries, e)
		if ce.Fresh {
			st.Includes.Fresh++
			continue
		}
		st.Includes.Stale++
		if ce.Age > st.Includes.OldestAge {
			st.Includes.OldestAge = ce.Age
		}
	}
	st.Includes.Total = len(st.Includes.Entries)
	return st, nil
}

// PurgeAll clears both areas of the shared slot. Best-effort: a missing dir
// is a no-op (success). Returns per-area counts plus the shared root so the
// caller reports one correct location instead of an area subdir.
func PurgeAll() (spdxRemoved, includesRemoved int, root string, err error) {
	root, err = Root()
	if err != nil {
		return 0, 0, "", err
	}
	spdxRemoved, err = spdx.Purge()
	if err != nil {
		return 0, 0, root, fmt.Errorf("purge SPDX: %w", err)
	}
	includesRemoved, err = projectfile.PurgeIncludes()
	if err != nil {
		return spdxRemoved, 0, root, fmt.Errorf("purge includes: %w", err)
	}
	return spdxRemoved, includesRemoved, root, nil
}

// WarmIncludesDir prefetches every HTTP include of the projectfile in dir
// into the shared slot. force revalidates even fresh entries. A directory
// with no projectfile is a benign (0, 0, nil) — the caller decides the
// message; an ambiguous directory (2+ documents) errors per spec §4.5.
// Per-URL failures warn and continue, so one dead remote never aborts the run.
func WarmIncludesDir(dir string, force bool) (warmed, total int, err error) {
	docs, err := filepath.Glob(filepath.Join(dir, projectfile.BaseName+".*"))
	if err != nil {
		return 0, 0, fmt.Errorf("scan %s for a projectfile: %w", dir, err)
	}
	if len(docs) == 0 {
		return 0, 0, nil
	}
	pfPath, err := projectfile.DetectPath(dir)
	if err != nil {
		return 0, 0, fmt.Errorf("detect projectfile in %s: %w", dir, err)
	}
	raw, err := projectfile.ReadRawBaseFromPath(pfPath)
	if err != nil {
		return 0, 0, fmt.Errorf("read projectfile: %w", err)
	}
	opts := projectfile.ReadOptions{ForceRefresh: force}
	includes := projectfile.AllHTTPIncludes(raw, dir, pfPath, opts)
	for _, ref := range includes {
		if werr := projectfile.WarmIncludeWithOptions(ref, opts); werr != nil {
			genlog.Warn("include warm failed", "url", ref, "err", werr.Error())
			continue
		}
		warmed++
	}
	return warmed, len(includes), nil
}
