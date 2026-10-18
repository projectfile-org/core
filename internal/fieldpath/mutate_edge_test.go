// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fieldpath

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

// mustParse fails fast on a selector typo instead of letting a nil Path mislead.
func mustParse(t *testing.T, sel string) Path {
	t.Helper()
	p, err := Parse(sel)
	require.NoError(t, err, "selector %q", sel)
	return p
}

// ── Set: error surfaces ──────────────────────────────────────────────────────

func TestSetIndexOutOfRange(t *testing.T) {
	_, err := Set(fixture(), mustParse(t, "keywords[9]"), "go")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

// TestSetIndexOutOfRangeEdges pins both sides of the valid index window.
func TestSetIndexOutOfRangeEdges(t *testing.T) {
	for _, tc := range []struct {
		sel     string
		wantErr bool
	}{
		{"keywords[0]", false},
		{"keywords[2]", false},
		{"keywords[3]", true},
	} {
		t.Run(tc.sel, func(t *testing.T) {
			_, err := Set(fixture(), mustParse(t, tc.sel), "go")
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestSetNegativeIndexCountsFromEnd pins the documented negative-index form.
func TestSetNegativeIndexCountsFromEnd(t *testing.T) {
	out, err := Set(fixture(), mustParse(t, "keywords[-1]"), "zig")
	require.NoError(t, err)
	assert.Equal(t, "zig", out.Keywords[len(out.Keywords)-1])

	out, err = Set(fixture(), mustParse(t, "keywords[-3]"), "zig")
	require.NoError(t, err)
	assert.Equal(t, "zig", out.Keywords[0])

	_, err = Set(fixture(), mustParse(t, "keywords[-4]"), "zig")
	require.Error(t, err, "one before the first element is out of range")
}

func TestSetSelectorMatchesNothing(t *testing.T) {
	_, err := Set(fixture(), mustParse(t, "repositories[role=nope]"), map[string]any{"url": "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no list item matches")
}

func TestSetSelectorSkipsNonMapItems(t *testing.T) {
	doc := fixture()
	doc.Keywords = []string{"a", "b"}
	_, err := Set(doc, mustParse(t, "keywords[role=origin]"), "z")
	require.Error(t, err, "a scalar list has no map item a selector can match")
}

// TestSetScalarIntoNonMapParent pins that a Set descending through a scalar reports the type.
func TestSetScalarIntoNonMapParent(t *testing.T) {
	_, err := Set(fixture(), mustParse(t, "identity.name.deeper"), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected map")
}

func TestSetIndexOnNonList(t *testing.T) {
	_, err := Set(fixture(), mustParse(t, "identity[0]"), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected list")
}

func TestSetSelectorOnNonList(t *testing.T) {
	_, err := Set(fixture(), mustParse(t, "identity[a=b]"), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected list")
}

func TestSetProjectionUnsupported(t *testing.T) {
	_, err := Set(fixture(), mustParse(t, "keywords[]"), "x")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "projection not supported")
}

func TestSetDeleteProjectionUnsupported(t *testing.T) {
	_, _, err := Delete(fixture(), mustParse(t, "keywords[]"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "projection not supported")
}

// ── Add ──────────────────────────────────────────────────────────────────────

func TestAddEmptySegments(t *testing.T) {
	_, _, err := Add(fixture(), Path{}, "x", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty path")
}

func TestAddTargetMustBeKey(t *testing.T) {
	_, _, err := Add(fixture(), mustParse(t, "keywords[0]"), "x", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "add target must be a key segment")
}

func TestAddOntoNonMap(t *testing.T) {
	_, _, err := Add(fixture(), mustParse(t, "identity.name.sub"), "x", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected map")
}

// TestAddRejectsNonListSlot pins that a scalar slot is an error, not a silent overwrite.
func TestAddRejectsNonListSlot(t *testing.T) {
	_, _, err := Add(fixture(), mustParse(t, "identity.name"), "x", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exists but is not a list")

	_, _, err = Add(fixture(), mustParse(t, "requirements.runtime"), "rust", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exists but is not a list")
}

// TestAddIndexIntoListOfMaps pins that Add always APPENDS, even onto an absent key.
func TestAddIndexIntoListOfMaps(t *testing.T) {
	doc := fixture()
	out, added, err := Add(doc, mustParse(t, "repositories[0].extra"), "value", false)
	require.NoError(t, err)
	assert.True(t, added)

	res, err := Resolve(out, mustParse(t, "repositories[0].extra"))
	require.NoError(t, err)
	v, ok := res.Single()
	require.True(t, ok)
	assert.Equal(t, []any{"value"}, v, "a new key lands holding a one-element list")
}

func TestAddIndexIntoExistingList(t *testing.T) {
	doc := fixture()
	out, added, err := Add(doc, mustParse(t, "repositories[role=origin].aliases"), "release", false)
	require.NoError(t, err)
	assert.True(t, added)

	res, err := Resolve(out, mustParse(t, "repositories[role=origin].aliases[0]"))
	require.NoError(t, err)
	v, ok := res.Single()
	require.True(t, ok)
	assert.Equal(t, "release", v)
}

// TestAddIndexBoundariesInTheWalker pins the index guard at both edges.
func TestAddIndexBoundariesInTheWalker(t *testing.T) {
	doc := fixture()
	for _, tc := range []struct {
		sel     string
		wantErr bool
	}{
		{"repositories[0].extra", false},
		{"repositories[1].extra", false},
		{"repositories[2].extra", true},
		{"repositories[-1].extra", false},
		{"repositories[-3].extra", true},
	} {
		t.Run(tc.sel, func(t *testing.T) {
			_, _, err := Add(doc, mustParse(t, tc.sel), "v", false)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestAddSelectorIntoListOfMaps(t *testing.T) {
	doc := fixture()
	out, added, err := Add(doc, mustParse(t, "repositories[role=origin].branch-extra"), "release", false)
	require.NoError(t, err)
	assert.True(t, added)
	assert.NotEmpty(t, out.Repositories)
}

// TestAddSelectorMatchesNothing proves Add never creates a record when nothing matches.
func TestAddSelectorMatchesNothing(t *testing.T) {
	_, _, err := Add(fixture(), mustParse(t, "repositories[role=nope].branch"), "release", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no list item matches")
}

func TestAddSelectorOnNonList(t *testing.T) {
	_, _, err := Add(fixture(), mustParse(t, "identity[a=b]"), "x", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "add target must be a key segment")
}

func TestAddIndexOnNonList(t *testing.T) {
	_, _, err := Add(fixture(), mustParse(t, "identity[0].sub"), "x", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected list")
}

// TestAddAllowDuplicateOverridesDedup is the opt-out contract.
func TestAddAllowDuplicateOverridesDedup(t *testing.T) {
	doc := fixture()

	out, added, err := Add(doc, mustParse(t, "keywords"), testRust, false)
	require.NoError(t, err)
	assert.False(t, added, "the value is already present")

	out, added, err = Add(out, mustParse(t, "keywords"), testRust, true)
	require.NoError(t, err)
	assert.True(t, added)
	assert.Len(t, out.Keywords, 4)
}

// TestAddWithoutIdentityRuleAlwaysAppends pins the rule registry's boundary.
func TestAddWithoutIdentityRuleAlwaysAppends(t *testing.T) {
	doc := fixture()
	out, added, err := Add(doc, mustParse(t, "com.example.build.extra"), "a", false)
	require.NoError(t, err)
	assert.True(t, added, "an unregistered address carries no dedup rule")

	_, added, err = Add(out, mustParse(t, "com.example.build.extra"), "a", false)
	require.NoError(t, err)
	assert.True(t, added, "with no rule to compare against, even the same value appends")
}

// TestAddIncludesDeduplicatesBothSpellings pins the spec §4.9 identity rule.
func TestAddIncludesDeduplicatesBothSpellings(t *testing.T) {
	const url = "https://example.com/inc.yaml"
	doc := &projectfile.Document{Includes: []string{url}}

	_, added, err := Add(doc, mustParse(t, "includes"), map[string]any{"url": url, "sha256": "abc"}, false)
	require.NoError(t, err)
	assert.False(t, added, "the mapping form is the same include as the string form")

	_, added, err = Add(doc, mustParse(t, "includes"), url, false)
	require.NoError(t, err)
	assert.False(t, added)
}

// TestAddAppendsToAbsentSlot covers the create-on-add path.
func TestAddAppendsToAbsentSlot(t *testing.T) {
	doc := &projectfile.Document{}
	out, added, err := Add(doc, mustParse(t, "keywords"), "zig", false)
	require.NoError(t, err)
	assert.True(t, added)
	assert.Equal(t, []string{"zig"}, out.Keywords)
}

// TestAddCreatesIntermediateMaps pins that Add can build the container chain.
func TestAddCreatesIntermediateMaps(t *testing.T) {
	doc := &projectfile.Document{}
	out, added, err := Add(doc, mustParse(t, "a.b.c"), "v", false)
	require.NoError(t, err)
	assert.True(t, added)

	res, err := Resolve(out, mustParse(t, "a.b.c[0]"))
	require.NoError(t, err)
	v, ok := res.Single()
	require.True(t, ok)
	assert.Equal(t, "v", v)
}

// ── Delete ───────────────────────────────────────────────────────────────────

func TestDeleteEmptySegments(t *testing.T) {
	_, _, err := Delete(fixture(), Path{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty path")
}

// TestDeleteThroughNonMapIsAbsentNotError pins that walking off a scalar is a missing address.
func TestDeleteThroughNonMapIsAbsentNotError(t *testing.T) {
	_, existed, err := Delete(fixture(), mustParse(t, "identity.name.deeper"))
	require.NoError(t, err)
	assert.False(t, existed)
}

func TestDeleteIndexOnNonListIsAbsent(t *testing.T) {
	_, existed, err := Delete(fixture(), mustParse(t, "identity[0].x"))
	require.NoError(t, err)
	assert.False(t, existed)
}

func TestDeleteIndexOutOfRangeIsAbsent(t *testing.T) {
	_, existed, err := Delete(fixture(), mustParse(t, "keywords[9]"))
	require.NoError(t, err)
	assert.False(t, existed)
}

func TestDeleteSelectorOnNonListIsAbsent(t *testing.T) {
	_, existed, err := Delete(fixture(), mustParse(t, "identity[a=b].x"))
	require.NoError(t, err)
	assert.False(t, existed)
}

func TestDeleteSelectorMatchesNothingIsAbsent(t *testing.T) {
	_, existed, err := Delete(fixture(), mustParse(t, "repositories[role=nope].url"))
	require.NoError(t, err)
	assert.False(t, existed)
}

// TestDeleteSelectorSkipsNonMapItems pins that a scalar entry cannot satisfy a predicate.
func TestDeleteSelectorSkipsNonMapItems(t *testing.T) {
	doc := fixture()
	_, existed, err := Delete(doc, mustParse(t, "keywords[a=b]"))
	require.NoError(t, err)
	assert.False(t, existed)
}

func TestDeleteUnknownKindIsRejected(t *testing.T) {
	_, _, err := Delete(fixture(), Path{Segments: []Segment{{Kind: SegKind(99)}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown segment kind")
}

// TestDeleteListEntryIndexBoundaries pins the removal guard at both edges.
func TestDeleteListEntryIndexBoundaries(t *testing.T) {
	for _, tc := range []struct {
		sel     string
		removed bool
	}{
		{"keywords[0]", true},
		{"keywords[2]", true},
		{"keywords[3]", false},
		{"keywords[-1]", true},
		{"keywords[-4]", false},
	} {
		t.Run(tc.sel, func(t *testing.T) {
			_, removed, err := Delete(fixture(), mustParse(t, tc.sel))
			require.NoError(t, err)
			assert.Equal(t, tc.removed, removed, "sel %q", tc.sel)
		})
	}
}

// TestDeleteListEntryRejectsNonListLeaf pins the private rewriter's own guard.
func TestDeleteListEntryRejectsNonListLeaf(t *testing.T) {
	_, err := deleteListEntry(
		map[string]any{"a": []any{1}},
		[]Segment{{Kind: SegKey, Key: "a"}},
		Segment{Kind: SegKey, Key: "b"},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleteListEntry called with kind")
}

// TestDeleteListEntrySelectorRemovesFirstMatchOnly pins the first-match-wins rule.
func TestDeleteListEntrySelectorRemovesFirstMatchOnly(t *testing.T) {
	root := map[string]any{"a": []any{
		map[string]any{"role": "origin", "v": 1},
		map[string]any{"role": "origin", "v": 2},
	}}
	removed, err := deleteListEntry(root, []Segment{{Kind: SegKey, Key: "a"}},
		Segment{Kind: SegSelector, Preds: []Predicate{{Key: "role", Value: "origin"}}})
	require.NoError(t, err)
	assert.True(t, removed)

	got := root["a"].([]any)
	require.Len(t, got, 1)
	assert.Equal(t, 2, got[0].(map[string]any)["v"], "the second match must survive")
}

func TestDeleteListEntrySelectorRemovesNothingWhenUnmatched(t *testing.T) {
	root := map[string]any{"a": []any{map[string]any{"role": "mirror"}}}
	removed, err := deleteListEntry(root, []Segment{{Kind: SegKey, Key: "a"}},
		Segment{Kind: SegSelector, Preds: []Predicate{{Key: "role", Value: "origin"}}})
	require.NoError(t, err)
	assert.False(t, removed)
	assert.Len(t, root["a"].([]any), 1)
}

func TestDeleteListEntryIndexOutOfRange(t *testing.T) {
	root := map[string]any{"a": []any{1, 2}}
	removed, err := deleteListEntry(root, []Segment{{Kind: SegKey, Key: "a"}}, Segment{Kind: SegIndex, Index: 7})
	require.NoError(t, err)
	assert.False(t, removed)
}

func TestDeleteListEntryMissingList(t *testing.T) {
	segs := []Segment{{Kind: SegKey, Key: "a"}}
	removed, err := deleteListEntry(map[string]any{}, segs, Segment{Kind: SegIndex, Index: 0})
	require.NoError(t, err)
	assert.False(t, removed)
}

// ── prefix matching ──────────────────────────────────────────────────────────

// TestKeyRunLengthCountsOnlyLeadingKeys pins the run of key segments at the head.
func TestKeyRunLength(t *testing.T) {
	cases := []struct {
		name string
		sel  string
		want int
	}{
		{"single key", "a", 1},
		{"two keys", "a.b", 2},
		{"three keys", "a.b.c", 3},
		{"keys then index", "a.b[0]", 2},
		{"keys then selector", "a.b[c=d]", 2},
		{"nested form", "org.projectfile.ci.nodes", 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, keyRunLength(mustParse(t, tc.sel).Segments), "sel %q", tc.sel)
		})
	}

	assert.Equal(t, 1, keyRunLength([]Segment{{Kind: SegIndex}}), "a leading index floors at one")
	assert.Equal(t, 1, keyRunLength(nil), "an empty slice floors at one")
	assert.Equal(t, 1, keyRunLength([]Segment{{Kind: SegProject}}), "a leading projection floors at one")
}

// TestJoinKeys prefixes the dot-joined spelling at each length of a run.
func TestJoinKeys(t *testing.T) {
	segs := mustParse(t, "a.b.c").Segments

	assert.Equal(t, "a", joinKeys(segs, 1))
	assert.Equal(t, "a.b", joinKeys(segs, 2))
	assert.Equal(t, "a.b.c", joinKeys(segs, 3))
	assert.Empty(t, joinKeys(segs, 0), "joining nothing yields an empty prefix")
}

// flatDoc carries an extension in the FLAT spelling the parser may leave behind.
func flatDoc() *projectfile.Document {
	return &projectfile.Document{
		Extensions: map[string]any{
			"com.example.build": map[string]any{"user": "ubuntu"},
		},
	}
}

// TestFlatExtensionKeyResolvesThroughTheNestedForm pins that both spellings agree.
func TestFlatExtensionKeyResolvesThroughTheNestedForm(t *testing.T) {
	res, err := Resolve(flatDoc(), mustParse(t, "com.example.build.user"))
	require.NoError(t, err)
	v, ok := res.Single()
	require.True(t, ok)
	assert.Equal(t, "ubuntu", v)
}

// TestAddIntoAFlatExtensionKeyLeavesNoTwin pins that one encoding of a namespace remains.
func TestAddIntoAFlatExtensionKeyLeavesNoTwin(t *testing.T) {
	out, _, err := Add(flatDoc(), mustParse(t, "com.example.build.image"), "alpine", false)
	require.NoError(t, err)

	assert.NotContains(t, out.Extensions, "com.example.build",
		"the flat key must not survive beside the branch the write landed in")
	assert.Len(t, out.Extensions, 1, "exactly one encoding of the namespace remains")

	res, err := Resolve(out, mustParse(t, "com.example.build.user"))
	require.NoError(t, err)
	v, ok := res.Single()
	require.True(t, ok)
	assert.Equal(t, "ubuntu", v, "the pre-existing sibling is still readable")

	res, err = Resolve(out, mustParse(t, "com.example.build.image"))
	require.NoError(t, err)
	v, ok = res.Single()
	require.True(t, ok)
	assert.Equal(t, []any{"alpine"}, v)
}

// TestSetOnAFlatExtensionKeyLeavesNoTwin is the write-side twin.
func TestSetOnAFlatExtensionKeyLeavesNoTwin(t *testing.T) {
	out, err := Set(flatDoc(), mustParse(t, "com.example.build.user"), "debian")
	require.NoError(t, err)

	assert.Len(t, out.Extensions, 1, "the flat key is folded into the one branch, never duplicated")
	assert.NotContains(t, out.Extensions, "com.example.build")

	res, err := Resolve(out, mustParse(t, "com.example.build.user"))
	require.NoError(t, err)
	v, ok := res.Single()
	require.True(t, ok)
	assert.Equal(t, "debian", v)
}

// TestNestedExtensionKeyDescendsWhenNoFlatKeyExists is the complement.
func TestNestedExtensionKeyDescendsWhenNoFlatKeyExists(t *testing.T) {
	doc := &projectfile.Document{}

	out, _, err := Add(doc, mustParse(t, "com.example.build"), "alpine", false)
	require.NoError(t, err)
	require.Len(t, out.Extensions, 1)

	res, err := Resolve(out, mustParse(t, "com.example.build[0]"))
	require.NoError(t, err)
	v, ok := res.Single()
	require.True(t, ok)
	assert.Equal(t, "alpine", v)
}

// TestAddThroughAFlatKeyIntoItsNestedMap pins descending through a matched flat key.
func TestAddThroughAFlatKeyIntoItsNestedMap(t *testing.T) {
	doc := &projectfile.Document{
		Extensions: map[string]any{
			"com.example.build": map[string]any{"user": "ubuntu", "tags": []any{"base"}},
		},
	}

	out, added, err := Add(doc, mustParse(t, "com.example.build.tags"), "extra", false)
	require.NoError(t, err)
	assert.True(t, added)
	assert.Len(t, out.Extensions, 1, "one encoding of the namespace, not two")

	res, err := Resolve(out, mustParse(t, "com.example.build.tags"))
	require.NoError(t, err)
	v, ok := res.Single()
	require.True(t, ok)
	assert.Equal(t, []any{"base", "extra"}, v)
}

// TestWalkIndexBounds pins the index window at both edges of a three-item list.
func TestWalkIndexBounds(t *testing.T) {
	doc := fixture()
	for _, tc := range []struct {
		sel     string
		wantErr bool
	}{
		{"keywords[0]", false},
		{"keywords[2]", false},
		{"keywords[3]", true},
		{"keywords[-1]", false},
		{"keywords[-3]", false},
		{"keywords[-4]", true},
	} {
		t.Run(tc.sel, func(t *testing.T) {
			_, err := Resolve(doc, mustParse(t, tc.sel))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestWalkIndexOnEmptyList pins that an empty list has no valid index at all.
func TestWalkIndexOnEmptyList(t *testing.T) {
	doc := &projectfile.Document{Keywords: []string{}}
	for _, sel := range []string{"keywords[0]", "keywords[-1]"} {
		_, err := Resolve(doc, mustParse(t, sel))
		require.Error(t, err, "sel %q", sel)
		assert.ErrorIs(t, err, ErrNotFound)
	}
}

// ── dottedKeyPath ────────────────────────────────────────────────────────────

// TestDottedKeyPathStopsAtFirstNonKey pins the boundary the identity registry depends on.
func TestDottedKeyPath(t *testing.T) {
	cases := []struct {
		sel  string
		want string
	}{
		{"keywords", "keywords"},
		{"keywords[0]", "keywords"},
		{"repositories[role=origin]", "repositories"},
		{"repositories[role=origin].url", "repositories"},
		{"repositories[0].branch", "repositories"},
		{"com.example.build.user", "com.example.build.user"},
		{"keywords[]", "keywords"},
	}
	for _, tc := range cases {
		t.Run(tc.sel, func(t *testing.T) {
			assert.Equal(t, tc.want, dottedKeyPath(mustParse(t, tc.sel).Segments))
		})
	}
	assert.Equal(t, "", dottedKeyPath(nil), "an empty path has no list address")
	assert.Equal(t, "", dottedKeyPath([]Segment{{Kind: SegIndex}}), "a leading index addresses no list")
}
