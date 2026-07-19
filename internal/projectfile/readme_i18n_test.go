// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kiotaprojectfile "kiota.ch/projectfile/core/pkg/projectfile"
)

// ExtractLocalizedStringForLang must prefer the requested lang, then fall back
// en → first non-empty, with Bare always winning (it is language-agnostic).
func TestExtractLocalizedStringForLang(t *testing.T) {
	multi := &kiotaprojectfile.LocalizedString{
		Langs: map[string]string{"en": "Hi", "es": "Hola", "uk": "Привіт"},
	}

	assert.Equal(t, "Hola", kiotaprojectfile.ExtractLocalizedStringForLang(multi, "es"),
		"requested lang wins when present")
	assert.Equal(t, "Привіт", kiotaprojectfile.ExtractLocalizedStringForLang(multi, "uk"),
		"requested lang wins when present")
	assert.Equal(t, "Hi", kiotaprojectfile.ExtractLocalizedStringForLang(multi, "en"),
		"en resolves directly")
	assert.Equal(t, "Hi", kiotaprojectfile.ExtractLocalizedStringForLang(multi, "fr"),
		"unknown lang falls back to en")
	assert.Equal(t, "Hi", kiotaprojectfile.ExtractLocalizedStringForLang(multi, ""),
		"empty lang behaves like ExtractLocalizedString")
	assert.Equal(t, "Hi", kiotaprojectfile.ExtractLocalizedString(multi),
		"ExtractLocalizedString matches empty-lang variant")

	// Bare form is language-agnostic and always wins, regardless of lang asked.
	bare := &kiotaprojectfile.LocalizedString{Bare: "Bare"}
	assert.Equal(t, "Bare", kiotaprojectfile.ExtractLocalizedStringForLang(bare, "es"))

	// en-only content falls back through to en for any requested lang.
	enOnly := &kiotaprojectfile.LocalizedString{Langs: map[string]string{"en": "Only"}}
	assert.Equal(t, "Only", kiotaprojectfile.ExtractLocalizedStringForLang(enOnly, "es"))

	// nil is safe.
	assert.Empty(t, kiotaprojectfile.ExtractLocalizedStringForLang(nil, "es"))
}

// GetReadmeExtension must preserve the lang→text map of an extras content
// entry so language-aware renderers can resolve per-lang, while Content still
// carries the default resolution for existing consumers.
func TestGetReadmeExtensionExtrasPreservesLangMap(t *testing.T) {
	doc := &kiotaprojectfile.Document{
		Extensions: map[string]any{
			"org.projectfile.readme": map[string]any{
				"extras": []any{
					map[string]any{
						"name": "notice",
						"content": map[string]any{
							"en": "Hello",
							"es": "Hola",
							"uk": "Привіт",
						},
					},
				},
			},
		},
	}

	ext, err := kiotaprojectfile.GetReadmeExtension(doc)
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.Len(t, ext.Extras, 1)

	assert.Equal(t, "notice", ext.Extras[0].Name)
	assert.Equal(t, "Hello", ext.Extras[0].Content,
		"Content carries the default (en) resolution for back-compat")
	require.NotNil(t, ext.Extras[0].ContentByLang,
		"ContentByLang preserves the raw map")
	assert.Equal(t, "Hola", ext.Extras[0].ContentByLang["es"])
	assert.Equal(t, "Привіт", ext.Extras[0].ContentByLang["uk"])
}

// A bare-string extras content entry must NOT populate ContentByLang — the
// bridge treats ContentByLang == nil as "single-language, no per-lang render".
func TestGetReadmeExtensionExtrasBareStringHasNilContentByLang(t *testing.T) {
	doc := &kiotaprojectfile.Document{
		Extensions: map[string]any{
			"org.projectfile.readme": map[string]any{
				"extras": []any{
					map[string]any{
						"name":    "notice",
						"content": "Just a string",
					},
				},
			},
		},
	}

	ext, err := kiotaprojectfile.GetReadmeExtension(doc)
	require.NoError(t, err)
	require.Len(t, ext.Extras, 1)
	assert.Equal(t, "Just a string", ext.Extras[0].Content)
	assert.Nil(t, ext.Extras[0].ContentByLang,
		"bare string has no lang map")
}
