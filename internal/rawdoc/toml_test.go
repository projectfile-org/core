// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package rawdoc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tomlKeys records the source order a body's top-level keys parse to.
func tomlKeys(t *testing.T, body string) []string {
	t.Helper()
	om, err := TOMLFromBytes([]byte(body))
	require.NoError(t, err)
	return om.Keys()
}

// TestParseTOMLAssignKey pins the key a top-level assignment line spells.
func TestParseTOMLAssignKey(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{"plain", "a = 1", "a"},
		{"no spaces around the equals", "a=1", "a"},
		{"dotted head", "a.b = 1", "a"},
		{"tab separated", "a\t=\t1", "a"},
		{"quoted", `"a b" = 1`, "a b"},
		{"quoted single character", `"a" = 1`, "a"},
		{"unterminated quote keeps the rest", `"a = 1`, "a = 1"},
		{"a bare word is not an assignment", "abc", ""},
		{"a key with only trailing spaces is not an assignment", "a  ", ""},
		{"an empty line spells no key", "", ""},
		{"a value with no key spells no key", "= 1", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseTOMLAssignKey(tc.line))
		})
	}
}

// TestParseTOMLHeaderHead pins the head a table header body spells.
func TestParseTOMLHeaderHead(t *testing.T) {
	cases := []struct {
		name string
		head string
		want string
	}{
		{"plain", "a]", "a"},
		{"stops at the dot", "a.b]", "a"},
		{"stops at the space", "a ]", "a"},
		{"stops at the tab", "a\t]", "a"},
		{"stops at the first closing bracket", "a]]", "a"},
		{"an empty body spells no head", "", ""},
		{"a leading closing bracket spells no head", "]a", ""},
		{"a leading dot spells no head", ".a]", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseTOMLHeaderHead(tc.head))
		})
	}
}

// TestTopLevelTOMLKeyOrderClassification pins what each line shape spells: keys are the top-level keys the document holds, all of which the scanner reads in source order, so a misread line drops its ONE key to the unordered tail behind a key that keeps its place.
func TestTopLevelTOMLKeyOrderClassification(t *testing.T) {
	before, after, arr, tbl, zoo, sub := "before", "after", "arr", "tbl", "zoo", "a"
	cases := []struct {
		name string
		body string
		keys []string
	}{
		{"array of tables header", "before = 0\n[[arr]]\nafter = 9\n", []string{before, arr, after}},
		{"table header between arrays", "[[arr]]\n[tbl]\n[[zoo]]\n", []string{arr, tbl, zoo}},
		{"dotted table header", "before = 0\n[a.b]\nafter = 9\n", []string{before, sub, after}},
		{"header with inner whitespace", "before = 0\n[a ]\nafter = 9\n", []string{before, sub, after}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed := map[string]any{}
			for _, k := range tc.keys {
				parsed[k] = nil
			}
			assert.Equal(t, tc.keys, topLevelTOMLKeyOrder([]byte(tc.body), parsed))
		})
	}
}

// TestTOMLKeyOrderTrimsLeadingWhitespaceOnAKeyLine pins that an indented key line still spells its key.
func TestTOMLKeyOrderTrimsLeadingWhitespaceOnAKeyLine(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, tomlKeys(t, "  a = 1\nb = 2\n"), "a space-indented key is scanned")
	assert.Equal(t, []string{"b", "a"}, tomlKeys(t, "\tb = 2\na = 1\n"), "a tab-indented key is scanned")
}

// TestTOMLKeyOrderHandlesTheLastLineWithoutANewline pins that the final line is scanned like any other.
func TestTOMLKeyOrderHandlesTheLastLineWithoutANewline(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, tomlKeys(t, "a = 1\nb = 2"))
}

// TestTOMLKeyOrderHandlesTrailingWhitespaceAtTheDocumentEnd pins that a whitespace-only tail is not a key.
func TestTOMLKeyOrderHandlesTrailingWhitespaceAtTheDocumentEnd(t *testing.T) {
	assert.Equal(t, []string{"a"}, tomlKeys(t, "a = 1\n  "))
}

// TestTOMLKeyOrderIgnoresACommentLineThatSpellsAKey pins that a comment is never scanned for keys, even when its text spells one.
func TestTOMLKeyOrderIgnoresACommentLineThatSpellsAKey(t *testing.T) {
	assert.Equal(t, []string{"last", "#"}, tomlKeys(t, "# .\nlast = 9\n\"#\" = 1\n"))
}

// TestTOMLKeyOrderQuotedKeyKeepsItsInnerSpaces pins that a quoted key is unquoted before it is recorded.
func TestTOMLKeyOrderQuotedKeyKeepsItsInnerSpaces(t *testing.T) {
	assert.Equal(t, []string{"a b", "c"}, tomlKeys(t, "\"a b\" = 1\nc = 2\n"))
}

// TestTOMLKeyOrderIgnoresCRLFLineEndings pins that a CRLF document records the same order as an LF one.
func TestTOMLKeyOrderIgnoresCRLFLineEndings(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, tomlKeys(t, "a = 1\r\nb = 2\r\n"))
}

// TestTOMLKeyOrderRecordsARepeatedTableHeaderOnce pins that a second spelling of a table adds no second entry.
func TestTOMLKeyOrderRecordsARepeatedTableHeaderOnce(t *testing.T) {
	assert.Equal(t, []string{"a"}, tomlKeys(t, "[[a]]\n[[a]]\n"))
}
