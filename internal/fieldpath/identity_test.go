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

// ── identity rules ───────────────────────────────────────────────────────────

// Identity fixture keys and names the tables below repeat, one named home each.
const (
	keyIdentity = "identity"
	keyType     = "type"
	keyEmail    = "email"
	valWebsite  = "website"
	valNil      = "nil"
	valPlain    = "plain"
	valHigh     = "high"
	valDemoted  = "demoted"
)

// TestIdentityForRegistry pins the table: an unregistered address has no dedup rule.
func TestIdentityForRegistry(t *testing.T) {
	for _, path := range []string{
		keyKeywords, "technologies", keyRepositories, keyLinks,
		"people", "organizations", "includes",
	} {
		assert.NotNil(t, IdentityFor(path), "path %q must carry a dedup rule", path)
	}

	for _, path := range []string{"", "stack", "requirements", "org.projectfile.links", "People"} {
		assert.Nil(t, IdentityFor(path), "path %q must have no rule, so Add always appends", path)
	}
}

func TestStringIdentity(t *testing.T) {
	key, ok := IdentityFor(keyKeywords)("rust")
	assert.True(t, ok)
	assert.Equal(t, "rust", key)

	for _, item := range []any{1, nil, map[string]any{}, []string{"rust"}} {
		key, ok = IdentityFor(keyKeywords)(item)
		assert.False(t, ok, "item %v is not a scalar string", item)
		assert.Empty(t, key)
	}
}

// TestIncludeIdentity covers both spellings of an includes entry.
func TestIncludeIdentity(t *testing.T) {
	fn := IdentityFor("includes")

	key, ok := fn("https://example.com/a.yaml")
	assert.True(t, ok)
	assert.Equal(t, "https://example.com/a.yaml", key)

	key, ok = fn(map[string]any{testURL: "https://example.com/a.yaml", "sha256": "abc"})
	assert.True(t, ok)
	assert.Equal(t, "https://example.com/a.yaml", key, "the pin must not change the identity")

	for _, item := range []any{nil, 42, map[string]any{}, map[string]any{testURL: 7}} {
		_, ok = fn(item)
		assert.False(t, ok, "item %v carries no url", item)
	}
}

func TestURLIdentity(t *testing.T) {
	fn := IdentityFor(keyRepositories)

	key, ok := fn(map[string]any{testURL: testRepoURL, testRole: testOrigin})
	assert.True(t, ok)
	assert.Equal(t, "url:"+testRepoURL, key, "role and branch must not change the identity")

	items := []any{valPlain, nil, map[string]any{}, map[string]any{testURL: ""}, map[string]any{testRole: testOrigin}}
	for _, item := range items {
		_, ok = fn(item)
		assert.False(t, ok, "item %v carries no url", item)
	}
}

// TestTypeURLIdentity pins that a link is identified by the (type, url) tuple.
func TestTypeURLIdentity(t *testing.T) {
	fn := IdentityFor(keyLinks)

	key, ok := fn(map[string]any{keyType: valWebsite, testURL: testRepoURL})
	assert.True(t, ok)
	assert.Equal(t, "type=website|url="+testRepoURL, key)

	same, _ := fn(map[string]any{keyType: valWebsite, testURL: testRepoURL, "label": "Site"})
	assert.Equal(t, key, same, "a differing label must not fork the identity")

	other, _ := fn(map[string]any{keyType: "source", testURL: testRepoURL})
	assert.NotEqual(t, key, other)

	other, _ = fn(map[string]any{keyType: valWebsite, testURL: "https://other.test"})
	assert.NotEqual(t, key, other)

	for _, item := range []map[string]any{{keyType: valWebsite}, {testURL: testRepoURL}} {
		key, ok = fn(item)
		assert.True(t, ok, "item %v", item)
		assert.Contains(t, key, "=")
	}

	for _, item := range []any{valPlain, nil, map[string]any{}, map[string]any{"label": "Site"}} {
		_, ok = fn(item)
		assert.False(t, ok, "item %v has neither type nor url", item)
	}
}

