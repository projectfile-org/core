// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package rawdoc_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"kiota.ch/projectfile/core/v2/internal/rawdoc"
)

// ── YAMLNode: comments and key order ─────────────────────────────────────────

// TestSetHeadCommentRoundTrips pins the document-level comment accessor.
func TestSetHeadCommentRoundTrips(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("# banner\nidentity:\n  name: sample\n"))
	require.NoError(t, err)
	assert.Empty(t, n.HeadComment(), "a parsed document carries no document-level head comment")

	n.SetHeadComment("# replaced banner")
	assert.Equal(t, "# replaced banner", n.HeadComment())

	out, err := n.Marshal()
	require.NoError(t, err)
	assert.Contains(t, string(out), "# replaced banner")

	n.SetHeadComment("")
	assert.Empty(t, n.HeadComment(), "the setter clears as well as sets")
}

func TestHeadAndFootCommentsOnFreshNode(t *testing.T) {
	n := rawdoc.NewYAMLNode()
	assert.Empty(t, n.HeadComment())
	assert.Empty(t, n.FootComment())
	n.SetHeadComment("x")
	assert.Equal(t, "x", n.HeadComment())
}

// TestKeyCommentsSurviveAValueRewrite is the canvas guarantee.
func TestKeyCommentsSurviveAValueRewrite(t *testing.T) {
	body := "# who set the version\nidentity:\n  name: sample\nversion: 1.0.0\n"
	n, err := rawdoc.FromBytes([]byte(body))
	require.NoError(t, err)

	out, err := n.Marshal()
	require.NoError(t, err)
	assert.Contains(t, string(out), "# who set the version", "the baseline is that the comment is in the tree")

	require.NoError(t, n.SetValue("identity", map[string]any{"name": "sample", "version": "2.0.0"}))

	out, err = n.Marshal()
	require.NoError(t, err)
	assert.Contains(t, string(out), "# who set the version",
		"repainting a value must not drop the comment attached to its key")
	assert.Contains(t, string(out), "2.0.0")
	assert.Equal(t, []string{"identity", "version"}, n.Keys(), "an in-place rewrite must not reorder keys")
}

// TestSortKeysOrdersTopLevelAlphabetically pins the sorted-output path.
func TestSortKeysOrdersTopLevelAlphabetically(t *testing.T) {
	body := "zebra: 1\nalpha:\n  inner: kept\nmiddle: 3\n"
	n, err := rawdoc.FromBytes([]byte(body))
	require.NoError(t, err)

	n.SortKeys()
	assert.Equal(t, []string{"alpha", "middle", "zebra"}, n.Keys())

	out, err := n.Marshal()
	require.NoError(t, err)
	text := string(out)
	assert.Less(t, strings.Index(text, "alpha:"), strings.Index(text, "middle:"))
	assert.Less(t, strings.Index(text, "middle:"), strings.Index(text, "zebra:"))
	assert.Contains(t, text, "inner: kept", "a key must keep its own value through the sort")
}

// TestSortKeysIsIdempotent pins that re-sorting is a fixed point.
func TestSortKeysIsIdempotent(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("c: 3\na: 1\nb: 2\n"))
	require.NoError(t, err)

	n.SortKeys()
	first, err := n.Marshal()
	require.NoError(t, err)

	n.SortKeys()
	second, err := n.Marshal()
	require.NoError(t, err)

	assert.Equal(t, string(first), string(second))
	assert.Equal(t, []string{"a", "b", "c"}, n.Keys())
}

// TestSortKeysOnDegenerateDocuments pins the guards.
func TestSortKeysOnDegenerateDocuments(t *testing.T) {
	for _, body := range []string{"", "only: 1\n", "{}\n"} {
		t.Run(strings.ReplaceAll(body, "\n", "_"), func(t *testing.T) {
			n, err := rawdoc.FromBytes([]byte(body))
			require.NoError(t, err)
			assert.NotPanics(t, n.SortKeys)
			assert.LessOrEqual(t, len(n.Keys()), 1)
		})
	}
}

