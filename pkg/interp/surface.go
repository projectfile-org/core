// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package interp is the public façade over internal/interp — the spec §3.8
// `${<fieldpath>}` expansion engine.
//
// It sits in core because all three consumers resolve the same references
// against the same grammar: the bridge renders README commands and badge URLs,
// the CLI composes a sink reference for m6e, and ci-resolver lowers a build ref
// into a workflow. A second engine in any of them would be free to disagree
// with the others about the same document.
package interp

import internal "kiota.ch/projectfile/core/v2/internal/interp"

const (
	// Marker is the opening delimiter of a reference. Exported because a caller
	// that must DROP a half-resolved value (a badge URL, a pull command) tests
	// the rendered string for a leftover reference.
	Marker = internal.Marker
)

var (
	Expand        = internal.Expand
	ExpandChecked = internal.ExpandChecked
	ExpandFanOut  = internal.ExpandFanOut
	Unresolved    = internal.Unresolved
)
