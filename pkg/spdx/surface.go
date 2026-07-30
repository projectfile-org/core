// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package spdx is the public façade over internal/spdx (Bridge Revolution
// Phase 1) — license-text resolution + expression (compound/exception) helpers
// the license bridge consumes. Type aliases carry the config structs; value
// aliases keep one implementation.
package spdx

import internal "kiota.ch/projectfile/core/v2/internal/spdx"

type (
	Options = internal.Options
	Vars    = internal.Vars
)

var (
	Text           = internal.Text
	Substitute     = internal.Substitute
	IsCompound     = internal.IsCompound
	SplitCompound  = internal.SplitCompound
	StripException = internal.StripException

	// Cache surface: Status reports the SPDX cache, WarmAll prefetches it, and
	// CacheDir/Purge let the `cache` command report and clear the real on-disk
	// path. SPDX warming now lives in pf-bridge (the binary that reads SPDX);
	// pf-cli no longer warms SPDX.
	Status   = internal.Status
	WarmAll  = internal.WarmAll
	CacheDir = internal.CacheDir
	Purge    = internal.Purge

	// SetCacheApp selects the per-binary cache slot ($XDG_CACHE_HOME/projectfile/
	// <app>/spdx). Call once at startup. Crosses as a setter, not a value alias.
	SetCacheApp = internal.SetCacheApp

	// SetEmbedded registers the caller's licence corpus as lookup tier 1. Core
	// ships no texts of its own — the corpus is a build artifact belonging to
	// whichever consumer renders LICENSE files (today: pf-bridge). Crosses as a
	// setter, not a value alias, per the house rule for mutable package state:
	// a `var X = internal.X` copies, so a consumer's write would never reach core.
	SetEmbedded = internal.SetEmbedded
)