// TestPersonIdentityKeyRule pins the precedence Add inherits from MergePeople.
func TestPersonIdentityKeyRule(t *testing.T) {
	fn := IdentityFor("people")

	byOrcid, ok := fn(map[string]any{"orcid": "https://orcid.org/0000-0002-1825-0097", keyEmail: testEmail})
	assert.True(t, ok)
	same, _ := fn(map[string]any{"orcid": "0000-0002-1825-0097", keyEmail: "b@x.test"})
	assert.Equal(t, byOrcid, same, "orcid outranks email")

	byEmail, ok := fn(map[string]any{keyEmail: testEmail, "family-names": "Buho", "given-names": "Damian"})
	assert.True(t, ok)
	assert.Contains(t, byEmail, testEmail)

	byName, ok := fn(map[string]any{"family-names": "Buho", "given-names": "Damian"})
	assert.True(t, ok)
	assert.Contains(t, byName, "name:damian buho", "the canonical name is lowercased")

	key, ok := fn(map[string]any{})
	assert.True(t, ok)
	assert.NotEmpty(t, key)

	_, ok = fn("not a map")
	assert.False(t, ok)
}

// TestOrganizationIdentityKeyRule is the entity twin of the person rule.
func TestOrganizationIdentityKeyRule(t *testing.T) {
	fn := IdentityFor("organizations")

	byName, ok := fn(map[string]any{keyName: "Kiota"})
	assert.True(t, ok)
	assert.Contains(t, byName, "name:kiota", "the canonical name is lowercased")

	byEmail, ok := fn(map[string]any{keyEmail: testEmail, keyName: "Kiota"})
	assert.True(t, ok)
	assert.NotEqual(t, byName, byEmail, "email outranks the name")

	_, ok = fn(map[string]any{})
	assert.True(t, ok)

	_, ok = fn(nil)
	assert.False(t, ok)
}

func TestStrField(t *testing.T) {
	m := map[string]any{"s": "v", "n": 7, valNil: nil}
	assert.Equal(t, "v", strField(m, "s"))
	for _, key := range []string{"n", valNil, "absent"} {
		assert.Equal(t, "", strField(m, key), "key %q is not a string", key)
	}
}

// ── fan-out ordering ─────────────────────────────────────────────────────────

// TestFanOutKeysOrdersByPriorityThenSpelling pins the ordering a rendered document depends on.
func TestFanOutKeysOrdersByPriorityThenSpelling(t *testing.T) {
	m := map[string]any{
		registryGHCR: map[string]any{testURL: "https://ghcr.io"},
		registryKiota: map[string]any{
			testURL:     "https://kiota.test",
			keyPriority: 90,
		},
		valAlpha:   map[string]any{testURL: "https://alpha.test"},
		"beta":     map[string]any{testURL: "https://beta.test"},
		valDemoted: map[string]any{testURL: "https://d.test", keyPriority: 10},
	}

	got := fanOutKeys(m)
	require.Len(t, got, 5)
	assert.Equal(t, registryKiota, got[0], "the highest priority leads regardless of spelling")
	assert.Equal(t, valDemoted, got[len(got)-1], "the lowest priority trails")
	assert.Equal(t, []string{valAlpha, "beta", registryGHCR}, got[1:4],
		"entries at the default rank fall back to the key ascending")
}

// TestFanOutKeysPriorityAcrossNumericTypes pins that every decoder's numeric type ranks the same.
func TestFanOutKeysPriorityAcrossNumericTypes(t *testing.T) {
	for name, priority := range map[string]any{
		"int":     60,
		"int64":   int64(60),
		"float64": float64(60),
	} {
		t.Run(name, func(t *testing.T) {
			m := map[string]any{
				valDemoted: map[string]any{keyPriority: 10},
				valHigh:    map[string]any{keyPriority: priority},
				valPlain:   map[string]any{},
			}
			assert.Equal(t, []string{valHigh, valPlain, valDemoted}, fanOutKeys(m),
				"priority 60 outranks the default rank of %d, which outranks 10", PriorityDefault)
		})
	}
}

// TestEntryPriorityDefaults pins the default rank and every shape that falls back to it.
func TestEntryPriorityDefaults(t *testing.T) {
	for name, in := range map[string]any{
		"scalar":     "not a map",
		valNil:       nil,
		"no key":     map[string]any{testURL: "x"},
		"wrong type": map[string]any{keyPriority: valHigh},
		"bool":       map[string]any{keyPriority: true},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, PriorityDefault, EntryPriority(in))
		})
	}

	assert.Equal(t, -3, EntryPriority(map[string]any{keyPriority: -3}),
		"a negative priority is honoured, not clamped")
	assert.Equal(t, 0, EntryPriority(map[string]any{keyPriority: 0}),
		"an explicit zero is a real rank, distinct from the default")
	assert.Equal(t, 50, PriorityDefault, "the default rank is one home for the number")
}

