// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// REUSE-IgnoreStart

package projectfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

const (
	testAlice      = "Alice"
	testAliceEmail = "alice@example.com"
	testBob        = "Bob"
	testGitignore  = ".gitignore"
	testSmith      = "Smith"
	testBobEmail   = "bob@example.com"
	testGenerate   = "generate"
)

func minimalDoc() *projectfile.Document {
	return &projectfile.Document{
		Schema: "https://projectfile.org/schema/v1.json",
		Identity: projectfile.Identity{
			Name:    "test-project",
			Version: "1.0.0",
		},
		License:  &projectfile.License{Spdx: "MIT"},
		Keywords: []string{"alpha", "beta"},
	}
}

func TestRoundTripYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projectfile.yaml")
	doc := minimalDoc()
	require.NoError(t, projectfile.Write(doc, path))
	got, err := projectfile.ReadFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, doc.Identity.Name, got.Identity.Name)
	assert.Equal(t, doc.Identity.Version, got.Identity.Version)
	assert.Equal(t, doc.License.Spdx, got.License.Spdx)
	assert.Equal(t, doc.Keywords, got.Keywords)
}

func TestRoundTripTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projectfile.toml")
	doc := minimalDoc()
	require.NoError(t, projectfile.Write(doc, path))
	got, err := projectfile.ReadFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, doc.Identity.Name, got.Identity.Name)
	assert.Equal(t, doc.Identity.Version, got.Identity.Version)
	assert.Equal(t, doc.License.Spdx, got.License.Spdx)
	assert.Equal(t, doc.Keywords, got.Keywords)
}

func TestRoundTripJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projectfile.json")
	doc := minimalDoc()
	require.NoError(t, projectfile.Write(doc, path))
	got, err := projectfile.ReadFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, doc.Identity.Name, got.Identity.Name)
	assert.Equal(t, doc.License.Spdx, got.License.Spdx)
	assert.Equal(t, doc.Keywords, got.Keywords)
}

// TestRepositoryIssuesRoundTrips guards the parse↔serialize gap where the
// boolean repository `issues` field was read/written nowhere — so a selector
// like repositories[issues=true] never matched and `set …issues true` was
// silently dropped. One case per encoding (the field crosses parse.go and
// serialize.go, both encoding-agnostic).
func TestRepositoryIssuesRoundTrips(t *testing.T) {
	for _, ext := range []string{"yaml", "toml", "json"} {
		t.Run(ext, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "projectfile."+ext)
			doc := minimalDoc()
			doc.Repositories = []projectfile.Repository{
				{URL: "ssh://git@codeberg.org/acme/proj.git", Type: "git", Issues: true, Role: projectfile.RepositoryRoleMirror},
				{URL: "ssh://git@kiota.ch/acme/proj.git", Type: "git", Role: projectfile.RepositoryRoleOrigin},
			}
			require.NoError(t, projectfile.Write(doc, path))
			got, err := projectfile.ReadFromPath(path)
			require.NoError(t, err)
			require.Len(t, got.Repositories, 2)
			assert.True(t, got.Repositories[0].Issues, "issues must survive the round-trip")
			assert.False(t, got.Repositories[1].Issues)
			assert.Equal(t, projectfile.RepositoryRoleOrigin, got.Repositories[1].Role)
		})
	}
}

func TestDetectPathFindsYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projectfile.yaml")
	require.NoError(t, os.WriteFile(path, []byte("---\nidentity:\n  name: demo\n"), 0o644))
	got, err := projectfile.DetectPath(dir)
	require.NoError(t, err)
	assert.Equal(t, path, got)
}

func TestDetectPathMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := projectfile.DetectPath(dir)
	assert.Error(t, err)
}

// TestDetectPathMultipleFails asserts the spec §3.5 fail-closed rule: when
// two or more projectfile.* documents coexist the consumer MUST fail and name
// every offending file, rather than selecting one by mtime side-channel.
func TestDetectPathMultipleFails(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile.yaml"), []byte("identity:\n  name: demo\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile.toml"), []byte("[identity]\nname = 'demo'\n"), 0o644))
	_, err := projectfile.DetectPath(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "projectfile.toml")
	assert.Contains(t, err.Error(), "projectfile.yaml")
}

