// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package sink

import (
	"reflect"
	"testing"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

// docWith builds a document carrying one extension namespace, which is all the
// reader here looks at.
func docWith(ns string, entries map[string]any) *projectfile.Document {
	return &projectfile.Document{Extensions: map[string]any{ns: entries}}
}

func names(sinks []Sink) []string {
	out := make([]string, 0, len(sinks))
	for _, s := range sinks {
		out = append(out, s.Name)
	}
	return out
}

// TestDeclaredRanksByPriority pins the order the whole plan exists for: a
// document whose priority order is the REVERSE of its spelling must resolve by
// priority, so a README recommends the preferred destination first and names the
// fallback last. An alphabetical reader would fail this rather than pass by luck.
func TestDeclaredRanksByPriority(t *testing.T) {
	doc := docWith(ExtensionNS, map[string]any{
		nameGHCR:  map[string]any{KeyHost: hostGHCR, keyPriorityTest: 90},
		"ecr":     map[string]any{KeyHost: "public.ecr.aws", keyPriorityTest: 80},
		nameKiota: map[string]any{KeyHost: hostKiota, KeyRole: RoleFallback, keyPriorityTest: 10},
	})
	sinks, err := Declared(doc)
	if err != nil {
		t.Fatalf("Declared: %v", err)
	}
	if want := []string{nameGHCR, "ecr", nameKiota}; !reflect.DeepEqual(names(sinks), want) {
		t.Errorf("got %v, want %v", names(sinks), want)
	}
}

// TestDeclaredKeepsNameOrderWithoutPriority: unranked entries keep a stable,
// deterministic order rather than Go's randomised map iteration.
func TestDeclaredKeepsNameOrderWithoutPriority(t *testing.T) {
	doc := docWith(ExtensionNS, map[string]any{
		"zulu":  map[string]any{KeyHost: "z.example"},
		"alpha": map[string]any{KeyHost: "a.example"},
	})
	sinks, _ := Declared(doc)
	if want := []string{"alpha", "zulu"}; !reflect.DeepEqual(names(sinks), want) {
		t.Errorf("got %v, want %v", names(sinks), want)
	}
}

// TestDefaultTemplateOmitsOwnerSegment is the trap the plan cost a session to
// find: `image.basename` ALREADY carries the project's namespace, so an owner
// segment defaulted into the path publishes `kiota.ch/b19/b19/ubuntu`. A bare
// `host:` entry must compose exactly what the fleet composed before sinks
// existed.
func TestDefaultTemplateOmitsOwnerSegment(t *testing.T) {
	s := Sink{Name: nameKiota, Entry: map[string]any{KeyHost: hostKiota}}
	got, ok := s.Compose(b19Coords)
	if !ok {
		t.Fatal("compose refused a bare host entry")
	}
	if want := "kiota.ch/b19/ubuntu/{B19_UBUNTU_SERIES}:latest"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestDefaultTemplateKeepsDeclaredOwner: a registry that forces one account on
// the whole fleet gets exactly one owner segment, and still needs no template.
func TestDefaultTemplateKeepsDeclaredOwner(t *testing.T) {
	s := Sink{Name: nameGHCR, Entry: map[string]any{KeyHost: hostGHCR, KeyOwner: ownerFleet}}
	got, ok := s.Compose(b19Coords)
	if !ok {
		t.Fatal("compose refused an entry declaring host and owner")
	}
	if want := "ghcr.io/damian-buho/b19/ubuntu/{B19_UBUNTU_SERIES}:latest"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestOwnerFallsBackToImageNamespace: a template written for an account-forcing
// registry keeps working on one that forces no account. The fallback applies to
// the VARIABLE, never to the default template's path — the two tests above pin
// the other half.
func TestOwnerFallsBackToImageNamespace(t *testing.T) {
	s := Sink{Name: "anon", Entry: map[string]any{
		KeyHost: "anon.example",
		KeyRef:  "${sink.host}/${sink.owner}/${image.name}:${image.tag}",
	}}
	got, ok := s.Compose(Coords{Basename: "b19/ubuntu", Tag: tagLatest})
	if !ok {
		t.Fatal("compose refused a template naming an undeclared owner")
	}
	if want := "anon.example/b19/ubuntu:latest"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestDeclaredSkipsUnaddressableEntry: an entry with neither a template nor a
// host addresses no authority. Composing from it would emit a reference starting
// with a slash, which is a push to a path on the default registry.
func TestDeclaredSkipsUnaddressableEntry(t *testing.T) {
	doc := docWith(ExtensionNS, map[string]any{
		"good": map[string]any{KeyHost: hostGHCR},
		"bad":  map[string]any{KeyOwner: ownerFleet, keyPriorityTest: 99},
	})
	sinks, _ := Declared(doc)
	if want := []string{"good"}; !reflect.DeepEqual(names(sinks), want) {
		t.Errorf("got %v, want %v", names(sinks), want)
	}
}

// TestDeclaredDoesNotEditTheDocument: the defaults the reader supplies are
// bindings, not data. A reader that wrote them onto the declared entry would
// persist `owner: ${image.namespace}` — a value nobody typed — on the next
// base write.
func TestDeclaredDoesNotEditTheDocument(t *testing.T) {
	entry := map[string]any{KeyHost: hostKiota}
	doc := docWith(ExtensionNS, map[string]any{nameKiota: entry})
	sinks, _ := Declared(doc)
	if _, ok := sinks[0].Compose(b19Coords); !ok {
		t.Fatal("compose refused a bare host entry")
	}
	if len(entry) != 1 {
		t.Errorf("reader wrote defaults onto the declared entry: %v", entry)
	}
}

// TestDeclaredAbsentNamespace: no sinks is a valid state, not an error. The
// ~130 projects that declare nothing must not make a caller fail.
func TestDeclaredAbsentNamespace(t *testing.T) {
	sinks, err := Declared(&projectfile.Document{})
	if err != nil || sinks != nil {
		t.Errorf("got (%v, %v), want (nil, nil)", sinks, err)
	}
}

// TestDeclaredRejectsNonMapNamespace: a scalar where the destinations should be
// is a typo in a document that meant to declare them. Reporting nothing would
// look identical to declaring nothing.
func TestDeclaredRejectsNonMapNamespace(t *testing.T) {
	doc := &projectfile.Document{Extensions: map[string]any{ExtensionNS: hostGHCR}}
	if _, err := Declared(doc); err == nil {
		t.Error("Declared accepted a scalar namespace")
	}
}

// TestRoleDefaultsToPrimary: role is what makes an entry reachable across the
// map at all — an entry addressable by no selector renders no pull line.
func TestRoleDefaultsToPrimary(t *testing.T) {
	if got := (Sink{Entry: map[string]any{}}).Role(); got != RolePrimary {
		t.Errorf("got %q, want %q", got, RolePrimary)
	}
	if got := (Sink{Entry: map[string]any{KeyRole: RoleFallback}}).Role(); got != RoleFallback {
		t.Errorf("got %q, want %q", got, RoleFallback)
	}
}

// TestRoutes reads the publish half: which pipeline sends where, and which sink
// it pulls its own base images from.
func TestRoutes(t *testing.T) {
	doc := docWith(PublishExtensionNS, map[string]any{
		"github":  map[string]any{keyPush: []any{nameGHCR, nameHubMain}, keyPull: nameGHCR},
		nameKiota: map[string]any{keyPush: []any{nameKiota}, keyPull: nameKiota},
	})
	routes, err := Routes(doc)
	if err != nil {
		t.Fatalf("Routes: %v", err)
	}
	want := []Route{
		{Forge: "github", Push: []string{nameGHCR, nameHubMain}, Pull: nameGHCR},
		{Forge: nameKiota, Push: []string{nameKiota}, Pull: nameKiota},
	}
	if !reflect.DeepEqual(routes, want) {
		t.Errorf("got %v, want %v", routes, want)
	}
}

// TestSelectRefusesUndeclaredSink: a route naming a destination that does not
// exist is a push to nowhere. The caller must be able to refuse rather than
// quietly send to one fewer place than the document promised.
func TestSelectRefusesUndeclaredSink(t *testing.T) {
	sinks := []Sink{{Name: nameGHCR, Entry: map[string]any{KeyHost: hostGHCR}}}
	selected, ok := Select(sinks, []string{nameGHCR, "typo"})
	if ok {
		t.Error("Select accepted a route naming an undeclared sink")
	}
	if want := []string{nameGHCR}; !reflect.DeepEqual(names(selected), want) {
		t.Errorf("got %v, want %v", names(selected), want)
	}
}
