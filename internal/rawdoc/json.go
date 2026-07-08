// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package rawdoc holds insertion-order-preserving wrappers around the three
// supported on-disk encodings (JSON, YAML, TOML). They exist so format drivers
// can round-trip a document without losing keys, ordering, or (for YAML)
// comments that the typed view does not know about.
package rawdoc

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// OrderedJSON preserves the insertion-order of keys in a top-level JSON object.
// Values are held as raw bytes so unknown sub-trees survive untouched.
type OrderedJSON struct {
	keys []string
	data map[string]json.RawMessage
}

func NewOrderedJSON() *OrderedJSON {
	return &OrderedJSON{data: map[string]json.RawMessage{}}
}

func (om *OrderedJSON) Keys() []string {
	out := make([]string, len(om.keys))
	copy(out, om.keys)
	return out
}

func (om *OrderedJSON) Has(key string) bool {
	_, ok := om.data[key]
	return ok
}

func (om *OrderedJSON) Get(key string) (json.RawMessage, bool) {
	v, ok := om.data[key]
	return v, ok
}

// Set assigns key to raw, appending it to the key-order on first insert.
func (om *OrderedJSON) Set(key string, raw json.RawMessage) {
	if om.data == nil {
		om.data = map[string]json.RawMessage{}
	}
	if _, exists := om.data[key]; !exists {
		om.keys = append(om.keys, key)
	}
	om.data[key] = raw
}

// Delete removes key from both the map and the key-order slice.
func (om *OrderedJSON) Delete(key string) {
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

func (om *OrderedJSON) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range om.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(om.data[k])
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func (om *OrderedJSON) UnmarshalJSON(data []byte) error {
	om.keys = nil
	om.data = map[string]json.RawMessage{}

	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok != json.Delim('{') {
		return fmt.Errorf("expected {, got %v", tok)
	}

	// Object iteration: keys are read via Token() (single string token),
	// values via Decode() (full JSON value). Mixing Decode for keys errors
	// "not at beginning of value" — the legacy code had the same bug but
	// hid it behind a silent fallback.
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := keyTok.(string)
		if !ok {
			return fmt.Errorf("expected string key, got %v", keyTok)
		}
		var val json.RawMessage
		if err := dec.Decode(&val); err != nil {
			return err
		}
		om.keys = append(om.keys, key)
		om.data[key] = val
	}
	return nil
}

// Clone returns an independent deep-copy of om. Raw bytes are duplicated.
func (om *OrderedJSON) Clone() *OrderedJSON {
	if om == nil {
		return nil
	}
	cp := &OrderedJSON{
		keys: make([]string, len(om.keys)),
		data: make(map[string]json.RawMessage, len(om.data)),
	}
	copy(cp.keys, om.keys)
	for k, v := range om.data {
		buf := make(json.RawMessage, len(v))
		copy(buf, v)
		cp.data[k] = buf
	}
	return cp
}
