// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fieldpath

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"kiota.ch/projectfile/core/internal/projectfile"
)

const (
	keyKeys   = "keys"
	keyValues = "values"
)

// ErrNotFound is returned when a Path resolves to an absent value. The
// CLI maps it to exit code 1 when no --default / --or-default was given.
var ErrNotFound = errors.New("path not found")

// Result is the typed outcome of resolving a Path. IsList is set for
// projections (`list[].field`) and whole-list addresses (`keywords`).
// IsPairs is set for map projections (`env{}`) and carries the ordered
// (key, value) pairs as alternating entries in Values; the formatter
// chooses KEY=VALUE / export KEY=VALUE / {KEY:VALUE} accordingly.
// Single-value addresses produce both flags false with a one-element
// Values slice — callers don't have to special-case the singleton.
type Result struct {
	Values  []any
	IsList  bool
	IsPairs bool
}

// Pair is one (key, value) entry of a map-projection Result. Carrying
// the typed value (not a string) lets downstream formatters render it
// in the encoding appropriate to their wire format.
type Pair struct {
	Key   string
	Value any
}

// Single is the convenience accessor for callers that expect exactly one
// value. Returns (nil, false) when the Result is empty.
func (r Result) Single() (any, bool) {
	if len(r.Values) == 0 {
		return nil, false
	}
	return r.Values[0], true
}

// Resolve walks p through doc.ToMap() and returns the addressed value(s).
// ToMap() canonicalises the document — typed fields and Extensions all
// live under one map keyed identically to the on-disk form, so a single
// walker handles both surfaces without branching on "extension vs. typed".
func Resolve(doc *projectfile.Document, p Path) (Result, error) {
	if doc == nil {
		return Result{}, fmt.Errorf("fieldpath: nil document")
	}
	root := doc.ToMap()
	return walk(root, p.Segments)
}

// Exists is the membership-test variant of Resolve — true when the path
// resolves to any value (including an empty list/map). Used by `get --exists`.
func Exists(doc *projectfile.Document, p Path) bool {
	res, err := Resolve(doc, p)
	if err != nil {
		return false
	}
	return len(res.Values) > 0
}

// walk is the recursive resolver. The function shape mirrors the segment
// kinds: SegKey consumes a contiguous run of key segments via longest-
// prefix flat-key matching (so extension keys like "org.projectfile.cli"
// match whether the parser left them as a dotted-table nest or a flat
// quoted key); SegIndex/SegSelector pick into a list; SegProject fans out.
func walk(cur any, segs []Segment) (Result, error) {
	if len(segs) == 0 {
		return Result{Values: []any{cur}}, nil
	}
	seg := segs[0]
	switch seg.Kind {
	case SegKey:
		return walkKey(cur, segs)
	case SegIndex:
		return walkIndex(cur, seg, segs[1:])
	case SegSelector:
		return walkSelector(cur, seg, segs[1:])
	case SegProject:
		return walkProject(cur, segs[1:])
	case SegMapProject:
		return walkMapProject(cur, segs[1:])
	}
	return Result{}, fmt.Errorf("fieldpath: unknown segment kind %d", seg.Kind)
}

// walkKey collects every contiguous SegKey at the head of segs and tries
// progressively shorter dot-joined prefixes against cur (which must be a
// map). The longest match wins — this is what makes "ext.com.example.build.user"
// resolve identically whether the producer wrote it as a flat key or as a TOML
// dotted table (the latter explodes into nested maps the walker recurses
// into segment-by-segment via the shorter-prefix fallthroughs).
func walkKey(cur any, segs []Segment) (Result, error) {
	m, ok := cur.(map[string]any)
	if !ok {
		return Result{}, fmt.Errorf("%w: expected map at %q, got %T", ErrNotFound, segs[0].Key, cur)
	}
	// Determine the run of consecutive SegKey segments.
	end := 1
	for end < len(segs) && segs[end].Kind == SegKey {
		end++
	}
	keys := make([]string, end)
	for i := range end {
		keys[i] = segs[i].Key
	}
	for n := end; n >= 1; n-- {
		key := strings.Join(keys[:n], ".")
		if v, ok := m[key]; ok {
			return walk(v, segs[n:])
		}
	}
	return Result{}, fmt.Errorf("%w: key %q absent", ErrNotFound, keys[0])
}

// walkIndex resolves list[N]; negative N counts from the end so list[-1]
// addresses the last element.
func walkIndex(cur any, seg Segment, rest []Segment) (Result, error) {
	list, ok := asList(cur)
	if !ok {
		return Result{}, fmt.Errorf("%w: expected list for index [%d], got %T", ErrNotFound, seg.Index, cur)
	}
	idx := seg.Index
	if idx < 0 {
		idx += len(list)
	}
	if idx < 0 || idx >= len(list) {
		return Result{}, fmt.Errorf("%w: index %d out of range [0, %d)", ErrNotFound, seg.Index, len(list))
	}
	return walk(list[idx], rest)
}

