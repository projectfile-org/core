// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fieldpath

import (
	"fmt"
	"strings"

	"kiota.ch/projectfile/core/internal/projectfile"
)

// Set replaces the value at p with value, creating any missing
// intermediate maps along the way. Existing flat-key extension entries
// (e.g. doc.Extensions["com.example.build"]) are honoured via the same
// longest-prefix-match the reader uses, so a Set never duplicates an
// extension under two encodings.
//
// Returns the mutated Document — the input is not modified. Callers that
// already cloned (dry-run) can ignore the return distinction; callers that
// will Write() should always use the returned Document.
func Set(doc *projectfile.Document, p Path, value any) (*projectfile.Document, error) {
	if doc == nil {
		return nil, fmt.Errorf("fieldpath: nil document")
	}
	if len(p.Segments) == 0 {
		return nil, fmt.Errorf("fieldpath: empty path")
	}
	root := doc.ToMap()
	if err := setIn(root, p.Segments, value); err != nil {
		return nil, err
	}
	return projectfile.FromMap(root), nil
}

// Add appends value to the list addressed by p, deduplicating by the
// registered IdentityFunc for the path (unless allowDuplicate is true).
// Returns added=true when the value was actually appended, false when a
// duplicate was detected — both are success outcomes, the bool exists so
// the caller can report "added" vs. "skipped" without re-resolving.
func Add(doc *projectfile.Document, p Path, value any, allowDuplicate bool) (*projectfile.Document, bool, error) {
	if doc == nil {
		return nil, false, fmt.Errorf("fieldpath: nil document")
	}
	if len(p.Segments) == 0 {
		return nil, false, fmt.Errorf("fieldpath: empty path")
	}
	root := doc.ToMap()
	added, err := addIn(root, p.Segments, value, allowDuplicate, dottedKeyPath(p.Segments))
	if err != nil {
		return nil, false, err
	}
	return projectfile.FromMap(root), added, nil
}

// Delete removes the value at p. Returns existed=true when the address
// resolved to something (and was removed); false when the path was absent.
// Caller decides whether absent → error (strict mode) or success (default).
//
// List-element deletions (`keywords[2]`, `repositories[role=origin]`) are
// routed through the parent-rewriter because Go slices need a re-slice
// AND a back-write into the holder map. Field-deletions on selector-
// addressed items (`repositories[role=origin].url`) stay on the normal
// walker — the leaf operates on a map and never reshapes a list.
func Delete(doc *projectfile.Document, p Path) (*projectfile.Document, bool, error) {
	if doc == nil {
		return nil, false, fmt.Errorf("fieldpath: nil document")
	}
	if len(p.Segments) == 0 {
		return nil, false, fmt.Errorf("fieldpath: empty path")
	}
	root := doc.ToMap()
	segs := p.Segments
	last := segs[len(segs)-1]
	var (
		existed bool
		err     error
	)
	if last.Kind == SegIndex || last.Kind == SegSelector {
		existed, err = deleteListEntry(root, segs[:len(segs)-1], last)
	} else {
		existed, err = deleteIn(root, segs)
	}
	if err != nil {
		return nil, false, err
	}
	return projectfile.FromMap(root), existed, nil
}

// dottedKeyPath joins the leading SegKey run into the dotted address used
// to look up identity rules (e.g. "repositories", "requirements.operating-system"). Stops
// at the first non-key segment so `repositories[role=origin]` doesn't get
// misregistered under "repositories.role=origin".
func dottedKeyPath(segs []Segment) string {
	parts := []string{}
	for _, s := range segs {
		if s.Kind != SegKey {
			break
		}
		parts = append(parts, s.Key)
	}
	return strings.Join(parts, ".")
}

