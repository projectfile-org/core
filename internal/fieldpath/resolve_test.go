// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fieldpath

import (
	"errors"
	"reflect"
	"testing"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

const (
	testKind      = "library"
	testRust      = "rust"
	testNamespace = "org.example"
	testName      = "demo"
	testContainer = "container"
	testRepoURL   = "https://example.com/repo.git"
	testUser      = "user"
	testEnvNS     = "com.example.env"
	testExample   = "example"
	testAlpine    = "alpine"
	testOrigin    = "origin"
	testWasm      = "wasm"
	testRole      = "role"
	testURL       = "url"
	testEnv       = "env"
)

// fixture is the minimal but representative document the read/write tests
// share. Covers a scalar (identity.namespace), a localized string
// (identity.title.en), a string list (keywords), an object list with
// selector-able fields (repositories, links), and an extension entry
// addressable via both flat-key (com.example.build.user) and dotted segments.
func fixture() *projectfile.Document {
	return &projectfile.Document{
		SpecVersion: "1",
		Kind:        testKind,
		Identity: projectfile.Identity{
			Namespace: testNamespace,
			Name:      testName,
			Title:     &projectfile.LocalizedString{Langs: map[string]string{"en": "Demo Title"}},
		},
		Keywords: []string{testRust, testWasm, testContainer},
		Repositories: []projectfile.Repository{
			{URL: testRepoURL, Role: testOrigin, Branch: "main"},
			{URL: "https://mirror.example.com/repo.git", Role: "mirror"},
		},
		Links: []projectfile.Link{
			{Type: "bugs", URL: "https://example.com/issues"},
			{Type: "source-code", URL: "https://example.com/src"},
		},
		Requirements: &projectfile.Requirements{
			Runtime: map[string]string{"node": ">=24"},
		},
		Extensions: map[string]any{
			"com.example.build": map[string]any{
				testUser: "ubuntu",
				"labels": map[string]any{"experimental": true},
			},
		},
	}
}

func TestResolveScalar(t *testing.T) {
	doc := fixture()
	p, _ := Parse("identity.namespace")
	r, err := Resolve(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := r.Single()
	if v != testNamespace {
		t.Fatalf("got %v, want %v", v, testNamespace)
	}
}

func TestResolveLocalizedLeaf(t *testing.T) {
	doc := fixture()
	p, _ := Parse("identity.title.en")
	r, err := Resolve(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := r.Single()
	if v != "Demo Title" {
		t.Fatalf("got %v, want Demo Title", v)
	}
}

func TestResolveListIndex(t *testing.T) {
	doc := fixture()
	p, _ := Parse("keywords[1]")
	r, err := Resolve(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := r.Single()
	if v != testWasm {
		t.Fatalf("got %v, want %v", v, testWasm)
	}
}

func TestResolveNegativeIndex(t *testing.T) {
	doc := fixture()
	p, _ := Parse("keywords[-1]")
	r, err := Resolve(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := r.Single()
	if v != testContainer {
		t.Fatalf("got %v, want %v", v, testContainer)
	}
}

func TestResolveSelector(t *testing.T) {
	doc := fixture()
	p, _ := Parse("repositories[role=origin].url")
	r, err := Resolve(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := r.Single()
	if v != testRepoURL {
		t.Fatalf("got %v, want origin URL", v)
	}
}

func TestResolveProjection(t *testing.T) {
	doc := fixture()
	p, _ := Parse("repositories[].url")
	r, err := Resolve(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	want := []any{testRepoURL, "https://mirror.example.com/repo.git"}
	if !reflect.DeepEqual(r.Values, want) {
		t.Fatalf("got %v, want %v", r.Values, want)
	}
}

func TestResolveWholeList(t *testing.T) {
	doc := fixture()
	p, _ := Parse("keywords")
	r, err := Resolve(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := r.Single()
	got, ok := asList(v)
	if !ok {
		t.Fatalf("expected list, got %T", v)
	}
	if len(got) != 3 {
		t.Fatalf("got %d items, want 3", len(got))
	}
}

// TestResolveMapProject covers the basic {} fan-out: pairs come back in
// deterministic (sorted) order with the IsPairs flag set, so downstream
// formatters can render them as KEY=VALUE / export / JSON object.
func TestResolveMapProject(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Namespace: testNamespace, Name: testName},
		Extensions: map[string]any{
			testEnvNS: map[string]any{
				"FOO": "1",
				"BAR": "2",
				"BAZ": "3",
			},
		},
	}
	p, err := Parse("ext.com.example.env{}")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Resolve(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsPairs {
		t.Fatalf("expected IsPairs=true")
	}
	got := make([]Pair, 0, len(r.Values))
	for _, v := range r.Values {
		got = append(got, v.(Pair))
	}
	want := []Pair{{Key: "BAR", Value: "2"}, {Key: "BAZ", Value: "3"}, {Key: "FOO", Value: "1"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pairs = %#v, want %#v", got, want)
	}
}

// TestResolveMapProjectDottedLabels confirms dotted keys survive the fan-out
// intact — `traefik.enable: "true"` is one pair, not a navigated tree. The
// raw key (with embedded dot) lands in Pair.Key untouched.
func TestResolveMapProjectDottedLabels(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Namespace: testNamespace, Name: testName},
		Extensions: map[string]any{
			"com.example.build": map[string]any{
				"labels": map[string]any{
					"traefik.enable":         "true",
					"traefik.docker.network": "edge",
				},
			},
		},
	}
	p, err := Parse("ext.com.example.build.labels{}")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Resolve(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsPairs || len(r.Values) != 2 {
		t.Fatalf("got %d pairs, want 2 — values=%#v", len(r.Values), r.Values)
	}
	for _, v := range r.Values {
		p := v.(Pair)
		if p.Key != "traefik.enable" && p.Key != "traefik.docker.network" {
			t.Fatalf("dotted key dropped or mangled: %q", p.Key)
		}
	}
}

// TestResolveMapProjectKeysValues exercises the only two legal trailers
// past `{}`: `.keys` and `.values`. Both downgrade IsPairs to IsList — the
// formatter then treats the result as a regular line-oriented list.
func TestResolveMapProjectKeysValues(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Namespace: testNamespace, Name: testName},
		Extensions: map[string]any{
			testEnvNS: map[string]any{"A": "1", "B": "2"},
		},
	}
	pk, _ := Parse("ext.com.example.env{}.keys")
	rk, err := Resolve(doc, pk)
	if err != nil {
		t.Fatal(err)
	}
	if rk.IsPairs || !rk.IsList {
		t.Fatalf("keys trailer: want IsList without IsPairs, got IsList=%v IsPairs=%v", rk.IsList, rk.IsPairs)
	}
	if !reflect.DeepEqual(rk.Values, []any{"A", "B"}) {
		t.Fatalf("keys = %v, want [A B]", rk.Values)
	}
	pv, _ := Parse("ext.com.example.env{}.values")
	rv, err := Resolve(doc, pv)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rv.Values, []any{"1", "2"}) {
		t.Fatalf("values = %v, want [1 2]", rv.Values)
	}
}

// TestResolveMapProjectInvalidTrailer rejects anything other than .keys /
// .values past `{}` — the walker has nowhere to navigate after fan-out.
func TestResolveMapProjectInvalidTrailer(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Namespace: testNamespace, Name: testName},
		Extensions: map[string]any{
			testEnvNS: map[string]any{"A": "1"},
		},
	}
	p, _ := Parse("ext.com.example.env{}.A")
	if _, err := Resolve(doc, p); err == nil {
		t.Fatalf("expected error for illegal trailer past {}")
	}
}

func TestResolveExtensionFlatKey(t *testing.T) {
	// The fixture's extension is encoded as a flat key "com.example.build" with
	// sub-fields under it. Address resolves via longest-prefix matching.
	doc := fixture()
	p, _ := Parse("ext.com.example.build.user")
	r, err := Resolve(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := r.Single()
	if v != "ubuntu" {
		t.Fatalf("got %v, want ubuntu", v)
	}
}

func TestResolveExtensionNested(t *testing.T) {
	// When the extension lives as nested maps (e.g. doc.Extensions["com"]
	// → map{"example": map{"build": ...}}), the same path should resolve.
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Namespace: testNamespace, Name: testName},
		Extensions: map[string]any{
			"com": map[string]any{
				testExample: map[string]any{
					"build": map[string]any{testUser: testAlpine},
				},
			},
		},
	}
	p, _ := Parse("ext.com.example.build.user")
	r, err := Resolve(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := r.Single()
	if v != testAlpine {
		t.Fatalf("got %v, want alpine", v)
	}
}

func TestResolveMissing(t *testing.T) {
	doc := fixture()
	p, _ := Parse("identity.version")
	if _, err := Resolve(doc, p); err == nil {
		t.Fatalf("expected ErrNotFound, got nil")
	}
}

// TestResolveListOpOnMap asserts the list operators ([N] index, [k=v] selector,
// [] projection) aimed at a MAP of named keys are refused with the DISTINCT
// ErrListOpOnMap — a grammar mismatch a reader surfaces LOUDLY — while the SAME
// operators on a scalar stay an ErrNotFound soft miss so --default / --or-default
// still apply. The map target (the com.example.build extension) stands in for
// org.projectfile.artifacts, the map-of-named-keys the artifacts model produced.
func TestResolveListOpOnMap(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		wantMap bool // true → ErrListOpOnMap; false → ErrNotFound
	}{
		{"selector-on-map", "ext.com.example.build[user=ubuntu]", true},
		{"index-on-map", "ext.com.example.build[0]", true},
		{"projection-on-map", "ext.com.example.build[]", true},
		{"index-on-scalar", "identity.namespace[0]", false},
		{"selector-on-scalar", "identity.namespace[x=y]", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := fixture()
			p, err := Parse(tc.path)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.path, err)
			}
			if _, err = Resolve(doc, p); err == nil {
				t.Fatalf("path %q: expected an error, got nil", tc.path)
			}
			if got := errors.Is(err, ErrListOpOnMap); got != tc.wantMap {
				t.Fatalf("path %q: ErrListOpOnMap=%v, want %v (err=%v)", tc.path, got, tc.wantMap, err)
			}
			// The two sentinels are mutually exclusive: a map grammar-mismatch is
			// never also a soft miss (that inversion is the whole point — one is
			// refused loudly, the other soft-exits under --default).
			if errors.Is(err, ErrNotFound) == tc.wantMap {
				t.Fatalf("path %q: ErrNotFound must be the inverse of ErrListOpOnMap (err=%v)", tc.path, err)
			}
		})
	}
}

