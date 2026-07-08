// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fieldpath

import (
	"reflect"
	"testing"

	"kiota.ch/projectfile/core/internal/projectfile"
)

func TestSetScalar(t *testing.T) {
	doc := fixture()
	p, _ := Parse("identity.version")
	out, err := Set(doc, p, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if out.Identity.Version != "1.2.3" {
		t.Fatalf("got %q, want 1.2.3", out.Identity.Version)
	}
}

func TestSetLocalizedLeaf(t *testing.T) {
	doc := fixture()
	p, _ := Parse("identity.title.en")
	out, err := Set(doc, p, "New Title")
	if err != nil {
		t.Fatal(err)
	}
	if out.Identity.Title == nil || out.Identity.Title.Langs["en"] != "New Title" {
		t.Fatalf("title.en not updated: %+v", out.Identity.Title)
	}
}

func TestSetListItemByIndex(t *testing.T) {
	doc := fixture()
	p, _ := Parse("keywords[1]")
	out, err := Set(doc, p, "go")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out.Keywords, []string{testRust, "go", testContainer}) {
		t.Fatalf("got %v, want [rust go container]", out.Keywords)
	}
}

func TestSetSelectorField(t *testing.T) {
	doc := fixture()
	p, _ := Parse("repositories[role=origin].branch")
	out, err := Set(doc, p, "release")
	if err != nil {
		t.Fatal(err)
	}
	var origin *projectfile.Repository
	for i := range out.Repositories {
		if out.Repositories[i].Role == testOrigin {
			origin = &out.Repositories[i]
			break
		}
	}
	if origin == nil || origin.Branch != "release" {
		t.Fatalf("origin branch not updated: %+v", origin)
	}
}

func TestSetExtensionFlatKey(t *testing.T) {
	doc := fixture()
	p, _ := Parse("ext.com.example.build.user")
	out, err := Set(doc, p, testAlpine)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := projectfile.LookupExtension(out, "com.example.build")
	if !ok {
		t.Fatalf("extension absent after set")
	}
	m, _ := got.(map[string]any)
	if m["user"] != testAlpine {
		t.Fatalf("got %v, want alpine", m["user"])
	}
}

func TestSetCreatesIntermediateMaps(t *testing.T) {
	doc := &projectfile.Document{Identity: projectfile.Identity{Namespace: "org.x", Name: "y"}}
	p, _ := Parse("ext.acme.things.color")
	out, err := Set(doc, p, "blue")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := projectfile.LookupExtension(out, "acme.things")
	if !ok {
		t.Fatalf("extension absent after set; extensions=%v", out.Extensions)
	}
	m, _ := got.(map[string]any)
	if m["color"] != "blue" {
		t.Fatalf("got %v, want blue", m["color"])
	}
}

func TestAddString(t *testing.T) {
	doc := fixture()
	p, _ := Parse("keywords")
	out, added, err := Add(doc, p, "go", false)
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Fatalf("expected added=true")
	}
	if !reflect.DeepEqual(out.Keywords, []string{testRust, testWasm, testContainer, "go"}) {
		t.Fatalf("got %v", out.Keywords)
	}
}

func TestAddStringDedup(t *testing.T) {
	doc := fixture()
	p, _ := Parse("keywords")
	out, added, err := Add(doc, p, testRust, false)
	if err != nil {
		t.Fatal(err)
	}
	if added {
		t.Fatalf("expected added=false (duplicate)")
	}
	if !reflect.DeepEqual(out.Keywords, []string{testRust, testWasm, testContainer}) {
		t.Fatalf("list mutated despite dedup: %v", out.Keywords)
	}
}

func TestAddRepositoryDedup(t *testing.T) {
	doc := fixture()
	p, _ := Parse("repositories")
	// Same URL as the existing origin — Add should be a no-op.
	dup := map[string]any{testURL: "https://example.com/repo.git", testRole: "extra"}
	_, added, err := Add(doc, p, dup, false)
	if err != nil {
		t.Fatal(err)
	}
	if added {
		t.Fatalf("expected dedup, got added=true")
	}
}

func TestAddRepositoryNew(t *testing.T) {
	doc := fixture()
	p, _ := Parse("repositories")
	newRepo := map[string]any{testURL: "https://archive.example.com/repo.git", testRole: "archive"}
	out, added, err := Add(doc, p, newRepo, false)
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Fatalf("expected added=true")
	}
	if len(out.Repositories) != 3 {
		t.Fatalf("got %d repos, want 3", len(out.Repositories))
	}
}

func TestDeleteKey(t *testing.T) {
	doc := fixture()
	p, _ := Parse("identity.title")
	out, existed, err := Delete(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	if !existed {
		t.Fatalf("expected existed=true")
	}
	if out.Identity.Title != nil {
		t.Fatalf("title still present: %+v", out.Identity.Title)
	}
}

func TestDeleteListIndex(t *testing.T) {
	doc := fixture()
	p, _ := Parse("keywords[1]")
	out, existed, err := Delete(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	if !existed {
		t.Fatalf("expected existed=true")
	}
	if !reflect.DeepEqual(out.Keywords, []string{testRust, testContainer}) {
		t.Fatalf("got %v, want [rust container]", out.Keywords)
	}
}

func TestDeleteListSelector(t *testing.T) {
	doc := fixture()
	p, _ := Parse("repositories[role=mirror]")
	out, existed, err := Delete(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	if !existed {
		t.Fatalf("expected existed=true")
	}
	if len(out.Repositories) != 1 || out.Repositories[0].Role != testOrigin {
		t.Fatalf("got %+v, want only origin repo", out.Repositories)
	}
}

func TestDeleteAbsent(t *testing.T) {
	doc := fixture()
	p, _ := Parse("identity.version")
	_, existed, err := Delete(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	if existed {
		t.Fatalf("expected existed=false")
	}
}

func TestDeleteExtensionLeaf(t *testing.T) {
	doc := fixture()
	p, _ := Parse("ext.com.example.build.labels.experimental")
	out, existed, err := Delete(doc, p)
	if err != nil {
		t.Fatal(err)
	}
	if !existed {
		t.Fatalf("expected existed=true")
	}
	got, ok := projectfile.LookupExtension(out, "com.example.build")
	if !ok {
		t.Fatalf("build namespace gone")
	}
	m, _ := got.(map[string]any)
	if labels, ok := m["labels"].(map[string]any); ok {
		if _, has := labels["experimental"]; has {
			t.Fatalf("experimental still present: %v", labels)
		}
	}
}
