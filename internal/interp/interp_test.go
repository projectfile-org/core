// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package interp_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"kiota.ch/projectfile/core/v2/pkg/interp"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// doc is the fixture every case resolves against: a typed field, a list the
// selector grammar must reach, and an extension namespace in the NESTED form
// (the on-disk shape every projectfile in this fleet uses).
func doc() *projectfile.Document {
	return &projectfile.Document{
		Identity: projectfile.Identity{Name: "ubuntu"},
		License:  &projectfile.License{Spdx: "MIT"},
		Links: []projectfile.Link{
			{Type: projectfile.LinkSourceCode, URL: "https://codeberg.org/b19/ubuntu"},
			{Type: projectfile.LinkSourceCode, URL: "https://github.com/damian-buho/b19-ubuntu"},
		},
		Extensions: map[string]any{
			"org": map[string]any{
				"projectfile": map[string]any{
					"status": "maintained",
					"readme": map[string]any{"shields-base": "https://img.shields.io"},
					"ci":     map[string]any{"matrix": map[string]any{"axes": map[string]any{"S": []any{"a", "b"}}}},
				},
			},
		},
	}
}

func TestExpand(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"typed field", "${identity.name}", "ubuntu"},
		{"nested field", "${license.spdx}", "MIT"},
		{"extension namespace", "${org.projectfile.status}", "maintained"},
		{"list selector takes the first match", "${links[type=source-code].url}", "https://codeberg.org/b19/ubuntu"},
		{"list index", "${links[1].url}", "https://github.com/damian-buho/b19-ubuntu"},
		{"several references in one string", "${identity.name}-${license.spdx}", "ubuntu-MIT"},
		{
			"composed URL", "${org.projectfile.readme.shields-base}/badge/license-${license.spdx}-blue",
			"https://img.shields.io/badge/license-MIT-blue",
		},

		// Everything below must survive untouched — this is the clause that
		// lets make variables, shell variables and matrix placeholders share a
		// document with field references (spec §3.8).
		{"make variable", "${B19_DOCKER_REGISTRY}/b19/fd", "${B19_DOCKER_REGISTRY}/b19/fd"},
		{"shell variable", "docker run -v $PWD:/w", "docker run -v $PWD:/w"},
		{"matrix placeholder", "b19/ubuntu/{S}", "b19/ubuntu/{S}"},
		{"missing field", "${identity.nope}", "${identity.nope}"},
		{"non-scalar value", "${links}", "${links}"},
		{"unterminated reference", "${identity.name", "${identity.name"},
		{"escape", "$${identity.name}", "${identity.name}"},
		{"lone dollar", "100$ and $", "100$ and $"},
		{"no dollar at all", "plain text", "plain text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, interp.Expand(doc(), tc.in))
		})
	}
}

// A nil document interpolates nothing but must never panic — the readme bridge
// renders badge fixtures with no document at all.
func TestExpandNilDocument(t *testing.T) {
	assert.Equal(t, "${identity.name}", interp.Expand(nil, "${identity.name}"))
}

// A resolved value that is itself a reference expands too, so a fragment can
// point at a field a second fragment defines. Depth is bounded, not unlimited.
func TestExpandRecursesIntoResolvedValues(t *testing.T) {
	d := doc()
	d.Identity.Title = &projectfile.LocalizedString{Bare: "${license.spdx} image"}
	assert.Equal(t, "MIT image", interp.Expand(d, "${identity.title}"))
}

func TestUnresolved(t *testing.T) {
	assert.True(t, interp.Unresolved("https://x/${a.b}"))
	assert.False(t, interp.Unresolved("https://x/resolved"))
}

// Repeated fixture keys, named so goconst sees one home per string.
const (
	artifactsNS  = "org.projectfile.artifacts"
	kindImage    = "image"
	keyKindField = "kind"
	keyRefField  = "ref"

	// Fixture values the scope cases share with the cases above.
	nameUbuntu    = "ubuntu"
	refIdentity   = "${identity.name}"
	seriesNoble   = "noble"
	keyOrgField   = "org"
	partsScope    = "me.dbuho.projectfile.image"
	sinksScope    = "me.dbuho.projectfile.sinks"
	projectfileNS = "me.dbuho.projectfile"
)

// fanOutDoc holds the artifacts shape the README recipes address: two images and
// one binary under org.projectfile.artifacts, keyed by name.
func fanOutDoc() *projectfile.Document {
	return &projectfile.Document{
		Identity: projectfile.Identity{Name: "demo"},
		Extensions: map[string]any{
			artifactsNS: map[string]any{
				"noble-image":    map[string]any{keyKindField: kindImage, keyRefField: "kiota.ch/b19/ubuntu/noble:latest"},
				"resolute-image": map[string]any{keyKindField: kindImage, keyRefField: "kiota.ch/b19/ubuntu/resolute:latest"},
				"cli":            map[string]any{"kind": "binary", "command": "pf-bridge"},
			},
		},
	}
}

