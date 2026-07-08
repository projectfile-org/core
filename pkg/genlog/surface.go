// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package genlog is the public façade over internal/genlog (Bridge Revolution
// Phase 1). It lets the movable trio (bridge/forge/scanners) log through core's
// structured surface without reaching into internal/. Value aliases keep one
// implementation. Curated to the calls the trio makes today; config toggles
// (Quiet/Verbose) stay internal — core sets them.
package genlog

import internal "kiota.ch/projectfile/core/internal/genlog"

var (
	Decision = internal.Decision
	Info     = internal.Info
	Warn     = internal.Warn
	Error    = internal.Error
	Section  = internal.Section
	Plain    = internal.Plain

	// Phase 2: the pf-bridge root drives these toggles across the boundary.
	// Setters (not var aliases) so the write reaches core's own package var.
	SetQuiet   = internal.SetQuiet
	SetVerbose = internal.SetVerbose

	// Phase 8: the projectfile CLI redirects log output in tests.
	SetOutput = internal.SetOutput
)
