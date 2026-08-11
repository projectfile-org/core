// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package sink

import (
	"reflect"
	"testing"
)

// Fixture literals shared across both test files of this package, declared once
// (goconst).
const (
	nameGHCR    = "ghcr"
	nameKiota   = "kiota"
	nameHubMain = "hub-main"
	hostGHCR    = "ghcr.io"
	hostKiota   = "kiota.ch"
	ownerFleet  = "damian-buho"
	tagLatest   = "latest"

	// keyPriorityTest spells the rank key the fixtures set. The reader takes it
	// through fieldpath.EntryPriority, which owns the name on the production
	// side, so the tests name it here rather than importing that package.
	keyPriorityTest = "priority"
)

// b19Coords is the fleet's hardest real image: three path labels, the last one a
// matrix placeholder. Every shape test uses it so the flatten/split rules are
// exercised against a placeholder rather than a tidy two-label name.
var b19Coords = Coords{Basename: "b19/ubuntu/{B19_UBUNTU_SERIES}", Tag: tagLatest}

func TestComposeNested(t *testing.T) {
	entry := map[string]any{KeyRef: "ghcr.io/damian-buho/${image.basename}:${image.tag}"}
	got, ok := Compose(entry, b19Coords)
	if !ok {
		t.Fatal("compose refused a resolvable template")
	}
	if want := "ghcr.io/damian-buho/b19/ubuntu/{B19_UBUNTU_SERIES}:latest"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestComposeTwoAccountsOneHost is the architectural falsifier: nobody will
// publish the same image to two Docker Hub accounts at once, but a model that
// CANNOT is one where the sink name secretly means a registry. Both entries name
// the same host, differ only in account, and neither is distinguishable to the
// composer — which is what proves the name is a label.
func TestComposeTwoAccountsOneHost(t *testing.T) {
	sinks := map[string]map[string]any{
		nameHubMain: {KeyRef: "docker.io/damianbuho/${image.flatname}:${image.tag}"},
		"hub-oss":   {KeyRef: "docker.io/buho-oss/${image.flatname}:${image.tag}"},
	}
	want := map[string]string{
		nameHubMain: "docker.io/damianbuho/b19-ubuntu-{B19_UBUNTU_SERIES}:latest",
		"hub-oss":   "docker.io/buho-oss/b19-ubuntu-{B19_UBUNTU_SERIES}:latest",
	}
	for name, entry := range sinks {
		got, ok := Compose(entry, b19Coords)
		if !ok {
			t.Fatalf("%s: compose refused", name)
		}
		if got != want[name] {
			t.Errorf("%s: got %q, want %q", name, got, want[name])
		}
	}
}

// TestComposeArbitraryShape pins the property the whole design exists for: a
// destination whose path grammar matches nothing else in the fleet is reachable
// by writing a template, with no code change. Here the account segment is the
// owner glued to the project's own top namespace, and the repository is the rest
// of the path flattened.
func TestComposeArbitraryShape(t *testing.T) {
	entry := map[string]any{
		"owner": ownerFleet,
		KeyRef:  "weird.example/${sink.owner}-${image.root}/${image.flatpath}:${image.tag}",
	}
	got, ok := Compose(entry, Coords{Basename: "b19/ubuntu/resolute", Tag: "1.2"})
	if !ok {
		t.Fatal("compose refused a resolvable template")
	}
	if want := "weird.example/damian-buho-b19/ubuntu-resolute:1.2"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestComposeSinkKeyIndirection: a sink key may itself interpolate, so a shape
// used by several sinks is named once on the entry instead of repeated inside
// every template. This is the recursion interp already does, reached for free.
func TestComposeSinkKeyIndirection(t *testing.T) {
	entry := map[string]any{
		"owner":   ownerFleet,
		"account": "${sink.owner}-${image.root}",
		KeyRef:    "weird.example/${sink.account}/${image.flatpath}:${image.tag}",
	}
	got, ok := Compose(entry, Coords{Basename: "b19/ubuntu/resolute", Tag: "1.2"})
	if !ok {
		t.Fatal("compose refused a resolvable template")
	}
	if want := "weird.example/damian-buho-b19/ubuntu-resolute:1.2"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestComposeRefusesUnresolved: a template naming something no coordinate
// answers must FAIL, not compose a ref with a hole in it. A silently shortened
// path is a push to the wrong repository.
func TestComposeRefusesUnresolved(t *testing.T) {
	entry := map[string]any{KeyRef: "ghcr.io/${sink.owner}/${image.basename}:${image.tag}"}
	if got, ok := Compose(entry, b19Coords); ok {
		t.Errorf("compose accepted a template with an unanswered reference: %q", got)
	}
}

// TestComposeKeepsForeignMakeVar: a make variable is not a field address, so it
// must survive verbatim for the layer that does expand it. Without this the
// composer would have to know which layer owns which name.
func TestComposeKeepsForeignMakeVar(t *testing.T) {
	entry := map[string]any{KeyRef: "$${SOURCE_DOCKER_REGISTRY}/${image.basename}:${image.tag}"}
	got, ok := Compose(entry, b19Coords)
	if !ok {
		t.Fatal("compose refused a template carrying an escaped make variable")
	}
	if want := "${SOURCE_DOCKER_REGISTRY}/b19/ubuntu/{B19_UBUNTU_SERIES}:latest"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestComposeForeignCoords is why Coords are a parameter and not a document
// read: the same sink composes a ref for ANOTHER project's image, which is what
// resolving a base image needs.
func TestComposeForeignCoords(t *testing.T) {
	entry := map[string]any{KeyRef: "docker.io/damianbuho/${image.flatname}:${image.tag}"}
	got, ok := Compose(entry, Coords{Basename: "o9s/loki", Tag: "3.1"})
	if !ok {
		t.Fatal("compose refused a resolvable template")
	}
	if want := "docker.io/damianbuho/o9s-loki:3.1"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestComposeNoTemplate(t *testing.T) {
	if got, ok := Compose(map[string]any{KeyHost: hostGHCR}, b19Coords); ok {
		t.Errorf("compose invented a ref for an entry declaring none: %q", got)
	}
}

// TestComposeFanOut: a sink key naming several values composes one ref each,
// rather than collapsing to the first.
func TestComposeFanOut(t *testing.T) {
	entry := map[string]any{
		"tags": []any{"latest", "stable"},
		KeyRef: "ghcr.io/acme/${image.basename}:${sink.tags[]}",
	}
	got, ok := ComposeFanOut(entry, Coords{Basename: "o9s/loki", Tag: "3.1"})
	if !ok {
		t.Fatal("compose refused a resolvable template")
	}
	want := []string{"ghcr.io/acme/o9s/loki:latest", "ghcr.io/acme/o9s/loki:stable"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestComposeIgnoresProjectFields: the scratch document carries coordinates and
// nothing else, so a template reaching for a real project field is refused
// rather than resolved. A sink template must mean the same thing in every
// project that uses it.
func TestComposeIgnoresProjectFields(t *testing.T) {
	entry := map[string]any{KeyRef: "ghcr.io/${identity.name}/${image.basename}:${image.tag}"}
	if got, ok := Compose(entry, b19Coords); ok {
		t.Errorf("compose resolved a project field on a coordinate-only document: %q", got)
	}
}
