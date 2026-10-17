// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

func visibilityDoc(value string) *projectfile.Document {
	doc := minimalDoc()
	doc.Identity.Visibility = value
	return doc
}

func TestVisibilityDefaultsToPrivate(t *testing.T) {
	// An omitted field is private, so a publisher that skips on IsPrivate skips
	// a document that never mentioned visibility at all.
	for _, value := range []string{"", projectfile.VisibilityPrivate} {
		doc := visibilityDoc(value)
		assert.True(t, projectfile.IsPrivate(doc), "visibility=%q", value)
		assert.False(t, projectfile.IsPublic(doc), "visibility=%q", value)
	}
}

func TestVisibilityPublicIsTheOnlyPublicAnswer(t *testing.T) {
	doc := visibilityDoc(projectfile.VisibilityPublic)
	assert.False(t, projectfile.IsPrivate(doc))
	assert.True(t, projectfile.IsPublic(doc))
	assert.Equal(t, projectfile.VisibilityPublic, projectfile.Visibility(doc))
}

func TestVisibilityNilDocument(t *testing.T) {
	assert.Equal(t, "", projectfile.Visibility(nil))
	assert.True(t, projectfile.IsPrivate(nil))
	assert.False(t, projectfile.IsPublic(nil))
}

func TestVisibilityRoundTrip(t *testing.T) {
	for _, value := range []string{projectfile.VisibilityPublic, projectfile.VisibilityPrivate} {
		dir := t.TempDir()
		path := filepath.Join(dir, "projectfile.yaml")
		require.NoError(t, projectfile.Write(visibilityDoc(value), path))
		got, err := projectfile.ReadFromPath(path)
		require.NoError(t, err)
		assert.Equal(t, value, got.Identity.Visibility)
		assert.NotContains(t, got.Identity.Extra, "visibility", "a known field must not also round-trip as an extra key")
	}
}

func TestVisibilityOmittedStaysOmitted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "projectfile.yaml")
	require.NoError(t, projectfile.Write(visibilityDoc(""), path))
	got, err := projectfile.ReadFromPath(path)
	require.NoError(t, err)
	assert.Equal(t, "", got.Identity.Visibility)
	assert.Empty(t, got.Identity.Extra)
}

func TestLinkTags(t *testing.T) {
	untagged := projectfile.Link{Type: projectfile.LinkSourceCode, URL: "https://forge.example/x"}
	assert.False(t, projectfile.LinkHasTag(untagged, projectfile.TagPublic))

	tagged := projectfile.Link{
		Type:  projectfile.LinkSourceCode,
		URL:   "https://forge.example/x",
		Extra: map[string]any{"tags": []any{"badges", projectfile.TagPublic}},
	}
	assert.True(t, projectfile.LinkHasTag(tagged, projectfile.TagPublic))
	assert.True(t, projectfile.LinkHasTag(tagged, "badges"))
	assert.False(t, projectfile.LinkHasTag(tagged, "support"))
	assert.Equal(t, []string{"badges", projectfile.TagPublic}, projectfile.LinkTags(tagged))
}

func TestLinkTagsRejectsAScalar(t *testing.T) {
	// A hand-edited `tags: public` is a mistake, not a tag list: the alias
	// deriver ignores it, so the validator must not read publicity out of it either.
	l := projectfile.Link{Extra: map[string]any{"tags": projectfile.TagPublic}}
	assert.False(t, projectfile.LinkHasTag(l, projectfile.TagPublic))
}
