// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"reflect"
	"sort"
)

// ResolveIncludesOnly fetches and merges all includes declared in raw without
// overlaying the base document. Returns the accumulated includes map (the
// "what would the includes contribute?" view), resolved TRANSITIVELY: an
// include's own includes are folded in too, so optimisation can strip base
// values that duplicate a transitive contribution, not just a direct one.
// Returns an empty (non-nil) map when no includes are declared — the optimize
// command uses this to decide whether optimisation is applicable. No
// self-seed is needed: optimize operates on the base document's raw map, and
// the base is never an include of itself; cycle safety comes from the shared
// resolveIncludesChain ancestor stack.
func ResolveIncludesOnly(raw map[string]any, baseDir string, opts ReadOptions) (map[string]any, error) {
	return resolveIncludesChain(raw, baseDir, opts, make(map[string]struct{}))
}

// StripRedundant removes keys from base whose values are deep-equal to the
// corresponding values in includes. Keys "includes", "$schema", and
// "spec_version" are always preserved — they are local-file concerns, not
// inherited data. Nested maps are recursed; when a sub-map becomes empty
// after stripping, the parent key is removed. Returns a sorted list of
// dotted paths that were removed.
func StripRedundant(base, includes map[string]any) []string {
	return stripRedundant(base, includes, "")
}

// entityListKeys are the reserved list keys whose entries merge by identity
// during include resolution (MergePeopleRaw / MergeOrganizationsRaw in
// deepMerge). stripRedundant MUST NOT touch them: a base entry's identity
// fields (email, orcid, name) are the LINK that attaches project-scoped
// data (from, to) to the include's full record. Stripping that link — or
// any sub-field of the entry — would orphan the project-specific data and
// re-introduce the schema violation the include-merge path exists to
// prevent. A genuinely redundant whole entry (an exact duplicate of an
// include person with no project-specific data) is left for the producer
// to remove by hand; automating it safely requires the same identity-aware
// comparison that has no business inside this generic deep-equal walker.
var entityListKeys = map[string]bool{
	keyPeople:        true,
	keyOrganizations: true,
}

func stripRedundant(base, includes map[string]any, prefix string) []string {
	var removed []string

	keys := sortedKeys(base)

	for _, key := range keys {
		if key == keyIncludes || key == keySchema || key == keySpecVersion {
			continue
		}
		if entityListKeys[key] {
			continue
		}
		incVal, exists := includes[key]
		if !exists {
			continue
		}
		baseVal := base[key]

		dotted := key
		if prefix != "" {
			dotted = prefix + "." + key
		}

		baseMap, baseIsMap := baseVal.(map[string]any)
		incMap, incIsMap := incVal.(map[string]any)
		if baseIsMap && incIsMap {
			subRemoved := stripRedundant(baseMap, incMap, dotted)
			removed = append(removed, subRemoved...)
			if len(baseMap) == 0 {
				delete(base, key)
			}
			continue
		}

		if reflect.DeepEqual(baseVal, incVal) {
			delete(base, key)
			removed = append(removed, dotted)
		}
	}

	return removed
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SortIncludes sorts the values of the top-level "includes" field in raw,
// in place. Include paths/URLs are an unordered set — sorting yields a
// stable, diff-friendly on-disk representation. Only "includes" is touched;
// every other list field keeps its declared order. A missing, empty, or
// non-string-homogeneous list is left untouched (order as authored).
func SortIncludes(raw map[string]any) {
	list, ok := raw[keyIncludes].([]any)
	if !ok || len(list) < 2 {
		return
	}
	strs := make([]string, len(list))
	for i, item := range list {
		s, ok := item.(string)
		if !ok {
			return
		}
		strs[i] = s
	}
	sort.Strings(strs)
	for i, s := range strs {
		list[i] = s
	}
}