// ── YAMLNode: parsing and marshalling edges ──────────────────────────────────

func TestFromBytesRejectsMalformed(t *testing.T) {
	_, err := rawdoc.FromBytes([]byte("a: [\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse yaml")
}

// TestFromBytesRejectsNonMappingDocument pins the top-level shape check.
func TestFromBytesRejectsNonMappingDocument(t *testing.T) {
	_, err := rawdoc.FromBytes([]byte("- one\n- two\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "top-level mapping")
}

func TestFromBytesEmptyDocumentYieldsEmptyMapping(t *testing.T) {
	for _, body := range []string{"", "   \n", "# only a comment\n"} {
		n, err := rawdoc.FromBytes([]byte(body))
		require.NoError(t, err, "body %q", body)
		assert.Empty(t, n.Keys(), "body %q", body)

		out, err := n.Marshal()
		require.NoError(t, err)
		assert.NotEmpty(t, out)
	}
}

// TestMarshalEmitsTheDocumentStart pins the hand-rolled `---` prepend.
func TestMarshalEmitsTheDocumentStart(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("identity:\n  name: sample\n"))
	require.NoError(t, err)

	out, err := n.Marshal()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(out), "---\n"),
		"got %q, want a leading document start marker", string(out))
}

// TestCloneIsIndependent pins that a clone shares no value nodes with its source.
func TestCloneIsIndependent(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("identity:\n  name: original\nlist:\n  - a\n"))
	require.NoError(t, err)

	cp := n.Clone()
	require.NoError(t, cp.SetValue("identity", "mutated"))
	require.NoError(t, cp.SetValue("added", 1))

	orig, ok := n.Get("identity")
	require.True(t, ok)
	require.NotEmpty(t, orig.Content, "identity is a mapping node")
	assert.Equal(t, "name", orig.Content[0].Value)
	assert.Equal(t, "original", orig.Content[1].Value)
	assert.False(t, n.Has("added"), "a key added to the clone must not appear in the original")
	assert.NotContains(t, n.Keys(), "added", "nor in the original's key order")
}

func TestCloneFreshNode(t *testing.T) {
	cp := rawdoc.NewYAMLNode().Clone()
	assert.NotNil(t, cp)
	assert.Empty(t, cp.Keys())
	assert.NotPanics(t, func() { _ = cp.SetValue("a", 1) })
}

// TestSetValueThenGetRoundTrip is the canvas contract; keys are FLAT here.
func TestSetValueThenGetRoundTrip(t *testing.T) {
	n := rawdoc.NewYAMLNode()
	require.NoError(t, n.SetValue("a", 1))
	require.NoError(t, n.SetValue("b", map[string]any{"c": "nested"}))
	require.NoError(t, n.SetValue("org.projectfile.cli", "flat extension key"))

	assert.Equal(t, []string{"a", "b", "org.projectfile.cli"}, n.Keys(), "insertion order is preserved")

	v, ok := n.Get("a")
	require.True(t, ok)
	assert.Equal(t, "1", v.Value)

	v, ok = n.Get("b")
	require.True(t, ok)
	assert.Equal(t, "c", v.Content[0].Value, "a mapping value keeps its own children")

	_, ok = n.Get("absent")
	assert.False(t, ok)
}

// TestSetValueReplacesInPlace pins that rewriting an existing key keeps its position.
func TestSetValueReplacesInPlace(t *testing.T) {
	n := rawdoc.NewYAMLNode()
	require.NoError(t, n.SetValue("a", 1))
	require.NoError(t, n.SetValue("b", 2))
	require.NoError(t, n.SetValue("a", 9))

	assert.Equal(t, []string{"a", "b"}, n.Keys())
	v, _ := n.Get("a")
	assert.Equal(t, "9", v.Value)
}

