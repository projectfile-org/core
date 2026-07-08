// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- Scenario model ---
//
// preSync  = merged doc (includes + base) BEFORE the operation ran
// postSync = merged doc AFTER  the operation mutated it
// base     = base-only doc (what should land on disk)
//
// The invariant: after ReconcileBase, `base` contains only base-local data
// plus the fields the operation changed — never raw include data that the
// operation did not touch.

const (
	testAppName   = "my-app"
	testNamespace = "org.example"
	testNode      = "node"
)

func p(given, family string) Person {
	return Person{GivenNames: given, FamilyNames: family, Roles: []string{testAuthor}}
}

// TestReconcileBaseNoChangesLeavesBaseUntouched is the fundamental invariant:
// when the sync did not mutate anything, base is returned verbatim.
func TestReconcileBaseNoChangesLeavesBaseUntouched(t *testing.T) {
	base := &Document{Identity: Identity{Name: testAppName}}
	pre := &Document{
		Identity: Identity{Name: testAppName, Namespace: testNamespace},
		Keywords: []string{"shared"},
	}
	post := pre.Clone()

	ReconcileBase(base, pre, post)

	assert.Equal(t, testAppName, base.Identity.Name, "base name unchanged")
	assert.Empty(t, base.Identity.Namespace, "include namespace NOT materialised")
	assert.Nil(t, base.Keywords, "include keywords NOT materialised")
}

// TestReconcileBaseIdentityChangeAppliesOnlyChangedSubField verifies that a
// sync which changes identity.version does NOT drag include-sourced
// identity.namespace into base.
func TestReconcileBaseIdentityChangeAppliesOnlyChangedSubField(t *testing.T) {
	base := &Document{Identity: Identity{Name: testAppName}}
	pre := &Document{
		Identity: Identity{Name: testAppName, Namespace: testNamespace},
	}
	post := &Document{
		Identity: Identity{Name: testAppName, Namespace: testNamespace, Version: "2.0.0"},
	}

	ReconcileBase(base, pre, post)

	assert.Equal(t, "2.0.0", base.Identity.Version, "changed version applied")
	assert.Empty(t, base.Identity.Namespace, "unchanged namespace NOT materialised")
}

// TestReconcileBaseKeywordsFullReplacement confirms that when the sync fully
// replaces a string slice (the standard mapper pattern for keywords), the new
// value lands in base.
func TestReconcileBaseKeywordsFullReplacement(t *testing.T) {
	base := &Document{}
	pre := &Document{Keywords: []string{"from-include"}}
	post := &Document{Keywords: []string{"ext-a", "ext-b"}}

	ReconcileBase(base, pre, post)

	assert.Equal(t, []string{"ext-a", "ext-b"}, base.Keywords)
}

// TestReconcileBaseKeywordsUnchangedNotMaterialised confirms that keywords
// from includes stay out of base when the sync didn't touch them.
func TestReconcileBaseKeywordsUnchangedNotMaterialised(t *testing.T) {
	base := &Document{}
	pre := &Document{Keywords: []string{"from-include"}}
	post := pre.Clone()

	ReconcileBase(base, pre, post)

	assert.Nil(t, base.Keywords)
}

// TestReconcileBasePeopleAppendNew verifies that new people from the external
// file are appended to base, while include-sourced people are NOT
// materialised.
func TestReconcileBasePeopleAppendNew(t *testing.T) {
	base := &Document{People: []Person{p("Bob", "Base")}}
	pre := &Document{People: []Person{
		p("Alice", "Include"), // from include
		p("Bob", "Base"),      // from base
	}}
	// Sync appended Charlie (from ext file) to the merged doc.
	post := &Document{People: []Person{
		p("Alice", "Include"),
		p("Bob", "Base"),
		p("Charlie", "Ext"),
	}}

	ReconcileBase(base, pre, post)

	assert.Len(t, base.People, 2)
	assert.Equal(t, "Bob", base.People[0].GivenNames)
	assert.Equal(t, "Charlie", base.People[1].GivenNames)
}

// TestReconcileBasePeopleModifyBaseEntry verifies that when the sync modifies
// an existing person that lives in base, the modification is applied.
func TestReconcileBasePeopleModifyBaseEntry(t *testing.T) {
	basePerson := Person{GivenNames: "Bob", FamilyNames: "Base", Email: "", Roles: []string{testAuthor}}
	base := &Document{People: []Person{basePerson}}
	pre := &Document{People: []Person{
		p("Alice", "Include"),
		basePerson,
	}}
	// Sync gap-filled Bob's email from the ext file.
	bobModified := basePerson
	bobModified.Email = "bob@ext.com"
	post := &Document{People: []Person{
		p("Alice", "Include"),
		bobModified,
	}}

	ReconcileBase(base, pre, post)

	assert.Len(t, base.People, 1)
	assert.Equal(t, "bob@ext.com", base.People[0].Email)
}

// TestReconcileBasePeopleModifyIncludeEntryNotWrittenToBase verifies that
// modifications to an include-sourced person do NOT create a phantom entry
// in base.
func TestReconcileBasePeopleModifyIncludeEntryNotWrittenToBase(t *testing.T) {
	base := &Document{}
	alice := p("Alice", "Include")
	pre := &Document{People: []Person{alice}}
	aliceModified := alice
	aliceModified.Email = "alice@ext.com"
	post := &Document{People: []Person{aliceModified}}

	ReconcileBase(base, pre, post)

	assert.Empty(t, base.People, "include-person modification must not create base entry")
}

