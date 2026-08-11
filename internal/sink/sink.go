// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package sink composes the artifact reference of one publish destination.
//
// A SINK is a named place an artifact goes. The name is a LABEL and nothing
// reads meaning into it: two sinks may address the same host under two accounts,
// and the code cannot tell (nor need to). What a sink carries is a `ref`
// template, and that template — not any rule in here — decides the shape of the
// reference:
//
//	hub-main: {ref: "docker.io/damianbuho/${image.flatname}:${image.tag}"}
//	hub-oss:  {ref: "docker.io/buho-oss/${image.flatname}:${image.tag}"}
//	ghcr:     {ref: "ghcr.io/damian-buho/${image.basename}:${image.tag}"}
//
// # Why there is no template engine here
//
// The `ref` is expanded by internal/interp — the same spec §3.8 engine that
// resolves a reference anywhere else — against a SCRATCH document this package
// builds, carrying the sink's own keys under `sink` and the image coordinates
// under `image`. Three properties fall out of that and none of them is coded
// for:
//
//   - The full address grammar works inside a `ref`. A sink may declare any key
//     it likes and reference it as `${sink.<key>}`, and that key's value may
//     itself interpolate — so a registry demanding `damian-buho-b19/ubuntu-noble`
//     is a template someone writes, never a branch someone adds here.
//   - Anything the scratch document cannot answer survives VERBATIM. A
//     `{B19_UBUNTU_SERIES}` matrix placeholder and a `${SOME_MAKE_VAR}` both
//     reach the layer that does resolve them, untouched.
//   - A multi-valued reference fans out, so one sink can compose one ref per
//     matrix cell without this package knowing what a matrix is.
package sink

import (
	"strings"

	"kiota.ch/projectfile/core/v2/internal/fieldpath"
	"kiota.ch/projectfile/core/v2/internal/genlog"
	"kiota.ch/projectfile/core/v2/internal/interp"
	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

// Keys a sink entry may carry that this package reads. Every OTHER key is
// opaque data the entry's own template may address as `${sink.<key>}` — which is
// the whole point: the vocabulary is open, so an unusual destination costs a
// template and not a schema change.
//
// Only KeyRef is read by the COMPOSER. The other three are read by the document
// reader (document.go) to build the default template and to rank and label the
// entry; a template that names them resolves them like any other key, and a
// template that ignores them is just as valid.
const (
	KeyRef   = "ref"
	KeyHost  = "host"
	KeyOwner = "owner"
	KeyRole  = "role"
)

// Scratch-document keys the two coordinate families land under. They are
// ordinary single-segment top-level keys, so `${sink.owner}` and
// `${image.flatname}` are plain field addresses with nothing special about them.
const (
	scopeSink  = "sink"
	scopeImage = "image"
)

// Coords are the image half of a template's bindings.
//
// They are passed EXPLICITLY rather than read from a document because the two
// callers need opposite things from the same composer: publishing binds the
// project's own basename, while resolving a base image binds a FOREIGN
// project's. A composer that only ever read `the current document` could serve
// the first and would silently answer the second with the wrong image.
type Coords struct {
	Basename string // e.g. b19/ubuntu/{B19_UBUNTU_SERIES}
	Tag      string // e.g. latest
}

// Compose renders one sink's reference.
//
// ok is false when the entry declares no `ref`, or when the template still
// carries an unresolved `${…}` after expansion. Refusing a half-resolved
// reference is the important half: a ref that silently lost a segment is a push
// to the wrong repository, and the caller must be able to tell that from a
// successful compose. A `{MATRIX_AXIS}` placeholder is NOT unresolved — it
// carries no `$` and is deliberately left for the matrix layer.
func Compose(entry map[string]any, coords Coords) (string, bool) {
	template, _ := entry[KeyRef].(string)
	if template == "" {
		genlog.Warn("sink: entry declares no ref template — nothing to compose", "basename", coords.Basename)
		return "", false
	}
	ref, resolved := interp.ExpandChecked(scratch(entry, coords), template)
	if !resolved {
		genlog.Warn("sink: ref template left an unresolved reference — refusing a partial ref",
			"template", template, "composed", ref)
		return "", false
	}
	genlog.Decision("sink_ref", ref, template, coords.Basename)
	return ref, true
}

// ComposeFanOut is Compose for a template whose references may name SEVERAL
// values — one line per combination, in the same order the readme fan-out uses.
// A sink whose ref addresses a multi-valued document field composes one
// reference per value instead of collapsing to the first.
func ComposeFanOut(entry map[string]any, coords Coords) ([]string, bool) {
	template, _ := entry[KeyRef].(string)
	if template == "" {
		genlog.Warn("sink: entry declares no ref template — nothing to compose", "basename", coords.Basename)
		return nil, false
	}
	refs, resolved := interp.ExpandFanOut(scratch(entry, coords), template)
	if !resolved {
		genlog.Warn("sink: ref template left an unresolved reference — refusing a partial ref",
			"template", template, "composed", strings.Join(refs, " "))
		return nil, false
	}
	return refs, true
}

// scratch builds the document the template expands against: the sink entry
// under `sink`, the decomposed coordinates under `image`.
//
// Both go on Extensions because a single-segment extension key serialises
// VERBATIM through ToMap (see nestExtension), which is what makes them ordinary
// addresses. Nothing else is on this document, so a template that reaches for a
// real project field resolves nothing and Compose refuses it — a sink reference
// must be composable from its own coordinates alone, or the same template would
// mean different things in different projects.
func scratch(entry map[string]any, coords Coords) *projectfile.Document {
	return &projectfile.Document{
		Extensions: map[string]any{
			scopeSink:  entry,
			scopeImage: fieldpath.ImageAtoms(coords.Basename, coords.Tag),
		},
	}
}
