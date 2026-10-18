// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

// licExtra is the out-of-schema key pair every entity carries in these fixtures.
var licExtra = map[string]any{"lic-extra": "x"}

// fullyPopulated carries a distinct value in every declared field of every entity.
func fullyPopulated() *projectfile.Document {
	return &projectfile.Document{
		SpecVersion: "1",
		Schema:      "https://projectfile.org/schema/v1.json",
		Kind:        "library",
		Identity: projectfile.Identity{
			Namespace:   "org.example",
			Name:        "demo",
			Version:     "1.2.3",
			Title:       &projectfile.LocalizedString{Bare: "Demo"},
			Summary:     &projectfile.LocalizedString{Bare: "Summary"},
			Description: &projectfile.LocalizedString{Langs: map[string]string{"en": "Desc"}},
			Created:     "2026-01-01",
			Released:    "2026-02-01",
			Modified:    "2026-03-01",
			Visibility:  "public",
			Extra:       map[string]any{"identity-extra": "x"},
		},
		Repositories: []projectfile.Repository{{
			URL:      "https://example.com/repo.git",
			Type:     "git",
			Path:     "src",
			Branch:   "main",
			Issues:   true,
			Packages: true,
			Role:     "origin",
			Extra:    map[string]any{"repo-extra": "x"},
		}},
		License:   &projectfile.License{Spdx: "MIT", Covers: "docs/", File: "LICENSE", Extra: licExtra},
		Copyright: &projectfile.Copyright{Year: 2026, Extra: map[string]any{"cp-extra": "x"}},
		People: []projectfile.Person{{
			FamilyNames:  "Smith",
			GivenNames:   "Alice",
			NameParticle: "de",
			NameSuffix:   "Jr",
			Alias:        "Al",
			DisplayName:  "Alice Smith",
			Email:        "alice@example.com",
			URL:          "https://alice.test",
			Handles:      map[string]any{"github": "alice"},
			Affiliation:  "Example Inc",
			Orcid:        "0000-0002-1825-0097",
			From:         "2019-01-01",
			To:           "2026-01-01",
			Roles:        []string{"author"},
			Extra:        map[string]any{"person-extra": "x"},
		}},
		Organizations: []projectfile.Organization{{
			Name:    "Example Inc",
			Alias:   "Ex",
			Email:   "info@example.com",
			URL:     "https://example.com",
			Handles: map[string]any{"github": "example"},
			Orcid:   "0000-0002-1825-0097",
			From:    "2020-01-01",
			To:      "2026-01-01",
			Roles:   []string{"maintainer"},
			Extra:   map[string]any{"org-extra": "x"},
		}},
		Keywords: []string{"rust", "wasm"},
		Stack:    []string{"go"},
		Requirements: &projectfile.Requirements{
			Browsers: []any{"chrome"},
			Runtime:  map[string]string{"node": ">=24"},
			Extra:    map[string]any{"req-extra": "x"},
		},
		Includes: []string{"inc.yaml", "https://example.com/b.yaml"},
		Links: []projectfile.Link{{
			Type:      "website",
			URL:       "https://example.com",
			Label:     &projectfile.LocalizedString{Bare: "Site"},
			Preferred: true,
			Derived:   true,
			Extra:     map[string]any{"link-extra": "x"},
		}},
		Extensions:  map[string]any{"com.example.build": map[string]any{"user": "ubuntu"}},
		IncludePins: map[string]string{"https://example.com/b.yaml": "abc123"},
	}
}

// sub walks a nested map by key, failing the test when the key is absent.
func sub(t *testing.T, m map[string]any, path ...string) any {
	t.Helper()
	var cur any = m
	for i, k := range path {
		mm, ok := cur.(map[string]any)
		require.True(t, ok, "path %v: element %d is %T, not a map", path, i, cur)
		cur, ok = mm[k]
		require.True(t, ok, "path %v: key %q is missing", path[:i+1], k)
	}
	return cur
}

// first returns the first element of a serialised entity list.
func first(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	list, ok := m[key].([]map[string]any)
	require.True(t, ok, "%q is %T, not a serialised entity list", key, m[key])
	require.NotEmpty(t, list, "%q must not be empty", key)
	return list[0]
}

