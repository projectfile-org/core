// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package rawdoc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// These cases drive the malformed shape every `i+1 < len` pair walk must tolerate: a mapping whose last element is a KEY with no value node.
const (
	danglingKey  = "dangling"
	danglingVal  = "painted"
	keptKey      = "kept"
	nestedBranch = "branch"
)

func scalarNode(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

func mappingNode(items ...*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: items}
}

func canvasOf(mapping *yaml.Node) *YAMLNode {
	return &YAMLNode{root: &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{mapping}}}
}

func contentKeys(m *yaml.Node) []string {
	out := []string{}
	for i := 0; i+1 < len(m.Content); i += 2 {
		out = append(out, m.Content[i].Value)
	}
	return out
}

func contentVal(m *yaml.Node, key string) string {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1].Value
		}
	}
	return ""
}

// TestPairWalkersTolerateADanglingKey pins every `i+1 < len` guard: the `<=` mutation steps onto the dangling key and reaches one element past the mapping.
func TestPairWalkersTolerateADanglingKey(t *testing.T) {
	// Keys reports only complete pairs, never the dangling key appended whole.
	n := canvasOf(mappingNode(scalarNode("first"), scalarNode("1"), scalarNode(danglingKey)))
	assert.True(t, n.Has("first"))
	assert.False(t, n.Has(danglingKey), "the dangling key has no value, so it is not a key")
	_, ok := n.Get(danglingKey)
	assert.False(t, ok)
	assert.Equal(t, []string{"first"}, n.Keys())

	// SetNode writes a fresh pair, never past the last value node.
	sn := canvasOf(mappingNode(scalarNode("first"), scalarNode("1"), scalarNode(danglingKey)))
	assert.NotPanics(t, func() { sn.SetNode(danglingKey, scalarNode(danglingVal)) })
	assert.True(t, sn.Has("first"), "the pairs before the dangling key are untouched")

	// Delete cannot remove a dangling key; it is a no-op, not a slice past the end.
	dn := canvasOf(mappingNode(scalarNode("first"), scalarNode("1"), scalarNode(danglingKey)))
	assert.NotPanics(t, func() { dn.Delete(danglingKey) })
	assert.Equal(t, []string{"first"}, dn.Keys(), "a dangling key is left untouched by delete")

	// A real delete beside it still works, so the guard is not masking the path.
	dn2 := canvasOf(mappingNode(scalarNode("first"), scalarNode("1"), scalarNode(danglingKey)))
	dn2.Delete("first")
	assert.Equal(t, []string{}, dn2.Keys(), "deleting the complete pair removes it")

	// SortKeys rebuilds from complete pairs only and drops the dangling key.
	srt := canvasOf(mappingNode(scalarNode("first"), scalarNode("1"), scalarNode(danglingKey)))
	assert.NotPanics(t, srt.SortKeys, "sorting never reaches past the last value node")
}

// TestSortKeysKeepsALoneKeyNode pins the re-spelled `<= 1` guard: a lone dangling key has fewer than one pair and is left exactly as it is.
func TestSortKeysKeepsALoneKeyNode(t *testing.T) {
	n := canvasOf(mappingNode(scalarNode("only")))

	n.SortKeys()

	m := n.mapping()
	require.Len(t, m.Content, 1, "sorting fewer than one pair touches nothing")
	assert.Equal(t, "only", m.Content[0].Value)
}

// TestPaintMappingToleratesADanglingKey is the nested twin: a sub-mapping with a dangling key is painted from its complete pairs, never indexing the missing value node.
func TestPaintMappingToleratesADanglingKey(t *testing.T) {
	inner := mappingNode(
		scalarNode(keptKey), scalarNode("old"),
		scalarNode(danglingKey), // dangling: no value follows
	)
	n := canvasOf(mappingNode(scalarNode(nestedBranch), inner))

	require.NoError(t, n.PaintMap(map[string]any{
		nestedBranch: map[string]any{keptKey: "new", danglingKey: danglingVal},
	}))

	branch, ok := n.Get(nestedBranch)
	require.True(t, ok)
	assert.Equal(t, []string{keptKey, danglingKey}, contentKeys(branch))
	assert.Equal(t, "new", contentVal(branch, keptKey), "the complete pair is repainted")
	assert.Equal(t, danglingVal, contentVal(branch, danglingKey), "the dangling key is added fresh")
}
