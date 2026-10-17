// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

// keyTags is the §139 additional-key name a link carries its advisory
// capability list under — an unknown key preserved in Link.Extra, not a
// reserved field.
const keyTags = "tags"

// TagPublic marks a link as pointing at an anonymously reachable destination.
// It is a HOST fact (the scanner proposes it from the hostname) and never a
// project decision: identity.visibility states whether the project is meant to
// be public, so a private project still carries `public` on a link to a public
// upstream or a public documentation site.
const TagPublic = "public"

// LinkTags returns a link's capability tags, nil when it declares none.
// A bare string is NOT a tag list: a hand-edited `tags: badges` is a mistake the
// forge-alias deriver must ignore rather than resolve, so the scalar shape
// AsStringList accepts elsewhere is deliberately rejected here.
func LinkTags(l Link) []string {
	switch items := l.Extra[keyTags].(type) {
	case []string:
		return items
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// LinkHasTag reports whether l carries tag, tolerating a nil Extra.
func LinkHasTag(l Link, tag string) bool {
	for _, t := range LinkTags(l) {
		if t == tag {
			return true
		}
	}
	return false
}
