// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kiotaprojectfile "kiota.ch/projectfile/core/pkg/projectfile"
)

// YAML keys reused across the shield fixtures below. Declared as constants to
// satisfy goconst (matches the testGenerate pattern in document_test.go).
const (
	keyShieldName = "name"
	keyShieldImg  = "img"
	keyShieldHref = "href"
)

// GetReadmeExtension must surface shields with all four fields preserved.
func TestGetReadmeExtensionShields(t *testing.T) {
	doc := &kiotaprojectfile.Document{
		Extensions: map[string]any{
			"org.projectfile.readme": map[string]any{
				"blocks": []any{"basics", "badges", "license"},
				"shields": []any{
					map[string]any{
						keyShieldName: "dockerhub pulls",
						keyShieldImg:  "https://img.shields.io/docker/pulls/foo",
						keyShieldHref: "https://hub.docker.com/r/foo",
						"alt":         "Docker Hub pulls",
					},
					map[string]any{
						// alt omitted — render layer must fall back to name.
						keyShieldName: "go report",
						keyShieldImg:  "https://goreportcard.com/badge/foo",
						keyShieldHref: "https://goreportcard.com/report/foo",
					},
				},
			},
		},
	}

	ext, err := kiotaprojectfile.GetReadmeExtension(doc)
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, []string{"basics", "badges", "license"}, ext.Blocks)
	require.Len(t, ext.Shields, 2)

	assert.Equal(t, "dockerhub pulls", ext.Shields[0].Name)
	assert.Equal(t, "https://img.shields.io/docker/pulls/foo", ext.Shields[0].Img)
	assert.Equal(t, "https://hub.docker.com/r/foo", ext.Shields[0].Href)
	assert.Equal(t, "Docker Hub pulls", ext.Shields[0].Alt)

	assert.Equal(t, "go report", ext.Shields[1].Name)
	assert.Empty(t, ext.Shields[1].Alt, "alt must stay empty so the render layer can fall back")
}

// Non-map shield entries are skipped silently, matching the extras behavior.
func TestGetReadmeExtensionShieldsSkipsMalformed(t *testing.T) {
	doc := &kiotaprojectfile.Document{
		Extensions: map[string]any{
			"org.projectfile.readme": map[string]any{
				"shields": []any{
					"not-a-map",
					map[string]any{keyShieldName: "ok", keyShieldImg: "i", keyShieldHref: "h"},
				},
			},
		},
	}

	ext, err := kiotaprojectfile.GetReadmeExtension(doc)
	require.NoError(t, err)
	require.NotNil(t, ext)
	require.Len(t, ext.Shields, 1)
	assert.Equal(t, "ok", ext.Shields[0].Name)
}