func TestMergePeopleRoleUnion(t *testing.T) {
	existing := []projectfile.Person{
		{Email: testAliceEmail, GivenNames: testAlice, FamilyNames: testSmith, Roles: []string{"author"}},
	}
	incoming := []projectfile.Person{
		{Email: testAliceEmail, GivenNames: testAlice, FamilyNames: testSmith, Roles: []string{"maintainer"}},
	}
	merged, conflicts := projectfile.MergePeople(existing, incoming)
	require.Empty(t, conflicts)
	require.Len(t, merged, 1)
	assert.Contains(t, merged[0].Roles, "author")
	assert.Contains(t, merged[0].Roles, "maintainer")
}

func TestMergePeopleGapFill(t *testing.T) {
	existing := []projectfile.Person{
		{Email: testBobEmail, FamilyNames: testBob},
	}
	incoming := []projectfile.Person{
		{Email: testBobEmail, FamilyNames: testBob, URL: "https://bob.dev"},
	}
	merged, conflicts := projectfile.MergePeople(existing, incoming)
	require.Empty(t, conflicts)
	require.Len(t, merged, 1)
	assert.Equal(t, "https://bob.dev", merged[0].URL)
}

func TestMergePeopleExistingWinsOnConflict(t *testing.T) {
	existing := []projectfile.Person{
		{Email: "carol@example.com", FamilyNames: testSmith},
	}
	incoming := []projectfile.Person{
		{Email: "carol@example.com", FamilyNames: "Jones"},
	}
	merged, conflicts := projectfile.MergePeople(existing, incoming)
	require.Len(t, conflicts, 1)
	assert.Equal(t, testSmith, merged[0].FamilyNames, "existing value must be kept on conflict")
	assert.Equal(t, "family-names", conflicts[0].Field)
}

func TestMergePeopleAppendNewIdentity(t *testing.T) {
	existing := []projectfile.Person{
		{Email: testAliceEmail, FamilyNames: testAlice},
	}
	incoming := []projectfile.Person{
		{Email: testBobEmail, FamilyNames: testBob},
	}
	merged, conflicts := projectfile.MergePeople(existing, incoming)
	require.Empty(t, conflicts)
	assert.Len(t, merged, 2)
}

func TestSetAndLookupExtension(t *testing.T) {
	doc := minimalDoc()
	const ns = "org.projectfile.ignores"
	val := map[string]any{testGenerate: []any{testGitignore}}
	projectfile.SetExtension(doc, ns, val)

	got, ok := projectfile.LookupExtension(doc, ns)
	require.True(t, ok)
	m, ok := got.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, val[testGenerate], m[testGenerate])
}

func TestLookupExtensionMissing(t *testing.T) {
	doc := minimalDoc()
	_, ok := projectfile.LookupExtension(doc, "org.projectfile.nonexistent")
	assert.False(t, ok)
}

func TestSetExtensionOverwritesNestedForm(t *testing.T) {
	doc := minimalDoc()
	// Simulate what go-toml produces for [org.projectfile.ignores]: a nested map.
	doc.Extensions = map[string]any{
		"org": map[string]any{
			"projectfile": map[string]any{
				"ignores": map[string]any{testGenerate: []any{testGitignore}},
			},
		},
	}
	// SetExtension should prune the nested form and write the flat key.
	newVal := map[string]any{testGenerate: []any{".dockerignore"}}
	projectfile.SetExtension(doc, "org.projectfile.ignores", newVal)

	got, ok := projectfile.LookupExtension(doc, "org.projectfile.ignores")
	require.True(t, ok)
	m := got.(map[string]any)
	assert.Equal(t, newVal[testGenerate], m[testGenerate])
}

// A deliberate empty map inside an extension namespace (e.g. org.projectfile.ci's
// zero-input `dispatch: {}` button) must survive serialization — pf-ci reads
// its presence as "button declared". pruneEmptyMaps must not touch extension data.
func TestToMapPreservesExtensionEmptyMap(t *testing.T) {
	doc := minimalDoc()
	projectfile.SetExtension(doc, "org.projectfile.ci", map[string]any{
		"nodes": map[string]any{
			"g": map[string]any{"goal": true, "dispatch": map[string]any{}},
		},
	})

	m := doc.ToMap()
	node := m["org"].(map[string]any)["projectfile"].(map[string]any)["ci"].(map[string]any)["nodes"].(map[string]any)["g"].(map[string]any)
	_, ok := node["dispatch"]
	assert.True(t, ok, "empty dispatch{} in extension must survive ToMap")
}

