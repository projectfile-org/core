// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package selector is the public façade over internal/selector (Bridge
// Revolution Phase 2) — the bubbletea picker/fill prompts the bridge/forge/scan
// commands drive. Promoted (not moved) because core-resident scaffold/usersetup
// also use it. Curated to the command layer's calls.
package selector

import internal "kiota.ch/projectfile/core/internal/selector"

type (
	Choices[T any] = internal.Choices[T]
	FillField      = internal.FillField
	MultiInput     = internal.MultiInput
)

var (
	ErrCancelled  = internal.ErrCancelled
	Fill          = internal.Fill
	NewMultiInput = internal.NewMultiInput
)

// Run is generic, so it is wrapped rather than value-aliased — a generic
// function cannot be taken as a value without instantiation.
func Run[T any](c Choices[T]) (T, error) { return internal.Run(c) }