// TestExpandFanOutMultipleValues is the series-image case: one declared command
// in a shared fragment, one rendered line per image the project publishes.
func TestExpandFanOutMultipleValues(t *testing.T) {
	got, _ := interp.ExpandFanOut(fanOutDoc(), "docker pull ${org.projectfile.artifacts{kind=image}.ref}")
	want := []string{
		"docker pull kiota.ch/b19/ubuntu/noble:latest",
		"docker pull kiota.ch/b19/ubuntu/resolute:latest",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %#v, want %#v", got, want)
	}
}

// TestExpandFanOutSingleValueMatchesExpand pins the invariant that keeps the
// common case cheap and identical: one match must render exactly what Expand
// renders, as a single line.
func TestExpandFanOutSingleValueMatchesExpand(t *testing.T) {
	doc := fanOutDoc()
	const s = "${org.projectfile.artifacts{kind=binary}.command} --help"
	got, _ := interp.ExpandFanOut(doc, s)
	if len(got) != 1 {
		t.Fatalf("lines = %#v, want exactly one", got)
	}
	if got[0] != interp.Expand(doc, s) {
		t.Fatalf("fan-out = %q, Expand = %q — a single match must agree", got[0], interp.Expand(doc, s))
	}
	if got[0] != "pf-bridge --help" {
		t.Fatalf("line = %q, want %q", got[0], "pf-bridge --help")
	}
}

// TestExpandFanOutUnresolvedStaysVerbatim: interp never invents a value. An
// address nothing answers survives as a literal `${…}` on one line, which is the
// signal the readme bridge tests to DROP the command rather than publish it.
func TestExpandFanOutUnresolvedStaysVerbatim(t *testing.T) {
	got, resolved := interp.ExpandFanOut(fanOutDoc(), "npm install ${org.projectfile.artifacts{kind=package}.name}")
	if len(got) != 1 || resolved {
		t.Fatalf("lines = %#v resolved = %v, want one unresolved line", got, resolved)
	}
	if !interp.Unresolved(got[0]) {
		t.Fatalf("line %q must still carry the reference verbatim", got[0])
	}
}

// TestExpandFanOutCrossProduct: two fan-out references in one string multiply
// out, which is what recursing on the substituted result buys instead of a
// dedicated case. Contrived — but a silent wrong answer here would be worse.
func TestExpandFanOutCrossProduct(t *testing.T) {
	doc := &projectfile.Document{
		Keywords: []string{"a", "b"},
		Extensions: map[string]any{
			artifactsNS: map[string]any{
				"one": map[string]any{keyKindField: kindImage, keyRefField: "x"},
				"two": map[string]any{keyKindField: kindImage, keyRefField: "y"},
			},
		},
	}
	got, _ := interp.ExpandFanOut(doc, "${keywords[]} ${org.projectfile.artifacts{kind=image}.ref}")
	want := []string{"a x", "a y", "b x", "b y"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines = %#v, want %#v", got, want)
	}
}

// TestExpandFanOutNoReferences: a plain command line is returned untouched as a
// single line, with no document walk at all.
func TestExpandFanOutNoReferences(t *testing.T) {
	got, _ := interp.ExpandFanOut(fanOutDoc(), "make dc-up")
	if !reflect.DeepEqual(got, []string{"make dc-up"}) {
		t.Fatalf("lines = %#v, want [make dc-up]", got)
	}
}

// TestExpandBraceBalancedReference is the regression for the reference reader
// ending at the FIRST `}`: with map forms in the address grammar, that brace
// belongs to the selector, and the truncated `artifacts{kind=image` resolves to
// nothing — silently leaving every artifact-driven command literal.
func TestExpandBraceBalancedReference(t *testing.T) {
	doc := fanOutDoc()
	got := interp.Expand(doc, "${org.projectfile.artifacts{kind=binary}.command} --version")
	assert.Equal(t, "pf-bridge --version", got)
	// Text after the reference must survive, and a following plain reference
	// must still resolve — proof `next` lands past the right brace.
	got = interp.Expand(doc, "run ${org.projectfile.artifacts{kind=binary}.command} in ${identity.name}")
	assert.Equal(t, "run pf-bridge in demo", got)
	// An unbalanced opener is not a reference: emitted verbatim, never a panic.
	assert.Equal(t, "${broken{", interp.Expand(doc, "${broken{"))
}

