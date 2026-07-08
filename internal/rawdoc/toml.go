// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package rawdoc

import (
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

// OrderedTOML records the parsed top-level keys of a TOML document together
// with the *source order* in which they appeared, so drivers can introspect a
// pre-existing file. Round-trip fidelity is limited to keys — go-toml/v2's
// encoder re-orders alphabetically within tables and does not preserve
// comments inside tables. This is documented limitation.
type OrderedTOML struct {
	keys []string
	data map[string]any
}

func NewOrderedTOML() *OrderedTOML {
	return &OrderedTOML{data: map[string]any{}}
}

// TOMLFromBytes parses TOML bytes into an OrderedTOML, best-effort recording
// the top-level key order from the source.
func TOMLFromBytes(data []byte) (*OrderedTOML, error) {
	m := map[string]any{}
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse toml: %w", err)
	}
	return &OrderedTOML{keys: topLevelTOMLKeyOrder(data, m), data: m}, nil
}

// Keys returns recorded top-level keys in source order (or, if Set was called,
// insertion order).
func (om *OrderedTOML) Keys() []string {
	out := make([]string, len(om.keys))
	copy(out, om.keys)
	return out
}

func (om *OrderedTOML) Has(key string) bool {
	_, ok := om.data[key]
	return ok
}

func (om *OrderedTOML) Get(key string) (any, bool) {
	v, ok := om.data[key]
	return v, ok
}

func (om *OrderedTOML) Set(key string, v any) {
	if om.data == nil {
		om.data = map[string]any{}
	}
	if _, exists := om.data[key]; !exists {
		om.keys = append(om.keys, key)
	}
	om.data[key] = v
}

func (om *OrderedTOML) Delete(key string) {
	if _, ok := om.data[key]; !ok {
		return
	}
	delete(om.data, key)
	for i, k := range om.keys {
		if k == key {
			om.keys = append(om.keys[:i], om.keys[i+1:]...)
			return
		}
	}
}

// AsMap returns a flat copy of the underlying data. Callers iterating the map
// lose key order — use Keys() to walk explicitly.
func (om *OrderedTOML) AsMap() map[string]any {
	cp := make(map[string]any, len(om.data))
	for k, v := range om.data {
		cp[k] = v
	}
	return cp
}

// Marshal emits TOML via go-toml/v2; within-table ordering is alphabetical.
// Top-level source ordering is *not* preserved here — this primitive is
// "round-trip keys, not bytes."
func (om *OrderedTOML) Marshal() ([]byte, error) {
	return toml.Marshal(om.AsMap())
}

func (om *OrderedTOML) Clone() *OrderedTOML {
	if om == nil {
		return nil
	}
	cp := &OrderedTOML{
		keys: make([]string, len(om.keys)),
		data: make(map[string]any, len(om.data)),
	}
	copy(cp.keys, om.keys)
	for k, v := range om.data {
		cp.data[k] = deepCopyTOMLValue(v)
	}
	return cp
}

func deepCopyTOMLValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = deepCopyTOMLValue(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = deepCopyTOMLValue(vv)
		}
		return out
	default:
		return v
	}
}

// topLevelTOMLKeyOrder scans the source bytes to record the first occurrence
// order of top-level keys/tables. Falls back to map iteration order for any
// keys the scanner missed so nothing is dropped.
func topLevelTOMLKeyOrder(data []byte, parsed map[string]any) []string {
	seen := map[string]bool{}
	var order []string
	add := func(k string) {
		if k == "" || seen[k] {
			return
		}
		if _, ok := parsed[k]; !ok {
			return
		}
		seen[k] = true
		order = append(order, k)
	}

	i := 0
	for i < len(data) {
		j := i
		for j < len(data) && (data[j] == ' ' || data[j] == '\t') {
			j++
		}
		eol := j
		for eol < len(data) && data[eol] != '\n' {
			eol++
		}
		line := string(data[j:eol])
		if len(line) > 0 && line[0] != '#' {
			switch {
			case len(line) >= 2 && line[0] == '[' && line[1] == '[':
				add(parseTOMLHeaderHead(line[2:]))
			case len(line) >= 1 && line[0] == '[':
				add(parseTOMLHeaderHead(line[1:]))
			default:
				if k := parseTOMLAssignKey(line); k != "" {
					add(k)
				}
			}
		}
		i = eol + 1
	}

	for k := range parsed {
		if !seen[k] {
			order = append(order, k)
			seen[k] = true
		}
	}
	return order
}

func parseTOMLHeaderHead(s string) string {
	out := make([]byte, 0, len(s))
	for i := range len(s) {
		c := s[i]
		if c == '.' || c == ']' || c == ' ' || c == '\t' {
			break
		}
		out = append(out, c)
	}
	return string(out)
}

func parseTOMLAssignKey(line string) string {
	out := make([]byte, 0, len(line))
	i := 0
	if i < len(line) && line[i] == '"' {
		i++
		for i < len(line) && line[i] != '"' {
			out = append(out, line[i])
			i++
		}
		return string(out)
	}
	for i < len(line) {
		c := line[i]
		if c == '=' || c == ' ' || c == '\t' || c == '.' {
			break
		}
		out = append(out, c)
		i++
	}
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i < len(line) && (line[i] == '=' || line[i] == '.') {
		return string(out)
	}
	return ""
}