// setIn implements Set. The walker descends through intermediate segments
// (creating maps as needed) and dispatches the final write to setLeaf.
func setIn(parent any, segs []Segment, value any) error {
	if len(segs) == 1 {
		return setLeaf(parent, segs[0], value)
	}
	seg := segs[0]
	switch seg.Kind {
	case SegKey:
		m, ok := parent.(map[string]any)
		if !ok {
			return fmt.Errorf("fieldpath: expected map at %q, got %T", seg.Key, parent)
		}
		// Longest-prefix flat key match — preserves existing extension
		// encoding (flat vs. nested) on edits.
		end := keyRunLength(segs)
		for n := end; n >= 1; n-- {
			key := joinKeys(segs, n)
			if v, ok := m[key]; ok {
				// Typed lists from ToMap ([]string, []map[string]any) must
				// be normalised to []any in the parent map BEFORE we recurse,
				// otherwise element-mutation in setLeaf operates on a fresh
				// copy and never reaches the document.
				if normalized, isList := normalizeList(v); isList {
					m[key] = normalized
					v = normalized
				}
				return setIn(v, segs[n:], value)
			}
		}
		// No existing prefix; create the immediate child and descend a single segment.
		child := getOrCreateChild(m, seg.Key, segs[1])
		return setIn(child, segs[1:], value)
	case SegIndex:
		list, ok := asListRef(parent)
		if !ok {
			return fmt.Errorf("fieldpath: expected list for index [%d], got %T", seg.Index, parent)
		}
		idx := normalizeIndex(seg.Index, len(list))
		if idx < 0 || idx >= len(list) {
			return fmt.Errorf("fieldpath: index %d out of range [0, %d)", seg.Index, len(list))
		}
		return setIn(list[idx], segs[1:], value)
	case SegSelector:
		list, ok := asListRef(parent)
		if !ok {
			return fmt.Errorf("fieldpath: expected list for selector, got %T", parent)
		}
		for _, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if matchPredicates(m, seg.Preds) {
				return setIn(item, segs[1:], value)
			}
		}
		return fmt.Errorf("fieldpath: no list item matches %v", seg.Preds)
	case SegProject:
		return fmt.Errorf("fieldpath: set with projection not supported")
	}
	return fmt.Errorf("fieldpath: unknown segment kind %d", seg.Kind)
}

// setLeaf writes value into the addressed slot. For SegIndex/SegSelector
// leaves the parent is the LIST containing the slot (one level higher
// than the intermediate-walk would land us), so the caller must hand us
// the list itself — which it does because setIn detects len==1 and calls
// us with the list value.
func setLeaf(parent any, seg Segment, value any) error {
	switch seg.Kind {
	case SegKey:
		m, ok := parent.(map[string]any)
		if !ok {
			return fmt.Errorf("fieldpath: expected map for key %q, got %T", seg.Key, parent)
		}
		m[seg.Key] = value
		return nil
	case SegIndex:
		list, ok := asListRef(parent)
		if !ok {
			return fmt.Errorf("fieldpath: expected list for index [%d], got %T", seg.Index, parent)
		}
		idx := normalizeIndex(seg.Index, len(list))
		if idx < 0 || idx >= len(list) {
			return fmt.Errorf("fieldpath: index %d out of range [0, %d)", seg.Index, len(list))
		}
		list[idx] = value
		return nil
	case SegSelector:
		list, ok := asListRef(parent)
		if !ok {
			return fmt.Errorf("fieldpath: expected list for selector, got %T", parent)
		}
		for i, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if matchPredicates(m, seg.Preds) {
				list[i] = value
				return nil
			}
		}
		return fmt.Errorf("fieldpath: no list item matches %v", seg.Preds)
	case SegProject:
		return fmt.Errorf("fieldpath: set with projection not supported")
	}
	return fmt.Errorf("fieldpath: unknown segment kind %d", seg.Kind)
}

