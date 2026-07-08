// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import "testing"

const testCher = "Cher"

func TestSplitGitName(t *testing.T) {
	cases := []struct {
		in, wantFamily, wantGiven string
	}{
		{"", "", ""},
		{"  ", "", ""},
		{testCher, testCher, ""},
		{"Alice Example", "Example", "Alice"},
		{"Alice Middle Example", "Example", "Alice Middle"},
		{"García Márquez, Gabriel", "García Márquez", "Gabriel"},
		{"Búho, Damián", "Búho", "Damián"},
		{"  Trailing Whitespace  ", "Whitespace", "Trailing"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			f, g := SplitGitName(c.in)
			if f != c.wantFamily || g != c.wantGiven {
				t.Errorf("SplitGitName(%q) = (%q, %q); want (%q, %q)", c.in, f, g, c.wantFamily, c.wantGiven)
			}
		})
	}
}

func TestAmbiguousGitName(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{testCher, true},
		{"Alice Example", false},
		{"Búho, Damián", false},
	}
	for _, c := range cases {
		if got := AmbiguousGitName(c.in); got != c.want {
			t.Errorf("AmbiguousGitName(%q)=%v want %v", c.in, got, c.want)
		}
	}
}
