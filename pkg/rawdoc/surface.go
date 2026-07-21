// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package rawdoc is the public façade over internal/rawdoc (Bridge Revolution
// Phase 1) — the lossless round-trip primitives (key-order/comment preserving)
// that syncer bridges carry as Document.Rest. Type aliases carry the full
// method set; value aliases keep one implementation.
package rawdoc

import internal "kiota.ch/projectfile/core/v2/internal/rawdoc"

type (
	OrderedJSON = internal.OrderedJSON
	YAMLNode    = internal.YAMLNode
	OrderedTOML = internal.OrderedTOML
)

var (
	NewOrderedJSON = internal.NewOrderedJSON
	NewYAMLNode    = internal.NewYAMLNode
	NewOrderedTOML = internal.NewOrderedTOML
	FromBytes      = internal.FromBytes
	TOMLFromBytes  = internal.TOMLFromBytes
)
