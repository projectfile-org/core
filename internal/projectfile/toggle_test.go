// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseToggleBoolTrue(t *testing.T) {
	tg := ParseToggle(true)
	assert.True(t, tg.Set())
	assert.Equal(t, "all", tg.Mode())
	assert.True(t, tg.Allow("anything", false), "bool true passes every key")
	assert.True(t, tg.Allow("anything", true))
}

func TestParseToggleBoolFalse(t *testing.T) {
	tg := ParseToggle(false)
	assert.True(t, tg.Set())
	assert.Equal(t, "none", tg.Mode())
	assert.False(t, tg.Allow("anything", true), "bool false blocks every key")
	assert.False(t, tg.Allow("anything", false))
}

func TestParseToggleAllowlist(t *testing.T) {
	tg := ParseToggle(map[string]any{
		"github.com":   true,
		"kiota.ch":     false,
		"codeberg.org": true,
	})
	assert.True(t, tg.Set())
	assert.Equal(t, "allowlist", tg.Mode())
	assert.True(t, tg.Allow("github.com", false), "true key passes regardless of default")
	assert.True(t, tg.Allow("codeberg.org", false))
	assert.False(t, tg.Allow("kiota.ch", true), "false key is blocked regardless of default")
	assert.False(t, tg.Allow("gitlab.com", true), "absent key is blocked in allowlist mode")
}

func TestParseToggleAllowlistBoolMap(t *testing.T) {
	tg := ParseToggle(map[string]bool{"mastodon": true})
	assert.Equal(t, "allowlist", tg.Mode())
	assert.True(t, tg.Allow("mastodon", false))
	assert.False(t, tg.Allow("github", true))
}

func TestParseToggleEmptyMapIsUnset(t *testing.T) {
	tg := ParseToggle(map[string]any{})
	assert.False(t, tg.Set(), "empty map is treated as unset, not allow-none")
	assert.Equal(t, "unset", tg.Mode())
	assert.True(t, tg.Allow("x", true), "unset honours caller default")
	assert.False(t, tg.Allow("x", false))
}

func TestParseToggleNilAndBogus(t *testing.T) {
	for _, raw := range []any{nil, "true", 42, []string{"x"}} {
		tg := ParseToggle(raw)
		assert.False(t, tg.Set(), "%v must parse as unset", raw)
		assert.Equal(t, "unset", tg.Mode())
	}
}

func TestZeroToggleUnset(t *testing.T) {
	var tg Toggle
	assert.False(t, tg.Set())
	assert.Equal(t, "unset", tg.Mode())
	assert.True(t, tg.Allow("k", true), "zero Toggle honours the caller default")
	assert.False(t, tg.Allow("k", false))
}

func TestEnabledGatesOptIn(t *testing.T) {
	assert.False(t, ParseToggle(nil).Enabled(), "unset is not enabled")
	assert.False(t, ParseToggle(false).Enabled(), "explicit false is not enabled")
	assert.False(t, ParseToggle(map[string]any{}).Enabled(), "empty map is unset")
	assert.True(t, ParseToggle(true).Enabled(), "bool true is enabled")
	assert.True(t, ParseToggle(map[string]any{"a": true}).Enabled(), "allowlist is enabled even if a key is false")
	assert.True(t, ParseToggle(map[string]any{"a": false}).Enabled(), "allowlist is enabled by presence, not by key values")
}
