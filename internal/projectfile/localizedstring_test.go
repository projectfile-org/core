// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	kiotaprojectfile "kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// ExtractLocalizedStringForLang must prefer the requested lang, then fall back
// en → first non-empty, with Bare always winning (it is language-agnostic).
// The readme/i18n fixtures that used to live alongside this test moved to
// bridge/internal/pfmodel when GetReadmeExtension left core; this core-only
// assertion stays because ExtractLocalizedString is core machinery.
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
