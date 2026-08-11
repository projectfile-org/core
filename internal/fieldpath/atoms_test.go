// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fieldpath

import (
	"strings"
	"testing"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

// TestImageAtomsMatchSynthetics is the drift guard between the two ways an
// image decomposes: the synthetic addresses (which read a document) and
// ImageAtoms (which reads a basename, for a FOREIGN project). They run the same
// split rules and must therefore agree on every address, or a sink template
// would compose one path for a base image and the README would document another.
//
// It iterates the synthetics MAP rather than a hand-written list, so a new
// image.* address that ImageAtoms does not answer fails here on the day it is
// added.
func TestImageAtomsMatchSynthetics(t *testing.T) {
	for _, tc := range []struct{ basename, tag string }{
		{"b19/ubuntu/{B19_UBUNTU_SERIES}", "latest"},
		{"b19/ubuntu", "1.2"},
		{"loki", "3.1"},
	} {
		doc := &projectfile.Document{
			Extensions: map[string]any{
				"org.projectfile.ci": map[string]any{"image": tc.basename, atomTag: tc.tag},
			},
		}
		atoms := ImageAtoms(tc.basename, tc.tag)
		for addr := range synthetics {
			want, ok := Synthetic(doc, addr)
			if !ok {
				t.Fatalf("%s: synthetic %s unresolved on a document declaring ci.image", tc.basename, addr)
			}
			tail := addr[strings.IndexByte(addr, '.')+1:]
			got, present := atoms[tail]
			if !present {
				t.Fatalf("%s: ImageAtoms answers no %q — a synthetic address it does not cover", tc.basename, tail)
			}
			if got != want {
				t.Errorf("%s: %s — ImageAtoms %q, synthetic %q", tc.basename, addr, got, want)
			}
		}
	}
}

// TestImageSplitsAreComplementary pins the two splits against each other on the
// fleet's three-label shape: the last-label split and the first-label split are
// different cuts of the same string, and a template composes from either.
func TestImageSplitsAreComplementary(t *testing.T) {
	atoms := ImageAtoms("b19/ubuntu/resolute", "latest")
	for addr, want := range map[string]string{
		atomBasename:  "b19/ubuntu/resolute",
		atomFlatname:  "b19-ubuntu-resolute",
		atomNamespace: "b19/ubuntu",
		atomName:      "resolute",
		atomRoot:      "b19",
		atomPath:      "ubuntu/resolute",
		atomFlatpath:  "ubuntu-resolute",
		atomTag:       "latest",
	} {
		if got := atoms[addr]; got != want {
			t.Errorf("image.%s: got %q, want %q", addr, got, want)
		}
	}
}

// TestImageAtomsBareBasename: a project with no namespace has an empty path, and
// the atom must be PRESENT-but-empty rather than missing — a template naming it
// renders an empty segment instead of dropping the whole reference.
func TestImageAtomsBareBasename(t *testing.T) {
	atoms := ImageAtoms("loki", "3.1")
	if got := atoms[atomRoot]; got != "loki" {
		t.Errorf("image.root: got %q, want %q", got, "loki")
	}
	for _, addr := range []string{atomPath, atomFlatpath, atomNamespace} {
		got, present := atoms[addr]
		if !present {
			t.Errorf("image.%s absent on a bare basename", addr)
		}
		if got != "" {
			t.Errorf("image.%s: got %q, want empty", addr, got)
		}
	}
}

// TestImageAtomsTagNotAPort: only a colon in the LAST label is a tag. A registry
// port in the basename must not be read as one, or those projects publish under
// a tag named after a port number.
func TestImageAtomsTagNotAPort(t *testing.T) {
	atoms := ImageAtoms("host:5000/ns/name", "latest")
	if got := atoms[atomRoot]; got != "host:5000" {
		t.Errorf("image.root: got %q, want %q", got, "host:5000")
	}
	if got := atoms[atomPath]; got != "ns/name" {
		t.Errorf("image.path: got %q, want %q", got, "ns/name")
	}
}