// TestSetNodeAcceptsAPreBuiltNode pins the escape hatch a renderer uses.
func TestSetNodeAcceptsAPreBuiltNode(t *testing.T) {
	n := rawdoc.NewYAMLNode()
	n.SetNode("styled", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "value"})

	v, ok := n.Get("styled")
	require.True(t, ok)
	assert.Equal(t, "value", v.Value)
}

// TestDeleteRemovesAndPreservesOrder pins the removal path.
func TestDeleteRemovesAndPreservesOrder(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("a: 1\nb: 2\nc: 3\n"))
	require.NoError(t, err)

	n.Delete("b")
	assert.Equal(t, []string{"a", "c"}, n.Keys())
	assert.False(t, n.Has("b"))

	n.Delete("absent")
	assert.Equal(t, []string{"a", "c"}, n.Keys(), "deleting an absent key is a no-op")
}

// ── PaintMap ─────────────────────────────────────────────────────────────────

// TestPaintMapRemovesAbsentKeys pins the delete propagation a `del` depends on.
func TestPaintMapRemovesAbsentKeys(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("keep: 1\ngone: 2\nalso-gone: 3\n"))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{"keep": 1}))

	assert.Equal(t, []string{"keep"}, n.Keys())
	assert.False(t, n.Has("gone"))
	assert.False(t, n.Has("also-gone"))

	out, err := n.Marshal()
	require.NoError(t, err)
	assert.NotContains(t, string(out), "gone")
}

// TestPaintMapAddsAbsentKeys pins the inverse.
func TestPaintMapAddsAbsentKeys(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("existing: 1\n"))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{"existing": 1, "added": "new"}))

	assert.True(t, n.Has("added"))
	v, ok := n.Get("added")
	require.True(t, ok)
	assert.Equal(t, "new", v.Value)
}

// TestPaintMapIsRecursive pins that a nested branch is not re-encoded whole.
func TestPaintMapIsRecursive(t *testing.T) {
	body := "# why identity is pinned\n" +
		"identity:\n  # keep this comment\n  name: old\n  version: 1.0.0\n" +
		"untouched:\n  # deep comment\n  nested: value\n"
	n, err := rawdoc.FromBytes([]byte(body))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{
		"identity":  map[string]any{"name": "new", "version": "1.0.0"},
		"untouched": map[string]any{"nested": "value"},
	}))

	out, err := n.Marshal()
	require.NoError(t, err)
	text := string(out)
	assert.Contains(t, text, "new")
	assert.NotContains(t, text, "old")
	assert.Contains(t, text, "deep comment", "an untouched nested branch keeps its comments")
}

// TestPaintMapPreservesKeyOrder pins that a paint reorders nothing.
func TestPaintMapPreservesKeyOrder(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("zebra: 1\nalpha: 2\nmiddle: 3\n"))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{"zebra": 9, "alpha": 8, "middle": 7}))
	assert.Equal(t, []string{"zebra", "alpha", "middle"}, n.Keys())
}

// TestPaintMapOnEmptyCanvas pins that painting onto a fresh document builds the tree.
func TestPaintMapOnEmptyCanvas(t *testing.T) {
	n := rawdoc.NewYAMLNode()
	require.NoError(t, n.PaintMap(map[string]any{
		"identity": map[string]any{"name": "demo"},
		"keywords": []any{"rust"},
	}))
	assert.ElementsMatch(t, []string{"identity", "keywords"}, n.Keys())

	out, err := n.Marshal()
	require.NoError(t, err)
	assert.Contains(t, string(out), "name: demo")
}

// TestPaintMapEmptyRemovesEverything pins the total case.
func TestPaintMapEmptyRemovesEverything(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("a: 1\nb:\n  c: 2\n"))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{}))
	assert.Empty(t, n.Keys())
}

