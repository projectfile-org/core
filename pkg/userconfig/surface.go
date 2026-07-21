// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package userconfig is the public façade over internal/userconfig (Bridge
// Revolution Phase 1) — XDG config load + path resolution + private-host check
// the bridges use for identity/generate fallbacks. Value aliases keep one
// implementation.
package userconfig

import internal "kiota.ch/projectfile/core/v2/internal/userconfig"

// Phase 8: the setup wizard (moved to the cli module) reads/writes the config.
type Config = internal.Config

var (
	Load          = internal.Load
	PathFor       = internal.PathFor
	IsPrivateHost = internal.IsPrivateHost

	// Phase 2: the pf-bridge root honours --ignore-user-config through this.
	SetIgnored = internal.SetIgnored

	// Phase 8: setup wizard config round-trip.
	ExistingPath = internal.ExistingPath
	Write        = internal.Write
)
