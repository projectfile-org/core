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
	// SetAttempts drives the total tries per request; Attempts reads it back.
	SetAttempts = internal.SetAttempts
	Attempts    = internal.Attempts
	// Delay reports the wait before an attempt, so a caller with its own loop shares the schedule.
	Delay = internal.Delay
	// Do sends a request under that timeout with bounded backoff-and-jitter retry.
	Do = internal.Do
	// Sleep waits one backoff step and reports whether the caller may try again.
	Sleep = internal.Sleep
)

// DefaultTimeout bounds one attempt until SetTimeout changes it.
const DefaultTimeout = internal.DefaultTimeout

// DefaultAttempts is the total tries per request until SetAttempts changes it.
const DefaultAttempts = internal.DefaultAttempts