// A native del-orphaned empty container is still pruned (the original contract).
func TestToMapPrunesNativeEmptyMap(t *testing.T) {
	doc := minimalDoc()
	doc.Requirements = &projectfile.Requirements{}

	m := doc.ToMap()
	_, ok := m["requirements"]
	assert.False(t, ok, "empty native requirements{} must be pruned from ToMap")
}

func TestRoundTripExtensions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projectfile.yaml")
	doc := minimalDoc()
	projectfile.SetExtension(doc, "org.projectfile.ignores", map[string]any{
		testGenerate: []any{testGitignore, ".dockerignore"},
	})
	require.NoError(t, projectfile.Write(doc, path))
	got, err := projectfile.ReadFromPath(path)
	require.NoError(t, err)
	_, ok := projectfile.LookupExtension(got, "org.projectfile.ignores")
	assert.True(t, ok, "extension must survive YAML round-trip")
}

func TestYAMLReuseHeadersPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projectfile.yaml")
	const content = "# SPDX-FileCopyrightText: 2026 Alice\n# SPDX-License-Identifier: MIT\n\n---\n$schema: https://projectfile.org/schema/v1.json\nidentity:\n  name: reuse-test\nlicense:\n  spdx: MIT\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	doc, err := projectfile.ReadFromPath(path)
	require.NoError(t, err)
	doc.Keywords = []string{"updated"}
	require.NoError(t, projectfile.Write(doc, path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	written := string(data)
	assert.Contains(t, written, "SPDX-FileCopyrightText: 2026 Alice")
	assert.Contains(t, written, "SPDX-License-Identifier: MIT")
	assert.Contains(t, written, "updated")
}

func TestYAMLInlineCommentsPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projectfile.yaml")
	const content = "---\n$schema: https://projectfile.org/schema/v1.json\n# technologies comment above key\ntechnologies:\n  - python\n# identity block comment\nidentity:\n  name: comment-test\nlicense:\n  spdx: MIT\n# extension comment\ncom:\n  example:\n    ci:\n      runner: linux\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	doc, err := projectfile.ReadFromPath(path)
	require.NoError(t, err)
	doc.Keywords = []string{"new-kw"}
	require.NoError(t, projectfile.Write(doc, path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	written := string(data)
	assert.Contains(t, written, "technologies comment above key")
	assert.Contains(t, written, "extension comment")
	assert.Contains(t, written, "new-kw")
	assert.Contains(t, written, "runner: linux")
}

func TestTOMLReuseHeadersPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projectfile.toml")
	const reuseBlock = "# SPDX-FileCopyrightText: 2026 Bob\n# SPDX-License-Identifier: Apache-2.0\n\n#:schema https://projectfile.org/schema/v1.json\n"
	require.NoError(t, os.WriteFile(path, []byte(reuseBlock+"spec_version = \"1\"\n[identity]\nname = \"reuse-toml\"\n[license]\nspdx = \"Apache-2.0\"\n"), 0o644))

	doc, err := projectfile.ReadFromPath(path)
	require.NoError(t, err)
	doc.Keywords = []string{"updated"}
	require.NoError(t, projectfile.Write(doc, path))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "SPDX-FileCopyrightText: 2026 Bob")
	assert.Contains(t, content, "SPDX-License-Identifier: Apache-2.0")
	assert.Contains(t, content, "#:schema https://projectfile.org/schema/v1.json")
	assert.Contains(t, content, "updated")
}

// Test values used across the §139-preservation tests. Hoisted to package
// consts so the goconst linter (which runs on tests in this repo) does not
// trip over the repeated literals.
const (
	testExtraKept    = "kept"
	testExtraMutated = "mutated"
	testExtraKey     = "k"
	testExtraVal     = "v"
	testRepoType     = "git"
)

var roundTripExts = []string{"yaml", "toml", "json"}