// addIn implements Add. Walks to the list slot and appends, deduping by
// the registered IdentityFunc unless allowDuplicate is true. dottedKey is
// the address used to look up the identity rule — see dottedKeyPath.
func addIn(parent any, segs []Segment, value any, allowDuplicate bool, dottedKey string) (bool, error) {
	if len(segs) == 0 {
		return false, fmt.Errorf("fieldpath: empty segments")
	}
	if len(segs) == 1 {
		seg := segs[0]
		if seg.Kind != SegKey {
			return false, fmt.Errorf("fieldpath: add target must be a key segment, got %v", seg.Kind)
		}
		m, ok := parent.(map[string]any)
		if !ok {
			return false, fmt.Errorf("fieldpath: expected map at %q, got %T", seg.Key, parent)
		}
		// ToMap() may emit lists as []string or []map[string]any (typed shapes
		// from serialize.go) rather than []any. asListRef normalises both so
		// the append/dedup loop below sees a uniform shape.
		var existing []any
		if v, ok := m[seg.Key]; ok {
			if list, lok := asListRef(v); lok {
				existing = list
			} else {
				return false, fmt.Errorf("fieldpath: %q exists but is not a list (%T)", seg.Key, v)
			}
		}
		// Identity dedup runs only when the path has a registered rule.
		// Without a rule we always append — keywords/string-list paths all
		// have rules, so the only "no rule" cases are user-invented paths.
		if !allowDuplicate {
			if id := IdentityFor(dottedKey); id != nil {
				newKey, ok := id(value)
				if ok {
					for _, item := range existing {
						k, okk := id(item)
						if okk && k == newKey {
							// Already present — Add is idempotent.
							return false, nil
						}
					}
				}
			}
		}
		m[seg.Key] = append(existing, value)
		return true, nil
	}
	// Intermediate walk mirrors setIn for the SegKey / SegSelector / SegIndex
	// cases; SegProject is not meaningful for add.
	seg := segs[0]
	switch seg.Kind {
	case SegKey:
		m, ok := parent.(map[string]any)
		if !ok {
			return false, fmt.Errorf("fieldpath: expected map at %q, got %T", seg.Key, parent)
		}
		end := keyRunLength(segs)
		for n := end; n >= 1; n-- {
			key := joinKeys(segs, n)
			if v, ok := m[key]; ok {
				if normalized, isList := normalizeList(v); isList {
					m[key] = normalized
					v = normalized
				}
				return addIn(v, segs[n:], value, allowDuplicate, dottedKey)
			}
		}
		child := getOrCreateChild(m, seg.Key, segs[1])
		return addIn(child, segs[1:], value, allowDuplicate, dottedKey)
	case SegIndex:
		list, ok := asListRef(parent)
		if !ok {
			return false, fmt.Errorf("fieldpath: expected list at index [%d], got %T", seg.Index, parent)
		}
		idx := normalizeIndex(seg.Index, len(list))
		if idx < 0 || idx >= len(list) {
			return false, fmt.Errorf("fieldpath: index %d out of range [0, %d)", seg.Index, len(list))
		}
		return addIn(list[idx], segs[1:], value, allowDuplicate, dottedKey)
	case SegSelector:
		list, ok := asListRef(parent)
		if !ok {
			return false, fmt.Errorf("fieldpath: expected list for selector, got %T", parent)
		}
		for _, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if matchPredicates(m, seg.Preds) {
				return addIn(item, segs[1:], value, allowDuplicate, dottedKey)
			}
		}
		return false, fmt.Errorf("fieldpath: no list item matches %v", seg.Preds)
	}
	return false, fmt.Errorf("fieldpath: unsupported segment %v in add path", seg.Kind)
}

// deleteIn implements Delete. Returns existed=true when the address
// pointed at a real value before the call.
func deleteIn(parent any, segs []Segment) (bool, error) {
	if len(segs) == 0 {
		return false, fmt.Errorf("fieldpath: empty segments")
	}
	if len(segs) == 1 {
		return deleteLeaf(parent, segs[0])
	}
	seg := segs[0]
	switch seg.Kind {
	case SegKey:
		m, ok := parent.(map[string]any)
		if !ok {
			return false, nil
		}
		end := keyRunLength(segs)
		for n := end; n >= 1; n-- {
			key := joinKeys(segs, n)
			if v, ok := m[key]; ok {
				if normalized, isList := normalizeList(v); isList {
					m[key] = normalized
					v = normalized
				}
				return deleteIn(v, segs[n:])
			}
		}
		return false, nil
	case SegIndex:
		list, ok := asListRef(parent)
		if !ok {
			return false, nil
		}
		idx := normalizeIndex(seg.Index, len(list))
		if idx < 0 || idx >= len(list) {
			return false, nil
		}
		return deleteIn(list[idx], segs[1:])
	case SegSelector:
		list, ok := asListRef(parent)
		if !ok {
			return false, nil
		}
		for _, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if matchPredicates(m, seg.Preds) {
				return deleteIn(item, segs[1:])
			}
		}
		return false, nil
	}
	return false, fmt.Errorf("fieldpath: unsupported segment %v in delete path", seg.Kind)
}

