// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"

	"kiota.ch/projectfile/core/v2/internal/rawdoc"
)

// YAMLOutputSorted forces YAML keys to be emitted in sorted (alphabetical)
// order. When false (default), key order from the existing file is preserved
// (canvas round-trip), or Go map iteration order for fresh files.
var YAMLOutputSorted bool

// SetYAMLOutputSorted crosses the toggle over the module boundary for the
// pf-bridge root (a value alias would copy the var).
func SetYAMLOutputSorted(b bool) { YAMLOutputSorted = b }

// YAMLOutputSortedEnabled reads the toggle across the module boundary — the
// projectfile CLI (optimize) consults it after the root sets it.
func YAMLOutputSortedEnabled() bool { return YAMLOutputSorted }

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".pf-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmp := f.Name()

	// committed gates the deferred cleanup: once the temp file is renamed
	// into place it must not be removed. Best-effort cleanup errors are
	// discarded — the caller needs the original failure, not a Close on a
	// doomed temp file. `_ =` marks each discard as deliberate.
	committed := false
	defer func() {
		if committed {
			return
		}
		_ = f.Close()
		_ = os.Remove(tmp)
	}()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Chmod(mode); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename temp to %s: %w", path, err)
	}
	committed = true
	return nil
}

// Write serialises doc back to disk, dispatching on the file extension. The
// projectfile's encoding is determined by its filename, not by an in-memory
// flag — so this matches the format detection done in Read/DetectPath.
// When the file already exists and is the same encoding, the writer preserves
// comments and key order by painting onto the existing node tree. For fresh
// files or cross-encoding converts, use WriteClean.
func Write(doc *Document, path string) error {
	return writeWithCanvas(doc, path, true)
}

// WriteClean serialises doc to disk without attempting to preserve comments
// or key order from any existing file. Used by the convert command which
// writes into a different encoding than the source.
func WriteClean(doc *Document, path string) error {
	return writeWithCanvas(doc, path, false)
}

func writeWithCanvas(doc *Document, path string, tryCanvas bool) error {
	ext := strings.ToLower(filepath.Ext(path))
	applyDiscriminator(doc, ext)
	switch ext {
	case extTOML:
		return writeTOMLFile(doc, path)
	case extYAML, extYAML2:
		return writeYAMLFile(doc, path, tryCanvas)
	case extJSON:
		return writeJSONFile(doc, path)
	default:
		return fmt.Errorf("unsupported projectfile extension %q", filepath.Ext(path))
	}
}

// applyDiscriminator stamps the encoding-specific spec discriminator onto doc.
// Spec §4.4: TOML documents require `spec_version = "1"` paired with the
// `#:schema` header comment (emitted by writeTOMLFile); YAML and JSON
// documents rely on $schema alone, set unconditionally upstream. Co-locating
// this with the format-dispatch keeps "what the discriminator looks like" in
// one file.
func applyDiscriminator(doc *Document, ext string) {
	if ext == ".toml" {
		doc.SpecVersion = "1"
	}
}

func writeTOMLFile(doc *Document, path string) error {
	existing, _ := os.ReadFile(path) // #nosec G304 -- path validated by Read/DetectPath
	leadingComments := extractLeadingComments(existing)
	schemaComment := ""
	for _, l := range leadingComments {
		if strings.HasPrefix(strings.TrimSpace(l), "#:schema") {
			schemaComment = l
			break
		}
	}

	raw := doc.ToMap()

	// TOML convention: schema is the file's #:schema header comment, not a
	// top-level $schema key. Lift it out so the marshalled body is comment-
	// free, and (when no on-disk header existed) derive a fresh header.
	if v, ok := raw[keySchema].(string); ok && v != "" {
		if schemaComment == "" {
			schemaComment = "#:schema " + v
		}
		delete(raw, keySchema)
	}

	data, err := toml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal TOML: %w", err)
	}

	out := string(data)
	// Rebuild the leading comment block: REUSE headers preserved, #:schema
	// kept or derived, then a blank line before the TOML body.
	var header []string
	for _, l := range leadingComments {
		if strings.HasPrefix(strings.TrimSpace(l), "#:schema") {
			continue
		}
		header = append(header, l)
	}
	if schemaComment != "" {
		header = append(header, schemaComment)
	}
	if len(header) > 0 {
		out = strings.Join(header, "\n") + "\n\n" + out
	}
	return atomicWriteFile(path, []byte(out), 0o644) // #nosec G306,G703 -- metadata file, not a secret; path validated upstream
}

func writeYAMLFile(doc *Document, path string, tryCanvas bool) error {
	raw := doc.ToMap()

	if tryCanvas {
		existing, _ := os.ReadFile(path) // #nosec G304 -- path validated upstream
		canvas, err := rawdoc.FromBytes(existing)
		if err == nil && len(canvas.Keys()) > 0 {
			if err := canvas.PaintMap(raw, ReservedKeys); err != nil {
				return fmt.Errorf("paint yaml: %w", err)
			}
			if YAMLOutputSorted {
				canvas.SortKeys()
			}
			data, err := canvas.Marshal()
			if err != nil {
				return fmt.Errorf("marshal YAML: %w", err)
			}
			return atomicWriteFile(path, data, 0o644) // #nosec G306,G703
		}
	}
	if YAMLOutputSorted {
		return writeYAMLCleanSorted(raw, path)
	}
	return writeYAMLClean(raw, path)
}

func writeYAMLClean(raw map[string]any, path string) error {
	var buf bytes.Buffer
	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(raw); err != nil {
		_ = enc.Close()
		return fmt.Errorf("marshal YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("marshal YAML: %w", err)
	}
	return atomicWriteFile(path, buf.Bytes(), 0o644) // #nosec G306,G703 -- metadata file, not a secret; path validated upstream
}

func writeYAMLCleanSorted(raw map[string]any, path string) error {
	node := rawdoc.NewYAMLNode()
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := node.SetValue(k, raw[k]); err != nil {
			return fmt.Errorf("set value %q: %w", k, err)
		}
	}
	data, err := node.Marshal()
	if err != nil {
		return fmt.Errorf("marshal YAML: %w", err)
	}
	return atomicWriteFile(path, data, 0o644)
}

func writeJSONFile(doc *Document, path string) error {
	raw := doc.ToMap()
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	return atomicWriteFile(path, append(data, '\n'), 0o644) // #nosec G306,G703 -- metadata file, not a secret; path validated upstream
}
