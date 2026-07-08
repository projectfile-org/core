// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package rawdoc_test

import (
	"encoding/json"
	"testing"

	"kiota.ch/projectfile/core/internal/rawdoc"
)

// FuzzOrderedJSON checks that JSON parsing and re-marshalling never panic.
func FuzzOrderedJSON(f *testing.F) {
	f.Add([]byte(`{"a":1,"b":2}`))
	f.Add([]byte(`{"name":"foo","version":"1.0.0","scripts":{"build":"tsc"}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(``))
	f.Add([]byte(`{"k":null}`))
	f.Add([]byte(`{"nested":{"deep":{"value":true}}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		om := rawdoc.NewOrderedJSON()
		if err := json.Unmarshal(data, om); err != nil {
			return // invalid JSON — not a bug
		}
		// Re-marshal must not panic.
		out, err := json.Marshal(om)
		if err != nil {
			t.Errorf("marshal after valid unmarshal failed: %v", err)
			return
		}
		// Clone must not panic.
		cp := om.Clone()
		if cp == nil {
			t.Error("Clone of non-nil OrderedJSON must not return nil")
			return
		}
		// Re-marshal of clone must not panic.
		_, _ = json.Marshal(cp)
		_ = out
	})
}

// FuzzYAMLNode checks that YAML parsing and re-marshalling never panic.
func FuzzYAMLNode(f *testing.F) {
	f.Add([]byte("foo: bar\nbaz: 42\n"))
	f.Add([]byte("# comment\nkey: value\n"))
	f.Add([]byte("---\nidentity:\n  name: test\n"))
	f.Add([]byte(""))
	f.Add([]byte("---"))
	f.Add([]byte(": invalid"))

	f.Fuzz(func(t *testing.T, data []byte) {
		y, err := rawdoc.FromBytes(data)
		if err != nil {
			return // invalid YAML — not a bug
		}
		// Marshal must not panic.
		_, _ = y.Marshal()
		// Clone must not panic.
		cp := y.Clone()
		if cp == nil {
			t.Error("Clone of non-nil YAMLNode must not return nil")
			return
		}
		_, _ = cp.Marshal()
	})
}
