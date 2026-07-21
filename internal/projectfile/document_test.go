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

	"kiota.ch/projectfile/core/internal/projectfile"
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
// zero-input `dispatch: {}` button) must survive serialization — ci-resolver reads
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

// REUSE-IgnoreEnd
