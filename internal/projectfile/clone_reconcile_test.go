// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Clone: deep-copy independence ────────────────────────────────────────────

// TestCloneIsolatesLocalizedStrings pins that a clone's Langs map is its own.
func TestCloneIsolatesLocalizedStrings(t *testing.T) {
	src := &Document{
		Identity: Identity{
			Title:   &LocalizedString{Langs: map[string]string{"en": "Hi", "es": "Hola"}},
			Summary: &LocalizedString{Bare: "Bare"},
		},
	}
	got := src.Clone()

	require.NotNil(t, got.Identity.Title)
	got.Identity.Title.Langs["en"] = "MUTATED"
	got.Identity.Title.Langs["fr"] = "added"

	assert.Equal(t, "Hi", src.Identity.Title.Langs["en"], "the source map must be untouched")
	assert.NotContains(t, src.Identity.Title.Langs, "fr")
	assert.Equal(t, "Bare", got.Identity.Summary.Bare, "a Bare field clones without a Langs map")
	assert.Nil(t, got.Identity.Summary.Langs)
}

func TestCloneNilLocalizedStringStaysNil(t *testing.T) {
	assert.Nil(t, (&Document{}).Clone().Identity.Title)
}

// TestCloneIsolatesRequirements pins the same for the Requirements maps.
func TestCloneIsolatesRequirements(t *testing.T) {
	src := &Document{
		Requirements: &Requirements{
			Browsers: []any{"chrome", "firefox"},
			Runtime:  map[string]string{"node": ">=24"},
			Extra:    map[string]any{"deep": map[string]any{"k": "v"}},
		},
	}
	got := src.Clone()

	got.Requirements.Runtime["node"] = ">=18"
	got.Requirements.Browsers.([]any)[0] = "safari"
	got.Requirements.Extra["deep"].(map[string]any)["k"] = "MUTATED"

	assert.Equal(t, ">=24", src.Requirements.Runtime["node"])
	assert.Equal(t, "chrome", src.Requirements.Browsers.([]any)[0])
	assert.Equal(t, "v", src.Requirements.Extra["deep"].(map[string]any)["k"])
}

func TestCloneNilRequirementsStaysNil(t *testing.T) {
	assert.Nil(t, (&Document{}).Clone().Requirements)
}

// TestCloneAnyCoversEveryShape walks the untyped deep-copy switch.
func TestCloneAnyCoversEveryShape(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   any
	}{
		{"nil", nil},
		{"string", "s"},
		{"int", 7},
		{"bool", true},
		{"float", 1.5},
		{"empty map", map[string]any{}},
		{"empty list", []any{}},
		{"map", map[string]any{"a": 1}},
		{"list", []any{1, "two"}},
		{"nested", map[string]any{"a": []any{map[string]any{"b": "c"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.in, cloneAny(tc.in))
		})
	}
}

// TestCloneAnyDeepCopyIsIndependent pins that the nested copy aliases nothing.
func TestCloneAnyDeepCopyIsIndependent(t *testing.T) {
	src := map[string]any{"a": []any{map[string]any{"b": "v"}}}
	got := cloneAny(src).(map[string]any)

	got["a"].([]any)[0].(map[string]any)["b"] = "MUTATED"
	got["a"] = append(got["a"].([]any), "extra")

	assert.Equal(t, "v", src["a"].([]any)[0].(map[string]any)["b"])
	assert.Len(t, src["a"].([]any), 1)
}

func TestCloneNilSlicesStayNil(t *testing.T) {
	got := (&Document{}).Clone()
	assert.Nil(t, got.Keywords)
	assert.Nil(t, got.Stack)
	assert.Nil(t, got.Links)
	assert.Nil(t, got.People)
	assert.Nil(t, got.Repositories)
}

// TestCloneIncludePinsIsIndependent pins the pin map clone.
func TestCloneIncludePinsIsIndependent(t *testing.T) {
	src := &Document{IncludePins: map[string]string{"https://a.test": "digest-a"}}
	got := src.Clone()

	got.IncludePins["https://a.test"] = "MUTATED"
	got.IncludePins["https://b.test"] = "added"

	assert.Equal(t, "digest-a", src.IncludePins["https://a.test"])
	assert.NotContains(t, src.IncludePins, "https://b.test")
}

// TestCloneNilIncludePinsStaysNil pins that an absent pin map does not become an empty one.
func TestCloneNilIncludePinsStaysNil(t *testing.T) {
	assert.Nil(t, (&Document{}).Clone().IncludePins)
}

// ── reconcile ────────────────────────────────────────────────────────────────

// TestReconcileOrgsAppliesAChangedOrg is the entity twin of the person path.
func TestReconcileOrgsAppliesAChangedOrg(t *testing.T) {
	base := &Document{Organizations: []Organization{{Name: "Kiota", URL: "https://old.test"}}}
	pre := &Document{Organizations: []Organization{
		{Name: "Kiota", URL: "https://old.test"},
		{Name: "Include Org", URL: "https://inc.test"},
	}}
	post := pre.Clone()
	post.Organizations[0].URL = "https://new.test"

	ReconcileBase(base, pre, post)
	require.Len(t, base.Organizations, 1)
	assert.Equal(t, "https://new.test", base.Organizations[0].URL)
	assert.Equal(t, "Kiota", base.Organizations[0].Name)
	assert.NotContains(t, orgNames(base), "Include Org", "an untouched include must not be materialised")
}

