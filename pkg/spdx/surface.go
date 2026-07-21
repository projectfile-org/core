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

	// Phase 8: the projectfile CLI `cache` verb warms + reports the SPDX cache.
	Status  = internal.Status
	WarmAll = internal.WarmAll

	// SetEmbedded registers the caller's licence corpus as lookup tier 1. Core
	// ships no texts of its own — the corpus is a build artifact belonging to
	// whichever consumer renders LICENSE files (today: pf-bridge). Crosses as a
	// setter, not a value alias, per the house rule for mutable package state:
	// a `var X = internal.X` copies, so a consumer's write would never reach core.
	SetEmbedded = internal.SetEmbedded
)