// TestSection4ExtraKeysRoundTrip guards spec §139: every §4 mapping struct
// (identity, repositories, license, copyright, people, organizations,
// requirements, links) MUST preserve unknown "additional" keys
// across parse→serialize→parse, for all three encodings. Before the fix these
// keys were silently dropped by the closed struct parsers.
func TestSection4ExtraKeysRoundTrip(t *testing.T) {
	for _, ext := range roundTripExts {
		t.Run(ext, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "projectfile."+ext)
			doc := minimalDoc()
			doc.Identity.Extra = map[string]any{"x-id-note": testExtraKept}
			doc.Repositories = []projectfile.Repository{
				{URL: "ssh://git@codeberg.org/acme/proj.git", Type: testRepoType, Role: projectfile.RepositoryRoleOrigin, Extra: map[string]any{"cffr": true}},
			}
			doc.License.Extra = map[string]any{"x-lic-note": testExtraKept}
			doc.Copyright = &projectfile.Copyright{Year: 2024, Extra: map[string]any{"x-cop-note": testExtraKept}}
			doc.People = []projectfile.Person{
				{FamilyNames: testSmith, GivenNames: testAlice, Email: testAliceEmail, Extra: map[string]any{"x-person-note": testExtraKept}},
			}
			doc.Organizations = []projectfile.Organization{
				{Name: "Acme", Extra: map[string]any{"x-org-note": testExtraKept}},
			}
			doc.Requirements = &projectfile.Requirements{Runtime: map[string]string{"node": ">=24"}, Extra: map[string]any{"x-req-note": testExtraKept}}
			doc.Links = []projectfile.Link{
				{Type: projectfile.LinkHomepage, URL: "https://example.com", Extra: map[string]any{"x-link-note": testExtraKept}},
			}
			require.NoError(t, projectfile.Write(doc, path))

			got, err := projectfile.ReadFromPath(path)
			require.NoError(t, err)

			assert.Equal(t, testExtraKept, got.Identity.Extra["x-id-note"], "identity extra key lost")
			require.Len(t, got.Repositories, 1)
			assert.Equal(t, true, got.Repositories[0].Extra["cffr"], "repository cffr key lost")
			require.NotNil(t, got.License)
			assert.Equal(t, testExtraKept, got.License.Extra["x-lic-note"], "license extra key lost")
			require.NotNil(t, got.Copyright)
			assert.Equal(t, testExtraKept, got.Copyright.Extra["x-cop-note"], "copyright extra key lost")
			require.Len(t, got.People, 1)
			assert.Equal(t, testExtraKept, got.People[0].Extra["x-person-note"], "person extra key lost")
			require.Len(t, got.Organizations, 1)
			assert.Equal(t, testExtraKept, got.Organizations[0].Extra["x-org-note"], "organization extra key lost")
			require.NotNil(t, got.Requirements)
			assert.Equal(t, testExtraKept, got.Requirements.Extra["x-req-note"], "requirements extra key lost")
			require.Len(t, got.Links, 1)
			assert.Equal(t, testExtraKept, got.Links[0].Extra["x-link-note"], "link extra key lost")
		})
	}
}