func orgNames(d *Document) []string {
	out := make([]string, len(d.Organizations))
	for i, o := range d.Organizations {
		out[i] = o.Name
	}
	return out
}

// TestReconcileOrgsAppendsNewOnes covers the growth path.
func TestReconcileOrgsAppendsNewOnes(t *testing.T) {
	base := &Document{Organizations: []Organization{{Name: "Kiota"}}}
	pre := &Document{Organizations: []Organization{{Name: "Kiota"}}}
	post := pre.Clone()
	post.Organizations = append(post.Organizations, Organization{Name: "New Co"})

	ReconcileBase(base, pre, post)
	require.Len(t, base.Organizations, 2)
	assert.Equal(t, "New Co", base.Organizations[1].Name)
}

// TestReconcileOrgsAppendsViaIdentityMerge pins that an appended org folds into base.
func TestReconcileOrgsAppendsViaIdentityMerge(t *testing.T) {
	base := &Document{Organizations: []Organization{{Name: "Kiota", URL: "https://kiota.test"}}}
	pre := &Document{Organizations: []Organization{{Name: "Kiota"}}}
	post := pre.Clone()
	post.Organizations = append(post.Organizations,
		Organization{Name: "Kiota", URL: "https://kiota.test", Alias: "K"})

	ReconcileBase(base, pre, post)
	require.Len(t, base.Organizations, 1, "the same identity must not be appended twice")
	assert.Equal(t, "K", base.Organizations[0].Alias)
}

func TestReconcileOrgsShrinkIsNotApplied(t *testing.T) {
	base := &Document{Organizations: []Organization{{Name: "Kiota"}}}
	pre := &Document{Organizations: []Organization{{Name: "Kiota"}, {Name: "Gone"}}}
	post := &Document{}

	ReconcileBase(base, pre, post)
	assert.Len(t, base.Organizations, 1, "reconcile only adds what the operation touched")
}

// TestReconcileReposDetectsAURLChange pins why the repo reconciler matches on the PRE-sync URL.
func TestReconcileReposDetectsAURLChange(t *testing.T) {
	base := &Document{Repositories: []Repository{{URL: "https://old.test", Role: "origin"}}}
	pre := &Document{Repositories: []Repository{{URL: "https://old.test", Role: "origin"}}}
	post := pre.Clone()
	post.Repositories[0].URL = "https://new.test"

	ReconcileBase(base, pre, post)
	require.Len(t, base.Repositories, 1)
	assert.Equal(t, "https://new.test", base.Repositories[0].URL)
}

func TestReconcileReposAppendsNew(t *testing.T) {
	base := &Document{Repositories: []Repository{{URL: "https://old.test"}}}
	pre := &Document{Repositories: []Repository{{URL: "https://old.test"}}}
	post := pre.Clone()
	post.Repositories = append(post.Repositories, Repository{URL: "https://added.test", Role: "mirror"})

	ReconcileBase(base, pre, post)
	require.Len(t, base.Repositories, 2)
	assert.Equal(t, "https://added.test", base.Repositories[1].URL)
}

// TestRepoURLExists pins the dedup helper reconcileRepos uses.
func TestRepoURLExists(t *testing.T) {
	repos := []Repository{{URL: "https://a.test"}, {URL: "https://b.test"}}
	assert.True(t, repoURLExists(repos, "https://a.test"))
	assert.True(t, repoURLExists(repos, "https://b.test"))
	assert.False(t, repoURLExists(repos, "https://c.test"))
	assert.False(t, repoURLExists(nil, "https://a.test"))
	assert.False(t, repoURLExists(repos, ""), "an empty URL matches nothing")
}

func TestLinkExists(t *testing.T) {
	links := []Link{{Type: "website", URL: "https://a.test"}}
	assert.True(t, linkExists(links, "website", "https://a.test"))
	assert.False(t, linkExists(links, "source-code", "https://a.test"), "the type is part of the key")
	assert.False(t, linkExists(links, "website", "https://b.test"), "the url is part of the key")
	assert.False(t, linkExists(nil, "website", "https://a.test"))
}

// TestReconcileCopyright lands the whole record only when the operation changed it.
func TestReconcileCopyright(t *testing.T) {
	base := &Document{}
	pre := &Document{Copyright: &Copyright{Year: 1999}}
	post := pre.Clone()
	ReconcileBase(base, pre, post)
	assert.Nil(t, base.Copyright, "an untouched include copyright is not materialised")

	base = &Document{}
	post.Copyright.Year = 2026
	ReconcileBase(base, pre, post)
	require.NotNil(t, base.Copyright)
	assert.Equal(t, 2026, base.Copyright.Year)
}

func TestReconcileNilDocumentsAreSafe(t *testing.T) {
	assert.NotPanics(t, func() {
		ReconcileBase(nil, nil, nil)
		ReconcileBase(&Document{}, nil, nil)
		ReconcileBase(&Document{}, &Document{}, nil)
	})
}

// TestReconcileRequirementsGranularity pins that runtime is reconciled as a whole map.
func TestReconcileRequirementsGranularity(t *testing.T) {
	base := &Document{}
	pre := &Document{Requirements: &Requirements{
		Runtime:  map[string]string{"node": ">=24"},
		Browsers: []any{"chrome"},
	}}
	post := pre.Clone()
	post.Requirements.Runtime["go"] = ">=1.25"

	ReconcileBase(base, pre, post)
	require.NotNil(t, base.Requirements)
	assert.Equal(t, ">=1.25", base.Requirements.Runtime["go"])
	assert.Nil(t, base.Requirements.Browsers, "an unchanged sibling field is not materialised")
}

