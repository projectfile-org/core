// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// REUSE-IgnoreStart

package projectfile_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

const (
	sampleYAML = "identity:\n  name: sample\n  version: 1.0.0\nkeywords:\n  - rust\n"
	sampleTOML = "keywords = [\"rust\"]\n\n[identity]\nname = \"sample\"\nversion = \"1.0.0\"\n"
	sampleJSON = `{"identity":{"name":"sample","version":"1.0.0"},"keywords":["rust"]}`
)

// seedDir writes one projectfile into a fresh dir and returns the dir.
func seedDir(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
	return dir
}

// isolateCache points XDG_CACHE_HOME at a fresh temp dir so a purge never touches the real slot.
func isolateCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	return dir
}

// ── the directory-level read surface ─────────────────────────────────────────

// TestReadDirectorySurface walks every entry point that begins from a directory.
func TestReadDirectorySurface(t *testing.T) {
	dir := seedDir(t, "projectfile.yaml", sampleYAML)

	doc, path, err := projectfile.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "projectfile.yaml"), path)
	assert.Equal(t, "sample", doc.Identity.Name)

	doc, _, err = projectfile.ReadWithOptions(dir, projectfile.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, "sample", doc.Identity.Name)

	raw, path, err := projectfile.ReadRaw(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "projectfile.yaml"), path)
	assert.Equal(t, "sample", raw["identity"].(map[string]any)["name"])

	raw, _, err = projectfile.ReadRawWithOptions(dir, projectfile.ReadOptions{})
	require.NoError(t, err)
	assert.Contains(t, raw, "identity")

	doc, path, err = projectfile.ReadBase(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "projectfile.yaml"), path)
	assert.Equal(t, "sample", doc.Identity.Name)
}

func TestReadEveryEncoding(t *testing.T) {
	for _, tc := range []struct {
		file string
		body string
	}{
		{"projectfile.yaml", sampleYAML},
		{"projectfile.yml", sampleYAML},
		{"projectfile.toml", sampleTOML},
		{"projectfile.json", sampleJSON},
	} {
		t.Run(tc.file, func(t *testing.T) {
			dir := seedDir(t, tc.file, tc.body)
			doc, path, err := projectfile.Read(dir)
			require.NoError(t, err)
			assert.Equal(t, tc.file, filepath.Base(path))
			assert.Equal(t, "sample", doc.Identity.Name, "every encoding must yield the same document")
			assert.Equal(t, []string{"rust"}, doc.Keywords)
		})
	}
}

// TestReadRawFromPathSkipsDetection is why this lower entry point exists.
func TestReadRawFromPathSkipsDetection(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "projectfile.toml")
	require.NoError(t, os.WriteFile(tomlPath, []byte(sampleTOML), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile.yaml"), []byte(sampleYAML), 0o644))

	raw, err := projectfile.ReadRawFromPath(tomlPath)
	require.NoError(t, err)
	assert.Equal(t, "sample", raw["identity"].(map[string]any)["name"])
}

func TestReadRawFromPathMissingFile(t *testing.T) {
	_, err := projectfile.ReadRawFromPath(filepath.Join(t.TempDir(), "nope.yaml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read ")
}

func TestReadRawFromPathUnsupportedExtension(t *testing.T) {
	p := filepath.Join(t.TempDir(), "projectfile.ini")
	require.NoError(t, os.WriteFile(p, []byte("[identity]\n"), 0o644))
	_, err := projectfile.ReadRawFromPath(p)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported projectfile extension")
}

func TestReadMalformedEveryEncoding(t *testing.T) {
	for _, tc := range []struct{ file, body string }{
		{"projectfile.yaml", "identity: [\n"},
		{"projectfile.yml", "identity: [\n"},
		{"projectfile.toml", "identity = \n"},
		{"projectfile.json", `{"identity":`},
	} {
		t.Run(tc.file, func(t *testing.T) {
			dir := seedDir(t, tc.file, tc.body)
			_, _, err := projectfile.Read(dir)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "parse ")
		})
	}
}

func TestReadFromPathMissingFile(t *testing.T) {
	_, err := projectfile.ReadFromPath(filepath.Join(t.TempDir(), "nope.yaml"))
	require.Error(t, err)
}

func TestReadNoProjectfile(t *testing.T) {
	_, _, err := projectfile.Read(t.TempDir())
	require.Error(t, err)
}

// TestReadBaseIgnoresIncludes is the base-write invariant at its source.
func TestReadBaseIgnoresIncludes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "inc.yaml"),
		[]byte("keywords:\n  - from-include\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile.yaml"),
		[]byte("identity:\n  name: base\nincludes:\n  - inc.yaml\n"), 0o644))

	base, _, err := projectfile.ReadBase(dir)
	require.NoError(t, err)
	assert.Empty(t, base.Keywords, "the base document must not carry include contributions")

	merged, _, err := projectfile.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"from-include"}, merged.Keywords, "the merged view does carry them")
}