func TestFanOutKeysOnEmptyAndScalarMap(t *testing.T) {
	assert.Empty(t, fanOutKeys(map[string]any{}))
	assert.Equal(t, []string{"a"}, fanOutKeys(map[string]any{"a": "scalar"}))
}

// ── resolve helpers ──────────────────────────────────────────────────────────

// TestResolveValueAgainstAnyRoot is the raw-map entry point.
func TestResolveValueAgainstAnyRoot(t *testing.T) {
	root := map[string]any{
		keyIdentity: map[string]any{keyName: "demo"},
		keyLinks:    []any{map[string]any{keyType: valWebsite, testURL: testRepoURL}},
	}

	res, err := ResolveValue(root, Path{Segments: []Segment{{Kind: SegKey, Key: keyIdentity}, {Kind: SegKey, Key: keyName}}})
	require.NoError(t, err)
	v, ok := res.Single()
	require.True(t, ok)
	assert.Equal(t, "demo", v)

	res, err = ResolveValue(root, Path{Segments: []Segment{
		{Kind: SegKey, Key: keyLinks},
		{Kind: SegIndex, Index: 0},
		{Kind: SegKey, Key: testURL},
	}})
	require.NoError(t, err)
	v, ok = res.Single()
	require.True(t, ok)
	assert.Equal(t, testRepoURL, v)
}

// TestResolveValueEmptyPathReturnsRoot pins that an empty path yields the root itself.
func TestResolveValueEmptyPathReturnsRoot(t *testing.T) {
	root := map[string]any{"a": 1}
	res, err := ResolveValue(root, Path{})
	require.NoError(t, err)
	v, ok := res.Single()
	require.True(t, ok)
	assert.Equal(t, root, v)
}

func TestResolveValueMissingKey(t *testing.T) {
	_, err := ResolveValue(map[string]any{}, Path{Segments: []Segment{{Kind: SegKey, Key: "nope"}}})
	require.Error(t, err)
}