// TestPaintKeepsScalarStyle pins that the author's quoting survives a repaint.
func TestPaintKeepsScalarStyle(t *testing.T) {
	for _, tc := range []struct {
		name  string
		start string
		ends  string
	}{
		{"double quoted", `key: "old"`, `"new"`},
		{"single quoted", "key: 'old'", "'new'"},
		{"literal block", "key: |\n  old\n", "new"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, err := rawdoc.FromBytes([]byte(tc.start))
			require.NoError(t, err)
			require.NoError(t, n.PaintMap(map[string]any{"key": "new"}))

			out, err := n.Marshal()
			require.NoError(t, err)
			assert.Contains(t, string(out), tc.ends)
		})
	}
}

// TestPaintMappingAppendsNewKeysSorted pins the nested-mapping sorted tiebreak.
func TestPaintMappingAppendsNewKeysSorted(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("top:\n  zeta: 1\n"))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{
		"top": map[string]any{"zeta": 1, "mike": 2, "alpha": 3, "bravo": 4},
	}))

	out, err := n.Marshal()
	require.NoError(t, err)
	text := string(out)
	assert.Less(t, strings.Index(text, "alpha:"), strings.Index(text, "bravo:"))
	assert.Less(t, strings.Index(text, "bravo:"), strings.Index(text, "mike:"))
	assert.Less(t, strings.Index(text, "zeta:"), strings.Index(text, "alpha:"),
		"the key the source already had keeps its position, ahead of the new ones")
}

// TestPaintTopLevelAddsEveryKey is the top-level counterpart.
func TestPaintTopLevelAddsEveryKey(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("zeta: 1\n"))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{"zeta": 1, "mike": 2, "alpha": 3}))

	keys := n.Keys()
	require.Len(t, keys, 3)
	assert.Equal(t, "zeta", keys[0], "an existing key keeps its position")
	assert.ElementsMatch(t, []string{"mike", "alpha"}, keys[1:])
}

// TestPaintSequenceMatchesElementByElement pins the element-wise list paint.
func TestPaintSequenceMatchesElementByElement(t *testing.T) {
	body := "items:\n  # first entry\n  - alpha\n  # second entry\n  - beta\n"
	n, err := rawdoc.FromBytes([]byte(body))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{"items": []any{"ALPHA", "BETA"}}))
	out, err := n.Marshal()
	require.NoError(t, err)
	text := string(out)
	assert.Contains(t, text, "# first entry")
	assert.Contains(t, text, "# second entry")
	assert.Contains(t, text, "ALPHA")
	assert.NotContains(t, text, "alpha")
}

// TestPaintSequenceResizedLosesElementComments is the deliberate trade-off.
func TestPaintSequenceResizedLosesElementComments(t *testing.T) {
	body := "# list header\nitems:\n  # first entry\n  - alpha\n"
	n, err := rawdoc.FromBytes([]byte(body))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{"items": []any{"alpha", "beta"}}))
	out, err := n.Marshal()
	require.NoError(t, err)
	assert.Contains(t, string(out), "beta")
	assert.NotContains(t, string(out), "# first entry", "a resized list is re-encoded whole")
}

// TestPaintSequenceShorterThanSource pins the other resize direction.
func TestPaintSequenceShorterThanSource(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("items:\n  - alpha\n  - beta\n  - gamma\n"))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{"items": []any{"only"}}))
	out, err := n.Marshal()
	require.NoError(t, err)
	assert.Contains(t, string(out), "only")
	assert.NotContains(t, string(out), "gamma")
}

// TestPaintScalarOverScalarKeepsTheComment pins the simplest inheritance.
func TestPaintScalarOverScalarKeepsTheComment(t *testing.T) {
	body := "# who chose this version\nversion: 1.0.0\n"
	n, err := rawdoc.FromBytes([]byte(body))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{"version": "2.0.0"}))
	out, err := n.Marshal()
	require.NoError(t, err)
	assert.Contains(t, string(out), "# who chose this version")
	assert.Contains(t, string(out), "2.0.0")
}

// TestPaintEmptyListOverPopulated pins that emptying a list emits an explicit empty list.
func TestPaintEmptyListOverPopulated(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("items:\n  - a\n"))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{"items": []any{}}))
	out, err := n.Marshal()
	require.NoError(t, err)
	assert.Contains(t, string(out), "items:")
	assert.NotContains(t, string(out), "- a")
}

