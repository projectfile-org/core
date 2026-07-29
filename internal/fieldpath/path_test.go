// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fieldpath

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want []Segment
	}{
		{
			in: "identity.namespace",
			want: []Segment{
				{Kind: SegKey, Key: "identity"},
				{Kind: SegKey, Key: "namespace"},
			},
		},
		{
			in: "identity.title.en",
			want: []Segment{
				{Kind: SegKey, Key: "identity"},
				{Kind: SegKey, Key: "title"},
				{Kind: SegKey, Key: "en"},
			},
		},
		{
			in: "keywords[0]",
			want: []Segment{
				{Kind: SegKey, Key: "keywords"},
				{Kind: SegIndex, Index: 0},
			},
		},
		{
			in: "keywords[-1]",
			want: []Segment{
				{Kind: SegKey, Key: "keywords"},
				{Kind: SegIndex, Index: -1},
			},
		},
		{
			in: "repositories[role=origin]",
			want: []Segment{
				{Kind: SegKey, Key: keyRepositories},
				{Kind: SegSelector, Preds: []Predicate{{Key: testRole, Value: testOrigin}}},
			},
		},
		{
			in: "links[type=bugs,name=foo]",
			want: []Segment{
				{Kind: SegKey, Key: "links"},
				{Kind: SegSelector, Preds: []Predicate{
					{Key: "type", Value: "bugs"},
					{Key: "name", Value: "foo"},
				}},
			},
		},
		{
			in: "repositories[].url",
			want: []Segment{
				{Kind: SegKey, Key: keyRepositories},
				{Kind: SegProject},
				{Kind: SegKey, Key: testURL},
			},
		},
		{
			in: `links[type=package-registry,name="my,thing"]`,
			want: []Segment{
				{Kind: SegKey, Key: "links"},
				{Kind: SegSelector, Preds: []Predicate{
					{Key: "type", Value: "package-registry"},
					{Key: "name", Value: "my,thing"},
				}},
			},
		},
		{
			// ext.NS shortcut — strips the ext. prefix, rest is the literal namespace.
			in: "ext.example.build.user",
			want: []Segment{
				{Kind: SegKey, Key: "example"},
				{Kind: SegKey, Key: "build"},
				{Kind: SegKey, Key: "user"},
			},
		},
		{
			in: "repositories[role=origin].url",
			want: []Segment{
				{Kind: SegKey, Key: keyRepositories},
				{Kind: SegSelector, Preds: []Predicate{{Key: testRole, Value: testOrigin}}},
				{Kind: SegKey, Key: testURL},
			},
		},
		{
			// Map projection — fans out pair results downstream.
			in: "env{}",
			want: []Segment{
				{Kind: SegKey, Key: testEnv},
				{Kind: SegMapProject},
			},
		},
		{
			in: "env{}.keys",
			want: []Segment{
				{Kind: SegKey, Key: testEnv},
				{Kind: SegMapProject},
				{Kind: SegKey, Key: "keys"},
			},
		},
		{
			in: "env{}.values",
			want: []Segment{
				{Kind: SegKey, Key: testEnv},
				{Kind: SegMapProject},
				{Kind: SegKey, Key: "values"},
			},
		},
		{
			// {} after a flat-key extension still tokenises into the same
			// key-run + projection shape (the walker handles longest-prefix).
			in: "ext.example.env{}",
			want: []Segment{
				{Kind: SegKey, Key: "example"},
				{Kind: SegKey, Key: testEnv},
				{Kind: SegMapProject},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := Parse(tc.in)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tc.in, err)
			}
			if !reflect.DeepEqual(got.Segments, tc.want) {
				t.Fatalf("Parse(%q) = %#v, want %#v", tc.in, got.Segments, tc.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	bad := []string{
		"",
		"a.[",
		"a.b[unclosed",
		`a[k=v"]`, // unterminated quote inside selector body
		"a[k]",    // predicate missing '='
		"a{x}",    // map projection rejects a body — only `{}` is legal
		"a{",      // unmatched '{'
		"{}",      // bare `{}` with no preceding key
	}
	for _, s := range bad {
		t.Run(s, func(t *testing.T) {
			if _, err := Parse(s); err == nil {
				t.Fatalf("Parse(%q) expected error, got nil", s)
			}
		})
	}
}

func TestPathString(t *testing.T) {
	in := "repositories[role=origin].url"
	p, err := Parse(in)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.String(); got != in {
		t.Fatalf("Path.String() = %q, want %q", got, in)
	}
}

// TestMapProjectStringRoundTrip walks the new {} grammar end-to-end through
// the String() renderer. Because Path.String() short-circuits to Raw when
// the path was parsed (cheaper, preserves the user's exact text), we
// construct the segments directly to exercise the rendering branch.
func TestMapProjectStringRoundTrip(t *testing.T) {
	cases := map[string][]Segment{
		"env{}": {
			{Kind: SegKey, Key: testEnv},
			{Kind: SegMapProject},
		},
		"env{}.keys": {
			{Kind: SegKey, Key: testEnv},
			{Kind: SegMapProject},
			{Kind: SegKey, Key: "keys"},
		},
		"env{}.values": {
			{Kind: SegKey, Key: testEnv},
			{Kind: SegMapProject},
			{Kind: SegKey, Key: "values"},
		},
	}
	for want, segs := range cases {
		t.Run(want, func(t *testing.T) {
			got := Path{Segments: segs}.String()
			if got != want {
				t.Fatalf("Path.String() = %q, want %q", got, want)
			}
		})
	}
}

// TestMapSelectorParse asserts the `{k=v}` body reaches the walker as a
// SegMapSelector carrying its predicates, and that the two curly forms stay
// distinguishable: an empty body is still the pairs fan-out, never a selector
// matching nothing.
func TestMapSelectorParse(t *testing.T) {
	p, err := Parse("org.projectfile.artifacts{kind=image}.ref")
	if err != nil {
		t.Fatal(err)
	}
	// Longest-prefix key matching happens at RESOLVE time, so the parser emits
	// one SegKey per dotted piece: org, projectfile, artifacts.
	last := p.Segments[len(p.Segments)-2]
	if last.Kind != SegMapSelector {
		t.Fatalf("segment kind = %v, want SegMapSelector", last.Kind)
	}
	want := []Predicate{{Key: keyKind, Value: kindImage}}
	if !reflect.DeepEqual(last.Preds, want) {
		t.Fatalf("preds = %#v, want %#v", last.Preds, want)
	}
	empty, err := Parse("env{}")
	if err != nil {
		t.Fatal(err)
	}
	if empty.Segments[1].Kind != SegMapProject {
		t.Fatalf("empty body must stay SegMapProject, got %v", empty.Segments[1].Kind)
	}
}

// TestMapSelectorStringRoundTrip renders the selector back in CURLY brackets —
// echoing a map address in list brackets would point the reader at the grammar
// that does not apply to their document.
func TestMapSelectorStringRoundTrip(t *testing.T) {
	got := Path{Segments: []Segment{
		{Kind: SegKey, Key: "artifacts"},
		{Kind: SegMapSelector, Preds: []Predicate{{Key: keyKind, Value: kindImage}, {Key: keyRegistry, Value: "oci"}}},
		{Kind: SegKey, Key: "ref"},
	}}.String()
	want := "artifacts{kind=image,registry=oci}.ref"
	if got != want {
		t.Fatalf("Path.String() = %q, want %q", got, want)
	}
}

// TestMapSelectorRejectsBareKeyList keeps the grammar tight: `{a,b}` (a subselect
// of keys) is not grammar, and must fail at parse rather than resolve to nothing.
func TestMapSelectorRejectsBareKeyList(t *testing.T) {
	if _, err := Parse("artifacts{image,binary}"); err == nil {
		t.Fatalf("expected a parse error for a bare key list in {}")
	}
}
