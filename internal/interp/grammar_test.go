// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package interp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

// TestSplitFilterFindsTheTrailingPipe pins the filter split.
func TestSplitFilterFindsTheTrailingPipe(t *testing.T) {
	cases := []struct {
		ref    string
		addr   string
		known  bool
		envOut string
	}{
		{"keywords | env", "keywords", true, "RUST"},
		{"keywords|env", "keywords", true, "RUST"},
		{"  keywords  |  env  ", "keywords", true, "RUST"},
		{"keywords | unknown-filter", "keywords", false, "rust"},
		{"keywords", "keywords", true, "rust"},
	}

	for _, tc := range cases {
		t.Run(tc.ref, func(t *testing.T) {
			addr, filter, known := splitFilter(tc.ref)
			assert.Equal(t, tc.addr, addr)
			assert.Equal(t, tc.known, known)
			assert.Equal(t, tc.envOut, filter("rust"),
				"an unknown filter degrades to the identity, not an error")
		})
	}
}

// TestSplitFilterIgnoresPipesInsideBrackets pins the depth counter.
func TestSplitFilterIgnoresPipesInsideBrackets(t *testing.T) {
	addr, filter, known := splitFilter("keywords[a|b]")
	assert.Equal(t, "keywords[a|b]", addr, "a bracketed pipe is part of the address")
	assert.True(t, known, "no filter was found, so none is reported")
	assert.Equal(t, "x", filter("x"))

	addr, filter, known = splitFilter("map[k=v] | env")
	assert.Equal(t, "map[k=v]", addr, "only a pipe at depth zero separates the filter")
	assert.True(t, known)
	assert.Equal(t, "X", filter("x"))

	addr, _, known = splitFilter("map{k|v | env}")
	assert.Equal(t, "map{k|v | env}", addr, "both pipes sit inside the braces")
	assert.True(t, known)
}

// TestSplitFilterIgnoresPipesInsideQuotes pins the quote tracking.
func TestSplitFilterIgnoresPipesInsideQuotes(t *testing.T) {
	addr, filter, known := splitFilter(`map["a|b"]`)
	assert.Equal(t, `map["a|b"]`, addr)
	assert.True(t, known)
	assert.Equal(t, "x", filter("x"))
}

// TestSplitFilterAppliesEnvTransform pins the one registered filter.
func TestSplitFilterAppliesEnvTransform(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"org.projectfile.ci", "ORG_PROJECTFILE_CI"},
		{"some-key", "SOME_KEY"},
		{"mixed.dots-and-dashes", "MIXED_DOTS_AND_DASHES"},
		{"", ""},
	} {
		t.Run(tc.in, func(t *testing.T) {
			_, filter, _ := splitFilter("ref | env")
			assert.Equal(t, tc.want, filter(tc.in))
		})
	}
}

// TestSpanReadsThePipeForm pins the `$[list | template]` reader at its shapes.
func TestSpanReadsThePipeForm(t *testing.T) {
	cases := []struct {
		name string
		s    string
		addr string
		tmpl string
		ok   bool
	}{
		{"simple", "$[keywords | ${e}]", "keywords", "${e}", true},
		{"padded", "$[ keywords | ${e} ]", "keywords", "${e}", true},
		{"nested brackets in the list", `$[map[a[0]] | ${e}]`, "map[a[0]]", "${e}", true},
		{"quoted pipe in the template", `$[keys | ${e | env}]`, "keys", "${e | env}", true},
		{"no opening bracket", "$keys", "", "", false},
		{"bracket at end of string", "$[", "", "", false},
		{"unclosed", "$[keys | ${e}", "", "", false},
		{"no pipe", "$[keys]", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr, tmpl, next, ok := span(tc.s, 0)
			assert.Equal(t, tc.ok, ok)
			if !ok {
				assert.Empty(t, addr)
				assert.Empty(t, tmpl)
				return
			}
			assert.Equal(t, tc.addr, addr)
			assert.Equal(t, tc.tmpl, tmpl)
			assert.Equal(t, byte(']'), tc.s[next-1], "next points just past the closing bracket")
			assert.LessOrEqual(t, next, len(tc.s))
		})
	}
}