// TestReconcileRequirementsNilPostClearsBase covers an operation that removed the block.
func TestReconcileRequirementsNilPostClearsBase(t *testing.T) {
	base := &Document{Requirements: &Requirements{Runtime: map[string]string{"node": ">=24"}}}
	pre := &Document{Requirements: &Requirements{Runtime: map[string]string{"node": ">=24"}}}

	ReconcileBase(base, pre, &Document{})
	assert.Nil(t, base.Requirements, "a removed requirements block lands in base")
}

// scalarIntermediate is an extension map whose middle segment is a scalar.
func scalarIntermediate() map[string]any {
	return map[string]any{"org": map[string]any{"projectfile": "scalar"}}
}

// emptyCliLeaf is an extension map whose leaf holds no keys at all.
func emptyCliLeaf() map[string]any {
	return map[string]any{"org": map[string]any{"projectfile": map[string]any{"cli": map[string]any{}}}}
}

// TestReconcileUnevenLengthsWalkExactlyTheOverlap pins the loop bound; one index further would overrun post.
func TestReconcileUnevenLengthsWalkExactlyTheOverlap(t *testing.T) {
	t.Run("people", func(t *testing.T) {
		base := &Document{People: []Person{p("Alice", "Smith"), p("Bob", "Jones")}}
		pre := &Document{People: []Person{p("Alice", "Smith"), p("Bob", "Jones")}}
		post := &Document{People: []Person{{GivenNames: "Alice", FamilyNames: "Smith", Email: "new@x.test"}}}

		ReconcileBase(base, pre, post)
		require.Len(t, base.People, 2, "the reconciler only edits; it never removes")
		assert.Equal(t, "new@x.test", base.People[0].Email, "the overlapping index is reconciled")
		assert.Equal(t, "Jones", base.People[1].FamilyNames, "past the overlap nothing is touched")
	})

	t.Run("organizations", func(t *testing.T) {
		base := &Document{Organizations: []Organization{{Name: "A"}, {Name: "B"}}}
		pre := &Document{Organizations: []Organization{{Name: "A"}, {Name: "B"}}}
		post := &Document{Organizations: []Organization{{Name: "A", Email: "new@a.test"}}}

		ReconcileBase(base, pre, post)
		require.Len(t, base.Organizations, 2)
		assert.Equal(t, "new@a.test", base.Organizations[0].Email)
	})

	t.Run("repositories", func(t *testing.T) {
		base := &Document{Repositories: []Repository{{URL: "https://a.test"}, {URL: "https://b.test"}}}
		pre := &Document{Repositories: []Repository{{URL: "https://a.test"}, {URL: "https://b.test"}}}
		post := &Document{Repositories: []Repository{{URL: "https://a.test", Branch: "main"}}}

		ReconcileBase(base, pre, post)
		require.Len(t, base.Repositories, 2)
		assert.Equal(t, "main", base.Repositories[0].Branch)
	})

	t.Run("links", func(t *testing.T) {
		base := &Document{Links: []Link{{Type: "a", URL: "https://a.test"}, {Type: "b", URL: "https://b.test"}}}
		pre := &Document{Links: []Link{{Type: "a", URL: "https://a.test"}, {Type: "b", URL: "https://b.test"}}}
		post := &Document{Links: []Link{{Type: "a", URL: "https://a.test", Preferred: true}}}

		ReconcileBase(base, pre, post)
		require.Len(t, base.Links, 2)
		assert.True(t, base.Links[0].Preferred)
	})
}

// TestReconcileEvenLengthsWalkEveryIndex pins that a change at the last index still lands.
func TestReconcileEvenLengthsWalkEveryIndex(t *testing.T) {
	base := &Document{People: []Person{p("Alice", "Smith"), p("Bob", "Jones")}}
	pre := &Document{People: []Person{p("Alice", "Smith"), p("Bob", "Jones")}}
	post := &Document{People: []Person{
		p("Alice", "Smith"),
		{GivenNames: "Bob", FamilyNames: "Jones", Email: "bob@new.test"},
	}}

	ReconcileBase(base, pre, post)
	require.Len(t, base.People, 2)
	assert.Equal(t, "bob@new.test", base.People[1].Email)

	links := &Document{Links: []Link{{Type: "a", URL: "https://a.test"}, {Type: "b", URL: "https://b.test"}}}
	ReconcileBase(links,
		&Document{Links: []Link{{Type: "a", URL: "https://a.test"}, {Type: "b", URL: "https://b.test"}}},
		&Document{Links: []Link{{Type: "a", URL: "https://a.test"}, {Type: "b", URL: "https://b2.test"}}})
	require.Len(t, links.Links, 2)
	assert.Equal(t, "https://b2.test", links.Links[1].URL)
}

