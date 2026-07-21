// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"strings"
)

// ExtractLocalizedString resolves a LocalizedString to a single string using
// the default precedence: Bare (language-agnostic) → en → first non-empty.
func ExtractLocalizedString(ls *LocalizedString) string {
	return ExtractLocalizedStringForLang(ls, "")
}

// ExtractLocalizedStringForLang resolves a LocalizedString for a specific
// language. Precedence: Bare (language-agnostic) → requested lang → en →
// first non-empty entry. An empty lang behaves like ExtractLocalizedString.
// The Bare form always wins because it is by convention language-agnostic.
func ExtractLocalizedStringForLang(ls *LocalizedString, lang string) string {
	if ls == nil {
		return ""
	}
	if ls.Bare != "" {
		return ls.Bare
	}
	if lang != "" {
		if v, ok := ls.Langs[lang]; ok && v != "" {
			return v
		}
	}
	if v, ok := ls.Langs["en"]; ok && v != "" {
		return v
	}
	for _, v := range ls.Langs {
		if v != "" {
			return v
		}
	}
	return ""
}

// SetLocalizedEN writes val into the English position of a LocalizedString.
// If the document was using the Bare form (or the field is nil), the result
// stays Bare — matching the file's existing style. If it was using Langs
// already, the "en" key is updated. Without this helper, mappers that wrote
// to Langs["en"] silently lost their write when the field was Bare, because
// the serializer prefers Bare.
func SetLocalizedEN(ls **LocalizedString, val string) {
	if *ls == nil {
		*ls = &LocalizedString{Bare: val}
		return
	}
	if len((*ls).Langs) == 0 {
		(*ls).Bare = val
		return
	}
	(*ls).Langs["en"] = val
	(*ls).Bare = ""
}

// Reverse-DNS namespace constants. The include-machinery fallback reads
// CLIExtensionNS (an includes-list source); the other strings are kept here
// as the single canonical spelling so the bridge-side pfmodel package can
// re-declare them without drift. The typed shapes of these namespaces and
// their accessors live in projectfile/bridge/internal/pfmodel.
const (
	CIExtensionNS              = "org.projectfile.ci"
	IgnoresExtensionNS         = "org.projectfile.ignores"
	EditorsExtensionNS         = "org.projectfile.editors"
	VulnerabilitiesExtensionNS = "org.projectfile.vulnerabilities"
	FundingExtensionNS         = "org.projectfile.funding"
	SecurityExtensionNS        = "org.projectfile.security"
	CodeOfConductExtensionNS   = "org.projectfile.code-of-conduct"
	ContributingExtensionNS    = "org.projectfile.contributing"
	CodeOwnersExtensionNS      = "org.projectfile.codeowners"
	CLIExtensionNS             = "org.projectfile.cli"
	ForgeExtensionNS           = "org.projectfile.forge"
	ConventionsExtensionNS     = "org.projectfile.conventions"
	SupportExtensionNS         = "org.projectfile.support"
	ReadmeExtensionNS          = "org.projectfile.readme"
	ReleaseExtensionNS         = "org.projectfile.release"
)

// SetExtension writes value into doc.Extensions for the given reverse-DNS
// namespace, after removing any nested-map form (the parse result of a TOML
// dotted-table header like `[org.projectfile.X]`). Writing under the flat key
// is what every other driver does; without the pre-prune the serialiser
// would emit BOTH the original dotted header AND the new quoted-dotted key
// on first sync — duplicate sections.
//
// Side-effect: a hand-written `[org.projectfile.X]` dotted header is
// rewritten as `["org.projectfile.X"]` on first sync. Both are valid TOML
// and round-trip identically; the visual change is one-time.
func SetExtension(doc *Document, ns string, value any) {
	if doc == nil || ns == "" {
		return
	}
	if doc.Extensions == nil {
		doc.Extensions = map[string]any{}
	}
	pruneNestedExtension(doc.Extensions, ns)
	doc.Extensions[ns] = value
}

// pruneNestedExtension walks the dotted-segment path under extMap and removes
// the leaf, then prunes any intermediate maps left empty. No-op when the
// nested form does not exist (which is the common case after the first sync).
func pruneNestedExtension(extMap map[string]any, ns string) {
	segments := strings.Split(ns, ".")
	if len(segments) < 2 {
		return
	}
	head, ok := extMap[segments[0]].(map[string]any)
	if !ok {
		return
	}
	chain := []map[string]any{head}
	parent := head
	for _, seg := range segments[1 : len(segments)-1] {
		next, ok := parent[seg].(map[string]any)
		if !ok {
			return
		}
		chain = append(chain, next)
		parent = next
	}
	leaf := segments[len(segments)-1]
	if _, has := parent[leaf]; !has {
		return
	}
	delete(parent, leaf)
	for i := len(chain) - 1; i > 0; i-- {
		if len(chain[i]) > 0 {
			return
		}
		delete(chain[i-1], segments[i])
	}
	if len(head) == 0 {
		delete(extMap, segments[0])
	}
}

// LookupExtension resolves a reverse-DNS namespace against the untyped
// Extensions map, transparently handling two on-disk encodings:
//
//  1. literal dotted key (YAML/JSON, or TOML quoted as ["org.projectfile.X"])
//     — landed directly under doc.Extensions[ns]
//  2. dotted-table header (TOML's [org.projectfile.X]) — exploded by go-toml
//     into nested maps, so we walk the segments
//
// Returns (nil, false) when the namespace is not present in either form.
func LookupExtension(doc *Document, ns string) (any, bool) {
	if doc == nil || doc.Extensions == nil || ns == "" {
		return nil, false
	}
	if v, ok := doc.Extensions[ns]; ok {
		return v, true
	}
	segments := strings.Split(ns, ".")
	cur, ok := doc.Extensions[segments[0]]
	if !ok {
		return nil, false
	}
	for _, seg := range segments[1:] {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[seg]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// AsStringList normalises a string-or-sequence-of-string value into a flat
// []string. Several spec fields are typed `any` because the encoding admits
// either a single scalar or a sequence (`license.file` per spec §4.4,
// `requirements.browsers`). Empty / whitespace-only entries are dropped so a
// caller can treat a zero-length result as "field absent". The single-string
// and sequence forms both round-trip; this is the read-side normaliser.
func AsStringList(v any) []string {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		if s := strings.TrimSpace(x); s != "" {
			return []string{s}
		}
		return nil
	case []string:
		out := make([]string, 0, len(x))
		for _, s := range x {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				if s = strings.TrimSpace(s); s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	}
	return nil
}

// boolVal reads m[key] as a bool, defaulting to false when missing or non-bool.
// Used by parse.go (Repository.Issues) and kept here next to the other
// extension-namespace helpers; pfmodel has its own copy for the accessors
// that moved.
func boolVal(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	if !ok {
		return false
	}
	return b
}