// TestSpanOpensOnlyAfterADollar pins the one character span inspects.
func TestSpanOpensOnlyAfterADollar(t *testing.T) {
	for _, s := range []string{"${keys | t}", "[keys | t]", "$keys[0]", "$"} {
		_, _, _, ok := span(s, 0)
		assert.False(t, ok, "s %q", s)
	}
	assert.True(t, SpanMarker != Marker, "the two openers are distinguishable")
}

// manyArtifacts builds a document whose artifact map is large enough that a
// cross-product fan-out would exceed the cap.
func manyArtifacts(n int) *projectfile.Document {
	items := map[string]any{}
	for i := range n {
		items[artifactName(i)] = map[string]any{
			"kind": "image",
			"cmd":  artifactName(i),
			"ref":  "example.test/" + artifactName(i),
		}
	}
	return &projectfile.Document{
		Identity:   projectfile.Identity{Name: "demo"},
		Extensions: map[string]any{"org.projectfile.artifacts": items},
	}
}

func artifactName(i int) string {
	return "a" + string(rune('a'+i/26)) + string(rune('a'+i%26))
}

// TestFanOutCapsTheCrossProduct pins the growth ceiling.
func TestFanOutCapsTheCrossProduct(t *testing.T) {
	doc := manyArtifacts(12)

	lines, resolved := ExpandFanOut(doc,
		"${org.projectfile.artifacts{kind=image}.ref} ${org.projectfile.artifacts{kind=image}.cmd}")
	assert.True(t, resolved)
	assert.LessOrEqual(t, len(lines), maxFanOut,
		"the fan-out is capped at %d lines, not %d", maxFanOut, 12*12)
	assert.NotEmpty(t, lines)
}

// TestFanOutBelowTheCapIsUncapped is the other edge.
func TestFanOutBelowTheCapIsUncapped(t *testing.T) {
	doc := manyArtifacts(2)

	lines, resolved := ExpandFanOut(doc,
		"${org.projectfile.artifacts{kind=image}.ref} ${org.projectfile.artifacts{kind=image}.cmd}")
	assert.True(t, resolved)
	assert.Len(t, lines, 4, "2x2 is under the cap and must be delivered whole")
}

// TestFanOutSourceLabelsOnlyMultipleValues pins the decision-trace label.
func TestFanOutSourceLabelsOnlyMultipleValues(t *testing.T) {
	assert.Equal(t, "fan-out", fanOutSource([]string{"a", "b"}))
	assert.Empty(t, fanOutSource([]string{"a"}))
	assert.Empty(t, fanOutSource(nil))
}

func TestUnresolved(t *testing.T) {
	assert.True(t, Unresolved("x "+Marker+" y"))
	assert.True(t, Unresolved(Marker))
	assert.False(t, Unresolved("fully resolved"))
	assert.False(t, Unresolved(""))
}

// TestUnresolvedIsAPrefixScan pins that the marker is the opening `${` itself.
func TestUnresolvedIsAPrefixScan(t *testing.T) {
	assert.True(t, Unresolved("kiota.ch/${identity.name}"))
	assert.True(t, Unresolved("${"), "the bare marker is enough")
	assert.False(t, Unresolved("kiota.ch/b19/ubuntu"))
}

// TestUnresolvedOnAFailedExpansion pins the end-to-end version of the same rule.
func TestUnresolvedOnAFailedExpansion(t *testing.T) {
	doc := &projectfile.Document{Identity: projectfile.Identity{Name: "demo"}}

	assert.True(t, Unresolved(Expand(doc, "${absent.key}")),
		"an unresolvable reference stays verbatim and marked")
	assert.False(t, Unresolved(Expand(doc, "${identity.name}")))

	// $$ is a literal escape: the template reaches the output untouched.
	assert.Equal(t, "${identity.name}", Expand(doc, "$${identity.name}"))
}