func TestExistsTrue(t *testing.T) {
	doc := fixture()
	p, _ := Parse("identity.namespace")
	if !Exists(doc, p) {
		t.Fatalf("expected Exists=true")
	}
}

func TestExistsFalse(t *testing.T) {
	doc := fixture()
	p, _ := Parse("identity.version")
	if Exists(doc, p) {
		t.Fatalf("expected Exists=false")
	}
}

// TestLookupDefaultOperatingSystemUnconstrained asserts there is no
// operating-system default. An absent operating-system is unconstrained, with
// no arch-coupled fallback (the old ["linux"] default had no spec basis).
// Platform targeting lives in the org.projectfile.operating-system extension
// field per spec §4.8a, so the pre-§4.8a requirements path is checked too: it
// must not resurrect a default either.
func TestLookupDefaultOperatingSystemUnconstrained(t *testing.T) {
	doc := fixture()
	for _, path := range []string{"org.projectfile.operating-system", "requirements.operating-system"} {
		if _, ok := LookupDefault(doc, path); ok {
			t.Fatalf("expected no operating-system default to apply at %s", path)
		}
	}
}

func TestLookupDefaultKind(t *testing.T) {
	doc := &projectfile.Document{Identity: projectfile.Identity{Namespace: "org.x", Name: "y"}}
	v, ok := LookupDefault(doc, "kind")
	if !ok || v != "library" {
		t.Fatalf("got (%v, %v), want (library, true)", v, ok)
	}
}

