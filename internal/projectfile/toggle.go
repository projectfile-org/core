// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

// Toggle is the parsed shape of a field that may be either a bool
// (all-or-nothing) or a map[string]bool (per-key allowlist). It backs the
// org.projectfile.contributing toggles recommend-to-star and
// recommend-to-follow, which share one grammar:
//
//	# bool — every candidate is included (true) or none is (false)
//	recommend-to-star: true
//
//	# map — only keys mapped to true are included; absent and false keys are
//	# dropped. This is an allowlist ("showing only specified").
//	recommend-to-star:
//	  github.com: true
//	  kiota.ch: false
//	  codeberg.org: true
//
// The zero Toggle is "unset": Allow falls back to the caller-supplied
// default, so each consumer picks its own resting state. Today every
// consumer uses default false (nothing recommended unless the field is set),
// but the default is per-call so a future consumer could choose otherwise.
type Toggle struct {
	mode  toggleMode
	allow map[string]bool
}

type toggleMode uint8

const (
	toggleUnset     toggleMode = iota // no value provided — caller default applies
	toggleAll                         // bool true  — every candidate passes
	toggleNone                        // bool false — no candidate passes
	toggleAllowlist                   // map        — only allow[key]==true passes
)

// ParseToggle interprets a raw extension value as a Toggle. Accepted shapes:
//
//   - bool          -> toggleAll (true) or toggleNone (false)
//   - map[string]any / map[string]bool -> toggleAllowlist (empty map -> unset,
//     so an explicit `{}` is treated as "left to default", not "allow none";
//     use `false` to disable everything)
//
// Any other shape (nil, string, number) yields the zero Toggle (unset) so a
// malformed value degrades to the consumer default rather than panicking.
func ParseToggle(raw any) Toggle {
	switch v := raw.(type) {
	case bool:
		if v {
			return Toggle{mode: toggleAll}
		}
		return Toggle{mode: toggleNone}
	case map[string]bool:
		if len(v) == 0 {
			return Toggle{}
		}
		return Toggle{mode: toggleAllowlist, allow: v}
	case map[string]any:
		if len(v) == 0 {
			return Toggle{}
		}
		allow := make(map[string]bool, len(v))
		for k, val := range v {
			allow[k] = boolish(val)
		}
		return Toggle{mode: toggleAllowlist, allow: allow}
	}
	return Toggle{}
}

// Allow reports whether key is permitted by this Toggle. The caller supplies
// defaultAllow, which is honoured only when the Toggle is unset — letting each
// consumer own its resting state. In allowlist mode absent keys and explicit
// false keys both fail; only true keys pass.
func (t Toggle) Allow(key string, defaultAllow bool) bool {
	switch t.mode {
	case toggleAll:
		return true
	case toggleNone:
		return false
	case toggleAllowlist:
		return t.allow[key]
	default:
		return defaultAllow
	}
}

// Set reports whether a concrete value was provided (bool or non-empty map).
// When false the consumer default governs via Allow.
func (t Toggle) Set() bool {
	return t.mode != toggleUnset
}

// Enabled reports whether the toggle is in an opt-in state (bool true or a
// non-empty allowlist map). Unset and explicit-false both return false. Use
// this to gate whole blocks whose mere presence means "the feature is on",
// independent of how many candidates survive the per-key filter.
func (t Toggle) Enabled() bool {
	return t.mode == toggleAll || t.mode == toggleAllowlist
}

// Mode exposes the parsed shape for decision tracing. Not used for decisions.
func (t Toggle) Mode() string {
	switch t.mode {
	case toggleAll:
		return "all"
	case toggleNone:
		return "none"
	case toggleAllowlist:
		return "allowlist"
	default:
		return "unset"
	}
}

func boolish(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true"
	}
	return false
}