// TestReconcileNewEntriesLandForEveryEntity pins the growth branch of each reconciler.
func TestReconcileNewEntriesLandForEveryEntity(t *testing.T) {
	people := &Document{People: []Person{p("Alice", "Smith")}}
	ReconcileBase(people,
		&Document{People: []Person{p("Alice", "Smith")}},
		&Document{People: []Person{p("Alice", "Smith"), {Email: "new@x.test"}}})
	assert.Len(t, people.People, 2)

	orgs := &Document{Organizations: []Organization{{Name: "A"}}}
	ReconcileBase(orgs,
		&Document{Organizations: []Organization{{Name: "A"}}},
		&Document{Organizations: []Organization{{Name: "A"}, {Name: "New"}}})
	assert.Len(t, orgs.Organizations, 2)

	links := &Document{Links: []Link{{Type: "a", URL: "https://a.test"}}}
	ReconcileBase(links,
		&Document{Links: []Link{{Type: "a", URL: "https://a.test"}}},
		&Document{Links: []Link{{Type: "a", URL: "https://a.test"}, {Type: "new", URL: "https://n.test"}}})
	assert.Len(t, links.Links, 2)

	repos := &Document{Repositories: []Repository{{URL: "https://a.test"}}}
	ReconcileBase(repos,
		&Document{Repositories: []Repository{{URL: "https://a.test"}}},
		&Document{Repositories: []Repository{{URL: "https://a.test"}, {URL: "https://n.test"}}})
	assert.Len(t, repos.Repositories, 2)
}

// ── merge internals ──────────────────────────────────────────────────────────

// TestMergePersonFieldsIgnoresAnEmptyHandleMap pins that a non-nil empty map is not "absent".
func TestMergePersonFieldsIgnoresAnEmptyHandleMap(t *testing.T) {
	cur := Person{FamilyNames: "Smith", Handles: map[string]any{"github": "alice"}}
	in := Person{Handles: map[string]any{}}

	got, conflicts := mergePersonFields(cur, in, "email:alice@example.com", nil)
	assert.Equal(t, map[string]any{"github": "alice"}, got.Handles)
	assert.Empty(t, conflicts)
}

// TestMergeOrgFieldsIgnoresAnEmptyHandleMap is the entity twin.
func TestMergeOrgFieldsIgnoresAnEmptyHandleMap(t *testing.T) {
	cur := Organization{Name: "Kiota", Handles: map[string]any{"github": "kiota"}}
	in := Organization{Handles: map[string]any{}}

	got, conflicts := mergeOrgFields(cur, in, "name:kiota", nil)
	assert.Equal(t, map[string]any{"github": "kiota"}, got.Handles)
	assert.Empty(t, conflicts)
}

// TestMergePersonFieldsAllocatesHandlesOnDemand pins the create branch.
func TestMergePersonFieldsAllocatesHandlesOnDemand(t *testing.T) {
	got, _ := mergePersonFields(Person{Email: "a@x.test"},
		Person{Handles: map[string]any{"github": "alice"}}, "email:a@x.test", nil)
	require.NotNil(t, got.Handles)
	assert.Equal(t, "alice", got.Handles["github"])
}

// TestMergePersonFieldsKeepsAnExistingHandle pins the collision branch.
func TestMergePersonFieldsKeepsAnExistingHandle(t *testing.T) {
	got, _ := mergePersonFields(
		Person{Handles: map[string]any{"github": "alice"}},
		Person{Handles: map[string]any{"github": "someone-else"}},
		"email:a@x.test", nil)
	assert.Equal(t, "alice", got.Handles["github"])
}

// TestCanonicalPersonNamePartOrder pins the tier-3 assembly order.
func TestCanonicalPersonNamePartOrder(t *testing.T) {
	full := Person{GivenNames: "Alice", NameParticle: "de", FamilyNames: "Smith", NameSuffix: "Jr"}
	assert.Equal(t, "alice de smith jr", canonicalPersonName(full))

	assert.Equal(t, "alice de smith", canonicalPersonName(Person{
		GivenNames: "Alice", NameParticle: "de", FamilyNames: "Smith",
	}))

	assert.Equal(t, "alice smith", canonicalPersonName(Person{GivenNames: "Alice", FamilyNames: "Smith"}))
	assert.Equal(t, "smith", canonicalPersonName(Person{FamilyNames: "Smith"}))
	assert.Equal(t, "alice", canonicalPersonName(Person{GivenNames: "Alice"}))
	assert.Empty(t, canonicalPersonName(Person{}))
}

// TestCanonicalPersonNameRawIsTheTypedTwin pins the raw-map twin agrees with the typed form.
func TestCanonicalPersonNameRawIsTheTypedTwin(t *testing.T) {
	raw := map[string]any{
		"given-names": "Alice", "name-particle": "de",
		"family-names": "Smith", "name-suffix": "Jr",
	}
	assert.Equal(t, canonicalPersonName(Person{
		GivenNames: "Alice", NameParticle: "de", FamilyNames: "Smith", NameSuffix: "Jr",
	}), canonicalPersonNameRaw(raw))

	assert.Equal(t, "display wins", canonicalPersonNameRaw(map[string]any{
		"display-name": "Display Wins", "family-names": "Ignored",
	}), "an explicit display name is the strongest name form")

	assert.Empty(t, canonicalPersonNameRaw(map[string]any{}))
}

// TestUnionRolesDropsEmptyAndDuplicates pins the set-union.
func TestUnionRoles(t *testing.T) {
	assert.Equal(t, []string{"author"}, unionRoles([]string{"author", "author", ""}, nil))
	assert.Equal(t, []string{"maintainer", "author"}, unionRoles([]string{"maintainer"}, []string{"author", "maintainer"}))
	assert.Empty(t, unionRoles(nil, nil))
	assert.Equal(t, []string{"a"}, unionRoles([]string{"", "a"}, []string{"", ""}))
}