// TestToMapEmitsEveryPopulatedField pins that a field the author set reaches the raw map.
func TestToMapEmitsEveryPopulatedField(t *testing.T) {
	m := fullyPopulated().ToMap()

	for _, tc := range []struct {
		path []string
		want any
	}{
		{[]string{"spec_version"}, "1"},
		{[]string{"$schema"}, "https://projectfile.org/schema/v1.json"},
		{[]string{"kind"}, "library"},
		{[]string{"identity", "namespace"}, "org.example"},
		{[]string{"identity", "name"}, "demo"},
		{[]string{"identity", "version"}, "1.2.3"},
		{[]string{"identity", "title"}, "Demo"},
		{[]string{"identity", "summary"}, "Summary"},
		{[]string{"identity", "description", "en"}, "Desc"},
		{[]string{"identity", "created"}, "2026-01-01"},
		{[]string{"identity", "released"}, "2026-02-01"},
		{[]string{"identity", "modified"}, "2026-03-01"},
		{[]string{"identity", "visibility"}, "public"},
		{[]string{"identity", "identity-extra"}, "x"},
	} {
		assert.Equal(t, tc.want, sub(t, m, tc.path...), "path %v", tc.path)
	}

	assert.Equal(t, "https://example.com/repo.git", first(t, m, "repositories")["url"])
	assert.Equal(t, "git", first(t, m, "repositories")["type"])
	assert.Equal(t, "src", first(t, m, "repositories")["path"])
	assert.Equal(t, "main", first(t, m, "repositories")["branch"])
	assert.Equal(t, true, first(t, m, "repositories")["issues"])
	assert.Equal(t, true, first(t, m, "repositories")["packages"])
	assert.Equal(t, "origin", first(t, m, "repositories")["role"])
	assert.Equal(t, "x", first(t, m, "repositories")["repo-extra"])

	assert.Equal(t, "MIT", sub(t, m, "license", "spdx"))
	assert.Equal(t, "docs/", sub(t, m, "license", "covers"))
	assert.Equal(t, "LICENSE", sub(t, m, "license", "file"))
	assert.Equal(t, "x", sub(t, m, "license", "lic-extra"))

	assert.Equal(t, 2026, sub(t, m, "copyright", "year"))
	assert.Equal(t, "x", sub(t, m, "copyright", "cp-extra"))

	p := first(t, m, "people")
	for k, want := range map[string]any{
		"family-names": "Smith", "given-names": "Alice", "name-particle": "de",
		"name-suffix": "Jr", "alias": "Al", "display-name": "Alice Smith",
		"email": "alice@example.com", "url": "https://alice.test",
		"affiliation": "Example Inc", "orcid": "0000-0002-1825-0097",
		"from": "2019-01-01", "to": "2026-01-01",
	} {
		assert.Equal(t, want, p[k], "people[%q]", k)
	}
	assert.Equal(t, []string{"author"}, p["roles"])
	assert.Equal(t, map[string]any{"github": "alice"}, p["handles"])
	assert.Equal(t, "x", p["person-extra"])

	o := first(t, m, "organizations")
	for k, want := range map[string]any{
		"name": "Example Inc", "alias": "Ex", "email": "info@example.com",
		"url": "https://example.com", "orcid": "0000-0002-1825-0097",
		"from": "2020-01-01", "to": "2026-01-01",
	} {
		assert.Equal(t, want, o[k], "organizations[%q]", k)
	}
	assert.Equal(t, []string{"maintainer"}, o["roles"])
	assert.Equal(t, map[string]any{"github": "example"}, o["handles"])
	assert.Equal(t, "x", o["org-extra"])

	assert.Equal(t, []string{"rust", "wasm"}, sub(t, m, "keywords"))
	assert.Equal(t, []string{"go"}, sub(t, m, "technologies"))
	assert.Equal(t, []any{"chrome"}, sub(t, m, "requirements", "browsers"))
	assert.Equal(t, map[string]string{"node": ">=24"}, sub(t, m, "requirements", "runtime"))
	assert.Equal(t, "x", sub(t, m, "requirements", "req-extra"))

	l := first(t, m, "links")
	assert.Equal(t, "website", l["type"])
	assert.Equal(t, "https://example.com", l["url"])
	assert.Equal(t, "Site", l["label"])
	assert.Equal(t, true, l["preferred"])
	assert.Equal(t, true, l["derived"])
	assert.Equal(t, "x", l["link-extra"])

	// Extensions explode their dotted namespace into nested mappings.
	assert.Equal(t, "ubuntu", sub(t, m, "com", "example", "build", "user"))

	single := (&projectfile.Document{
		Extensions: map[string]any{"reserved-key": map[string]any{"k": "v"}},
	}).ToMap()
	assert.Equal(t, map[string]any{"k": "v"}, single["reserved-key"],
		"a single-segment key is the on-spec encoding")
}

// TestToMapOmitsEveryUnsetField is the twin: a zero field must not reach the raw map.
func TestToMapOmitsEveryUnsetField(t *testing.T) {
	m := (&projectfile.Document{}).ToMap()

	assert.NotContains(t, m, "identity")
	for _, k := range []string{
		"spec_version", "$schema", "kind", "repositories", "license",
		"copyright", "people", "organizations", "keywords", "technologies",
		"requirements", "includes", "links",
	} {
		assert.NotContains(t, m, k, "an unset field must not be emitted")
	}
}