// deleteLeaf removes the addressed slot. For SegIndex/SegSelector the
// removal must rewrite the parent list in place — Go slices need a
// re-slice plus a back-pointer-aware write, which we accomplish by
// walking from the grandparent.
//
// However, by the time we recurse into setIn/deleteIn for an intermediate
// list, we hold the list directly (asListRef returns []any). Slice-
// rewriting that requires shortening the list requires updating the
// grandparent map's slot. To keep the recursion simple, deleteLeaf
// returns ok=false for these cases here and we re-route at the parent
// level via deleteListItemAtParent.
//
// Cleaner alternative: walk one segment ahead in deleteIn and detect the
// "last segment is index/selector" case there, dispatching to a
// parent-aware deletion. That is what we do below — see the special-case
// branch in Delete that calls deleteParentList before falling into deleteIn.
func deleteLeaf(parent any, seg Segment) (bool, error) {
	switch seg.Kind {
	case SegKey:
		m, ok := parent.(map[string]any)
		if !ok {
			return false, nil
		}
		if _, has := m[seg.Key]; !has {
			return false, nil
		}
		delete(m, seg.Key)
		return true, nil
	case SegIndex, SegSelector:
		// Handled by Delete's pre-check; reaching here means the caller
		// found the list AND we need to remove an entry. We do it by
		// rewriting the slice header on the list-holder, which requires
		// finding the parent. We implement that via the longer Delete
		// path; in practice deleteIn dispatches via the parent walker
		// (deleteListEntry) and never lands here.
		return false, fmt.Errorf("fieldpath: list-element delete must go via parent walker")
	case SegProject:
		return false, fmt.Errorf("fieldpath: delete with projection not supported")
	}
	return false, fmt.Errorf("fieldpath: unknown segment kind %d", seg.Kind)
}

// Delete's full implementation handles the list-element case here by
// walking one segment short and rewriting the parent slot.
func init() {
	// Sanity: ensure init order doesn't matter; this is a no-op marker.
}

// deleteListEntry shortens the list at segs (which addresses the list
// itself, plus a final index/selector indicating which entry to drop) by
// rewriting the parent's slot. Returns existed=true when the entry was
// found and removed.
func deleteListEntry(rootMap map[string]any, listSegs []Segment, lastSeg Segment) (bool, error) {
	// Resolve the list using setIn-style traversal but stop one step short
	// so we can rewrite the holder. We re-implement a small parent walk
	// here because the regular walker hides the holder reference.
	holder, key, listVal, ok, err := resolveListHolder(rootMap, listSegs)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	out := make([]any, 0, len(listVal))
	removed := false
	switch lastSeg.Kind {
	case SegIndex:
		idx := normalizeIndex(lastSeg.Index, len(listVal))
		if idx < 0 || idx >= len(listVal) {
			return false, nil
		}
		for i, item := range listVal {
			if i == idx {
				removed = true
				continue
			}
			out = append(out, item)
		}
	case SegSelector:
		// Remove the first matching entry to mirror the "first match wins"
		// rule selectors use for read; broader removal would be a footgun.
		for _, item := range listVal {
			if !removed {
				if m, ok := item.(map[string]any); ok && matchPredicates(m, lastSeg.Preds) {
					removed = true
					continue
				}
			}
			out = append(out, item)
		}
	default:
		return false, fmt.Errorf("fieldpath: deleteListEntry called with kind %v", lastSeg.Kind)
	}
	if !removed {
		return false, nil
	}
	holder[key] = out
	return true, nil
}