// ── write: clean + sorted ────────────────────────────────────────────────────

// TestWriteCleanDropsTheCanvas is the convert contract.
func TestWriteCleanDropsTheCanvas(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "projectfile.yaml")
	require.NoError(t, os.WriteFile(src, []byte("# leading comment\nidentity:\n  name: sample\n"), 0o644))

	doc, err := projectfile.ReadFromPath(src)
	require.NoError(t, err)

	dst := filepath.Join(dir, "projectfile.json")
	require.NoError(t, projectfile.WriteClean(doc, dst))

	body, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "leading comment", "a clean write carries no comment over")

	reread, err := projectfile.ReadFromPath(dst)
	require.NoError(t, err)
	assert.Equal(t, "sample", reread.Identity.Name)
}

func TestWriteCleanEveryEncoding(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".yaml", ".yml", ".toml", ".json"} {
		t.Run(ext, func(t *testing.T) {
			p := filepath.Join(dir, "out"+ext)
			require.NoError(t, projectfile.WriteClean(minimalDoc(), p))
			got, err := projectfile.ReadFromPath(p)
			require.NoError(t, err)
			assert.Equal(t, "test-project", got.Identity.Name)
		})
	}
}

// TestYAMLOutputSortedToggle pins the cross-module toggle reaches core.
func TestYAMLOutputSortedToggle(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "probe.yaml")

	projectfile.SetYAMLOutputSorted(true)
	t.Cleanup(func() { projectfile.SetYAMLOutputSorted(false) })
	assert.True(t, projectfile.YAMLOutputSortedEnabled(), "the getter must see what the setter set")

	require.NoError(t, projectfile.Write(&projectfile.Document{}, p))
	body, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.NotEmpty(t, body)
}

