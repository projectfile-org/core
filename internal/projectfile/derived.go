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

// ciImageOverride reads org.projectfile.ci.image — the portable, explicit
// image-name override — from the (includes-merged) document, "" when unset.
func ciImageOverride(doc *Document) string {
	v, ok := LookupExtension(doc, CIExtensionNS)
	if !ok {
		return ""
	}
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	img, _ := m["image"].(string)
	return img
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