// TestRepositoriesOmitUnsetOptionals walks one repository per optional-field combination.
func TestRepositoriesOmitUnsetOptionals(t *testing.T) {
	onlyURL := first(t, (&projectfile.Document{
		Repositories: []projectfile.Repository{{URL: "https://a.test"}},
	}).ToMap(), "repositories")
	assert.Equal(t, map[string]any{"url": "https://a.test"}, onlyURL)

	noFlags := first(t, (&projectfile.Document{
		Repositories: []projectfile.Repository{{URL: "https://a.test", Role: "mirror"}},
	}).ToMap(), "repositories")
	assert.NotContains(t, noFlags, "issues", "a false flag is omitted, not written as false")
	assert.NotContains(t, noFlags, "packages")
}

// TestLinksAlwaysCarryTypeAndURL pins the two link keys that have no guard.
func TestLinksAlwaysCarryTypeAndURL(t *testing.T) {
	l := first(t, (&projectfile.Document{
		Links: []projectfile.Link{{Type: "website", URL: "https://a.test"}},
	}).ToMap(), "links")
	assert.Equal(t, map[string]any{"type": "website", "url": "https://a.test"}, l)

	blank := first(t, (&projectfile.Document{Links: []projectfile.Link{{}}}).ToMap(), "links")
	assert.Contains(t, blank, "type", "type and url are unconditional")
	assert.Contains(t, blank, "url")
}

// TestEntityListsOmitEmptyRolesAndHandles pins that an entity with neither emits neither key.
func TestEntityListsOmitEmptyRolesAndHandles(t *testing.T) {
	for _, tc := range []struct {
		key string
		doc *projectfile.Document
	}{
		{"people", &projectfile.Document{People: []projectfile.Person{{GivenNames: "Alice"}}}},
		{"organizations", &projectfile.Document{Organizations: []projectfile.Organization{{Name: "Kiota"}}}},
	} {
		t.Run(tc.key, func(t *testing.T) {
			got := first(t, tc.doc.ToMap(), tc.key)
			assert.NotContains(t, got, "roles")
			assert.NotContains(t, got, "handles")
		})
	}
}

// TestEntityListsEmitOneRoleAndOneHandle is the twin at the other edge.
func TestEntityListsEmitOneRoleAndOneHandle(t *testing.T) {
	p := first(t, (&projectfile.Document{
		People: []projectfile.Person{{
			GivenNames: "Alice",
			Roles:      []string{"author"},
			Handles:    map[string]any{"github": "alice"},
		}},
	}).ToMap(), "people")
	assert.Equal(t, []string{"author"}, p["roles"])
	assert.Equal(t, map[string]any{"github": "alice"}, p["handles"])

	o := first(t, (&projectfile.Document{
		Organizations: []projectfile.Organization{{
			Name:    "Kiota",
			Roles:   []string{"maintainer"},
			Handles: map[string]any{"github": "kiota"},
		}},
	}).ToMap(), "organizations")
	assert.Equal(t, []string{"maintainer"}, o["roles"])
	assert.Equal(t, map[string]any{"github": "kiota"}, o["handles"])
}

// TestEntityListsOmitEmptyDates pins the from/to guards at the same edge.
func TestEntityListsOmitEmptyDates(t *testing.T) {
	p := first(t, (&projectfile.Document{
		People: []projectfile.Person{{Email: "a@x.test"}},
	}).ToMap(), "people")
	assert.NotContains(t, p, "from")
	assert.NotContains(t, p, "to")

	o := first(t, (&projectfile.Document{
		Organizations: []projectfile.Organization{{Name: "Kiota"}},
	}).ToMap(), "organizations")
	assert.NotContains(t, o, "from")
	assert.NotContains(t, o, "to")
}

// TestLocalizedStringToMapShapes pins the three-way encoding.
func TestLocalizedStringToMapShapes(t *testing.T) {
	m := (&projectfile.Document{
		Identity: projectfile.Identity{
			Title:   &projectfile.LocalizedString{Bare: "Bare", Langs: map[string]string{"en": "Hi"}},
			Summary: &projectfile.LocalizedString{Langs: map[string]string{"en": "Hi"}},
		},
	}).ToMap()
	id := m["identity"].(map[string]any)
	assert.Equal(t, "Bare", id["title"], "Bare is language-agnostic and wins")
	assert.Equal(t, map[string]any{"en": "Hi"}, id["summary"])

	empty := (&projectfile.Document{
		Identity: projectfile.Identity{Title: &projectfile.LocalizedString{}},
	}).ToMap()
	assert.Nil(t, empty["identity"].(map[string]any)["title"],
		"a localized string with neither form serialises as a nil")
}

