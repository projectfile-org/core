// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSchema      = "$schema"
	testKindValue   = "SoftwareSourceCode"
	testIncludes    = "includes"
	testCommitStyle = "commit-style"
	testIncludeA    = "../a.yaml"
	testIncludeB    = "../b.yaml"
	testIncludeC    = "../c.yaml"
)

func TestStripRedundant_ScalarMatch(t *testing.T) {
	base := map[string]any{
		keyKind:    testKindValue,
		keyLicense: map[string]any{keySpdx: testMIT},
	}
	includes := map[string]any{
		keyKind: testKindValue,
	}
	removed := StripRedundant(base, includes)
	assert.Equal(t, []string{keyKind}, removed)
	assert.NotContains(t, base, keyKind)
	assert.Contains(t, base, keyLicense)
}

func TestStripRedundant_NestedMapFullMatch(t *testing.T) {
	base := map[string]any{
		keyLicense: map[string]any{keySpdx: testMIT},
	}
	includes := map[string]any{
		keyLicense: map[string]any{keySpdx: testMIT},
	}
	removed := StripRedundant(base, includes)
	assert.Equal(t, []string{keyLicense + "." + keySpdx}, removed)
	assert.NotContains(t, base, keyLicense)
}

func TestStripRedundant_NestedMapPartialMatch(t *testing.T) {
	base := map[string]any{
		testIdentity: map[string]any{
			keyName:      "cli",
			keyNamespace: "org.projectfile",
			"created":    "2026-05-28",
		},
	}
	includes := map[string]any{
		testIdentity: map[string]any{
			keyNamespace: "org.projectfile",
		},
	}
	removed := StripRedundant(base, includes)
	assert.Equal(t, []string{testIdentity + "." + keyNamespace}, removed)

	ident := base[testIdentity].(map[string]any)
	assert.NotContains(t, ident, keyNamespace)
	assert.Equal(t, "cli", ident[keyName])
	assert.Equal(t, "2026-05-28", ident["created"])
}

func TestStripRedundant_SliceMatch(t *testing.T) {
	base := map[string]any{
		keyTechnologies: []any{testDocker, "go"},
	}
	includes := map[string]any{
		keyTechnologies: []any{testDocker, "go"},
	}
	removed := StripRedundant(base, includes)
	assert.Equal(t, []string{keyTechnologies}, removed)
	assert.NotContains(t, base, keyTechnologies)
}

func TestStripRedundant_SliceMismatch(t *testing.T) {
	base := map[string]any{
		keyTechnologies: []any{testDocker, "go"},
	}
	includes := map[string]any{
		keyTechnologies: []any{testDocker},
	}
	removed := StripRedundant(base, includes)
	assert.Empty(t, removed)
	assert.Contains(t, base, keyTechnologies)
}

func TestStripRedundant_PreservesIncludes(t *testing.T) {
	base := map[string]any{
		testIncludes:   []any{"../parent.yaml"},
		testSchema:     "https://projectfile.org/schema/v1.json",
		keySpecVersion: "1",
		keyKind:        testKindValue,
	}
	includes := map[string]any{
		keyKind: testKindValue,
	}
	removed := StripRedundant(base, includes)
	assert.Equal(t, []string{keyKind}, removed)
	assert.Contains(t, base, testIncludes)
	assert.Contains(t, base, testSchema)
	assert.Contains(t, base, keySpecVersion)
}

func TestStripRedundant_NoOverlap(t *testing.T) {
	base := map[string]any{
		testIdentity: map[string]any{keyName: "cli"},
	}
	includes := map[string]any{
		keyLicense: map[string]any{keySpdx: testMIT},
	}
	removed := StripRedundant(base, includes)
	assert.Empty(t, removed)
	assert.Contains(t, base, testIdentity)
}

func TestStripRedundant_DeeplyNested(t *testing.T) {
	base := map[string]any{
		testOrg: map[string]any{
			BaseName: map[string]any{
				"conventions": map[string]any{
					testCommitStyle: "conventional",
					"workflow":      "git-flow",
				},
			},
		},
	}
	includes := map[string]any{
		testOrg: map[string]any{
			BaseName: map[string]any{
				"conventions": map[string]any{
					testCommitStyle: "conventional",
				},
			},
		},
	}
	removed := StripRedundant(base, includes)
	assert.Equal(t, []string{"org.projectfile.conventions.commit-style"}, removed)

	org := base[testOrg].(map[string]any)
	pf := org[BaseName].(map[string]any)
	conv := pf["conventions"].(map[string]any)
	assert.NotContains(t, conv, testCommitStyle)
	assert.Equal(t, "git-flow", conv["workflow"])
}

func TestStripRedundant_EmptyIncludes(t *testing.T) {
	base := map[string]any{
		"kind": testKindValue,
	}
	removed := StripRedundant(base, map[string]any{})
	assert.Empty(t, removed)
	assert.Contains(t, base, keyKind)
}

