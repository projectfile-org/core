// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

// Visibility returns the DECLARED identity.visibility value, or "" when the
// document omits it. The two are different answers and callers that need the
// spec's default want IsPrivate, not a comparison against VisibilityPublic.
func Visibility(doc *Document) string {
	if doc == nil {
		return ""
	}
	return doc.Identity.Visibility
}

// IsPrivate reports whether a consumer must treat the project as private:
// an explicit `private`, and equally a document that declares no visibility at
// all (spec §4.3 — absence resolves to private).
func IsPrivate(doc *Document) bool {
	return Visibility(doc) != VisibilityPublic
}

// IsPublic reports whether the document explicitly declares `public`. Only
// this answer authorises an irreversible disclosure, so nothing else may be
// read as public by omission.
func IsPublic(doc *Document) bool {
	return Visibility(doc) == VisibilityPublic
}