// Repeated artifacts-fixture keys and values, named so goconst sees one home each.
const (
	artifactsNS = "org.projectfile.artifacts"
	keyKind     = "kind"
	keyRef      = "ref"
	keyName     = "name"
	keyRegistry = "registry"
	kindImage   = "image"
	registryNpm = "npm"
)

// artifactsDoc is the map-of-named-keys the `{k=v}` selector exists for: an
// org.projectfile.artifacts subtree holding two images, one binary, and one
// scalar entry. Named keys are deliberately NOT in the order the selector must
// return them (sorted), and `stray` is a scalar so the walker's skip-non-map
// arm is exercised by every case below.
func artifactsDoc() *projectfile.Document {
	return &projectfile.Document{
		Identity: projectfile.Identity{Namespace: testNamespace, Name: testName},
		Extensions: map[string]any{
			artifactsNS: map[string]any{
				"web-image":  map[string]any{keyKind: kindImage, keyRef: "kiota.ch/x/web:latest"},
				"cli-binary": map[string]any{keyKind: "binary", "path": "dist/x", "command": "x"},
				"api-image":  map[string]any{keyKind: kindImage, keyRef: "kiota.ch/x/api:latest"},
				"stray":      "not-a-map",
			},
		},
	}
}

// TestResolveMapSelectorFansOut is the contract the README recipes rest on: a
// map selector returns EVERY match, not the first, because a map of named keys
// has no order in which "first" would mean anything. Sorted-key iteration makes
// the fan-out deterministic (api-image before web-image).
func TestResolveMapSelectorFansOut(t *testing.T) {
	p, err := Parse("org.projectfile.artifacts{kind=image}.ref")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Resolve(artifactsDoc(), p)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsList {
		t.Fatalf("expected IsList=true for a fan-out, got %#v", r)
	}
	want := []any{"kiota.ch/x/api:latest", "kiota.ch/x/web:latest"}
	if !reflect.DeepEqual(r.Values, want) {
		t.Fatalf("refs = %#v, want %#v (sorted by artifact name)", r.Values, want)
	}
}