// TestLookupEntriesResolutionOrder pins the span list lookup order.
func TestLookupEntriesResolutionOrder(t *testing.T) {
	doc := &projectfile.Document{Keywords: []string{"root"}}
	entry := map[string]any{"keywords": []any{"from-entry"}}

	got, ok := lookupEntries(doc, "keywords", nil, entry)
	assert.True(t, ok)
	assert.Len(t, got, 1, "the addressed value becomes the one entry the template walks")
	assert.Equal(t, []any{"from-entry"}, got[0], "the enclosing entry wins")

	got, ok = lookupEntries(doc, "keywords", []string{"absent"}, nil)
	assert.True(t, ok)
	assert.Len(t, got, 1)
	assert.Equal(t, []string{"root"}, got[0], "with no entry and no scope, the root answers")

	_, ok = lookupEntries(&projectfile.Document{}, "keywords", nil, nil)
	assert.False(t, ok, "an address nothing carries is unresolved, not empty")
}

// optionalDoc declares one complete entry and one lacking the field a template names.
func optionalDoc() *projectfile.Document {
	return &projectfile.Document{
		Identity: projectfile.Identity{Name: "demo"},
		Extensions: map[string]any{
			"org.projectfile.artifacts": map[string]any{
				"good": map[string]any{
					"kind":  "service",
					"ports": []any{map[string]any{"port": int64(9101)}},
				},
				// A different kind, so a {kind=service} selector leaves it out.
				"partial": map[string]any{
					"kind":  "library",
					"ports": []any{map[string]any{"host": "x"}},
				},
			},
		},
	}
}

// TestSpanOverAnAbsentOptionalListRendersEmpty pins the empty-piece path.
func TestSpanOverAnAbsentOptionalListRendersEmpty(t *testing.T) {
	got, resolved := ExpandChecked(optionalDoc(), "run $[?org.projectfile.nope[] | --publish ${.}] img")
	assert.True(t, resolved, "an optional span that contributes nothing is not a failure")
	assert.Equal(t, "run img", got)
}

// TestEmptyOptionalSpanAtEndOfString pins the guard that protects the index.
func TestEmptyOptionalSpanAtEndOfString(t *testing.T) {
	got, resolved := ExpandChecked(optionalDoc(), "run $[?org.projectfile.nope[] | x]")
	assert.True(t, resolved)
	assert.Equal(t, "run ", got,
		"only the space AFTER the span is swallowed; the one before it separated the words")
}

// TestPresentOptionalSpanRendersItsValues is the other edge.
func TestPresentOptionalSpanRendersItsValues(t *testing.T) {
	const tmpl = "run $[?org.projectfile.artifacts{kind=service}.ports[] | -p ${port}] img"
	got, resolved := ExpandChecked(optionalDoc(), tmpl)
	assert.True(t, resolved)
	assert.Equal(t, "run -p 9101 img", got)
}

// TestSpanEntryMissStaysVerbatim pins the half-composed refusal.
func TestSpanEntryMissStaysVerbatim(t *testing.T) {
	got, resolved := ExpandChecked(optionalDoc(),
		"$[org.projectfile.artifacts{kind=library}.ports[] | --publish ${port}]")
	assert.False(t, resolved)
	assert.Equal(t, "$[org.projectfile.artifacts{kind=library}.ports[] | --publish ${port}]", got,
		"the template reaches the output untouched")
}

// TestSpanOverAnAbsentListStaysVerbatim is the non-optional twin.
func TestSpanOverAnAbsentListStaysVerbatim(t *testing.T) {
	got, resolved := ExpandChecked(optionalDoc(), "x $[org.projectfile.nope[] | ${.}] y")
	assert.False(t, resolved)
	assert.Equal(t, "x $[org.projectfile.nope[] | ${.}] y", got)
}

// TestSpanOverAnEmptyListRendersNothing pins that an empty list differs from an absent one.
func TestSpanOverAnEmptyListRendersNothing(t *testing.T) {
	doc := optionalDoc()
	doc.Extensions["org.projectfile.artifacts"].(map[string]any)["none"] = map[string]any{
		"kind":  "service",
		"ports": []any{},
	}

	got, resolved := ExpandChecked(doc, "run $[?org.projectfile.artifacts{none}.ports[] | -p ${port}] img")
	assert.True(t, resolved)
	assert.Equal(t, "run img", got)
}
