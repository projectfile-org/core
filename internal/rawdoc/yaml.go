// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package rawdoc

import (
	"bytes"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// YAMLNode wraps a *yaml.Node mapping so format drivers can mutate individual
// top-level keys while preserving the rest of the document — comments, ordering,
// anchors, and unknown keys included.
type YAMLNode struct {
	root *yaml.Node
}

// NewYAMLNode returns a YAMLNode wrapping a fresh empty mapping document.
func NewYAMLNode() *YAMLNode {
	doc := &yaml.Node{
		Kind: yaml.DocumentNode,
		Content: []*yaml.Node{
			{Kind: yaml.MappingNode, Tag: "!!map"},
		},
	}
	return &YAMLNode{root: doc}
}

// FromBytes parses YAML and returns a YAMLNode. An empty or whitespace-only
// document yields a node with an empty top-level mapping.
func FromBytes(data []byte) (*YAMLNode, error) {
	root := &yaml.Node{}
	if err := yaml.Unmarshal(data, root); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if root.Kind == 0 {
		return NewYAMLNode(), nil
	}
	if root.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("expected yaml document, got kind %d", root.Kind)
	}
	if len(root.Content) == 0 {
		root.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	if root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected top-level mapping, got kind %d", root.Content[0].Kind)
	}
	return &YAMLNode{root: root}, nil
}

// Marshal returns the YAML bytes for the wrapped document.
//
// yaml.v3 never emits the YAML document start marker (---) for any node kind
// including DocumentNode. We prepend it ourselves so the output is standard
// YAML — callers such as projectfile.Write and the CFF writer depend on this.
func (y *YAMLNode) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(y.root); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// mapping returns the top-level mapping node.
func (y *YAMLNode) mapping() *yaml.Node {
	return y.root.Content[0]
}

// HeadComment / FootComment manage the document-level comments.
func (y *YAMLNode) HeadComment() string { return y.root.HeadComment }
func (y *YAMLNode) FootComment() string { return y.root.FootComment }

func (y *YAMLNode) SetHeadComment(c string) { y.root.HeadComment = c }

// Keys returns the top-level keys in source order.
func (y *YAMLNode) Keys() []string {
	m := y.mapping()
	out := make([]string, 0, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		out = append(out, m.Content[i].Value)
	}
	return out
}

func (y *YAMLNode) Has(key string) bool {
	_, ok := y.find(key)
	return ok
}

// Get returns the value node for key.
func (y *YAMLNode) Get(key string) (*yaml.Node, bool) {
	return y.find(key)
}

func (y *YAMLNode) find(key string) (*yaml.Node, bool) {
	m := y.mapping()
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1], true
		}
	}
	return nil, false
}

// SetValue assigns key to val (encoded from the given Go value). If the key
// exists, the value is replaced in place — preserving the key node's comments
// and surrounding ordering. New keys are appended.
func (y *YAMLNode) SetValue(key string, val any) error {
	encoded := &yaml.Node{}
	if err := encoded.Encode(val); err != nil {
		return fmt.Errorf("encode yaml value for %q: %w", key, err)
	}
	y.SetNode(key, encoded)
	return nil
}

// SetNode replaces the value node for key (preserving key-node comments),
// or appends a new key/value pair.
func (y *YAMLNode) SetNode(key string, val *yaml.Node) {
	m := y.mapping()
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = val
			return
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	m.Content = append(m.Content, keyNode, val)
}

// Delete removes a top-level key/value pair.
func (y *YAMLNode) Delete(key string) {
	m := y.mapping()
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}

// SortKeys reorders the top-level keys alphabetically. Comments attached
// to key nodes are preserved.
func (y *YAMLNode) SortKeys() {
	m := y.mapping()
	if len(m.Content) < 2 {
		return
	}
	type pair struct {
		key *yaml.Node
		val *yaml.Node
	}
	pairs := make([]pair, 0, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		pairs = append(pairs, pair{key: m.Content[i], val: m.Content[i+1]})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].key.Value < pairs[j].key.Value
	})
	m.Content = make([]*yaml.Node, 0, len(pairs)*2)
	for _, p := range pairs {
		m.Content = append(m.Content, p.key, p.val)
	}
}

// PaintMap replaces all top-level keys in the node with the values from
// fresh, preserving comments and structure. Keys present in the node but
// absent from fresh are removed, regardless of whether they are reserved
// or extension keys. This ensures deletions targeting extension subtrees
// (e.g. del org.pf-cli.includes) propagate through the canvas round-trip.
func (y *YAMLNode) PaintMap(fresh map[string]any, _ map[string]bool) error {
	for _, k := range y.Keys() {
		if _, ok := fresh[k]; !ok {
			y.Delete(k)
		}
	}
	for k, v := range fresh {
		if err := y.SetValue(k, v); err != nil {
			return err
		}
	}
	return nil
}

// Clone deep-copies the wrapped document.
func (y *YAMLNode) Clone() *YAMLNode {
	if y == nil {
		return nil
	}
	return &YAMLNode{root: cloneYAMLNode(y.root)}
}

func cloneYAMLNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	cp := *n
	if len(n.Content) > 0 {
		cp.Content = make([]*yaml.Node, len(n.Content))
		for i, c := range n.Content {
			cp.Content[i] = cloneYAMLNode(c)
		}
	}
	if n.Alias != nil {
		cp.Alias = cloneYAMLNode(n.Alias)
	}
	return &cp
}
