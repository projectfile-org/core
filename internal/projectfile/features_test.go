// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testFeaturesNS = "org.projectfile.ci"

func levelDoc(v any) *Document {
	return &Document{Extensions: map[string]any{testFeaturesNS: map[string]any{FeaturesLevelKey: v}}}
}

func TestCheckFeaturesLevel_AbsentIsCompatible(t *testing.T) {
	require.NoError(t, CheckFeaturesLevel(&Document{}, testFeaturesNS, 1))
	require.NoError(t, CheckFeaturesLevel(&Document{Extensions: map[string]any{}}, testFeaturesNS, 1))
	require.NoError(t, CheckFeaturesLevel(&Document{Extensions: map[string]any{testFeaturesNS: map[string]any{}}}, testFeaturesNS, 1))
	require.NoError(t, CheckFeaturesLevel(&Document{Extensions: map[string]any{testFeaturesNS: "not-a-map"}}, testFeaturesNS, 1))
}

func TestCheckFeaturesLevel_CurrentAndOlderPass(t *testing.T) {
	require.NoError(t, CheckFeaturesLevel(levelDoc(1), testFeaturesNS, 1))
	require.NoError(t, CheckFeaturesLevel(levelDoc(0), testFeaturesNS, 1))
	require.NoError(t, CheckFeaturesLevel(levelDoc(int64(1)), testFeaturesNS, 1))
	require.NoError(t, CheckFeaturesLevel(levelDoc(float64(1)), testFeaturesNS, 1))
}

func TestCheckFeaturesLevel_NewerFails(t *testing.T) {
	err := CheckFeaturesLevel(levelDoc(2), testFeaturesNS, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), testFeaturesNS)
	assert.Contains(t, err.Error(), "2")
	assert.Contains(t, err.Error(), "at most 1")
}

func TestCheckFeaturesLevel_MalformedFails(t *testing.T) {
	for _, v := range []any{"2", 1.5, -1, true, []any{1}} {
		err := CheckFeaturesLevel(levelDoc(v), testFeaturesNS, 9)
		require.Error(t, err, "value %v must be rejected", v)
		assert.Contains(t, err.Error(), "non-negative integer")
	}
}
