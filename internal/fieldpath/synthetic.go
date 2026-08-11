// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fieldpath

import (
	"strings"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

// Canonical dotted addresses of the synthetic image.* fields.
//
// The set is deliberately TOTAL over the basename: every prefix/suffix split a
// registry path grammar can ask for is already an address, so an unusual
// registry is a `ref` template someone writes rather than a code change someone
// makes. `b19/ubuntu/resolute` decomposes both ways —
//
//	basename  b19/ubuntu/resolute      flatname  b19-ubuntu-resolute
//	root      b19                      path      ubuntu/resolute
//	namespace b19/ubuntu               name      resolute
//	                                   flatpath  ubuntu-resolute
//
// — so `${sink.owner}-${image.root}/${image.flatpath}` composes
// `damian-buho-b19/ubuntu-resolute` with nothing hardcoded.
const (
	AddrImageBasename  = imageScope + "." + atomBasename
	AddrImageNamespace = imageScope + "." + atomNamespace
	AddrImageName      = imageScope + "." + atomName
	AddrImageFlatname  = imageScope + "." + atomFlatname
	AddrImageTag       = imageScope + "." + atomTag
	AddrImageRoot      = imageScope + "." + atomRoot
	AddrImagePath      = imageScope + "." + atomPath
	AddrImageFlatpath  = imageScope + "." + atomFlatpath
)

// The address TAIL of each atom, which is also the key ImageAtoms parks it
// under. One spelling serves both: an atom addressed as `${image.flatpath}` and
// the same atom keyed `flatpath` on a scratch document must be the same word, or
// a template resolves nothing on the very map built to answer it.
const (
	imageScope    = "image"
	atomBasename  = "basename"
	atomNamespace = "namespace"
	atomName      = "name"
	atomFlatname  = "flatname"
	atomTag       = "tag"
	atomRoot      = "root"
	atomPath      = "path"
	atomFlatpath  = "flatpath"
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
	AddrImageFlatname:  imageFlatname,
	AddrImageTag:       projectfile.ImageTag,
	AddrImageRoot:      imageRoot,
	AddrImagePath:      imagePath,
	AddrImageFlatpath:  imageFlatpath,
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
// identity halves — everything before the LAST `/` as the namespace, the final
// label (minus any `:tag`) as the name. Present iff the basename itself is
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

// imageFlatname is the basename with every `/` folded to `-`, for a registry
// whose account namespace is the only nesting it allows: GHCR and ECR both put
// every image of an owner side by side, so `b19/ubuntu/{B19_UBUNTU_SERIES}` has
// to travel as `b19-ubuntu-{B19_UBUNTU_SERIES}`. A matrix placeholder survives
// untouched — it carries no `/` — so the flattening composes with the fan-out
// instead of fighting it.
//
// This is derived, never declared: a project that stated its flat name too would
// own two spellings of one image and could drift between them.
func imageFlatname(doc *projectfile.Document) (string, bool) {
	base, ok := projectfile.ImageBasename(doc)
	if !ok {
		return "", false
	}
	return flatten(base), true
}

// ImageAtoms decomposes a basename and tag into the whole `image.*` subtree,
// keyed by the address TAIL (`basename`, `flatpath`, …) so the result grafts
// under one `image` key of a document map.
//
// It exists for the caller that must decompose a FOREIGN project's image — a
// base image, a tool image — where the synthetics above cannot help because they
// read the document they are given. Both paths run the same three rules
// (splitBasename, splitRoot, fold `/` to `-`), and TestImageAtomsMatchSynthetics
// pins that they agree, so a new atom cannot land on one path only.
func ImageAtoms(basename, tag string) map[string]any {
	namespace, name := splitBasename(basename)
	root, path := splitRoot(basename)
	return map[string]any{
		atomBasename:  basename,
		atomFlatname:  flatten(basename),
		atomNamespace: namespace,
		atomName:      name,
		atomRoot:      root,
		atomPath:      path,
		atomFlatpath:  flatten(path),
		atomTag:       tag,
	}
}

// flatten folds every `/` to `-`, the one rule a registry with no nesting needs.
// A `{MATRIX_AXIS}` placeholder carries no `/`, so flattening composes with the
// fan-out instead of fighting it.
func flatten(path string) string { return strings.ReplaceAll(path, "/", "-") }

// imageRoot / imagePath are the FIRST-label split of the basename, the mirror of
// the last-label split imageNamespace/imageName does. A registry that forces one
// account per fleet needs the project's own top namespace as a name PART
// (`damian-buho-b19/…`), which no last-label split can hand it.
//
// A bare basename has no `/`: root is the whole name and path is EMPTY-but-
// present, so a template naming `${image.path}` on such a project renders an
// empty segment rather than dropping the whole reference — the same contract
// imageNamespace already applies to a namespace-less basename.
func imageRoot(doc *projectfile.Document) (string, bool) {
	base, ok := projectfile.ImageBasename(doc)
	if !ok {
		return "", false
	}
	root, _ := splitRoot(base)
	return root, true
}

func imagePath(doc *projectfile.Document) (string, bool) {
	base, ok := projectfile.ImageBasename(doc)
	if !ok {
		return "", false
	}
	_, path := splitRoot(base)
	return path, true
}

// imageFlatpath is imagePath with its remaining `/` folded to `-`. It is the
// half of flatname that survives having the root lifted out as an account name.
func imageFlatpath(doc *projectfile.Document) (string, bool) {
	path, ok := imagePath(doc)
	if !ok {
		return "", false
	}
	return flatten(path), true
}

// splitRoot splits a basename at its FIRST `/` — `b19/ubuntu/resolute` →
// (b19, ubuntu/resolute); a bare `ubuntu` → (ubuntu, ""). Any `:tag` suffix is
// dropped from the tail, matching splitBasename: a tag is never part of a path.
func splitRoot(image string) (root, path string) {
	root, path, _ = strings.Cut(image, "/")
	if i := strings.LastIndex(path, ":"); i >= 0 {
		path = path[:i]
	}
	if path == "" {
		if i := strings.LastIndex(root, ":"); i >= 0 {
			root = root[:i]
		}
	}
	return root, path
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
