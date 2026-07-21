// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile_test

import (
	"os"
	"path/filepath"
	"testing"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

// FuzzReadYAML checks that the YAML parser never panics on arbitrary input.
func FuzzReadYAML(f *testing.F) {
	f.Add([]byte("---\nidentity:\n  name: foo\n  version: 1.0.0\n"))
	f.Add([]byte("---\n$schema: https://projectfile.org/schema/v1.json\nidentity:\n  name: bar\n"))
	f.Add([]byte(""))
	f.Add([]byte("---"))
	f.Add([]byte("not: valid: yaml: :::"))

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		path := filepath.Join(dir, "projectfile.yaml")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return
		}
		// Must not panic; errors are fine.
		_, _ = projectfile.ReadFromPath(path)
	})
}

// FuzzReadTOML checks that the TOML parser never panics on arbitrary input.
func FuzzReadTOML(f *testing.F) {
	f.Add([]byte("[identity]\nname = \"foo\"\nversion = \"1.0.0\"\n"))
	f.Add([]byte("#:schema https://projectfile.org/schema/v1.json\nspec_version = \"1\"\n[identity]\nname = \"bar\"\n"))
	f.Add([]byte(""))
	f.Add([]byte("[[["))
	f.Add([]byte("key = value-without-quotes"))

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		path := filepath.Join(dir, "projectfile.toml")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return
		}
		_, _ = projectfile.ReadFromPath(path)
	})
}

// FuzzReadJSON checks that the JSON parser never panics on arbitrary input.
func FuzzReadJSON(f *testing.F) {
	f.Add([]byte(`{"identity":{"name":"foo","version":"1.0.0"}}`))
	f.Add([]byte(`{"$schema":"https://projectfile.org/schema/v1.json","identity":{"name":"bar"}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(``))
	f.Add([]byte(`{"key": [[[`))

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		path := filepath.Join(dir, "projectfile.json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return
		}
		_, _ = projectfile.ReadFromPath(path)
	})
}