// registriesDoc carries three registries whose PRIORITY order is the reverse of
// their alphabetical order, so a run that still sorted by key alone would fail
// this test rather than pass it by accident. `kiota` is the fallback and must
// render last despite sorting first by spelling.
func registriesDoc() *projectfile.Document {
	return &projectfile.Document{
		Identity: projectfile.Identity{Namespace: testNamespace, Name: testName},
		Extensions: map[string]any{
			"org.projectfile.registries": map[string]any{
				"kiota": map[string]any{keyRef: "kiota.ch/x/web:edge", keyPriority: 10},
				"ghcr":  map[string]any{keyRef: "ghcr.io/o/x-web:latest", keyPriority: 90},
				"ecr":   map[string]any{keyRef: "public.ecr.aws/o/x-web:latest", keyPriority: 80},
			},
		},
	}
}

// TestResolveMapProjectPriorityOrder is the ordering primitive the registry
// recipes rest on: a reader must be told the recommended registry first and the
// last-resort one last, and a map cannot say that positionally.
func TestResolveMapProjectPriorityOrder(t *testing.T) {
	p, err := Parse("org.projectfile.registries{}.keys")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Resolve(registriesDoc(), p)
	if err != nil {
		t.Fatal(err)
	}
	want := []any{"ghcr", "ecr", "kiota"}
	if !reflect.DeepEqual(r.Values, want) {
		t.Fatalf("keys = %#v, want %#v (priority descending, not alphabetical)", r.Values, want)
	}
}

