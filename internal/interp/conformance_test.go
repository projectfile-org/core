// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package interp_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/interp"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// specificationDir is the specification clone .scripts/fetch-specification.sh makes, overridable with PF_SPECIFICATION_DIR.
func specificationDir() string {
	if dir := os.Getenv("PF_SPECIFICATION_DIR"); dir != "" {
		return dir
	}
	return filepath.Join("..", "..", "_specification")
}

// TestConformanceInterpolation runs every spec §3.8 vector: each case's out, and its lines when declared.
func TestConformanceInterpolation(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(specificationDir(), "spec", "conformance", "interpolation", "*.yaml"))
	require.NoError(t, err)
	if len(files) == 0 {
		t.Fatalf("no interpolation vectors under %s: run .scripts/fetch-specification.sh", specificationDir())
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file) // #nosec G304 -- vector file from the specification clone
			require.NoError(t, err)
			vec, err := projectfile.ReadRawFromBytes(file, data)
			require.NoError(t, err)
			raw, ok := vec["document"].(map[string]any)
			require.True(t, ok, "vector without a document mapping")
			doc := projectfile.FromMap(raw)
			cases, ok := vec["cases"].([]any)
			require.True(t, ok, "vector without cases")
			for i, item := range cases {
				c, ok := item.(map[string]any)
				require.True(t, ok, "case %d is not a mapping", i)
				in, _ := c["in"].(string)
				assert.Equal(t, c["out"], interp.Expand(doc, in), "case %d out: %s", i, in)
				want, hasLines := c["lines"].([]any)
				if !hasLines {
					continue
				}
				got, _ := interp.ExpandFanOut(doc, in)
				wantLines := make([]string, len(want))
				for j, line := range want {
					wantLines[j], _ = line.(string)
				}
				assert.Equal(t, wantLines, got, "case %d lines: %s", i, in)
			}
		})
	}
}
