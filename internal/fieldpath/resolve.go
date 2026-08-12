// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fieldpath

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

const (
	keyKeys     = "keys"
	keyValues   = "values"
	keyPriority = "priority"
)

// PriorityDefault is the rank a map entry that declares no `priority` sorts at.
// It sits above the "pin to the front" values (10, 0) and below the "pin behind
// the defaults" values (100, 200), so either end of the scale has room. This is
// the one home for the number: the bridge module's pfmodel.PriorityDefault
// documents the same value for the sorts it owns and must not diverge.
const PriorityDefault = 50

// ErrNotFound is returned when a Path resolves to an absent value. The
// CLI maps it to exit code 1 when no --default / --or-default was given.
var ErrNotFound = errors.New("path not found")

// ErrListOpOnMap is returned when a LIST operator — `[N]` index, `[k=v]`
// selector, or `[]` projection — is applied to a value that turns out to be a
// MAP of named keys (e.g. org.projectfile.artifacts). This is a GRAMMAR
// mismatch, not an absent value: the map has no list to index/select/project
// over, so the caller almost certainly wants a direct key (`.<name>`) or the
// `{}` map-projection form. Kept DISTINCT from ErrNotFound — and NOT wrapping
// it — so a reader refuses it LOUDLY (a usage error a user must fix) instead of
// laundering it into a soft "path absent" miss that reads as a typo. This is the
// make-plane twin of the ci-resolver interpolator's structural refusal of a
// `[`-bearing reference (internal/ci/interp.go): one neutral rule, two engine
// spellings.
var ErrListOpOnMap = errors.New("list operator on a map of named keys")

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
	case SegMapSelector:
		return walkMapSelector(cur, seg, segs[1:])
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
		return Result{}, listOpMiss(cur, fmt.Sprintf("index [%d]", seg.Index))
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
		return Result{}, listOpMiss(cur, "selector "+selectorLabel(seg.Preds))
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
		return Result{}, listOpMiss(cur, "projection []")
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

// EntryPriority reads an entry's `priority`, defaulting when the entry is not a
// map or declares none. A map of named keys carries no order of its own, so this
// is the only channel through which a document can state one.
//
// Exported inside core so a consumer listing the same entries in Go reads
// "unset" through this function rather than through a second copy of the rule —
// a list that disagreed with the resolver's `{}` order would present the entries
// in one order and act on them in another.
func EntryPriority(v any) int {
	entry, ok := v.(map[string]any)
	if !ok {
		return PriorityDefault
	}
	switch n := entry[keyPriority].(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return PriorityDefault
}

// fanOutKeys orders a map's keys for fan-out: `priority` DESCENDING first, then
// the key ascending to break ties. Sorting by key alone is deterministic but
// arbitrary — it makes `kiota` precede `ghcr` on spelling, so a README would
// recommend the fallback registry before the one the project prefers. Priority
// is what lets the document say which entry leads; the key tiebreak is what
// keeps two entries of equal rank from swapping between runs.
func fanOutKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		pi, pj := EntryPriority(m[keys[i]]), EntryPriority(m[keys[j]])
		if pi != pj {
			return pi > pj
		}
		return keys[i] < keys[j]
	})
	return keys
}

