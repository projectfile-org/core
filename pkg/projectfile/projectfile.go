// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package projectfile is the public facade over the internal projectfile
// reader. It exposes the typed Document and the read/accessor surface an
// external module (e.g. d9t/ci-resolve) needs, without that module importing
// the internal package directly. Adding readers here keeps the internal model
// untouched while the import seam stays stable.
package projectfile

import internal "kiota.ch/projectfile/core/internal/projectfile"

// Document is the parsed, in-memory representation of a projectfile document.
// It is a type ALIAS for the internal model, so values cross the package
// boundary with identical fields and method set — no copy, no conversion.
type Document = internal.Document

// Read parses the projectfile document at an explicit path and returns the
// typed Document. Uses the internal explicit-path reader (internal.Read takes a
// directory and returns a 3-tuple; the facade's contract is path-in).
func Read(path string) (*Document, error) {
	// Delegate to the internal reader — the facade adds no parsing of its own.
	return internal.ReadFromPath(path)
}

// DetectPath returns the projectfile document path within dir, honouring the
// recommended encoding precedence.
func DetectPath(dir string) (string, error) {
	return internal.DetectPath(dir)
}

// Stack returns the document's stack[] tags (nil for a nil document) — the
// derivation input the resolver joins against the tool catalog.
func Stack(doc *Document) []string {
	if doc == nil {
		// A nil document carries no stack; report empty rather than panic.
		return nil
	}
	return doc.Stack
}

// Extension returns the opaque subtree for a reverse-DNS namespace (e.g.
// "org.projectfile.ci") as a mapping, and whether it is present as one. Always
// use this accessor — never read Document.Extensions directly — so flat and
// nested storage resolve. The internal lookup yields an untyped value; we
// narrow to a mapping (the only shape CI extension subtrees take). A present
// but non-mapping namespace reports (nil, false).
func Extension(doc *Document, namespace string) (map[string]any, bool) {
	raw, ok := internal.LookupExtension(doc, namespace)
	if !ok {
		return nil, false
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	return m, true
}
