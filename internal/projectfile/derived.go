// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import "strings"

// ImageBasename computes the project's container-image basename — the ONE home
// of the rule every lowering reads (m6e's M6E_IMAGE_BASENAME via `projectfile
// get image.basename`, ci-resolver's pf-ci via the library) so a cloud-built ref
// and an m6e-built ref of the same project can never disagree:
//
//	explicit org.projectfile.ci.image            (wins verbatim), else
//	<last-label(identity.namespace)>/<identity.name>  (bare name when no namespace)
//
// Returns ("", false) when neither an explicit image nor an identity name is
// available — a project with no container build never reads the basename, so a
// miss is "absent", not an error.
func ImageBasename(doc *Document) (string, bool) {
	if img := ciImageOverride(doc); img != "" {
		return img, true
	}
	name := doc.Identity.Name
	if name == "" {
		return "", false
	}
	if ns := lastLabel(doc.Identity.Namespace); ns != "" {
		return ns + "/" + name, true
	}
	return name, true
}

// ImageTagDefault is the tag a project that names none publishes under.
const ImageTagDefault = "latest"

// ImageTag computes the tag half of a project's image reference, the companion
// of ImageBasename and owned here for the same reason: a registry ref template
// composes the two, and a second copy of this rule would let a README document a
// tag the build never pushed.
//
//	explicit org.projectfile.ci.tag              (wins verbatim), else
//	the `:tag` suffix of an explicit ci.image,   else
//	"latest"
//
// It carries the basename's presence rather than its own: a tag with no image to
// hang on is not a value any caller can use, so a project with no container build
// answers nothing and the reference drops whole.
func ImageTag(doc *Document) (string, bool) {
	base, ok := ImageBasename(doc)
	if !ok {
		return "", false
	}
	if tag := ciStringField(doc, "tag"); tag != "" {
		return tag, true
	}
	// Only a `:` in the LAST path label is a tag; a registry host may carry a
	// port (`host:5000/ns/name`), which is part of the path, not of the tag.
	if i := strings.LastIndex(base, ":"); i > strings.LastIndex(base, "/") {
		return base[i+1:], true
	}
	return ImageTagDefault, true
}

// ciImageOverride reads org.projectfile.ci.image — the portable, explicit
// image-name override — from the (includes-merged) document, "" when unset.
func ciImageOverride(doc *Document) string {
	return ciStringField(doc, "image")
}

// ciStringField reads one string key out of org.projectfile.ci on the
// (includes-merged) document, "" when the namespace or the key is absent.
func ciStringField(doc *Document, key string) string {
	v, ok := LookupExtension(doc, CIExtensionNS)
	if !ok {
		return ""
	}
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

// lastLabel returns the final dotted label of a reverse-DNS namespace
// (org.example.b19 => b19) — the docker-namespace segment of the basename. It
// mirrors the m6e reader's `${NAMESPACE##*.}` so both sides agree on the segment.
func lastLabel(ns string) string {
	if i := strings.LastIndex(ns, "."); i >= 0 {
		return ns[i+1:]
	}
	return ns
}
