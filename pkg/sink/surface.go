// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package sink is the public façade over internal/sink — reading the named
// publish destinations a document declares, and composing each one's artifact
// reference from its `ref` template.
//
// All three consumers compose the same references and must agree on them byte
// for byte: the bridge writes the pull command a README advertises, the CLI
// answers m6e with the ref a build pushes to, and ci-resolver lowers the same
// ref into a workflow. Two implementations of this would let a project document
// a path its build never pushed.
package sink

import internal "kiota.ch/projectfile/core/v2/internal/sink"

type (
	Coords = internal.Coords
	Sink   = internal.Sink
	Route  = internal.Route
)

const (
	// The two namespaces a document declares its publish plane in. Exported so
	// a consumer writing the composed view back onto a document names the same
	// namespace the reader read.
	ExtensionNS        = internal.ExtensionNS
	PublishExtensionNS = internal.PublishExtensionNS

	// Roles a README introduces differently. A fragment reaches a key across
	// every entry only through a `{role=…}` selector, so these are the strings
	// the fragments select on.
	RolePrimary  = internal.RolePrimary
	RoleFallback = internal.RoleFallback

	// Entry keys the model itself reads. Exported so a consumer emitting a
	// composed entry spells the keys from here rather than as literals.
	KeyRef   = internal.KeyRef
	KeyHost  = internal.KeyHost
	KeyOwner = internal.KeyOwner
	KeyRole  = internal.KeyRole
)

var (
	Declared = internal.Declared
	Routes   = internal.Routes
	ByName   = internal.ByName
	Select   = internal.Select

	// Compose / ComposeFanOut over a raw entry map. A caller holding a Sink uses
	// its methods instead — those apply the model's defaults first.
	Compose       = internal.Compose
	ComposeFanOut = internal.ComposeFanOut
)
