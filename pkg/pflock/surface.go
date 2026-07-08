// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package pflock is the public façade over internal/pflock (Bridge Revolution
// Phase 2) — the file lock the bridge/forge write paths take on a projectfile.
// Promoted (not moved) because core-resident scaffold also locks. Curated to
// the command layer's calls.
package pflock

import internal "kiota.ch/projectfile/core/internal/pflock"

var (
	WithLock        = internal.WithLock
	WithLockTimeout = internal.WithLockTimeout
)
