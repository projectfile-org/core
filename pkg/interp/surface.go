// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package interp is the public façade over internal/interp — the spec §3.8
// `${<fieldpath>}` expansion engine.
//
// It sits in core because all three consumers resolve the same references
// against the same grammar: the bridge renders README commands and badge URLs,
// the CLI resolves a template for m6e, and ci-resolver lowers a build reference
// into a workflow. A second engine in any of them would be free to disagree
// with the others about the same document.
//
// This is the ONLY computation core performs on document values. Every rule that
// composes one value out of others — an artifact reference, a destination path —
// is a template DECLARED in a projectfile, expanded here. Core therefore holds no
// vocabulary of any domain: adding a part or a destination is an edit to a
// document, never a change here and never a release.
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

	// ExpandIn / ExpandFanOutIn take SCOPES: addresses whose subtree answers a
	// reference before the document root. They are what lets a template stay
	// short (`${series}`, not the full address at every reference) and what lets
	// ONE template describe a subject chosen by the caller — the same string
	// aimed at another scope composes another subject's reference.
	ExpandIn       = internal.ExpandIn
	ExpandFanOutIn = internal.ExpandFanOutIn
)
