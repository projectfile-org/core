// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"

	"kiota.ch/projectfile/core/v2/internal/genlog"
)

const (
	extTOML  = ".toml"
	extYAML  = ".yaml"
	extYAML2 = ".yml"
	extJSON  = ".json"
)

const BaseName = "projectfile"

// IncludeFailLevel is the minimum severity of an include-resolution problem
// that aborts a read. Lower severities are reported as warnings (visible even
// without --verbose) and the offending include is skipped, so a
// transiently-missing fragment never blocks consumers that tolerate partial
// data — e.g. m6e-sync while an include is being fixed upstream.
type IncludeFailLevel int8

const (
	// FailOnError aborts only on hard failures: parse errors, HTTP errors,
	// cycles, permission denied. A missing local include is a warning and is
	// skipped. This is the default.
	FailOnError IncludeFailLevel = iota
	// FailOnWarning additionally aborts when a local include file does not
	// exist on disk, restoring the pre-lenient strict behaviour.
	FailOnWarning
)

// ReadOptions controls include resolution behaviour during Read operations.
type ReadOptions struct {
	Offline bool
	// FailOn is the minimum include-resolution severity that aborts a read.
	// Defaults to FailOnError (missing local includes warn and are skipped).
	FailOn IncludeFailLevel
}

func sanitizePath(dir, file string) (string, error) {
	abs, err := filepath.Abs(filepath.Join(dir, file))
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// DetectPath locates the projectfile.* sibling in dir to read. Per spec
// §3.5 a project root MUST contain at most one projectfile.* file: when two
// or more coexist the consumer MUST fail and SHOULD name every file found,
// rather than picking one by a side-channel signal such as modification
// time. We therefore return an error listing all offending paths (sorted, so
// the diagnostic is deterministic) instead of selecting a winner. Earlier
// revisions picked the newest mtime; that tie-break was removed from the spec.
func DetectPath(dir string) (string, error) {
	candidates := []string{
		BaseName + ".yaml",
		BaseName + ".yml",
		BaseName + ".toml",
		BaseName + ".json",
	}
	hits := []string{}
	for _, name := range candidates {
		p, err := sanitizePath(dir, name)
		if err != nil {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			continue
		}
		hits = append(hits, p)
	}
	if len(hits) == 0 {
		return "", fmt.Errorf("no projectfile found in %s", dir)
	}
	// 2+ documents is a fail-closed error (spec §3.5): surface the ambiguity
	// naming every sibling so a human resolves it. Sort for deterministic output.
	if len(hits) > 1 {
		sort.Strings(hits)
		names := make([]string, len(hits))
		for i, h := range hits {
			names[i] = filepath.Base(h)
		}
		joined := strings.Join(names, ", ")
		genlog.Info("multiple projectfiles present; failing per spec §3.5",
			"count", len(hits),
			"paths", joined)
		return "", fmt.Errorf("multiple projectfiles in %s: %s — remove all but one", dir, joined)
	}
	genlog.Info("single projectfile detected", "path", filepath.Base(hits[0]))
	return hits[0], nil
}

func Read(dir string) (*Document, string, error) {
	return ReadWithOptions(dir, ReadOptions{})
}

func ReadWithOptions(dir string, opts ReadOptions) (*Document, string, error) {
	raw, path, err := ReadRawWithOptions(dir, opts)
	if err != nil {
		return nil, "", err
	}
	return parseRawDocument(raw), path, nil
}

// ReadRaw is Read without the parseRawDocument step — it returns the parsed
// but un-typed projectfile, suitable for callers that need the as-written
// shape (schema validation, encoding-agnostic dumps).
func ReadRaw(dir string) (map[string]any, string, error) {
	return ReadRawWithOptions(dir, ReadOptions{})
}

func ReadRawWithOptions(dir string, opts ReadOptions) (map[string]any, string, error) {
	path, err := DetectPath(dir)
	if err != nil {
		return nil, "", err
	}
	raw, err := ReadRawFromPathWithOptions(path, opts)
	return raw, path, err
}

// ReadRawFromPath reads the projectfile at an explicit path, dispatching on
// extension. Unlike ReadRaw, it does NOT call DetectPath — callers that must
// distinguish between coexisting projectfile.<ext> variants (e.g. the
// `convert` command) need this lower-level entry point.
func ReadRawFromPath(path string) (map[string]any, error) {
	return ReadRawFromPathWithOptions(path, ReadOptions{})
}

func ReadRawFromPathWithOptions(path string, opts ReadOptions) (map[string]any, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- caller-provided path
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	raw, err := ReadRawFromBytes(path, data)
	if err != nil {
		return nil, err
	}
	return resolveIncludes(raw, filepath.Dir(path), path, opts)
}

// ReadRawFromBytes parses already-read projectfile bytes, dispatching on the
// extension of path (used to pick the parser). Callers that need to mutate
// the source bytes between read and parse — e.g. the `get --expand-env`
// envsubst pass — use this entry point to avoid the temp-file dance the
// dasel-era buildah backend needed.
func ReadRawFromBytes(path string, data []byte) (map[string]any, error) {
	var raw map[string]any
	ext := filepath.Ext(path)
	switch ext {
	case extTOML:
		if err := toml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	case extYAML, extYAML2:
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	case extJSON:
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported projectfile extension: %s", ext)
	}
	return raw, nil
}

// ReadFromPath is ReadRawFromPath plus the typed-document parse step.
// Mirrors Read for the explicit-path case.
func ReadFromPath(path string) (*Document, error) {
	return ReadFromPathWithOptions(path, ReadOptions{})
}

func ReadFromPathWithOptions(path string, opts ReadOptions) (*Document, error) {
	raw, err := ReadRawFromPathWithOptions(path, opts)
	if err != nil {
		return nil, err
	}
	return parseRawDocument(raw), nil
}

// ReadBase reads the projectfile at dir WITHOUT resolving includes.
// Mutation commands (scan, set, add, del) use this as the write target so
// include-inherited values are not materialised into the base file on write-back.
// Use Read when the merged effective view is needed (display, bridge sync).
func ReadBase(dir string) (*Document, string, error) {
	path, err := DetectPath(dir)
	if err != nil {
		return nil, "", err
	}
	doc, err := ReadBaseFromPath(path)
	return doc, path, err
}

// ReadBaseFromPath is ReadBase for an explicit path.
func ReadBaseFromPath(path string) (*Document, error) {
	raw, err := ReadRawBaseFromPath(path)
	if err != nil {
		return nil, err
	}
	return parseRawDocument(raw), nil
}

// ReadRawBaseFromPath reads and parses the projectfile at path without
// resolving includes. Returns the raw map for callers that need the
// as-written data (e.g. extracting the includes list for cache warming).
func ReadRawBaseFromPath(path string) (map[string]any, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- caller-provided path
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	raw, err := ReadRawFromBytes(path, data)
	if err != nil {
		return nil, err
	}
	// Intentionally skip resolveIncludes — returns as-written document only.
	return raw, nil
}

// extractLeadingComments returns all comment lines at the top of data, before
// the first non-comment non-blank line (and before any YAML --- marker). REUSE
// headers (SPDX-FileCopyrightText, SPDX-License-Identifier) and the TOML
// #:schema directive are the primary occupants.
func extractLeadingComments(data []byte) []string {
	if data == nil {
		return nil
	}
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)
		if trimmed == "---" {
			break
		}
		if trimmed == "" {
			lines = append(lines, "")
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			lines = append(lines, raw)
			continue
		}
		break
	}
	return lines
}