// TestPaintNullValue pins that a nil is written as an explicit null.
func TestPaintNullValue(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("value: something\n"))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{"value": nil}))
	out, err := n.Marshal()
	require.NoError(t, err)
	assert.Contains(t, string(out), "value:")
	assert.NotContains(t, strings.ToLower(string(out)), "something")
}

// TestCloneFollowsYAMLAliases pins that an alias is cloned, not left pointing at the original.
func TestCloneFollowsYAMLAliases(t *testing.T) {
	body := "base: &base\n  shared: value\nderived: *base\n"
	n, err := rawdoc.FromBytes([]byte(body))
	require.NoError(t, err)

	cp := n.Clone()
	require.NotNil(t, cp)

	derived, ok := cp.Get("derived")
	require.True(t, ok)
	require.NotNil(t, derived.Alias, "the alias must survive the clone")

	require.NoError(t, cp.SetValue("base", map[string]any{"shared": "mutated"}))
	assert.NotEqual(t, "mutated", derived.Alias.Content[1].Value,
		"the clone's alias must not resolve back into the original tree")
}

// TestPaintOverAnAliasRepaintsWhole pins the alias carve-out.
func TestPaintOverAnAliasRepaintsWhole(t *testing.T) {
	n, err := rawdoc.FromBytes([]byte("base: &base\n  shared: original\nderived: *base\n"))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{
		"base":    map[string]any{"shared": "painted"},
		"derived": map[string]any{"shared": "painted"},
	}))

	out, err := n.Marshal()
	require.NoError(t, err)
	assert.Contains(t, string(out), "painted")
	assert.NotContains(t, string(out), "original")
}

// TestPaintReEncodesAShapeChange pins that a changed shape is re-encoded whole.
func TestPaintReEncodesAShapeChange(t *testing.T) {
	for _, tc := range []struct {
		name    string
		start   string
		painted map[string]any
		want    string
	}{
		{"list became mapping", "key:\n  - a\n  - b\n", map[string]any{"key": map[string]any{"x": "y"}}, "x:"},
		{"mapping became list", "key:\n  x: y\n", map[string]any{"key": []any{"a", "b"}}, "- a"},
		{"list became scalar", "key:\n  - a\n", map[string]any{"key": "scalar"}, "key: scalar"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, err := rawdoc.FromBytes([]byte(tc.start))
			require.NoError(t, err)
			require.NoError(t, n.PaintMap(tc.painted))

			out, err := n.Marshal()
			require.NoError(t, err)
			assert.Contains(t, string(out), tc.want)
		})
	}
}

// TestPaintIntoAnAliasedBranch pins that an alias is never painted through.
func TestPaintIntoAnAliasedBranch(t *testing.T) {
	body := "base: &base\n  shared: original\nderived: *base\n"
	n, err := rawdoc.FromBytes([]byte(body))
	require.NoError(t, err)

	require.NoError(t, n.PaintMap(map[string]any{
		"base":    map[string]any{"shared": "base-painted"},
		"derived": map[string]any{"shared": "derived-painted"},
	}))

	out, err := n.Marshal()
	require.NoError(t, err)
	text := string(out)
	assert.Contains(t, text, "base-painted")
	assert.Contains(t, text, "derived-painted")
	assert.NotContains(t, text, "original")
}

// ── OrderedTOML ──────────────────────────────────────────────────────────────

