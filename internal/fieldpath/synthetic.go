// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fieldpath

import (
	"strings"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

// Canonical dotted addresses of the synthetic image.* fields.
const (
	AddrImageBasename  = "image.basename"
	AddrImageNamespace = "image.namespace"
	AddrImageName      = "image.name"
)

// synthetics are addresses whose value is COMPUTED from real document fields
// rather than read from one. They exist so a rule several consumers would
// otherwise each re-derive has exactly one home: a project's container-image
// basename is owned by projectfile.ImageBasename and read identically by m6e
// (M6E_IMAGE_BASENAME via `pf-cli get image.basename`), by ci-resolver, and — the
// reason they moved here — by `${…}` interpolation inside a shared projectfile
// fragment.
//
// That last consumer is what a command-layer table could not serve. A fragment
// writing `docker pull ${org.projectfile.readme.registry}/${image.basename}:latest`
// reaches every project in the fleet, including the ~130 that never declare an
// explicit `ci.image`, and resolves to the SAME path the build actually pushed —
// where the readme's old private fallback (parsing the source-code link URL) could
// disagree with it.
var synthetics = map[string]func(*projectfile.Document) (string, bool){
	AddrImageBasename:  projectfile.ImageBasename,
	AddrImageNamespace: imageNamespace,
	AddrImageName:      imageName,
}

// Synthetic resolves a synthetic address. ok is false for an unknown address and
// for one that is known but not determinable from this document (a project with
// no identity name has no image basename), which callers treat as an ordinary
// missing value so --default / --or-default still apply.
func Synthetic(doc *projectfile.Document, addr string) (string, bool) {
	fn, known := synthetics[addr]
	if !known || doc == nil {
		return "", false
	}
	return fn(doc)
}

// IsSynthetic reports whether addr names a synthetic field. Lets a caller list or
// document the set without reaching into the map.
func IsSynthetic(addr string) bool {
	_, known := synthetics[addr]
	return known
}

// imageNamespace / imageName split the synthetic image.basename into its two
// identity halves — the last `/`-separated label as the namespace, the rest
// (minus any `:tag`) as the name. Present iff the basename itself is
// determinable; a bare (namespace-less) basename yields an EMPTY-but-present
// namespace, matching the ci-resolver behaviour, because an empty identity
// build-arg is valid.
func imageNamespace(doc *projectfile.Document) (string, bool) {
	base, ok := projectfile.ImageBasename(doc)
	if !ok {
		return "", false
	}
	ns, _ := splitBasename(base)
	return ns, true
}

func imageName(doc *projectfile.Document) (string, bool) {
	base, ok := projectfile.ImageBasename(doc)
	if !ok {
		return "", false
	}
	_, name := splitBasename(base)
	return name, true
}

// splitBasename splits an image basename into (namespace, name): the last `/`
// label is the namespace, the remainder (minus any `:tag`) the name. `b19/ubuntu`
// → (b19, ubuntu); a bare `ubuntu` → ("", ubuntu). The basename RULE itself lives
// in projectfile.ImageBasename; only this split is here (kept in sync with
// ci-resolver's basenameParts).
func splitBasename(image string) (namespace, name string) {
	name = image
	if i := strings.LastIndex(name, "/"); i >= 0 {
		namespace, name = name[:i], name[i+1:]
	}
	if i := strings.LastIndex(name, ":"); i >= 0 {
		name = name[:i]
	}
	return namespace, name
}
