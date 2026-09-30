// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package netfetch is the public façade over internal/netfetch, the HTTP timeout and retry policy of every core fetch.
package netfetch

import internal "kiota.ch/projectfile/core/v2/internal/netfetch"

var (
	// SetTimeout drives the per-attempt timeout a --timeout flag sets; Timeout reads it back.
	SetTimeout = internal.SetTimeout
	Timeout    = internal.Timeout
	// Do sends a request under that timeout with bounded backoff-and-jitter retry.
	Do = internal.Do
)

// DefaultTimeout bounds one attempt until SetTimeout changes it.
const DefaultTimeout = internal.DefaultTimeout