// TestMergeRawEntityDirection pins the include-merge direction.
func TestMergeRawEntityDirection(t *testing.T) {
	loser := map[string]any{"name": "from-include", "role": "author"}
	winner := map[string]any{"name": "from-base"}

	got := mergeRawEntity(loser, winner)
	assert.Equal(t, "from-base", got["name"], "the base document wins")
	assert.Equal(t, "author", got["role"], "a field the base omits is filled from the include")

	got = mergeRawEntity(map[string]any{"email": "a@x.test"}, map[string]any{"email": ""})
	assert.Equal(t, "a@x.test", got["email"], "a string keeps the loser when the winner is empty")

	got = mergeRawEntity(map[string]any{"n": 1}, map[string]any{"n": 0})
	assert.Equal(t, 0, got["n"], "a non-string scalar is replaced by the winner outright")

	assert.Empty(t, mergeRawEntity(map[string]any{}, map[string]any{}))
}

// ── extension pruning ────────────────────────────────────────────────────────

// TestPruneNestedExtensionRemovesTheWholeChain pins that an emptied chain goes to its root.
func TestPruneNestedExtensionRemovesTheWholeChain(t *testing.T) {
	m := map[string]any{
		"org": map[string]any{
			"projectfile": map[string]any{
				"cli": map[string]any{"only": 1},
			},
		},
	}
	pruneNestedExtension(m, "org.projectfile.cli.only")
	assert.Empty(t, m)
}

// TestPruneNestedExtensionKeepsSiblings pins that a surviving sibling keeps its ancestors.
func TestPruneNestedExtensionKeepsSiblings(t *testing.T) {
	m := map[string]any{
		"org": map[string]any{
			"projectfile": map[string]any{
				"cli": map[string]any{"only": 1},
				"ci":  map[string]any{"nodes": 1},
			},
		},
	}
	pruneNestedExtension(m, "org.projectfile.cli.only")

	pf := m["org"].(map[string]any)["projectfile"].(map[string]any)
	assert.NotContains(t, pf, "cli", "the emptied namespace goes")
	assert.Contains(t, pf, "ci", "a surviving sibling keeps its ancestors alive")
}

// TestPruneNestedExtensionIgnoresAbsentShapes pins every no-op guard.
func TestPruneNestedExtensionIgnoresAbsentShapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    map[string]any
		ns   string
	}{
		{"single segment", map[string]any{"a": map[string]any{"b": 1}}, "a"},
		{"head is not a map", map[string]any{"org": "scalar"}, "org.projectfile.cli"},
		{"missing intermediate", map[string]any{"org": map[string]any{}}, "org.projectfile.cli.x"},
		{"intermediate is not a map", scalarIntermediate(), "org.projectfile.cli.x"},
		{"leaf absent", emptyCliLeaf(), "org.projectfile.cli.absent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := len(mustJSON(t, tc.m))
			pruneNestedExtension(tc.m, tc.ns)
			assert.Equal(t, before, len(mustJSON(t, tc.m)), "a shape this cannot prune is left alone")
		})
	}
}

// TestPruneNestedExtensionThreeSegments pins the chain walk on a deeper namespace.
func TestPruneNestedExtensionThreeSegments(t *testing.T) {
	m := map[string]any{
		"com": map[string]any{
			"example": map[string]any{
				"build": map[string]any{"only": 1},
			},
		},
	}
	pruneNestedExtension(m, "com.example.build.only")
	assert.Empty(t, m)
}

// TestPruneNestedExtensionTwoSegmentChainHasNoMiddle pins the chain loop's lower bound.
func TestPruneNestedExtensionTwoSegmentChainHasNoMiddle(t *testing.T) {
	m := map[string]any{"com": map[string]any{"example": map[string]any{"only": 1}}}
	pruneNestedExtension(m, "com.example.only")
	assert.Empty(t, m)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

// ── parse / serialize helpers ────────────────────────────────────────────────

// TestToStrSliceAcceptsBothDecodedShapes pins both the []any and []string shapes.
func TestToStrSliceAcceptsBothDecodedShapes(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, toStrSlice([]any{"a", "b"}))
	assert.Equal(t, []string{"a", "b"}, toStrSlice([]string{"a", "b"}))
	assert.Equal(t, []string{"a"}, toStrSlice([]any{"a", 7, nil}), "non-strings are dropped")
	assert.Nil(t, toStrSlice(nil))
	assert.Nil(t, toStrSlice("scalar"), "a bare string is not a list")
}

func TestListVal(t *testing.T) {
	got, ok := listVal(map[string]any{"a": []any{1, "two"}}, "a")
	assert.True(t, ok)
	assert.Equal(t, []any{1, "two"}, got)

	got, ok = listVal(map[string]any{"a": []map[string]any{{"x": "y"}}}, "a")
	assert.True(t, ok, "ToMap's []map[string]any shape is accepted too")
	assert.Equal(t, []any{map[string]any{"x": "y"}}, got)

	got, ok = listVal(map[string]any{}, "a")
	assert.False(t, ok)
	assert.Empty(t, got)

	got, ok = listVal(map[string]any{"a": "scalar"}, "a")
	assert.False(t, ok, "a scalar is not a list")
	assert.Empty(t, got)

	got, ok = listVal(nil, "a")
	assert.False(t, ok)
	assert.Empty(t, got)
}

