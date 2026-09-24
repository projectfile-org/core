// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package cache is the public façade over internal/cache — the shared-slot
// view (status, purge, warm) every pf-* cache command renders, so the three
// binaries stop diverging in locations, counts and messages.
package cache

import internal "kiota.ch/projectfile/core/v2/internal/cache"

// Status shapes — aliases, so values cross the boundary with identical fields.
type (
	Entry      = internal.Entry
	AreaStatus = internal.AreaStatus
	Summary    = internal.Summary
)

// Slot areas.
const (
	AreaSPDX     = internal.AreaSPDX
	AreaIncludes = internal.AreaIncludes
)

// Shared-slot functions — value aliases keep one implementation.
var (
	Root            = internal.Root
	Status          = internal.Status
	PurgeAll        = internal.PurgeAll
	WarmIncludesDir = internal.WarmIncludesDir
)