// resolveListHolder walks segs and returns the map that holds the list,
// the key under which it's stored, and the list itself. Used by
// deleteListEntry to rewrite the slice in place.
func resolveListHolder(root map[string]any, segs []Segment) (holder map[string]any, key string, list []any, ok bool, err error) {
	// All segments must be SegKey for the holder walk; nested intermediate
	// indices on the way to a deletable list are out of scope (would be a
	// pathological case like "outerList[2].innerList" which the spec
	// doesn't have).
	for _, s := range segs {
		if s.Kind != SegKey {
			return nil, "", nil, false, fmt.Errorf("fieldpath: list holder must be addressed by keys, got %v", s.Kind)
		}
	}
	cur := any(root)
	for i := 0; i < len(segs); i++ {
		m, mok := cur.(map[string]any)
		if !mok {
			return nil, "", nil, false, nil
		}
		// Longest-prefix flat key match at this level
		end := len(segs) - i
		matched := 0
		var matchedVal any
		for n := end; n >= 1; n-- {
			keys := make([]string, n)
			for j := range n {
				keys[j] = segs[i+j].Key
			}
			joined := strings.Join(keys, ".")
			if v, has := m[joined]; has {
				matched = n
				matchedVal = v
				break
			}
		}
		if matched == 0 {
			return nil, "", nil, false, nil
		}
		if i+matched == len(segs) {
			// This is the holder/key/list level.
			list, lok := asListRef(matchedVal)
			if !lok {
				return nil, "", nil, false, fmt.Errorf("fieldpath: %q is not a list", segs[i].Key)
			}
			holderKey := strings.Join(keysOf(segs[i:i+matched]), ".")
			return m, holderKey, list, true, nil
		}
		cur = matchedVal
		i += matched - 1
	}
	return nil, "", nil, false, nil
}

func keysOf(segs []Segment) []string {
	out := make([]string, len(segs))
	for i, s := range segs {
		out[i] = s.Key
	}
	return out
}

// keyRunLength returns the count of consecutive SegKey segments at the
// head of segs — used by both the read and write walkers for longest-
// prefix flat-key matching.
func keyRunLength(segs []Segment) int {
	end := 0
	for end < len(segs) && segs[end].Kind == SegKey {
		end++
	}
	if end == 0 {
		end = 1
	}
	return end
}

// joinKeys joins the first n SegKey segments into a dot-separated string.
func joinKeys(segs []Segment, n int) string {
	parts := make([]string, n)
	for i := range n {
		parts[i] = segs[i].Key
	}
	return strings.Join(parts, ".")
}

// normalizeIndex resolves a possibly-negative index against a list length.
// Negative indices count from the end (so -1 is the last element).
func normalizeIndex(i, n int) int {
	if i < 0 {
		return i + n
	}
	return i
}

// getOrCreateChild ensures m[key] exists with a shape appropriate for the
// next segment: a map when the next step is a key access, a slice when
// the next step is index/selector/projection.
func getOrCreateChild(m map[string]any, key string, next Segment) any {
	if v, ok := m[key]; ok {
		return v
	}
	switch next.Kind {
	case SegKey:
		child := map[string]any{}
		m[key] = child
		return child
	default:
		child := []any{}
		m[key] = child
		return child
	}
}

// normalizeList converts a typed slice ([]string, []map[string]any) into
// []any so subsequent in-place element mutations propagate through the
// shared underlying array. Returns (nil, false) when v is not a list
// shape we know how to convert.
func normalizeList(v any) ([]any, bool) {
	switch x := v.(type) {
	case []any:
		return x, true
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out, true
	case []map[string]any:
		out := make([]any, len(x))
		for i, m := range x {
			out[i] = m
		}
		return out, true
	}
	return nil, false
}

// asListRef is the mutation-side equivalent of asList. It returns []any
// directly so the caller can mutate elements in place — the typed slices
// asList knows about ([]string, []map[string]any) are converted to []any
// because mutating those typed copies wouldn't propagate back to the
// document map. ToMap() emits []string and []map[string]any in places, so
// callers that need to *mutate* the list (rather than just read elements)
// should reassign via the parent.
func asListRef(v any) ([]any, bool) {
	if x, ok := v.([]any); ok {
		return x, true
	}
	if x, ok := v.([]map[string]any); ok {
		out := make([]any, len(x))
		for i, m := range x {
			out[i] = m
		}
		return out, true
	}
	if x, ok := v.([]string); ok {
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out, true
	}
	return nil, false
}