// TestExpandFanOutEscapeIsNotUnresolved is why resolution is REPORTED rather than
// scanned for: `$${literal}` expands to a literal `${literal}`, and a caller that
// dropped every line containing `${` would throw away a perfectly good command.
func TestExpandFanOutEscapeIsNotUnresolved(t *testing.T) {
	got, resolved := interp.ExpandFanOut(fanOutDoc(), "echo $${literal}")
	assert.Equal(t, []string{"echo ${literal}"}, got)
	assert.True(t, resolved, "an escaped $ is not an unresolved reference")
}

// TestExpandRefusesMultiValue: the single-value entry point leaves a fan-out
// reference verbatim instead of silently rendering the first of several values.
func TestExpandRefusesMultiValue(t *testing.T) {
	const s = "docker pull ${org.projectfile.artifacts{kind=image}.ref}"
	got, resolved := interp.ExpandChecked(fanOutDoc(), s)
	assert.Equal(t, s, got)
	assert.False(t, resolved)
}

// TestExpandFanOutNestedReference is the series-image chain end to end: the
// artifact's ref is itself written in terms of a matrix-axis LIST, so the
// fan-out has to be found one level down in the substituted value.
func TestExpandFanOutNestedReference(t *testing.T) {
	doc := &projectfile.Document{
		Extensions: map[string]any{
			"org.projectfile.ci": map[string]any{
				"matrix": map[string]any{"axes": map[string]any{
					"B19_UBUNTU_SERIES": []any{"resolute", "noble"},
				}},
			},
			artifactsNS: map[string]any{
				"image": map[string]any{
					"kind":      "image",
					keyRefField: "kiota.ch/b19/ubuntu/${org.projectfile.ci.matrix.axes.B19_UBUNTU_SERIES[]}:latest",
				},
			},
		},
	}
	got, resolved := interp.ExpandFanOut(doc, "docker pull ${org.projectfile.artifacts{kind=image}.ref}")
	assert.True(t, resolved)
	assert.Equal(t, []string{
		"docker pull kiota.ch/b19/ubuntu/resolute:latest",
		"docker pull kiota.ch/b19/ubuntu/noble:latest",
	}, got)
}

// scopedDoc is the whole model this engine has to serve, as DATA: a map of parts
// the project declares, and a map of destinations whose templates compose those
// parts. Nothing here is a vocabulary — `series` and `mood` are keys somebody
// typed, and core must never have heard of either.
func scopedDoc() *projectfile.Document {
	return &projectfile.Document{
		Identity: projectfile.Identity{Name: nameUbuntu},
		Extensions: map[string]any{
			"me": map[string]any{"dbuho": map[string]any{"projectfile": map[string]any{
				kindImage: map[string]any{
					keyOrgField: "b19",
					"name":      refIdentity,
					"series":    "resolute",
					"tag":       "latest",
					"mood":      "pissed",
				},
				"sinks": map[string]any{
					"kiota": map[string]any{keyRefField: "kiota.ch/${org}/${name}-${series}:${tag}"},
					"ghcr":  map[string]any{keyRefField: "ghcr.io/buho/${name}-is-fucking-${mood}:${tag}"},
					"hub":   map[string]any{keyRefField: "docker.io/damian-buho/${org}-${name}-${series}-${tag}"},
				},
			}}},
		},
	}
}

// TestExpandInComposesEveryShapeFromOneVocabulary is the exit criterion of the
// whole model: four unrelated path grammars, each one a template, none of them a
// code path. A registry that nests, one that flattens, one that puts the series
// in the tag — and one that interpolates a field invented after the tool shipped.
func TestExpandInComposesEveryShapeFromOneVocabulary(t *testing.T) {
	cases := []struct {
		sink string
		want string
	}{
		{"kiota", "kiota.ch/b19/ubuntu-resolute:latest"},
		{"ghcr", "ghcr.io/buho/ubuntu-is-fucking-pissed:latest"},
		{"hub", "docker.io/damian-buho/b19-ubuntu-resolute-latest"},
	}
	for _, tc := range cases {
		t.Run(tc.sink, func(t *testing.T) {
			ref := "${" + sinksScope + "." + tc.sink + "." + keyRefField + "}"
			got, resolved := interp.ExpandIn(scopedDoc(), ref, partsScope)
			assert.True(t, resolved, "a half-composed reference is a push to the wrong repository")
			assert.Equal(t, tc.want, got)
		})
	}
}

// A part may itself be a reference. `name: ${identity.name}` is what lets ONE
// shared fragment declare the parts for the whole fleet, so a project states only
// what makes it different.
func TestExpandInRecursesIntoAScopedPart(t *testing.T) {
	got, resolved := interp.ExpandIn(scopedDoc(), "${name}", partsScope)
	assert.True(t, resolved)
	assert.Equal(t, nameUbuntu, got)
}

