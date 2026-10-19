// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package spdx

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Repeated IDs the grammar cases drive, so every case spells them the same way.
const (
	idMIT     = "MIT"
	idGPLOnly = "GPL-2.0-only"
)

// TestValidSPDXIDPinsTheAlphabet walks every accepted character class plus one rejected neighbour.
func TestValidSPDXIDPinsTheAlphabet(t *testing.T) {
	accepted := []string{
		idMIT, "Zlib", "Apache-2.0", "CC-BY-4.0", "GPL-2.0+", "x:y",
		"0BSD", "a", "z", "A", "Z", "0", "9",
		"AGPL-1.0-only", "BSD-3-Clause-No-Nuclear-License",
	}
	for _, id := range accepted {
		assert.True(t, validSPDXID(id), "id %q", id)
	}

	rejected := []string{
		"", "has space", "sl/ash", "../escape", "under_score",
		"back\\slash", "quote\"", "star*", "tilde~", "at@",
		"percent%", "hash#", "question?", "new\nline", "tab\there",
		"nul\x00byte", "unicode-é", "emoji-🙂",
	}
	for _, id := range rejected {
		assert.False(t, validSPDXID(id), "id %q", id)
	}
}

// TestStripExceptionBoundaries pins that the cut needs a space on BOTH sides of WITH.
func TestStripExceptionBoundaries(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", idMIT, idMIT},
		{"padded plain", "  MIT  ", idMIT},
		{"with exception", "GPL-2.0-only WITH Classpath-exception-2.0", idGPLOnly},
		{"padded with exception", "  GPL-2.0-only WITH Classpath-exception-2.0  ", idGPLOnly},
		{"lowercase with", "GPL-2.0-only with Classpath-exception-2.0", idGPLOnly},
		{"mixed case with", "GPL-2.0-only WiTh Classpath-exception-2.0", idGPLOnly},
		{"leading with, no left space", "WITH x", "WITH x"},
		{"no spaces around with", "MITWITHExtra", "MITWITHExtra"},
		{"trailing with, no right space", "MIT WITH", "MIT WITH"},
		{"with surrounded by spaces", "MIT WITH x", idMIT},
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, StripException(tc.in))
		})
	}
}

// TestSetEmbeddedNilClearsTheTier pins that nil turns tier 1 off rather than registering an empty corpus.
func TestSetEmbeddedNilClearsTheTier(t *testing.T) {
	SetEmbedded(fstest.MapFS{"MIT.txt": &fstest.MapFile{Data: []byte("m")}})
	require.NotEmpty(t, EmbeddedIDs())

	SetEmbedded(nil)
	t.Cleanup(func() { SetEmbedded(nil) })

	assert.Nil(t, EmbeddedIDs())
	assert.Nil(t, embeddedFS)

	_, ok := embeddedText(idMIT)
	assert.False(t, ok, "tier 1 is skipped rather than answering with nothing")
}

// TestEmbeddedTextReadsTheRootOnly pins that a root entry wins over a nested namesake.
func TestEmbeddedTextReadsTheRootOnly(t *testing.T) {
	SetEmbedded(fstest.MapFS{
		"MIT.txt":          &fstest.MapFile{Data: []byte("root")},
		"nested/MIT.txt":   &fstest.MapFile{Data: []byte("nested")},
		"deep/a/b/MIT.txt": &fstest.MapFile{Data: []byte("deep")},
	})
	t.Cleanup(func() { SetEmbedded(nil) })

	got, ok := embeddedText(idMIT)
	require.True(t, ok)
	assert.Equal(t, "root", string(got))

	_, ok = embeddedText("ABSENT")
	assert.False(t, ok)
}

// TestEmbeddedTextEmptyBodyStillCounts pins that an empty registered text is still a hit.
func TestEmbeddedTextEmptyBodyStillCounts(t *testing.T) {
	SetEmbedded(fstest.MapFS{"Empty.txt": &fstest.MapFile{Data: nil}})
	t.Cleanup(func() { SetEmbedded(nil) })

	got, ok := embeddedText("Empty")
	assert.True(t, ok)
	assert.Empty(t, got)
}
