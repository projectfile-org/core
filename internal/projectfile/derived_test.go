// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import "testing"

const (
	testCustomImage = "acme/custom"
	testNamespace19 = "com.example.b19"
	testUbuntu      = "ubuntu"
)

// TestImageBasename pins the single-home rule every lowering reads (m6e via
// `projectfile get image.basename`, ci-resolver via the pkg façade): an explicit
// org.projectfile.ci.image wins, else the basename is
// <last-label(identity.namespace)>/<identity.name> (bare name when no namespace),
// and an unnamed project with no override is absent.
func TestImageBasename(t *testing.T) {
	for _, c := range []struct {
		name    string
		ns      string
		pf      string
		image   string // org.projectfile.ci.image override
		want    string
		present bool
	}{
		{"reverse-dns derivation", testNamespace19, testUbuntu, "", "b19/ubuntu", true},
		{"bare namespace derivation", "b19", testUbuntu, "", "b19/ubuntu", true},
		{"no namespace => bare name", "", testUbuntu, "", testUbuntu, true},
		{"explicit override wins over derivation", testNamespace19, testUbuntu, testCustomImage, testCustomImage, true},
		{"override with no identity", "", "", testCustomImage, testCustomImage, true},
		{"no override, no name => absent", testNamespace19, "", "", "", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			doc := &Document{
				Identity: Identity{Namespace: c.ns, Name: c.pf},
			}
			if c.image != "" {
				doc.Extensions = map[string]any{
					CIExtensionNS: map[string]any{"image": c.image},
				}
			}
			got, present := ImageBasename(doc)
			if present != c.present {
				t.Fatalf("present = %v, want %v", present, c.present)
			}
			if got != c.want {
				t.Errorf("basename = %q, want %q", got, c.want)
			}
		})
	}
}

// TestCIImageOverrideNestedForm covers the TOML dotted-table encoding, where
// go-toml explodes [org.projectfile.ci] into nested maps — LookupExtension must
// still find image, so the override is honoured regardless of on-disk encoding.
func TestCIImageOverrideNestedForm(t *testing.T) {
	doc := &Document{
		Identity: Identity{Namespace: testNamespace19, Name: testUbuntu},
		Extensions: map[string]any{
			testOrg: map[string]any{
				"projectfile": map[string]any{
					"ci": map[string]any{"image": testCustomImage},
				},
			},
		},
	}
	if got, present := ImageBasename(doc); !present || got != testCustomImage {
		t.Errorf("nested-form override = %q (present=%v), want %q", got, present, testCustomImage)
	}
}
