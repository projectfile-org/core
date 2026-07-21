// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile_test

import (
	"path/filepath"
	"testing"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// fixture reads the testdata document through the public facade.
func fixture(t *testing.T) *projectfile.Document {
	t.Helper()
	path := filepath.Join("testdata", "projectfile.yaml")
	doc, err := projectfile.Read(path)
	if err != nil {
		t.Fatalf("Read(%q) failed: %v", path, err)
	}
	return doc
}

func TestStack(t *testing.T) {
	got := projectfile.Stack(fixture(t))
	want := []string{"docker", "go"}
	if len(got) != len(want) {
		t.Fatalf("Stack length = %d, want %d (got %v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Stack[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStackNilDocument(t *testing.T) {
	if got := projectfile.Stack(nil); got != nil {
		t.Errorf("Stack(nil) = %v, want nil", got)
	}
}

func TestExtensionProjectfileCI(t *testing.T) {
	sub, ok := projectfile.Extension(fixture(t), "org.projectfile.ci")
	if !ok {
		t.Fatal("org.projectfile.ci not found")
	}
	if _, has := sub["lint"]; !has {
		t.Errorf("org.projectfile.ci missing lint key, got %v", sub)
	}
}

func TestExtensionExampleCI(t *testing.T) {
	sub, ok := projectfile.Extension(fixture(t), "com.example.ci")
	if !ok {
		t.Fatal("com.example.ci not found")
	}
	if sub["runner"] != "linux-amd64" {
		t.Errorf("com.example.ci runner = %v, want linux-amd64", sub["runner"])
	}
}

func TestExtensionAbsent(t *testing.T) {
	if _, ok := projectfile.Extension(fixture(t), "com.never-heard-of.this"); ok {
		t.Error("absent namespace reported present")
	}
}