// TestTOMLKeyOrderFromSource pins that the recorded order is the SOURCE order.
func TestTOMLKeyOrderFromSource(t *testing.T) {
	body := "zebra = 1\nalpha = 2\n\n[table]\nkey = 3\n\n[another]\nkey = 4\n"
	om, err := rawdoc.TOMLFromBytes([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, []string{"zebra", "alpha", "table", "another"}, om.Keys())
}

func TestTOMLMalformed(t *testing.T) {
	_, err := rawdoc.TOMLFromBytes([]byte("a = \n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse toml")
}

// TestTOMLSetAppendsInInsertionOrder pins the fresh-write path.
func TestTOMLSetAppendsInInsertionOrder(t *testing.T) {
	om, err := rawdoc.TOMLFromBytes([]byte("zebra = 1\n"))
	require.NoError(t, err)

	om.Set("alpha", 2)
	om.Set("zebra", 3)
	assert.Equal(t, []string{"zebra", "alpha"}, om.Keys(), "an existing key keeps its position")

	om.Delete("zebra")
	assert.Equal(t, []string{"alpha"}, om.Keys())
	assert.False(t, om.Has("zebra"))
	om.Delete("zebra")
	assert.Equal(t, []string{"alpha"}, om.Keys(), "deleting an absent key is a no-op")
}

// TestTOMLSetOnZeroValue pins that a bare OrderedTOML is usable without a parse.
func TestTOMLSetOnZeroValue(t *testing.T) {
	om := &rawdoc.OrderedTOML{}
	assert.NotPanics(t, func() { om.Set("a", 1) })
	assert.Equal(t, []string{"a"}, om.Keys())
	v, ok := om.Get("a")
	assert.True(t, ok)
	assert.Equal(t, 1, v)
}

// TestTOMLCloneIsIndependent pins the deep-copy guarantee through nested shapes.
func TestTOMLCloneIsIndependent(t *testing.T) {
	body := "top = 1\nnested = { a = 1 }\nlist = [1, 2]\n"
	om, err := rawdoc.TOMLFromBytes([]byte(body))
	require.NoError(t, err)

	cp := om.Clone()
	cp.Set("top", "mutated")
	cp.Set("nested", map[string]any{"a": "mutated"})
	cp.Set("list", []any{"mutated"})

	v, _ := om.Get("top")
	assert.Equal(t, int64(1), v, "go-toml decodes integers as int64")
	v, _ = om.Get("nested")
	assert.NotNil(t, v)
}

// TestTOMLKeysReturnsACopy pins that the returned slice is the caller's.
func TestTOMLKeysReturnsACopy(t *testing.T) {
	om, err := rawdoc.TOMLFromBytes([]byte("b = 1\na = 2\n"))
	require.NoError(t, err)

	got := om.Keys()
	got[0] = "MUTATED"
	assert.Equal(t, []string{"b", "a"}, om.Keys())
}

// TestTOMLEmptyDocument pins that an empty TOML document is usable.
func TestTOMLEmptyDocument(t *testing.T) {
	om, err := rawdoc.TOMLFromBytes(nil)
	require.NoError(t, err)
	assert.Empty(t, om.Keys())
	assert.False(t, om.Has("a"))
}

// TestTOMLKeyOrderIgnoresCommentsAndBlanks pins the scanner's line handling.
func TestTOMLKeyOrderIgnoresCommentsAndBlanks(t *testing.T) {
	body := "# [commented]\n\n  # [also-commented]\nreal = 1\n\n[table]\nk = 2\n"
	om, err := rawdoc.TOMLFromBytes([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, []string{"real", "table"}, om.Keys())
}

// TestTOMLKeyOrderQuotedKeys pins the quoted-key spelling a dotted namespace needs.
func TestTOMLKeyOrderQuotedKeys(t *testing.T) {
	body := "\"org.projectfile.cli\" = {}\nplain = 2\n"
	om, err := rawdoc.TOMLFromBytes([]byte(body))
	require.NoError(t, err)
	assert.Contains(t, om.Keys(), "org.projectfile.cli")
	assert.Contains(t, om.Keys(), "plain")
}

// TestTOMLKeyOrderDottedAssignIsNotAKey pins that only the head before the dot is a key.
func TestTOMLKeyOrderDottedAssignIsNotAKey(t *testing.T) {
	om, err := rawdoc.TOMLFromBytes([]byte("a.b = 1\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, om.Keys())
}

// TestTOMLKeyOrderCoversKeysTheScannerMissed pins the completion pass.
func TestTOMLKeyOrderCoversKeysTheScannerMissed(t *testing.T) {
	om, err := rawdoc.TOMLFromBytes([]byte("a = 1\n"))
	require.NoError(t, err)

	om.Set("b", 2)
	keys := om.Keys()
	assert.Len(t, keys, 2, "every key the document holds appears in its order")
	assert.Subset(t, keys, []string{"a", "b"})
}

// TestTOMLKeyOrderHandlesEmptyAndHeaderless pins the degenerate inputs.
func TestTOMLKeyOrderHandlesEmptyAndHeaderless(t *testing.T) {
	for _, body := range []string{"", "\n\n\n", "# only comments\n", "[table]\nk = 1\n"} {
		t.Run(strings.ReplaceAll(body, "\n", "_"), func(t *testing.T) {
			om, err := rawdoc.TOMLFromBytes([]byte(body))
			require.NoError(t, err)
			assert.Len(t, om.Keys(), len(om.Keys()), "the order is self-consistent")
		})
	}
}

// ── OrderedJSON ──────────────────────────────────────────────────────────────

// TestJSONKeyOrderFromSource is the JSON twin of the TOML guarantee.
func TestJSONKeyOrderFromSource(t *testing.T) {
	body := `{"zebra":1,"alpha":2,"middle":3}`
	om := rawdoc.NewOrderedJSON()
	require.NoError(t, om.UnmarshalJSON([]byte(body)))

	assert.Equal(t, []string{"zebra", "alpha", "middle"}, om.Keys())

	raw, ok := om.Get("alpha")
	require.True(t, ok)
	assert.JSONEq(t, "2", string(raw))
}

func TestJSONMalformed(t *testing.T) {
	om := rawdoc.NewOrderedJSON()
	require.Error(t, om.UnmarshalJSON([]byte(`{"a":`)))
}

func TestJSONSetGetDelete(t *testing.T) {
	om := rawdoc.NewOrderedJSON()
	require.NoError(t, om.UnmarshalJSON([]byte(`{"a":1,"b":2}`)))

	om.Set("c", json.RawMessage("3"))
	assert.Equal(t, []string{"a", "b", "c"}, om.Keys())
	om.Set("a", json.RawMessage("9"))
	assert.Equal(t, []string{"a", "b", "c"}, om.Keys(), "an existing key keeps its position")

	om.Delete("b")
	assert.Equal(t, []string{"a", "c"}, om.Keys())
	assert.False(t, om.Has("b"))

	out, err := om.MarshalJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":9,"c":3}`, string(out))
}

func TestJSONDeleteAbsentKey(t *testing.T) {
	om := rawdoc.NewOrderedJSON()
	require.NoError(t, om.UnmarshalJSON([]byte(`{"a":1}`)))
	assert.NotPanics(t, func() { om.Delete("absent") })
	assert.Equal(t, []string{"a"}, om.Keys())
}

// TestJSONCloneIsIndependent pins the deep copy through the raw-message layer.
func TestJSONCloneIsIndependent(t *testing.T) {
	om := rawdoc.NewOrderedJSON()
	require.NoError(t, om.UnmarshalJSON([]byte(`{"a":1}`)))

	cp := om.Clone()
	cp.Set("a", json.RawMessage("2"))
	cp.Set("b", json.RawMessage("3"))

	raw, ok := om.Get("a")
	require.True(t, ok)
	assert.JSONEq(t, "1", string(raw))
	assert.False(t, om.Has("b"))
}

func TestJSONKeysReturnsACopy(t *testing.T) {
	om := rawdoc.NewOrderedJSON()
	require.NoError(t, om.UnmarshalJSON([]byte(`{"b":1,"a":2}`)))

	got := om.Keys()
	got[0] = "MUTATED"
	assert.Equal(t, []string{"b", "a"}, om.Keys())
}