// walkMapProject fans out a map's entries as (key, value) Pair entries in
// the Result. Iteration order is priority-descending then key-ascending (see
// fanOutKeys), so a document that ranks its entries is honoured and one that
// does not still renders identically on every run. Allowed trailers are
// `keys` and `values` only (e.g. `env{}.keys`),
// which strip one half of each pair and downgrade the result to a normal
// list. Anything else past `{}` is a usage error: maps fan out to pairs,
// not back into a single value, so further navigation has nowhere to land.
func walkMapProject(cur any, rest []Segment) (Result, error) {
	m, ok := cur.(map[string]any)
	if !ok {
		return Result{}, fmt.Errorf("%w: expected map for projection {}, got %T", ErrNotFound, cur)
	}
	keys := fanOutKeys(m)

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

// walkMapSelector resolves map{k=v,...} — the entries of a map of NAMED keys
// whose fields match every predicate — and applies the remaining segments to
// each, flattening into one value slice. `org.projectfile.artifacts{kind=image}.ref`
// is the motivating address: pick every artifact a project publishes as a
// container image, read each one's pull reference.
//
// It fans out where the LIST selector `[k=v]` returns only the first match, and
// that asymmetry is deliberate rather than an oversight: a list is ordered, so
// "the first item matching" is a value the author can reason about, while a map
// of named keys has no order of its own. Returning "the first" from such a
// container would hand back an arbitrary entry and call it a selection. Fanning
// out is the only honest reading, and it makes `{k=v}` the map twin of `[]`
// projection with a filter rather than of `[k=v]`.
//
// The ORDER of the fan-out comes from `priority` on each entry (see fanOutKeys),
// which is how a document states a preference a map cannot express positionally.
//
// Entries whose value is not a map are skipped (a scalar has no field to match),
// as are matches where the remainder does not resolve — same tolerance as
// walkProject, so an optional field on one of several matches does not fail the
// whole address.
func walkMapSelector(cur any, seg Segment, rest []Segment) (Result, error) {
	m, ok := cur.(map[string]any)
	if !ok {
		return Result{}, fmt.Errorf("%w: expected map for selector %s, got %T",
			ErrNotFound, mapSelectorLabel(seg.Preds), cur)
	}
	keys := fanOutKeys(m)

	out := Result{IsList: true}
	for _, k := range keys {
		entry, ok := m[k].(map[string]any)
		if !ok || !matchPredicates(entry, seg.Preds) {
			continue
		}
		if len(rest) == 0 {
			out.Values = append(out.Values, entry)
			continue
		}
		sub, err := walk(entry, rest)
		if err != nil {
			continue
		}
		out.Values = append(out.Values, sub.Values...)
	}
	if len(out.Values) == 0 {
		return Result{}, fmt.Errorf("%w: no map entry matches %s",
			ErrNotFound, mapSelectorLabel(seg.Preds))
	}
	return out, nil
}

// selectorLabel renders a selector's predicates back into the `[k=v,...]`
// grammar the user typed, so an error message echoes their input rather than
// Go's struct formatting. Mirrors Path.String's selector arm.
func selectorLabel(preds []Predicate) string {
	return "[" + PredicateBody(preds) + "]"
}

// mapSelectorLabel is selectorLabel's map twin — the `{k=v,...}` spelling, so
// an error about a map selector never echoes the address in list brackets.
func mapSelectorLabel(preds []Predicate) string {
	return "{" + PredicateBody(preds) + "}"
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

// listOpMiss classifies an asList failure for a list operator (index /
// selector / projection), given a human-readable op label. A MAP under a list
// operator is a GRAMMAR mismatch — org.projectfile.artifacts is a map of named
// keys, so `artifacts[kind=binary]` has no list to select over — and is refused
// LOUDLY via the distinct ErrListOpOnMap (readers propagate it), pointing the
// user at the curly-brace twin of whatever they wrote. Any other non-list (a
// scalar leaf, an absent hop) stays an ErrNotFound so --default / --or-default
// and the soft exit still apply. Keeps the "what went wrong" split in ONE place
// so every list-operator walker classifies identically.
func listOpMiss(cur any, op string) error {
	if _, isMap := cur.(map[string]any); isMap {
		return fmt.Errorf("%w: %s cannot address a map of named keys — index a "+
			"key directly (e.g. `.<name>`), select entries with `{k=v}`, or fan "+
			"the whole map out with `{}`", ErrListOpOnMap, op)
	}
	return fmt.Errorf("%w: expected list for %s, got %T", ErrNotFound, op, cur)
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
