// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import "strings"

// SplitGitName best-effort splits a flat display name (the kind that lives
// in `git config user.name` or `git log %aN`) into projectfile's
// family-names / given-names pair, per spec §5.5.5:
//
//   - "Family, Given" comma form → ("Family", "Given").
//   - "Last" (single token) → ("Last", ""); legal for mononymous people.
//   - "First Middle Last" → last whitespace token is family, rest is given.
//
// The heuristic is intentionally simple: producers SHOULD set `display-name`
// explicitly when the default would misorder a name (Hungarian, East Asian
// in native order, mononyms with cultural ordering). Callers that detect an
// ambiguous case (`Ambiguous() == true`) SHOULD log so the user knows the
// entry needs review.
func SplitGitName(s string) (family, given string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	// Comma form is unambiguous — emit as authored.
	if i := strings.Index(s, ","); i >= 0 {
		family = strings.TrimSpace(s[:i])
		given = strings.TrimSpace(s[i+1:])
		return family, given
	}
	fields := strings.Fields(s)
	if len(fields) == 1 {
		return fields[0], ""
	}
	family = fields[len(fields)-1]
	given = strings.Join(fields[:len(fields)-1], " ")
	return family, given
}

// AmbiguousGitName reports whether s is a single-token flat name — the
// case where the heuristic produced family-names only and the caller
// should warn the user that the entry needs review.
func AmbiguousGitName(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.Contains(s, ",") {
		return false
	}
	return len(strings.Fields(s)) == 1
}