// TestToAnySliceAcceptsBothShapes pins the include-entry list widening.
func TestToAnySliceAcceptsBothShapes(t *testing.T) {
	assert.Equal(t, []any{"a"}, toAnySlice([]string{"a"}))
	assert.Equal(t, []any{"a"}, toAnySlice([]any{"a"}))
	assert.Empty(t, toAnySlice(nil))
	assert.Empty(t, toAnySlice("scalar"))
}

// TestPeopleToMapListRoundTrip is the load-bearing guard for the entity merge.
func TestPeopleToMapListRoundTrip(t *testing.T) {
	in := []Person{
		{GivenNames: "Alice", FamilyNames: "Smith", Email: "alice@example.com", Roles: []string{"author"}},
		{DisplayName: "Only A Display Name", Orcid: "0000-0002-1825-0097"},
		{Email: "handles@example.com", Handles: map[string]any{"github": "handles"}},
	}
	got := peopleToMapList(in)
	require.Len(t, got, 3)

	back := parsePeople(listOf(got))
	require.Len(t, back, 3)
	assert.Equal(t, "Alice", back[0].GivenNames)
	assert.Equal(t, "alice@example.com", back[0].Email)
	assert.Equal(t, "author", back[0].Roles[0])
	assert.Equal(t, "0000-0002-1825-0097", back[1].Orcid)
	assert.Equal(t, "handles", back[2].Handles["github"])
}

func TestPeopleToMapListEmptyAndNil(t *testing.T) {
	assert.Empty(t, peopleToMapList(nil))
	assert.Empty(t, peopleToMapList([]Person{}))
	assert.Empty(t, parsePeople(nil))
}

// TestOrganizationsToMapListRoundTrip is the organization twin.
func TestOrganizationsToMapListRoundTrip(t *testing.T) {
	in := []Organization{
		{Name: "Kiota", Email: "a@kiota.test", Roles: []string{"maintainer"}},
		{Name: "Handles Co", Handles: map[string]any{"github": "co"}},
	}
	got := organizationsToMapList(in)
	require.Len(t, got, 2)

	back := parseOrganizations(listOf(got))
	require.Len(t, back, 2)
	assert.Equal(t, "Kiota", back[0].Name)
	assert.Equal(t, "a@kiota.test", back[0].Email)
	assert.Equal(t, "maintainer", back[0].Roles[0])
	assert.Equal(t, "co", back[1].Handles["github"])
}

func TestOrganizationsToMapListEmptyAndNil(t *testing.T) {
	assert.Empty(t, organizationsToMapList(nil))
	assert.Empty(t, organizationsToMapList([]Organization{}))
	assert.Empty(t, parseOrganizations(nil))
}

// listOf adapts a serialised entity list back to the parse-time []any shape.
func listOf(in []map[string]any) []any {
	out := make([]any, len(in))
	for i, m := range in {
		out[i] = m
	}
	return out
}

// TestParsePeopleSkipsNonMapEntries pins that one bad row does not lose the rest.
func TestParsePeopleSkipsNonMapEntries(t *testing.T) {
	got := parsePeople([]any{
		map[string]any{"given-names": "Alice"},
		"scalar",
		42,
		map[string]any{"given-names": "Bob"},
	})
	require.Len(t, got, 2)
	assert.Equal(t, "Alice", got[0].GivenNames)
	assert.Equal(t, "Bob", got[1].GivenNames)
}

func TestParseOrganizationsSkipsNonMapEntries(t *testing.T) {
	got := parseOrganizations([]any{"scalar", map[string]any{"name": "Kiota"}})
	require.Len(t, got, 1)
	assert.Equal(t, "Kiota", got[0].Name)
}

// TestPersonIdentityKeyPrecedence pins the tier order the entity merge rests on.
func TestPersonIdentityKeyPrecedence(t *testing.T) {
	assert.Equal(t, "orcid:0000-0002-1825-0097",
		PersonIdentityKey(Person{Orcid: "https://orcid.org/0000-0002-1825-0097"}))
	assert.Equal(t, "email:alice@example.com", PersonIdentityKey(Person{Email: "Alice@Example.com"}))
	assert.Contains(t, PersonIdentityKey(Person{GivenNames: "Alice", FamilyNames: "Smith"}), "alice smith")
	assert.Contains(t, PersonIdentityKey(Person{DisplayName: "Alice Smith"}), "alice smith")
	assert.NotEqual(t,
		PersonIdentityKey(Person{GivenNames: "Alice", FamilyNames: "Smith"}),
		PersonIdentityKey(Person{GivenNames: "Bob", FamilyNames: "Jones"}))
}

// TestSamePersonDecisiveMiss pins that two differing ORCIDs stop the fall-through.
func TestSamePersonDecisiveMiss(t *testing.T) {
	assert.False(t, samePerson(
		Person{Orcid: "0000-0001", DisplayName: "Same"},
		Person{Orcid: "0000-0002", DisplayName: "Same"},
	))
	assert.True(t, samePerson(
		Person{Orcid: "0000-0001"},
		Person{Orcid: "0000-0001", DisplayName: "anything"},
	))
	assert.False(t, samePerson(Person{}, Person{}), "two empty records are not the same person")
}