// TestIncludesToListPinsPinRendering pins the spec §4.9 write-back shape.
func TestIncludesToListPinsPinRendering(t *testing.T) {
	plain := (&projectfile.Document{Includes: []string{"a.yaml"}}).ToMap()
	assert.Equal(t, []string{"a.yaml"}, plain["includes"], "no pins means a plain string list")

	pinned := (&projectfile.Document{
		Includes:    []string{"a.yaml", "https://example.com/b.yaml"},
		IncludePins: map[string]string{"https://example.com/b.yaml": "ABC123"},
	}).ToMap()
	assert.Equal(t,
		[]any{"a.yaml", map[string]any{"url": "https://example.com/b.yaml", "sha256": "ABC123"}},
		pinned["includes"])
}

// TestRequirementsOmitUnsetHalves pins each half of requirementsToMap.
func TestRequirementsOmitUnsetHalves(t *testing.T) {
	runtimeOnly := sub(t, (&projectfile.Document{
		Requirements: &projectfile.Requirements{Runtime: map[string]string{"node": ">=24"}},
	}).ToMap(), "requirements").(map[string]any)
	assert.NotContains(t, runtimeOnly, "browsers")

	browsersOnly := sub(t, (&projectfile.Document{
		Requirements: &projectfile.Requirements{Browsers: []any{"chrome"}},
	}).ToMap(), "requirements").(map[string]any)
	assert.NotContains(t, browsersOnly, "runtime")

	pruned := (&projectfile.Document{Requirements: &projectfile.Requirements{}}).ToMap()
	assert.NotContains(t, pruned, "requirements")
}

// TestStripRedundantKeepsReservedKeys pins that the local-file keys are never stripped.
func TestStripRedundantKeepsReservedKeys(t *testing.T) {
	inc := map[string]any{
		"spec_version": "1",
		"$schema":      "https://projectfile.org/schema/v1.json",
		"includes":     []any{"inc.yaml"},
		"keywords":     []any{"rust"},
	}
	base := map[string]any{
		"spec_version": "1",
		"$schema":      "https://projectfile.org/schema/v1.json",
		"includes":     []any{"inc.yaml"},
		"keywords":     []any{"rust"},
		"extra":        "kept",
	}
	removed := projectfile.StripRedundant(base, inc)
	assert.Equal(t, []string{"keywords"}, removed)
	assert.Contains(t, base, "spec_version")
	assert.Contains(t, base, "$schema")
	assert.Contains(t, base, "includes")
	assert.Contains(t, base, "extra")
}

// TestStripRedundantKeepsEntityLists pins the entity exemption.
func TestStripRedundantKeepsEntityLists(t *testing.T) {
	entry := map[string]any{"email": "a@x.test", "name": "A"}
	inc := map[string]any{"people": []any{entry}}
	base := map[string]any{"people": []any{entry}}

	removed := projectfile.StripRedundant(base, inc)
	assert.Empty(t, removed, "an entity list is never stripped, even when byte-identical")
	assert.Contains(t, base, "people")
}

// TestStripRedundantRemovesEmptiedParents pins that a sub-map emptied by stripping takes its parent with it.
func TestStripRedundantRemovesEmptiedParents(t *testing.T) {
	inc := map[string]any{"outer": map[string]any{"inner": "same"}}
	base := map[string]any{"outer": map[string]any{"inner": "same"}}
	removed := projectfile.StripRedundant(base, inc)
	assert.Equal(t, []string{"outer.inner"}, removed)
	assert.NotContains(t, base, "outer")
}

// TestResolveIncludesOnlyWithoutIncludes pins the signal optimize branches on.
func TestResolveIncludesOnlyWithoutIncludes(t *testing.T) {
	got, err := projectfile.ResolveIncludesOnly(
		map[string]any{"identity": map[string]any{"name": "x"}}, t.TempDir(), projectfile.ReadOptions{})
	require.NoError(t, err)
	assert.NotNil(t, got, "an empty non-nil map marks the result applicable-or-not")
	assert.Empty(t, got)
}

// TestResolveIncludesOnlyFoldsTransitively is what lets StripRedundant drop an indirect duplicate.
func TestResolveIncludesOnlyFoldsTransitively(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "deep.yaml"), []byte("keywords:\n  - deep\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mid.yaml"),
		[]byte("includes:\n  - deep.yaml\nidentity:\n  name: mid\n"), 0o644))

	got, err := projectfile.ResolveIncludesOnly(
		map[string]any{"includes": []any{"mid.yaml"}}, dir, projectfile.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, []any{"deep"}, got["keywords"], "a transitive contribution must reach the caller")
}

// TestXDGCacheDirHomeFallback pins the XDG-unset branch.
func TestXDGCacheDirHomeFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", home)
	got, err := projectfile.XDGCacheDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".cache"), got)
}

// TestXDGCacheDirUnresolvable pins the only failure of the shared resolution.
func TestXDGCacheDirUnresolvable(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	_, err := projectfile.XDGCacheDir()
	require.Error(t, err)
}