func TestWriteUnsupportedExtension(t *testing.T) {
	err := projectfile.Write(minimalDoc(), filepath.Join(t.TempDir(), "projectfile.ini"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported projectfile extension")
}

// ── organization merge ───────────────────────────────────────────────────────

func TestOrganizationIdentityKey(t *testing.T) {
	assert.Equal(t, "orcid:0000-0002-1825-0097",
		projectfile.OrganizationIdentityKey(projectfile.Organization{Orcid: "https://orcid.org/0000-0002-1825-0097"}))
	assert.Equal(t, "email:a@x.test",
		projectfile.OrganizationIdentityKey(projectfile.Organization{Email: "A@X.test"}))
	assert.Equal(t, "name:kiota",
		projectfile.OrganizationIdentityKey(projectfile.Organization{Name: "  Kiota "}))
	assert.Equal(t, "name:", projectfile.OrganizationIdentityKey(projectfile.Organization{}))
}

// TestMergeOrganizationsNoMatchAppends is the entity twin of the person path.
func TestMergeOrganizationsNoMatchAppends(t *testing.T) {
	merged, conflicts := projectfile.MergeOrganizations(
		[]projectfile.Organization{{Name: "Kiota"}},
		[]projectfile.Organization{{Name: "Other"}},
	)
	require.Len(t, merged, 2)
	assert.Empty(t, conflicts)
	assert.Equal(t, "Kiota", merged[0].Name, "existing keeps its position")
	assert.Equal(t, "Other", merged[1].Name)
}

func TestMergeOrganizationsUnionsRoles(t *testing.T) {
	merged, _ := projectfile.MergeOrganizations(
		[]projectfile.Organization{{Name: "Kiota", Roles: []string{"maintainer", "sponsor"}}},
		[]projectfile.Organization{{Name: "Kiota", Roles: []string{"sponsor", "author"}}},
	)
	require.Len(t, merged, 1)
	assert.Equal(t, []string{"maintainer", "sponsor", "author"}, merged[0].Roles)
}

// TestMergeOrganizationsFillsBlanksAndKeepsExisting pins the per-field rule.
func TestMergeOrganizationsFillsBlanksAndKeepsExisting(t *testing.T) {
	merged, conflicts := projectfile.MergeOrganizations(
		[]projectfile.Organization{{Name: "Kiota", URL: ""}},
		[]projectfile.Organization{{Name: "Kiota", URL: "https://kiota.test", Alias: "K"}},
	)
	require.Len(t, merged, 1)
	assert.Equal(t, "https://kiota.test", merged[0].URL, "an empty existing field is filled")
	assert.Equal(t, "K", merged[0].Alias, "an absent existing field is filled")
	assert.Empty(t, conflicts)

	merged, conflicts = projectfile.MergeOrganizations(
		[]projectfile.Organization{{Name: "Kiota", URL: "https://old.test"}},
		[]projectfile.Organization{{Name: "Kiota", URL: "https://new.test"}},
	)
	require.Len(t, merged, 1)
	assert.Equal(t, "https://old.test", merged[0].URL, "existing wins")
	require.Len(t, conflicts, 1)
	assert.Equal(t, "url", conflicts[0].Field)
	assert.Equal(t, "https://old.test", conflicts[0].Existing)
	assert.Equal(t, "https://new.test", conflicts[0].Incoming)
}

// TestMergeOrganizationsMergesHandles pins handle unioning.
func TestMergeOrganizationsMergesHandles(t *testing.T) {
	merged, _ := projectfile.MergeOrganizations(
		[]projectfile.Organization{{Name: "Kiota", Handles: map[string]any{"github": "kiota"}}},
		[]projectfile.Organization{{Name: "Kiota", Handles: map[string]any{"github": "other", "gitlab": "kiota"}}},
	)
	require.Len(t, merged, 1)
	assert.Equal(t, "kiota", merged[0].Handles["github"], "an existing handle is kept")
	assert.Equal(t, "kiota", merged[0].Handles["gitlab"], "a new handle is added")
}

// TestMergeOrganizationsMatchesOnORCID pins the decisive top tier.
func TestMergeOrganizationsMatchesOnORCID(t *testing.T) {
	merged, _ := projectfile.MergeOrganizations(
		[]projectfile.Organization{{Name: "Kiota", Orcid: "0000-0002-1825-0097"}},
		[]projectfile.Organization{{Name: "Renamed", Orcid: "https://orcid.org/0000-0002-1825-0097"}},
	)
	require.Len(t, merged, 1)
	assert.Equal(t, "Kiota", merged[0].Name)
}

// TestMergeOrganizationsMatchesOnEmailFallsThroughToName pins the middle tier.
func TestMergeOrganizationsMatchesOnEmailFallsThroughToName(t *testing.T) {
	merged, _ := projectfile.MergeOrganizations(
		[]projectfile.Organization{{Name: "Kiota", Email: "old@kiota.test"}},
		[]projectfile.Organization{{Name: "Kiota", Email: "new@kiota.test"}},
	)
	require.Len(t, merged, 1, "a user who switched email must not be duplicated")

	merged, _ = projectfile.MergeOrganizations(
		[]projectfile.Organization{{Name: "Kiota", Email: "a@kiota.test"}},
		[]projectfile.Organization{{Name: "Other", Email: "a@kiota.test"}},
	)
	require.Len(t, merged, 1, "a shared email collapses them even with different names")
}

func TestMergeOrganizationsEmptyInputs(t *testing.T) {
	merged, conflicts := projectfile.MergeOrganizations(nil, nil)
	assert.Empty(t, merged)
	assert.Empty(t, conflicts)
}

// ── localized string helpers ─────────────────────────────────────────────────

// TestSetLocalizedENFollowsTheDocumentsForm is the trap this helper exists to avoid.
func TestSetLocalizedENFollowsTheDocumentsForm(t *testing.T) {
	var nilField *projectfile.LocalizedString
	projectfile.SetLocalizedEN(&nilField, "First")
	require.NotNil(t, nilField)
	assert.Equal(t, "First", nilField.Bare, "an absent field starts in the Bare form")

	bare := &projectfile.LocalizedString{Bare: "Old"}
	projectfile.SetLocalizedEN(&bare, "New")
	assert.Equal(t, "New", bare.Bare, "a Bare document stays Bare")
	assert.Empty(t, bare.Langs)

	langs := &projectfile.LocalizedString{Langs: map[string]string{"en": "Old", "es": "Viejo"}}
	projectfile.SetLocalizedEN(&langs, "New")
	assert.Equal(t, "New", langs.Langs["en"])
	assert.Equal(t, "Viejo", langs.Langs["es"], "other languages are untouched")
	assert.Empty(t, langs.Bare, "Bare is cleared so the map is the single source")
}

// TestLocalizedStringRoundTripsBothForms pins that either spelling survives a cycle.
func TestLocalizedStringRoundTripsBothForms(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name string
		ls   *projectfile.LocalizedString
	}{
		{"bare", &projectfile.LocalizedString{Bare: "Solo"}},
		{"langs", &projectfile.LocalizedString{Langs: map[string]string{"en": "Hi", "es": "Hola"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(dir, tc.name+".yaml")
			doc := minimalDoc()
			doc.Identity.Title = tc.ls
			require.NoError(t, projectfile.Write(doc, p))
			got, err := projectfile.ReadFromPath(p)
			require.NoError(t, err)
			require.NotNil(t, got.Identity.Title)
			assert.Equal(t, tc.ls, got.Identity.Title)
		})
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// TestAsStringList covers the string-or-sequence normalisation spec §4.4 admits.
func TestAsStringList(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []string
	}{
		{"nil", nil, nil},
		{"scalar", "one", []string{"one"}},
		{"scalar padded", "  one  ", []string{"one"}},
		{"blank scalar", "   ", nil},
		{"empty scalar", "", nil},
		{"string slice", []string{"a", "b"}, []string{"a", "b"}},
		{"string slice padded", []string{" a ", "", "  ", "b"}, []string{"a", "b"}},
		{"any slice", []any{"a", "b"}, []string{"a", "b"}},
		{"any slice with non-strings", []any{"a", 7, nil, "b"}, []string{"a", "b"}},
		{"any slice all non-strings", []any{7, nil}, []string{}},
		{"any slice all blank", []any{"", "  "}, []string{}},
		{"unsupported kind", 42, nil},
		{"bool", true, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, projectfile.AsStringList(tc.in))
		})
	}
}

// TestAsStringListBothSpellingsAgree pins the encoding-agnostic get/set pair.
func TestAsStringListBothSpellingsAgree(t *testing.T) {
	assert.Equal(t, []string{"a"}, projectfile.AsStringList("a"))
	assert.Equal(t, []string{"a", "b"}, projectfile.AsStringList([]string{"a", "b"}))
	assert.Equal(t, []string{"a", "b"}, projectfile.AsStringList([]any{"a", "b"}))
	assert.Equal(t, projectfile.AsStringList([]string{"a", "b"}), projectfile.AsStringList([]any{"a", "b"}))
}

// ── include entries + cache surface ──────────────────────────────────────────

// TestIncludeEntriesReadsBothForms pins the spec §4.9 shapes and pin normalisation.
func TestIncludeEntriesReadsBothForms(t *testing.T) {
	got := projectfile.IncludeEntries(map[string]any{
		"includes": []any{
			"https://example.com/a.yaml",
			map[string]any{"url": "https://example.com/b.yaml", "sha256": "ABCDEF"},
			map[string]any{"sha256": "only-pin"},
		},
	})
	require.Len(t, got, 3)
	assert.Equal(t, "https://example.com/a.yaml", got[0].Ref)
	assert.Empty(t, got[0].SHA256)
	assert.Equal(t, "https://example.com/b.yaml", got[1].Ref)
	assert.Equal(t, "abcdef", got[1].SHA256, "a pin is stored lowercase")
	assert.Empty(t, got[2].Ref, "a mapping with no url yields an empty ref")
}

func TestIncludeEntriesFromExtensionNamespace(t *testing.T) {
	got := projectfile.IncludeEntries(map[string]any{
		"org.projectfile.cli": map[string]any{"includes": []any{"https://example.com/legacy.yaml"}},
	})
	require.Len(t, got, 1)
	assert.Equal(t, "https://example.com/legacy.yaml", got[0].Ref)
}

func TestIncludeEntriesNone(t *testing.T) {
	assert.Nil(t, projectfile.IncludeEntries(map[string]any{}))
	assert.Nil(t, projectfile.IncludeEntries(nil))
}

// TestHTTPIncludesFiltersLocalRefs pins that warm is handed only fetchable URLs.
func TestHTTPIncludes(t *testing.T) {
	got := projectfile.HTTPIncludes(map[string]any{
		"includes": []any{
			"https://example.com/a.yaml",
			"inc.yaml",
			"http://example.com/b.yaml",
			"../relative.yaml",
		},
	})
	assert.Equal(t, []string{"https://example.com/a.yaml", "http://example.com/b.yaml"}, got)
}

func TestIncludeDigest(t *testing.T) {
	assert.Equal(t,
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		projectfile.IncludeDigest(nil))
	assert.Len(t, projectfile.IncludeDigest([]byte("body")), 64)
	assert.NotEqual(t, projectfile.IncludeDigest([]byte("a")), projectfile.IncludeDigest([]byte("b")))
}

func TestWarmIncludeRejectsLocalRef(t *testing.T) {
	require.Error(t, projectfile.WarmInclude("inc.yaml"))
	require.Error(t, projectfile.WarmIncludeWithOptions("inc.yaml", projectfile.ReadOptions{}))
}

// TestWarmIncludeFillsTheCache is the warm command's load-bearing path.
func TestWarmIncludeFillsTheCache(t *testing.T) {
	isolateCache(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("identity:\n  name: warm\n"))
	}))
	t.Cleanup(srv.Close)

	require.NoError(t, projectfile.WarmInclude(srv.URL+"/inc.yaml"))

	dir, err := projectfile.IncludesCacheDir()
	require.NoError(t, err)
	entries, err := filepath.Glob(filepath.Join(dir, "*"))
	require.NoError(t, err)
	require.NotEmpty(t, entries, "a warmed include must land in the cache slot")
}