// Four parts deep is a real fleet chain — a forge path over `flatpath` over `name`
// over `identity.name` — and the literal at its end counts against no depth.
func TestExpandInResolvesFourNestedParts(t *testing.T) {
	d := scopedDoc()
	parts := d.Extensions["me"].(map[string]any)["dbuho"].(map[string]any)["projectfile"].(map[string]any)[kindImage].(map[string]any)
	parts["flatpath"] = "${org}-${name}"
	parts["github"] = map[string]any{"path": "damian-buho/${flatpath}"}
	got, resolved := interp.ExpandIn(d, "https://github.com/${github.path}", partsScope)
	assert.True(t, resolved)
	assert.Equal(t, "https://github.com/damian-buho/b19-ubuntu", got)

	parts["deeper"] = "${github.path}"
	_, resolved = interp.ExpandIn(d, "${deeper}", partsScope)
	assert.False(t, resolved, "a fifth reference is past the cap and stays verbatim")
}

// A NEW key needs no release. This is the falsifier for "core knows about
// images": if it did, `mood` could not resolve.
func TestExpandInResolvesAKeyCoreNeverHeardOf(t *testing.T) {
	got, resolved := interp.ExpandIn(scopedDoc(), "${mood}", partsScope)
	assert.True(t, resolved)
	assert.Equal(t, "pissed", got)
}

// Scopes are tried in order, and the FIRST one that answers wins. That is how a
// destination overrides a part the fleet declares without editing the fleet.
func TestExpandInFirstScopeWins(t *testing.T) {
	doc := scopedDoc()
	sinks, _ := projectfile.LookupExtension(doc, sinksScope)
	ghcr := sinks.(map[string]any)["ghcr"].(map[string]any)
	ghcr["series"] = seriesNoble
	got, resolved := interp.ExpandIn(doc, "${series}", sinksScope+".ghcr", partsScope)
	assert.True(t, resolved)
	assert.Equal(t, seriesNoble, got)
}

// A scope that answers nothing falls through to the root, so a caller may offer
// an optional scope without testing that it exists first.
func TestExpandInFallsThroughToTheRoot(t *testing.T) {
	got, resolved := interp.ExpandIn(scopedDoc(), refIdentity, "me.dbuho.nothing.here")
	assert.True(t, resolved)
	assert.Equal(t, nameUbuntu, got)
}

// A matrix axis is a PART like any other, so the series moves into the tag by
// editing one template. `{AXIS}` carries no `$`: it survives composition and is
// substituted per cell by the layer that owns the matrix.
func TestExpandInLeavesAMatrixAxisForTheMatrixLayer(t *testing.T) {
	doc := scopedDoc()
	parts, _ := projectfile.LookupExtension(doc, partsScope)
	parts.(map[string]any)["series"] = "{B19_UBUNTU_SERIES}"
	got, resolved := interp.ExpandIn(doc, "kiota.ch/${org}/${name}:${tag}-${series}", partsScope)
	assert.True(t, resolved)
	assert.Equal(t, "kiota.ch/b19/ubuntu:latest-{B19_UBUNTU_SERIES}", got)
}

// One template, a FOREIGN subject. Resolving a base image is the same call under
// another scope — the reason a scope is an address the caller picks rather than a
// document the composer builds.
func TestExpandInComposesAForeignSubject(t *testing.T) {
	doc := scopedDoc()
	base, _ := projectfile.LookupExtension(doc, projectfileNS)
	base.(map[string]any)["base-image"] = map[string]any{
		keyOrgField: "b19", "name": "fd", "series": seriesNoble, "tag": "1.2.3",
	}
	tmpl := "${" + sinksScope + ".kiota." + keyRefField + "}"
	own, ownOK := interp.ExpandIn(doc, tmpl, partsScope)
	assert.True(t, ownOK)
	assert.Equal(t, "kiota.ch/b19/ubuntu-resolute:latest", own)

	foreign, foreignOK := interp.ExpandIn(doc, tmpl, projectfileNS+".base-image")
	assert.True(t, foreignOK)
	assert.Equal(t, "kiota.ch/b19/fd-noble:1.2.3", foreign)
}

// A template composes ONLY under a scope. With none, `${org}` is not a document
// address and survives verbatim, so `resolved` is false and the caller refuses
// the reference. That is the guard against a ref that silently lost a segment and
// pushes to the wrong repository — it cannot half-compose by falling back to the
// root.
func TestExpandRefusesToComposeWithoutAScope(t *testing.T) {
	got, resolved := interp.ExpandChecked(scopedDoc(), "${"+sinksScope+".kiota."+keyRefField+"}")
	assert.False(t, resolved)
	assert.Equal(t, "kiota.ch/${org}/${name}-${series}:${tag}", got)
}