// TestSplitGitNameCommaForm pins the unambiguous spelling.
func TestSplitGitNameCommaForm(t *testing.T) {
	for _, tc := range []struct{ in, family, given string }{
		{"Smith, Alice", "Smith", "Alice"},
		{"Alice Smith, Jr", "Alice Smith", "Jr"},
		{"  Smith ,  Alice  ", "Smith", "Alice"},
		{"Smith,Alice", "Smith", "Alice"},
	} {
		family, given := SplitGitName(tc.in)
		assert.Equal(t, tc.family, family, "in %q", tc.in)
		assert.Equal(t, tc.given, given, "in %q", tc.in)
	}
}

// TestSplitGitNameCommasInsideAreNotSpecial pins that only the FIRST comma splits.
func TestSplitGitNameCommasInsideAreNotSpecial(t *testing.T) {
	family, given := SplitGitName("Smith, Alice, Jr")
	assert.Equal(t, "Smith", family)
	assert.Equal(t, "Alice, Jr", given)
}

// TestSplitGitNameCommaAtIndexZero pins the comma guard at its boundary.
func TestSplitGitNameCommaAtIndexZero(t *testing.T) {
	family, given := SplitGitName(",Alice")
	assert.Empty(t, family)
	assert.Equal(t, "Alice", given)

	family, given = SplitGitName(",")
	assert.Empty(t, family)
	assert.Empty(t, given)

	family, given = SplitGitName(", ,")
	assert.Empty(t, family, "the comma at index zero splits an empty leading field")
	assert.Equal(t, ",", given, "everything after that first comma is the given part")
}

func TestSplitGitNameNoComma(t *testing.T) {
	family, given := SplitGitName("  Alice Smith  ")
	assert.Equal(t, "Smith", family)
	assert.Equal(t, "Alice", given)

	family, given = SplitGitName("Alice")
	assert.Equal(t, "Alice", family)
	assert.Empty(t, given, "a single token is a family name by default")

	family, given = SplitGitName("   ")
	assert.Empty(t, family)
	assert.Empty(t, given)
}

// TestSplitGitNameMultiWordForms pins the no-comma assembly at its shapes.
func TestSplitGitNameMultiWordForms(t *testing.T) {
	family, given := SplitGitName("Alicevan der Berg")
	assert.Equal(t, "Berg", family)
	assert.Equal(t, "Alicevan der", given)

	family, given = SplitGitName("Ludwig van Beethoven")
	assert.Equal(t, "Beethoven", family)
	assert.Equal(t, "Ludwig van", given)
}

// ── numeric coercion ─────────────────────────────────────────────────────────

// TestAsNonNegativeIntPinsEveryKindBoundary walks the reflect switch at its edges.
func TestAsNonNegativeIntPinsEveryKindBoundary(t *testing.T) {
	cases := []struct {
		name  string
		in    any
		want  int
		wantK bool
	}{
		{"zero", 0, 0, true},
		{"one", 1, 1, true},
		{"negative is rejected", -1, 0, false},
		{"int8", int8(127), 127, true},
		{"int16", int16(32767), 32767, true},
		{"int32", int32(2147483647), 2147483647, true},
		{"int64 max", int64(math.MaxInt64), math.MaxInt64, true},
		{"int64 negative is rejected", int64(-5), 0, false},
		{"uint", uint(3), 3, true},
		{"uint8", uint8(255), 255, true},
		{"uint64 max int", uint64(math.MaxInt), math.MaxInt, true},
		{"uint64 beyond max int", uint64(math.MaxInt) + 1, 0, false},
		{"float32 whole", float32(2), 2, true},
		{"float64 whole", float64(4), 4, true},
		{"fractional is rejected", 1.5, 0, false},
		{"negative float is rejected", -2.0, 0, false},
		{"string is rejected", "7", 0, false},
		{"bool is rejected", true, 0, false},
		{"nil is rejected", nil, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := asNonNegativeInt(tc.in)
			assert.Equal(t, tc.wantK, ok, "in %v", tc.in)
			assert.Equal(t, tc.want, got, "in %v", tc.in)
		})
	}
}

// TestAsNonNegativeIntFloatRange pins the float branch across its usable span.
func TestAsNonNegativeIntFloatRange(t *testing.T) {
	for _, tc := range []struct {
		name  string
		in    float64
		want  int
		wantK bool
	}{
		{"small whole", 3, 3, true},
		{"zero", 0, 0, true},
		{"exact 2^53", 1 << 53, 1 << 53, true},
		{"just below 2^63", math.Pow(2, 62), 1 << 62, true},
		{"negative", -1, 0, false},
		{"negative fractional", -0.5, 0, false},
		{"fractional", 2.5, 0, false},
		{"negative zero is whole", math.Copysign(0, -1), 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := asNonNegativeInt(tc.in)
			assert.Equal(t, tc.wantK, ok, "in %v", tc.in)
			assert.Equal(t, tc.want, got, "in %v", tc.in)
		})
	}
}

// TestAsNonNegativeIntZeroIsAccepted pins that zero is a real level, not a missing value.
func TestAsNonNegativeIntZeroIsAccepted(t *testing.T) {
	got, ok := asNonNegativeInt(float64(0))
	assert.True(t, ok)
	assert.Equal(t, 0, got)

	got, ok = asNonNegativeInt(uint(0))
	assert.True(t, ok)
	assert.Equal(t, 0, got)

	got, ok = asNonNegativeInt(int64(0))
	assert.True(t, ok)
	assert.Equal(t, 0, got)
}

