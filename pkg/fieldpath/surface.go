// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package fieldpath is the public façade over internal/fieldpath (Bridge
// Revolution Phase 2) — the dotted-path grammar. Stays in core (get/set/add/del
// own it); exposed so the moved derive pass can parse + inspect link selectors.
// Curated to derive's use (Parse + Path/Segment inspection).
package fieldpath

import internal "kiota.ch/projectfile/core/v2/internal/fieldpath"

type (
	Path    = internal.Path
	Segment = internal.Segment
	SegKind = internal.SegKind

	// Phase 8: the projectfile CLI (get/set/add/del) names these result types.
	Result = internal.Result
	Pair   = internal.Pair
)

const (
	SegKey      = internal.SegKey
	SegSelector = internal.SegSelector

	// Synthetic-field addresses. Resolve already answers them; these are for the
	// CLI naming them in help text and tests without keeping its own copies.
	//
	// The set is TOTAL over the basename — both the last-label split
	// (namespace/name) and the first-label split (root/path), plus the flattened
	// forms — so an unusual registry path grammar is a `ref` template someone
	// writes rather than a code change someone makes.
	AddrImageBasename  = internal.AddrImageBasename
	AddrImageNamespace = internal.AddrImageNamespace
	AddrImageName      = internal.AddrImageName
	AddrImageFlatname  = internal.AddrImageFlatname
	AddrImageTag       = internal.AddrImageTag
	AddrImageRoot      = internal.AddrImageRoot
	AddrImagePath      = internal.AddrImagePath
	AddrImageFlatpath  = internal.AddrImageFlatpath

	// PriorityDefault is the rank an unranked map entry fans out at. Promoted
	// so the bridge's own priority sorts read the number from here instead of
	// keeping a second copy that could drift.
	PriorityDefault = internal.PriorityDefault
)

var (
	Parse = internal.Parse

	// Phase 8: get/set/add/del resolve + mutate through the grammar.
	Resolve        = internal.Resolve
	LookupDefault  = internal.LookupDefault
	FormatScalar   = internal.FormatScalar
	Set            = internal.Set
	Add            = internal.Add
	Delete         = internal.Delete
	ErrNotFound    = internal.ErrNotFound
	ErrListOpOnMap = internal.ErrListOpOnMap
)