// TestReconcileBaseRepositoryURLChange verifies that a sync which changes the
// primary repo URL updates the base entry when it existed in base, and does
// not duplicate include-sourced repos.
func TestReconcileBaseRepositoryURLChange(t *testing.T) {
	baseRepo := Repository{URL: "https://old.git", Role: RepositoryRoleOrigin}
	base := &Document{Repositories: []Repository{baseRepo}}
	pre := &Document{Repositories: []Repository{
		{URL: testIncludeURL},
		baseRepo,
	}}
	post := &Document{Repositories: []Repository{
		{URL: testIncludeURL},
		{URL: "https://new.git", Role: RepositoryRoleOrigin},
	}}

	ReconcileBase(base, pre, post)

	assert.Len(t, base.Repositories, 1)
	assert.Equal(t, "https://new.git", base.Repositories[0].URL)
	assert.Equal(t, RepositoryRoleOrigin, base.Repositories[0].Role)
}

// TestReconcileBaseRepositoryFromIncludeNotMaterialised verifies that repos
// from includes stay out of base when unchanged.
func TestReconcileBaseRepositoryFromIncludeNotMaterialised(t *testing.T) {
	base := &Document{}
	pre := &Document{Repositories: []Repository{{URL: testIncludeURL}}}
	post := pre.Clone()

	ReconcileBase(base, pre, post)

	assert.Empty(t, base.Repositories)
}

// TestReconcileBaseLinkNewAppended verifies that a new link from the ext file
// is appended to base.
func TestReconcileBaseLinkNewAppended(t *testing.T) {
	base := &Document{}
	pre := &Document{Links: []Link{{Type: LinkHomepage, URL: testIncludeURL}}}
	post := &Document{Links: []Link{
		{Type: LinkHomepage, URL: testIncludeURL},
		{Type: "bugs", URL: testIncludeURL + "/issues"},
	}}

	ReconcileBase(base, pre, post)

	assert.Len(t, base.Links, 1)
	assert.Equal(t, "bugs", base.Links[0].Type)
	assert.Equal(t, "https://include.git/issues", base.Links[0].URL)
}

// TestReconcileBaseLinkModifiedInBase verifies that modifying a base-local
// link updates it in place.
func TestReconcileBaseLinkModifiedInBase(t *testing.T) {
	base := &Document{Links: []Link{{Type: LinkHomepage, URL: "https://old-home"}}}
	pre := &Document{Links: []Link{{Type: LinkHomepage, URL: "https://old-home"}}}
	post := &Document{Links: []Link{{Type: LinkHomepage, URL: "https://new-home"}}}

	ReconcileBase(base, pre, post)

	assert.Len(t, base.Links, 1)
	assert.Equal(t, "https://new-home", base.Links[0].URL)
}

// TestReconcileBaseRequirementsSubFieldChange verifies per-sub-field diffing:
// a change to requirements.runtime does NOT materialise include-sourced
// requirements.operating-system.
func TestReconcileBaseRequirementsSubFieldChange(t *testing.T) {
	base := &Document{}
	pre := &Document{Requirements: &Requirements{
		OS:      []string{"linux"},
		Runtime: map[string]string{testNode: "^20"},
	}}
	post := &Document{Requirements: &Requirements{
		OS:      []string{"linux"},
		Runtime: map[string]string{testNode: "^24"},
	}}

	ReconcileBase(base, pre, post)

	assert.NotNil(t, base.Requirements)
	assert.Equal(t, map[string]string{testNode: "^24"}, base.Requirements.Runtime,
		"changed runtime applied")
	assert.Nil(t, base.Requirements.OS,
		"unchanged include OS NOT materialised")
}

// TestReconcileBaseExtensionsChangedNamespace verifies that when the sync
// writes an extension namespace, it lands in base; unchanged ones do not.
func TestReconcileBaseExtensionsChangedNamespace(t *testing.T) {
	base := &Document{}
	pre := &Document{Extensions: map[string]any{
		"org.projectfile.ignores": map[string]any{"generate": []any{".gitignore"}},
	}}
	post := &Document{Extensions: map[string]any{
		"org.projectfile.ignores": map[string]any{"generate": []any{".gitignore"}},
		"org.projectfile.funding": map[string]any{"custom": []any{"https://opencollective.com/x"}},
	}}

	ReconcileBase(base, pre, post)

	_, hasIgnores := LookupExtension(base, "org.projectfile.ignores")
	assert.False(t, hasIgnores, "unchanged include extension NOT materialised")

	funding, hasFunding := LookupExtension(base, "org.projectfile.funding")
	assert.True(t, hasFunding, "changed extension written to base")
	assert.NotNil(t, funding)
}

// TestReconcileBaseLicenseChange verifies the full-replacement path for
// pointer fields.
func TestReconcileBaseLicenseChange(t *testing.T) {
	base := &Document{}
	pre := &Document{License: &License{Spdx: "Apache-2.0"}}
	post := &Document{License: &License{Spdx: testMIT}}

	ReconcileBase(base, pre, post)

	assert.NotNil(t, base.License)
	assert.Equal(t, testMIT, base.License.Spdx)
}

func TestReconcileBaseLicenseUnchangedNotMaterialised(t *testing.T) {
	base := &Document{}
	pre := &Document{License: &License{Spdx: "Apache-2.0"}}
	post := pre.Clone()

	ReconcileBase(base, pre, post)

	assert.Nil(t, base.License, "unchanged include license NOT materialised")
}

// TestReconcileBaseNilDocsIsSafe verifies defensive nil handling.
func TestReconcileBaseNilDocsIsSafe(t *testing.T) {
	assert.NotPanics(t, func() { ReconcileBase(nil, nil, nil) })
	assert.NotPanics(t, func() { ReconcileBase(&Document{}, nil, &Document{}) })
}