// TestApplyDiscriminator pins the per-format rule.
func TestApplyDiscriminator(t *testing.T) {
	toml := &Document{SpecVersion: "9"}
	applyDiscriminator(toml, ".toml")
	assert.Equal(t, "1", toml.SpecVersion, "TOML's discriminator is authoritative")

	yamlDoc := &Document{SpecVersion: "9"}
	applyDiscriminator(yamlDoc, ".yaml")
	assert.Equal(t, "9", yamlDoc.SpecVersion, "other encodings keep what the document said")
}

// TestWriteTOMLLiftsTheSchemaHeader pins the TOML convention.
func TestWriteTOMLLiftsTheSchemaHeader(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "projectfile.toml")

	doc := &Document{Schema: "https://projectfile.org/schema/v1.json", Identity: Identity{Name: "demo"}}
	require.NoError(t, Write(doc, p))

	body, err := os.ReadFile(p)
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, "#:schema https://projectfile.org/schema/v1.json")
	assert.NotContains(t, text, "\n$schema", "the schema is a comment, not a body key")

	require.NoError(t, Write(doc, p))
	again, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(again), "#:schema"),
		"the existing header is reused rather than duplicated")
	assert.Contains(t, string(again), "[identity]")
}

// TestWriteTOMLWithNoHeaderHasNoLeadingBlank pins the empty-header guard.
func TestWriteTOMLWithNoHeaderHasNoLeadingBlank(t *testing.T) {
	p := filepath.Join(t.TempDir(), "projectfile.toml")
	require.NoError(t, Write(&Document{Identity: Identity{Name: "demo"}}, p))

	body, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "#:schema", "no schema means no header")
	assert.Contains(t, string(body), "[identity]")
}

// TestWriteTOMLKeepsLeadingComments pins that a hand-written note survives.
func TestWriteTOMLKeepsLeadingComments(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "projectfile.toml")
	require.NoError(t, os.WriteFile(p, []byte("# hand-written note\n\n[identity]\nname = \"demo\"\n"), 0o644))

	require.NoError(t, Write(&Document{Identity: Identity{Name: "demo"}}, p))

	body, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Contains(t, string(body), "# hand-written note")
}

// TestWriteYAMLRepaintsOntoTheCanvas pins the round-trip that keeps comments.
func TestWriteYAMLRepaintsOntoTheCanvas(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "projectfile.yaml")
	require.NoError(t, os.WriteFile(p,
		[]byte("# banner comment\nidentity:\n  name: old\n  version: 1.0.0\n"), 0o644))

	doc, err := ReadFromPath(p)
	require.NoError(t, err)
	doc.Identity.Name = "new"
	require.NoError(t, Write(doc, p))

	body, err := os.ReadFile(p)
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, "# banner comment", "the canvas keeps what a re-encode drops")
	assert.Contains(t, text, "name: new")
	assert.NotContains(t, text, "name: old")
	assert.Less(t, strings.Index(text, "# banner comment"), strings.Index(text, "identity:"),
		"the comment keeps its position above the key it annotates")
}

// TestWriteYAMLOntoAnEmptyFileTakesTheCleanPath pins the canvas emptiness guard.
func TestWriteYAMLOntoAnEmptyFileTakesTheCleanPath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "projectfile.yaml")
	require.NoError(t, os.WriteFile(p, nil, 0o644))

	require.NoError(t, Write(&Document{Identity: Identity{Name: "demo"}}, p))

	got, err := ReadFromPath(p)
	require.NoError(t, err)
	assert.Equal(t, "demo", got.Identity.Name)
}

// TestWriteYAMLOntoAMalformedFileTakesTheCleanPath pins the parse guard.
func TestWriteYAMLOntoAMalformedFileTakesTheCleanPath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "projectfile.yaml")
	require.NoError(t, os.WriteFile(p, []byte("identity: [\n"), 0o644))

	require.NoError(t, Write(&Document{Identity: Identity{Name: "demo"}}, p))

	got, err := ReadFromPath(p)
	require.NoError(t, err, "the malformed file must have been replaced, not preserved")
	assert.Equal(t, "demo", got.Identity.Name)
}

// TestWriteYAMLSortedRendersAlphabetically pins the sorted-output branch.
func TestWriteYAMLSortedRendersAlphabetically(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "projectfile.yaml")
	require.NoError(t, os.WriteFile(p,
		[]byte("spec_version: \"1\"\nidentity:\n  name: demo\nkeywords:\n  - rust\n"), 0o644))

	doc := &Document{
		SpecVersion: "1",
		Identity:    Identity{Name: "demo"},
		Keywords:    []string{"rust"},
	}
	SetYAMLOutputSorted(true)
	t.Cleanup(func() { SetYAMLOutputSorted(false) })
	require.NoError(t, Write(doc, p))

	body, err := os.ReadFile(p)
	require.NoError(t, err)
	text := string(body)
	assert.Less(t, indexOf(text, "identity:"), indexOf(text, "keywords:"),
		"sorted output reorders even the keys the canvas already carried")
	assert.Less(t, indexOf(text, "keywords:"), indexOf(text, "spec_version:"))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