func TestResolveNilDocument(t *testing.T) {
	_, err := Resolve(nil, Path{Segments: []Segment{{Kind: SegKey, Key: keyIdentity}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil document")
}

// TestUnknownSegmentKindIsRejected pins that an unrecognised kind is an error, not an empty result.
func TestUnknownSegmentKindIsRejected(t *testing.T) {
	_, err := ResolveValue(map[string]any{"a": 1}, Path{Segments: []Segment{{Kind: SegKind(99)}}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown segment kind")
}

func TestResultSingle(t *testing.T) {
	_, ok := Result{}.Single()
	assert.False(t, ok, "an empty Result has no single value")

	v, ok := Result{Values: []any{"first", "second"}}.Single()
	assert.True(t, ok)
	assert.Equal(t, "first", v, "Single takes the first, not a consensus")

	v, ok = Result{Values: []any{nil}}.Single()
	assert.True(t, ok, "an explicit nil still counts as present")
	assert.Nil(t, v)
}

// ── raw output formatting ────────────────────────────────────────────────────

// TestFormatScalar pins the wire shapes `get --raw` emits per value kind.
func TestFormatScalar(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{valNil, nil, ""},
		{"string", valPlain, valPlain},
		{"empty string", "", ""},
		{"string with spaces is NOT quoted", "two words", "two words"},
		{"bool", true, "true"},
		{"int", 7, "7"},
		{"int32", int32(-3), "-3"},
		{"int64", int64(1 << 40), "1099511627776"},
		{"float32", float32(1.5), "1.5"},
		{"float64", 2.25, "2.25"},
		{"any list", []any{"a", 1}, "a 1"},
		{"string list", []string{"a", "b"}, "a b"},
		{"empty list", []any{}, ""},
		{"empty string list", []string{}, ""},
		{"unhandled kind falls through", struct{ A int }{A: 1}, "{1}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, FormatScalar(tc.in))
		})
	}
}

// TestFormatScalarMapIsKeySorted pins that a map leaf renders deterministically.
func TestFormatScalarMapIsKeySorted(t *testing.T) {
	m := map[string]any{"zebra": 1, valAlpha: "a", "middle": true}
	want := "alpha=a\nmiddle=true\nzebra=1"
	got := FormatScalar(m)
	assert.Equal(t, want, got)
	assert.Equal(t, got, FormatScalar(m), "map rendering must be stable across calls")
}

func TestFormatScalarEmptyMap(t *testing.T) {
	assert.Equal(t, "", FormatScalar(map[string]any{}))
}

// TestFormatScalarNested pins that a container inside a container recurses.
func TestFormatScalarNested(t *testing.T) {
	got := FormatScalar([]any{map[string]any{"a": 1}, []any{"x", "y"}, nil})
	assert.Equal(t, "a=1 x y ", got, "list members join on a space, and nil renders empty")
}

// TestFormatScalarRoundTripsThroughList pins that the []any and []string forms render identically.
func TestFormatScalarRoundTripsThroughList(t *testing.T) {
	anyForm := FormatScalar([]any{"a", "b", "c"})
	strForm := FormatScalar([]string{"a", "b", "c"})
	assert.Equal(t, anyForm, strForm)
	assert.NotContains(t, strForm, "\n", "a flat list stays on one line")
}

// ── bracket grammar ──────────────────────────────────────────────────────────

// TestUnquoteOnlyStripMatchingQuotes pins the minimal unquoting rule.
func TestUnquoteOnlyStripMatchingQuotes(t *testing.T) {
	assert.Equal(t, "a,b", unquote(`"a,b"`), "quoting protects a value with the grammar's own delimiters")
	assert.Equal(t, valPlain, unquote(valPlain))
	assert.Equal(t, `"unbalanced`, unquote(`"unbalanced`))
	assert.Equal(t, `balanced"`, unquote(`balanced"`))
	assert.Equal(t, `"`, unquote(`"`), "a single quote character is not a quoted string")
	assert.Empty(t, unquote(""))
}

// TestUnquoteInsideSelectors pins that a quoted predicate value reaches the matcher intact.
func TestUnquoteInsideSelectors(t *testing.T) {
	p, err := Parse(`links[type="source-code, x"].url`)
	require.NoError(t, err)
	require.Len(t, p.Segments, 3)
	require.Len(t, p.Segments[1].Preds, 1)
	assert.Equal(t, "source-code, x", p.Segments[1].Preds[0].Value)
}

// TestParsePresencePredicate pins the existence-only selector form.
func TestParsePresencePredicate(t *testing.T) {
	p, err := Parse("repositories[preferred].url")
	require.NoError(t, err)
	require.Len(t, p.Segments[1].Preds, 1)
	assert.Equal(t, "preferred", p.Segments[1].Preds[0].Key)
	assert.Empty(t, p.Segments[1].Preds[0].Value)

	res, err := Resolve(&projectfile.Document{
		Repositories: []projectfile.Repository{{URL: "https://a.test", Extra: map[string]any{"preferred": true}}},
	}, p)
	require.NoError(t, err)
	v, ok := res.Single()
	require.True(t, ok)
	assert.Equal(t, "https://a.test", v)
}

// TestParseEmptySelectorBody pins the degenerate bracket body.
func TestParseEmptySelectorBody(t *testing.T) {
	p, err := Parse("keywords[]")
	require.NoError(t, err)
	require.Len(t, p.Segments, 2)
	assert.Equal(t, SegProject, p.Segments[1].Kind, "an empty body is a projection")

	p, err = Parse("keywords[0].extra")
	require.NoError(t, err)
	assert.Equal(t, SegIndex, p.Segments[1].Kind)
	assert.Equal(t, 0, p.Segments[1].Index)
	assert.Equal(t, "extra", p.Segments[2].Key)
}

// TestParseRejectsUnclosedBrackets pins the grammar's error surface.
func TestParseRejectsUnclosedBrackets(t *testing.T) {
	for _, sel := range []string{
		"keywords[0", "keywords[0]]", "keywords[",
		"repositories[role=origin",
	} {
		t.Run(sel, func(t *testing.T) {
			_, err := Parse(sel)
			require.Error(t, err, "sel %q", sel)
		})
	}
}

// TestParseRejectsTrailingTextAfterBracket pins that only ONE bracket group may follow a key.
func TestParseRejectsTrailingTextAfterBracket(t *testing.T) {
	_, err := Parse("keywords[0] junk")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected text after bracket")
}

// TestQuotedSelectorsProtectTheGrammarsDelimiters pins why the grammar quotes at all.
func TestQuotedSelectorsProtectTheGrammarsDelimiters(t *testing.T) {
	for _, tc := range []struct {
		sel   string
		value string
	}{
		{`links[type="a]b"].url`, "a]b"},
		{`links[type="a}b"].url`, "a}b"},
		{`links[type="a,b"].url`, "a,b"},
		{`links[type="a=b"].url`, "a=b"},
		{`links[type="a.b"].url`, "a.b"},
		{`links[type="a[b]c"].url`, "a[b]c"},
	} {
		t.Run(tc.sel, func(t *testing.T) {
			p, err := Parse(tc.sel)
			require.NoError(t, err, "the quotes must protect the delimiter")

			doc := &projectfile.Document{Links: []projectfile.Link{
				{Type: tc.value, URL: "https://matched.test"},
				{Type: "other", URL: "https://other.test"},
			}}
			res, err := Resolve(doc, p)
			require.NoError(t, err)
			v, ok := res.Single()
			require.True(t, ok)
			assert.Equal(t, "https://matched.test", v, "the quoted value must match literally")
		})
	}
}

// TestUnterminatedQuoteIsRejected pins that a half-typed quoted value says so.
func TestUnterminatedQuoteIsRejected(t *testing.T) {
	sel := `links[type="a].url`
	_, err := Parse(sel)
	require.Error(t, err, "an unterminated quote must not parse")
}

// TestQuotedDotIsNotAPathSeparator pins that the dot stays inside the quoted value.
func TestQuotedDotIsNotAPathSeparator(t *testing.T) {
	p, err := Parse(`map{k="a.b"}.value`)
	require.NoError(t, err)
	require.Len(t, p.Segments, 3)
	assert.Equal(t, "map", p.Segments[0].Key)
	assert.Equal(t, "a.b", p.Segments[1].Preds[0].Value)
	assert.Equal(t, "value", p.Segments[2].Key)
}

// TestQuotedCommaIsNotAPredicateSeparator is the twin for the predicate parser.
func TestQuotedCommaIsNotAPredicateSeparator(t *testing.T) {
	p, err := Parse(`list[a="x,y", b=2]`)
	require.NoError(t, err)
	require.Len(t, p.Segments, 2)
	require.Equal(t, SegSelector, p.Segments[1].Kind)
	require.Len(t, p.Segments[1].Preds, 2, "a quoted comma must not fork a second predicate")
	assert.Equal(t, "a", p.Segments[1].Preds[0].Key)
	assert.Equal(t, "x,y", p.Segments[1].Preds[0].Value)
	assert.Equal(t, "b", p.Segments[1].Preds[1].Key)
	assert.Equal(t, "2", p.Segments[1].Preds[1].Value)
}

// TestFindBracketEndRejectsANonBracket pins the guard that keeps the scanner from guessing.
func TestFindBracketEndRejectsANonBracket(t *testing.T) {
	assert.Equal(t, -1, findBracketEnd(""), "an empty body has no end")
	assert.Equal(t, -1, findBracketEnd("abc]"), "a body opening with neither bracket has no end")
	assert.Equal(t, 1, findBracketEnd("[]"))
	assert.Equal(t, 1, findBracketEnd("{}"))
}

// TestFindBracketEndSeesInsideQuotes pins that a quoted closer does not terminate the scan.
func TestFindBracketEndSeesInsideQuotes(t *testing.T) {
	assert.Equal(t, 6, findBracketEnd(`[a="]"]`), "a quoted ] must not close the bracket")
	assert.Equal(t, -1, findBracketEnd(`[a="]`), "an unterminated quote means no close was found")
}

// TestBracketEndBoundaries pins the close index at its first and last positions.
func TestBracketEndBoundaries(t *testing.T) {
	assert.Equal(t, 1, findBracketEnd("[]"), "an empty body closes immediately")
	assert.Equal(t, 2, findBracketEnd("[0]"))
	assert.Equal(t, 3, findBracketEnd("[10]"))
	assert.Equal(t, -1, findBracketEnd("[0"), "an unclosed body is not addressable")
}