func TestStripRedundant_FullMatchLeavesMinimalBase(t *testing.T) {
	base := map[string]any{
		testSchema:      "https://projectfile.org/schema/v1.json",
		testIncludes:    []any{"../parent.yaml"},
		keyKind:         testKindValue,
		keyLicense:      map[string]any{keySpdx: testMIT},
		keyTechnologies: []any{testDocker, "go"},
	}
	includes := map[string]any{
		keyKind:         testKindValue,
		keyLicense:      map[string]any{keySpdx: testMIT},
		keyTechnologies: []any{testDocker, "go"},
	}
	removed := StripRedundant(base, includes)
	assert.Equal(t, []string{keyKind, keyLicense + "." + keySpdx, keyTechnologies}, removed)
	assert.Contains(t, base, testIncludes)
	assert.Contains(t, base, testSchema)
	assert.NotContains(t, base, keyKind)
	assert.NotContains(t, base, "license")
	assert.NotContains(t, base, "technologies")
}

// TestStripRedundant_ProtectsEntityListIdentityLink reproduces the scenario
// the entityListKeys guard exists for: a base document carries a project-
// scoped [[people]].from alongside an identity link (email) whose full
// record arrives via an include. Even though the email value is deep-equal
// to the include's, stripping it would orphan the entry — include
// resolution could no longer match it to the include person, the schema's
// required family-names would re-fire on the now-sparse entry, and the
// producer's `from` would be lost. The whole `people` key is therefore
// untouchable by stripRedundant.
func TestStripRedundant_ProtectsEntityListIdentityLink(t *testing.T) {
	base := map[string]any{
		keyPeople: []any{
			map[string]any{
				keyEmail: testEmail,
				testFrom: "2026-06-23",
			},
		},
	}
	includes := map[string]any{
		keyPeople: []any{
			map[string]any{
				testFamilyNames: testBuho,
				testGivenNames:  testDamian,
				keyEmail:        testEmail,
				keyOrcid:        "0009-0001-2345-6789",
			},
		},
	}
	removed := StripRedundant(base, includes)
	assert.Empty(t, removed, "people entries are identity-merge units; stripRedundant must not touch them")
	people, ok := base[keyPeople].([]any)
	require.True(t, ok, "people list must survive untouched")
	require.Len(t, people, 1)
	p := people[0].(map[string]any)
	assert.Equal(t, testEmail, p[keyEmail], "identity link preserved")
	assert.Equal(t, "2026-06-23", p[testFrom], "project-scoped field preserved")
}

// TestStripRedundant_ProtectsOrganizations is the organizations analogue.
func TestStripRedundant_ProtectsOrganizations(t *testing.T) {
	base := map[string]any{
		keyOrganizations: []any{
			map[string]any{keyName: testAcme, testFrom: "2020"},
		},
	}
	includes := map[string]any{
		keyOrganizations: []any{
			map[string]any{keyName: testAcme, keyURL: "https://acme.example"},
		},
	}
	removed := StripRedundant(base, includes)
	assert.Empty(t, removed)
	assert.Contains(t, base, keyOrganizations)
}

func TestSortIncludes_ReordersValues(t *testing.T) {
	raw := map[string]any{
		keyIncludes: []any{testIncludeC, testIncludeA, testIncludeB},
	}
	SortIncludes(raw)
	assert.Equal(t, []any{testIncludeA, testIncludeB, testIncludeC}, raw[keyIncludes])
}

func TestSortIncludes_AlreadySorted(t *testing.T) {
	raw := map[string]any{
		keyIncludes: []any{testIncludeA, testIncludeB},
	}
	SortIncludes(raw)
	assert.Equal(t, []any{testIncludeA, testIncludeB}, raw[keyIncludes])
}

func TestSortIncludes_MissingField(t *testing.T) {
	raw := map[string]any{keyKind: testKindValue}
	SortIncludes(raw)
	assert.NotContains(t, raw, keyIncludes)
}

func TestSortIncludes_LeavesOtherListsAlone(t *testing.T) {
	raw := map[string]any{
		keyIncludes:     []any{testIncludeB, testIncludeA},
		keyTechnologies: []any{testDocker, "go"},
	}
	SortIncludes(raw)
	assert.Equal(t, []any{testIncludeA, testIncludeB}, raw[keyIncludes])
	assert.Equal(t, []any{testDocker, "go"}, raw[keyTechnologies], "non-includes lists must keep declared order")
}

func TestSortIncludes_NonStringElementLeftAsAuthored(t *testing.T) {
	orig := []any{123, "a"}
	raw := map[string]any{keyIncludes: orig}
	SortIncludes(raw)
	assert.Equal(t, orig, raw[keyIncludes])
}