// TestClonePreservesExtra guards the deep-copy of Extra across Clone: a shared
// reference would let a mutation in the clone leak into the original, which is
// catastrophic for the dry-run planning path that clones before mutating.
func TestClonePreservesExtra(t *testing.T) {
	doc := minimalDoc()
	doc.Repositories = []projectfile.Repository{
		{URL: "u", Extra: map[string]any{testExtraKey: testExtraVal}},
	}
	doc.Identity.Extra = map[string]any{testExtraKey: testExtraVal}
	doc.People = []projectfile.Person{{FamilyNames: "n", Extra: map[string]any{testExtraKey: testExtraVal}}}
	doc.License.Extra = map[string]any{testExtraKey: testExtraVal}
	doc.Copyright = &projectfile.Copyright{Year: 1, Extra: map[string]any{testExtraKey: testExtraVal}}
	doc.Organizations = []projectfile.Organization{{Name: "n", Extra: map[string]any{testExtraKey: testExtraVal}}}
	doc.Requirements = &projectfile.Requirements{Extra: map[string]any{testExtraKey: testExtraVal}}
	doc.Links = []projectfile.Link{{Type: "t", URL: "u", Extra: map[string]any{testExtraKey: testExtraVal}}}

	cp := doc.Clone()
	// Mutate every Extra in the clone; none must reach the original.
	cp.Repositories[0].Extra[testExtraKey] = testExtraMutated
	cp.Identity.Extra[testExtraKey] = testExtraMutated
	cp.People[0].Extra[testExtraKey] = testExtraMutated
	cp.License.Extra[testExtraKey] = testExtraMutated
	cp.Copyright.Extra[testExtraKey] = testExtraMutated
	cp.Organizations[0].Extra[testExtraKey] = testExtraMutated
	cp.Requirements.Extra[testExtraKey] = testExtraMutated
	cp.Links[0].Extra[testExtraKey] = testExtraMutated

	assert.Equal(t, testExtraVal, doc.Repositories[0].Extra[testExtraKey], "repository Extra shared with clone")
	assert.Equal(t, testExtraVal, doc.Identity.Extra[testExtraKey], "identity Extra shared with clone")
	assert.Equal(t, testExtraVal, doc.People[0].Extra[testExtraKey], "person Extra shared with clone")
	assert.Equal(t, testExtraVal, doc.License.Extra[testExtraKey], "license Extra shared with clone")
	assert.Equal(t, testExtraVal, doc.Copyright.Extra[testExtraKey], "copyright Extra shared with clone")
	assert.Equal(t, testExtraVal, doc.Organizations[0].Extra[testExtraKey], "organization Extra shared with clone")
	assert.Equal(t, testExtraVal, doc.Requirements.Extra[testExtraKey], "requirements Extra shared with clone")
	assert.Equal(t, testExtraVal, doc.Links[0].Extra[testExtraKey], "link Extra shared with clone")
}

// TestKnownKeysNotDuplicatedInExtra guards the known-keys sets: a spec key
// must never be misclassified as "extra" and written twice (once typed, once
// raw). This catches a known-set that drifted out of sync with the parser. The
// observable signal is that Extra stays empty when only known keys are present.
func TestKnownKeysNotDuplicatedInExtra(t *testing.T) {
	for _, ext := range roundTripExts {
		t.Run(ext, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "projectfile."+ext)
			doc := minimalDoc()
			// Every known key set; no foreign keys anywhere.
			doc.Repositories = []projectfile.Repository{
				{URL: "u", Type: testRepoType, Path: "p", Branch: "b", Issues: true, Role: projectfile.RepositoryRoleOrigin},
			}
			doc.License = &projectfile.License{Spdx: "MIT", Covers: "project", File: "LICENSE"}
			doc.Copyright = &projectfile.Copyright{Year: 2024}
			doc.People = []projectfile.Person{{FamilyNames: "f", GivenNames: "g", Email: "e", URL: "x", Roles: []string{"maintainer"}}}
			doc.Links = []projectfile.Link{{Type: "t", URL: "u", Preferred: true, Derived: true}}
			require.NoError(t, projectfile.Write(doc, path))

			got, err := projectfile.ReadFromPath(path)
			require.NoError(t, err)

			assert.Empty(t, got.Identity.Extra, "identity Extra must be empty with only known keys")
			require.Len(t, got.Repositories, 1)
			assert.Empty(t, got.Repositories[0].Extra, "repository Extra must be empty with only known keys")
			require.NotNil(t, got.License)
			assert.Empty(t, got.License.Extra, "license Extra must be empty with only known keys")
			require.NotNil(t, got.Copyright)
			assert.Empty(t, got.Copyright.Extra, "copyright Extra must be empty with only known keys")
			require.Len(t, got.People, 1)
			assert.Empty(t, got.People[0].Extra, "person Extra must be empty with only known keys")
			require.Len(t, got.Links, 1)
			assert.Empty(t, got.Links[0].Extra, "link Extra must be empty with only known keys")
		})
	}
}

func TestToMapCoversOnlyLicenseOmitsEmptySpdx(t *testing.T) {
	doc := minimalDoc()
	doc.License = &projectfile.License{Covers: "project"}
	m := doc.ToMap()
	lic, ok := m["license"].(map[string]any)
	require.True(t, ok, "covers-only license must survive ToMap")
	assert.Equal(t, "project", lic["covers"])
	_, hasSpdx := lic["spdx"]
	assert.False(t, hasSpdx, "empty spdx must not be emitted")
}

// REUSE-IgnoreEnd