// walkSelector resolves list[k=v,...] by linear scan over the list and
// returning the first item whose every predicate matches. The scan is
// deliberately not configurable (no "all", "last") — selector semantics
// match what derive/fields.go already implements so the existing
// links[type=X] paths keep their meaning.
func walkSelector(cur any, seg Segment, rest []Segment) (Result, error) {
	list, ok := asList(cur)
	if !ok {
		return Result{}, fmt.Errorf("%w: expected list for selector %v, got %T", ErrNotFound, seg.Preds, cur)
	}
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if matchPredicates(m, seg.Preds) {
			return walk(item, rest)
		}
	}
	return Result{}, fmt.Errorf("%w: no list item matches %v", ErrNotFound, seg.Preds)
}

// walkProject applies the remaining segments to each item in the list and
// flattens the results into a single value slice. Items where the
// remainder doesn't resolve are silently skipped — the alternative
// (failing the whole projection on the first miss) breaks the common case
// of optional fields like `repositories[].branch` where some entries lack
// the field.
func walkProject(cur any, rest []Segment) (Result, error) {
	list, ok := asList(cur)
	if !ok {
		return Result{}, fmt.Errorf("%w: expected list for projection, got %T", ErrNotFound, cur)
	}
	out := Result{IsList: true}
	for _, item := range list {
		if len(rest) == 0 {
			out.Values = append(out.Values, item)
			continue
		}
		sub, err := walk(item, rest)
		if err != nil {
			continue
		}
		out.Values = append(out.Values, sub.Values...)
	}
	if len(out.Values) == 0 {
		return Result{}, fmt.Errorf("%w: projection produced no items", ErrNotFound)
	}
	return out, nil
}

// walkMapProject fans out a map's entries as (key, value) Pair entries in
// the Result. Iteration order is sorted-by-key — deterministic, and the
// best we can do until walk() carries a rawdoc reference for true source
// order. Allowed trailers are `keys` and `values` only (e.g. `env{}.keys`),
// which strip one half of each pair and downgrade the result to a normal
// list. Anything else past `{}` is a usage error: maps fan out to pairs,
// not back into a single value, so further navigation has nowhere to land.
func walkMapProject(cur any, rest []Segment) (Result, error) {
	m, ok := cur.(map[string]any)
	if !ok {
		return Result{}, fmt.Errorf("%w: expected map for projection {}, got %T", ErrNotFound, cur)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(rest) == 0 {
		out := Result{IsPairs: true, Values: make([]any, 0, len(keys))}
		for _, k := range keys {
			out.Values = append(out.Values, Pair{Key: k, Value: m[k]})
		}
		return out, nil
	}

	// Only `keys` / `values` are legal trailers. Reject everything else
	// up-front so the error message points at the addressing mistake rather
	// than at a misleading "key not found" deep inside the value.
	if len(rest) > 1 || rest[0].Kind != SegKey || (rest[0].Key != keyKeys && rest[0].Key != keyValues) {
		return Result{}, fmt.Errorf("fieldpath: only `keys` or `values` may follow {}, got %v", rest)
	}
	out := Result{IsList: true, Values: make([]any, 0, len(keys))}
	wantKeys := rest[0].Key == keyKeys
	for _, k := range keys {
		if wantKeys {
			out.Values = append(out.Values, k)
		} else {
			out.Values = append(out.Values, m[k])
		}
	}
	return out, nil
}

// matchPredicates returns true when every predicate's key resolves on m
// to a string that equals the predicate value. Non-string leaf values are
// stringified via fmt.Sprintf so a boolean `issues=true` predicate works
// against an `issues: true` field. Comparison is case-sensitive throughout.
func matchPredicates(m map[string]any, preds []Predicate) bool {
	for _, pr := range preds {
		v, ok := m[pr.Key]
		if !ok {
			return false
		}
		var s string
		switch x := v.(type) {
		case string:
			s = x
		default:
			s = fmt.Sprintf("%v", x)
		}
		if s != pr.Value {
			return false
		}
	}
	return true
}

// asList normalises the several list shapes the projectfile parsers
// produce — []any from raw JSON/YAML/TOML parses, []map[string]any from
// serialize.go's helpers, and the typed []string-style slices ToMap also
// emits — into a unified []any the walker iterates uniformly.
func asList(v any) ([]any, bool) {
	switch x := v.(type) {
	case []any:
		return x, true
	case []map[string]any:
		out := make([]any, len(x))
		for i, m := range x {
			out[i] = m
		}
		return out, true
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out, true
	}
	return nil, false
}

// FormatScalar renders one resolved value to the wire format raw output
// expects: bare strings come out unquoted, scalars use %v, maps/lists
// fall through to a stable JSON line. Used by cmd/get.go.
func FormatScalar(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool, int, int32, int64, float32, float64:
		return fmt.Sprintf("%v", x)
	case []any:
		return formatList(x)
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return formatList(out)
	case map[string]any:
		return formatMap(x)
	}
	return fmt.Sprintf("%v", v)
}

func formatList(xs []any) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = FormatScalar(x)
	}
	return strings.Join(parts, " ")
}

// formatMap renders a map as key-sorted "k=v" lines joined by newlines —
// matches what the plan's raw-output spec describes for map leaves.
func formatMap(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s=%s", k, FormatScalar(m[k]))
	}
	return b.String()
}