// TestWarmIncludeWithOptionsOfflineWritesNothing pins the offline degradation rule.
func TestWarmIncludeWithOptionsOfflineWritesNothing(t *testing.T) {
	isolateCache(t)
	_ = projectfile.WarmIncludeWithOptions("https://example.com/a.yaml", projectfile.ReadOptions{Offline: true})

	dir, err := projectfile.IncludesCacheDir()
	require.NoError(t, err)
	entries, err := filepath.Glob(filepath.Join(dir, "*"))
	require.NoError(t, err)
	assert.Empty(t, entries, "an offline warm must not write a cache entry")
}

func TestPurgeInclude(t *testing.T) {
	isolateCache(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("a: 1\n"))
	}))
	t.Cleanup(srv.Close)

	url := srv.URL + "/inc.yaml"
	require.NoError(t, projectfile.WarmInclude(url))

	removed, err := projectfile.PurgeInclude(url)
	require.NoError(t, err)
	assert.True(t, removed)

	removed, err = projectfile.PurgeInclude(url)
	require.NoError(t, err)
	assert.False(t, removed, "purging an absent include is a clean no-op")
}

// TestResolveIncludesOnlyWithNetworkRefs pins the warm walker's own spelling.
func TestResolveIncludesOnlyWithNetworkRefs(t *testing.T) {
	isolateCache(t)
	got, err := projectfile.ResolveIncludesOnly(map[string]any{
		"includes": []any{"https://example.com/never.yaml"},
	}, t.TempDir(), projectfile.ReadOptions{Offline: true})
	require.NoError(t, err, "an uncached offline include is skipped, not fatal")
	assert.Empty(t, got)
}