// TestResolveMapSelectorPriorityTieBreak proves the second half of the rule.
// Equal priorities must not leave the order to Go's map iteration, which varies
// per run and would make a generated README churn.
func TestResolveMapSelectorPriorityTieBreak(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Namespace: testNamespace, Name: testName},
		Extensions: map[string]any{
			artifactsNS: map[string]any{
				"zeta":  map[string]any{keyKind: kindImage, keyRef: "z", keyPriority: 50},
				"alpha": map[string]any{keyKind: kindImage, keyRef: "a", keyPriority: 50},
				"mid":   map[string]any{keyKind: kindImage, keyRef: "m"},
			},
		},
	}
	p, _ := Parse("org.projectfile.artifacts{kind=image}.ref")
	for range 10 {
		r, err := Resolve(doc, p)
		if err != nil {
			t.Fatal(err)
		}
		want := []any{"a", "m", "z"}
		if !reflect.DeepEqual(r.Values, want) {
			t.Fatalf("refs = %#v, want %#v (an undeclared priority equals the default)", r.Values, want)
		}
	}
}

// TestResolveMapSelectorSingleMatch covers the overwhelmingly common shape — one
// artifact of a kind. The Result still carries IsList (it is a projection), and
// Single() hands the lone value back so an interpolating caller needs no
// special case for "exactly one".
func TestResolveMapSelectorSingleMatch(t *testing.T) {
	p, _ := Parse("org.projectfile.artifacts{kind=binary}.command")
	r, err := Resolve(artifactsDoc(), p)
	if err != nil {
		t.Fatal(err)
	}
	v, ok := r.Single()
	if !ok || v != "x" {
		t.Fatalf("Single() = (%v, %v), want (x, true)", v, ok)
	}
}

// TestResolveMapSelectorNoMatch: a kind nothing declares is an ErrNotFound soft
// miss, NOT the loud ErrListOpOnMap. That is what lets a shared recipe fragment
// name every ecosystem unconditionally — a project that ships no npm package
// leaves the reference unresolved and the command drops, rather than failing the
// whole render.
func TestResolveMapSelectorNoMatch(t *testing.T) {
	p, _ := Parse("org.projectfile.artifacts{kind=npm-package}.name")
	_, err := Resolve(artifactsDoc(), p)
	if err == nil {
		t.Fatalf("expected an error for an unmatched kind")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound (soft miss), got %v", err)
	}
	if errors.Is(err, ErrListOpOnMap) {
		t.Fatalf("an unmatched map selector is a miss, not a grammar mismatch: %v", err)
	}
}

// TestResolveMapSelectorPartialRemainder: a match whose remainder is absent is
// skipped rather than failing the address — same tolerance as `[]` projection.
// Here both images match `kind=image` but only one carries `digest`.
func TestResolveMapSelectorPartialRemainder(t *testing.T) {
	doc := artifactsDoc()
	arts := doc.Extensions[artifactsNS].(map[string]any)
	arts["web-image"].(map[string]any)["digest"] = "sha256:beef"
	p, _ := Parse("org.projectfile.artifacts{kind=image}.digest")
	r, err := Resolve(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Values, []any{"sha256:beef"}) {
		t.Fatalf("values = %#v, want only the entry carrying digest", r.Values)
	}
}

// TestResolveMapSelectorOnList: the curly form aimed at a LIST is the mirror of
// ErrListOpOnMap and must not silently match nothing.
func TestResolveMapSelectorOnList(t *testing.T) {
	p, _ := Parse("links{type=source-code}.url")
	if _, err := Resolve(fixture(), p); err == nil {
		t.Fatalf("expected an error for a map selector aimed at a list")
	}
}

// TestResolveMapSelectorMultiPredicate: predicates are conjunctive, so a second
// term narrows the fan-out — `registry=npm` picks one of two packages.
func TestResolveMapSelectorMultiPredicate(t *testing.T) {
	doc := &projectfile.Document{
		Identity: projectfile.Identity{Namespace: testNamespace, Name: testName},
		Extensions: map[string]any{
			artifactsNS: map[string]any{
				"lib-npm":  map[string]any{keyKind: "package", keyRegistry: registryNpm, keyName: "a"},
				"lib-pypi": map[string]any{keyKind: "package", keyRegistry: "pypi", keyName: "b"},
			},
		},
	}
	p, _ := Parse("org.projectfile.artifacts{kind=package,registry=npm}.name")
	r, err := Resolve(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Values, []any{"a"}) {
		t.Fatalf("values = %#v, want [a]", r.Values)
	}
}
