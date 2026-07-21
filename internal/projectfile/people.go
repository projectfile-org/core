// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import "strings"

// PersonConflict describes a non-mergeable disagreement between two records
// that were deemed the same person. The merge keeps the existing value; the
// caller surfaces conflicts to the user.
type PersonConflict struct {
	IdentityKey string
	Field       string
	Existing    string
	Incoming    string
}

// PersonIdentityKey returns a short identifier for a single person, used in
// PersonConflict messages and external diagnostics. It is NOT the matching
// algorithm — see samePerson for that.
func PersonIdentityKey(p Person) string {
	if orcid := normaliseORCID(p.Orcid); orcid != "" {
		return "orcid:" + orcid
	}
	if e := normaliseEmail(p.Email); e != "" {
		return "email:" + e
	}
	return "name:" + canonicalPersonName(p)
}

func OrganizationIdentityKey(o Organization) string {
	if orcid := normaliseORCID(o.Orcid); orcid != "" {
		return "orcid:" + orcid
	}
	if e := normaliseEmail(o.Email); e != "" {
		return "email:" + e
	}
	return "name:" + strings.ToLower(strings.TrimSpace(o.Name))
}

// samePerson decides whether two records describe the same person using a
// tiered match strategy. The tiers are ordered by reliability of the
// identifier; a "decisive miss" at a higher tier stops fall-through.
//
//  1. ORCID — if BOTH have an ORCID, it is decisive (equal → same; different → distinct).
//  2. Email — if BOTH have an email AND they match → same. Different emails are
//     NOT decisive; we fall through to the name tier so users who switch email
//     between sources are not duplicated.
//  3. Name — if canonical display names are non-empty and match → same.
//
// Falls back to "distinct" otherwise.
func samePerson(a, b Person) bool {
	ao := normaliseORCID(a.Orcid)
	bo := normaliseORCID(b.Orcid)
	if ao != "" && bo != "" {
		return ao == bo
	}
	ae := normaliseEmail(a.Email)
	be := normaliseEmail(b.Email)
	if ae != "" && be != "" && ae == be {
		return true
	}
	an := canonicalPersonName(a)
	bn := canonicalPersonName(b)
	if an != "" && an == bn {
		return true
	}
	return false
}

func sameOrg(a, b Organization) bool {
	ao := normaliseORCID(a.Orcid)
	bo := normaliseORCID(b.Orcid)
	if ao != "" && bo != "" {
		return ao == bo
	}
	ae := normaliseEmail(a.Email)
	be := normaliseEmail(b.Email)
	if ae != "" && be != "" && ae == be {
		return true
	}
	an := strings.ToLower(strings.TrimSpace(a.Name))
	bn := strings.ToLower(strings.TrimSpace(b.Name))
	if an != "" && an == bn {
		return true
	}
	return false
}

func normaliseORCID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "orcid.org/")
	return strings.ToLower(s)
}

func normaliseEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func canonicalPersonName(p Person) string {
	if p.DisplayName != "" {
		return strings.ToLower(strings.TrimSpace(p.DisplayName))
	}
	var parts []string
	if p.GivenNames != "" {
		parts = append(parts, p.GivenNames)
	}
	if p.NameParticle != "" {
		parts = append(parts, p.NameParticle)
	}
	if p.FamilyNames != "" {
		parts = append(parts, p.FamilyNames)
	}
	if p.NameSuffix != "" {
		parts = append(parts, p.NameSuffix)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// MergePeople merges incoming records into existing. Matching uses samePerson
// (tiered ORCID/email/name strategy). Rules for matched pairs:
//   - Roles: set-union, existing-order preserved, then incoming-only roles.
//   - Non-role string fields: existing wins; if existing is empty, incoming
//     fills (ORCID gets the same treatment).
//   - Hard conflicts (both sides non-empty, values differ): recorded in
//     conflicts, existing value is kept.
//
// Returned slice has length >= len(existing); new identities are appended in
// incoming order.
func MergePeople(existing, incoming []Person) (merged []Person, conflicts []PersonConflict) {
	merged = make([]Person, len(existing))
	copy(merged, existing)

	for _, in := range incoming {
		idx := findPersonMatch(merged, in)
		if idx < 0 {
			merged = append(merged, in)
			continue
		}
		cur := merged[idx]
		cur.Roles = unionRoles(cur.Roles, in.Roles)
		cur, conflicts = mergePersonFields(cur, in, PersonIdentityKey(cur), conflicts)
		merged[idx] = cur
	}
	return merged, conflicts
}

func MergeOrganizations(existing, incoming []Organization) (merged []Organization, conflicts []PersonConflict) {
	merged = make([]Organization, len(existing))
	copy(merged, existing)

	for _, in := range incoming {
		idx := findOrgMatch(merged, in)
		if idx < 0 {
			merged = append(merged, in)
			continue
		}
		cur := merged[idx]
		cur.Roles = unionRoles(cur.Roles, in.Roles)
		cur, conflicts = mergeOrgFields(cur, in, OrganizationIdentityKey(cur), conflicts)
		merged[idx] = cur
	}
	return merged, conflicts
}

// MergePeopleRaw is the raw-map counterpart of MergePeople for the include
// resolution path. What we are trying to do: let a base document carry
// project-scoped fields (e.g. [[people]].from) on a person whose identity
// and contact fields arrive via an include — without duplicating the entry
// and tripping the schema's required-field checks.
//
// Identity match mirrors samePerson (orcid decisive when both present →
// email match when both present → canonical name match). The direction is
// deepMerge's: winner wins, loser fills. Roles are unioned with loser
// order preserved first (consistent with deepMerge's "loser first, then
// winner" slice contract). Conflicts are silent — include resolution is
// a transparent composition step, not a user-facing sync like bridge.
//
// Raw maps are used (rather than round-tripping through []Person) so keys
// the producer is still editing survive — include resolution runs before
// validate, and silently dropping unknown keys here would mask the real
// schema error the user needs to see.
func MergePeopleRaw(winner, loser []any) []any {
	return mergeEntityListRaw(winner, loser, canonicalPersonNameRaw)
}

// MergeOrganizationsRaw is the organization analogue of MergePeopleRaw.
// Identity tier is the same (orcid → email → name); only the canonical
// name extractor differs (organizations use a single `name` field).
func MergeOrganizationsRaw(winner, loser []any) []any {
	return mergeEntityListRaw(winner, loser, canonicalOrgNameRaw)
}

// mergeEntityListRaw unifies two raw entity-list slices by identity. The
// `nameCanon` callback supplies the tier-3 canonical name (persons compose
// it from given/family/etc.; organizations read `name` directly). The
// shared body implements the orcid → email → name fallthrough that
// samePerson/sameOrg also implement — kept in sync deliberately.
func mergeEntityListRaw(winner, loser []any, nameCanon func(map[string]any) string) []any {
	out := make([]any, 0, len(loser)+len(winner))
	out = append(out, loser...)
	for _, w := range winner {
		wm, ok := w.(map[string]any)
		if !ok {
			out = append(out, w)
			continue
		}
		idx := findRawEntityMatch(out, wm, nameCanon)
		if idx < 0 {
			out = append(out, w)
			continue
		}
		if existing, ok := out[idx].(map[string]any); ok {
			out[idx] = mergeRawEntity(existing, wm)
		} else {
			out[idx] = w
		}
	}
	return out
}

// findRawEntityMatch is the raw-map twin of findPersonMatch/findOrgMatch.
// Identity tiering MUST stay aligned with samePerson/sameOrg: orcid is
// decisive when both sides have one; email matches only when both have
// one and they agree (mismatch is NOT decisive — a user switching email
// between sources should not split into two entries); canonical name is
// the final tier.
func findRawEntityMatch(haystack []any, needle map[string]any, nameCanon func(map[string]any) string) int {
	no := normaliseORCID(strVal(needle, keyOrcid))
	ne := normaliseEmail(strVal(needle, keyEmail))
	nn := nameCanon(needle)
	for i := range haystack {
		hm, ok := haystack[i].(map[string]any)
		if !ok {
			continue
		}
		ho := normaliseORCID(strVal(hm, keyOrcid))
		if no != "" && ho != "" {
			if no == ho {
				return i
			}
			continue
		}
		he := normaliseEmail(strVal(hm, keyEmail))
		if ne != "" && he != "" && ne == he {
			return i
		}
		hn := nameCanon(hm)
		if nn != "" && nn == hn {
			return i
		}
	}
	return -1
}

// mergeRawEntity applies "winner wins, loser fills" per key. Roles are
// unioned (loser first, then winner-only). Nested maps recurse via
// deepMerge so a key like `handles` composes the same way top-level maps
// do. Strings keep loser when winner is empty (gap-fill direction); all
// other scalar types are replaced by winner outright.
func mergeRawEntity(loser, winner map[string]any) map[string]any {
	out := make(map[string]any, len(loser)+len(winner))
	for k, v := range loser {
		out[k] = v
	}
	for k, wv := range winner {
		lv, exists := out[k]
		if !exists {
			out[k] = wv
			continue
		}
		if k == keyRoles {
			out[k] = unionRoles(toStrSlice(lv), toStrSlice(wv))
			continue
		}
		if lMap, ok := lv.(map[string]any); ok {
			if wMap, ok := wv.(map[string]any); ok {
				out[k] = deepMerge(wMap, lMap)
				continue
			}
		}
		// "Winner wins, loser fills": keep loser when winner is empty so
		// a sparse override (e.g. setting only `from`) doesn't blank the
		// fields the include contributed.
		if wStr, ok := wv.(string); ok && wStr == "" {
			continue
		}
		out[k] = wv
	}
	return out
}

// canonicalPersonNameRaw is the raw-map twin of canonicalPersonName. Used
// as the tier-3 identity match for raw people entries.
func canonicalPersonNameRaw(m map[string]any) string {
	if dn := strings.TrimSpace(strVal(m, "display-name")); dn != "" {
		return strings.ToLower(dn)
	}
	var parts []string
	for _, k := range []string{keyGivenNames, "name-particle", keyFamilyNames, "name-suffix"} {
		if s := strings.TrimSpace(strVal(m, k)); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// canonicalOrgNameRaw is the raw-map twin of the name branch of sameOrg.
func canonicalOrgNameRaw(m map[string]any) string {
	return strings.ToLower(strings.TrimSpace(strVal(m, "name")))
}

func findPersonMatch(haystack []Person, needle Person) int {
	for i, p := range haystack {
		if samePerson(p, needle) {
			return i
		}
	}
	return -1
}

func findOrgMatch(haystack []Organization, needle Organization) int {
	for i, o := range haystack {
		if sameOrg(o, needle) {
			return i
		}
	}
	return -1
}

// unionRoles preserves the order of existing roles, then appends incoming
// roles that are not already present.
func unionRoles(existing, incoming []string) []string {
	seen := make(map[string]bool, len(existing)+len(incoming))
	out := make([]string, 0, len(existing)+len(incoming))
	for _, r := range existing {
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	for _, r := range incoming {
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}

// mergePersonFields applies "existing wins, incoming fills" per-field and
// records hard conflicts.
func mergePersonFields(cur, in Person, key string, conflicts []PersonConflict) (Person, []PersonConflict) {
	mergeStr := func(name string, cur, in string) (string, *PersonConflict) {
		if cur == "" {
			return in, nil
		}
		if in == "" || cur == in {
			return cur, nil
		}
		return cur, &PersonConflict{IdentityKey: key, Field: name, Existing: cur, Incoming: in}
	}
	type entry struct {
		field string
		cur   *string
		in    string
	}
	for _, e := range []entry{
		{keyFamilyNames, &cur.FamilyNames, in.FamilyNames},
		{keyGivenNames, &cur.GivenNames, in.GivenNames},
		{"name-particle", &cur.NameParticle, in.NameParticle},
		{"name-suffix", &cur.NameSuffix, in.NameSuffix},
		{"alias", &cur.Alias, in.Alias},
		{"display-name", &cur.DisplayName, in.DisplayName},
		{keyEmail, &cur.Email, in.Email},
		{keyURL, &cur.URL, in.URL},
		{"affiliation", &cur.Affiliation, in.Affiliation},
		{keyOrcid, &cur.Orcid, in.Orcid},
	} {
		v, c := mergeStr(e.field, *e.cur, e.in)
		*e.cur = v
		if c != nil {
			conflicts = append(conflicts, *c)
		}
	}
	if len(in.Handles) > 0 {
		if cur.Handles == nil {
			cur.Handles = make(map[string]any, len(in.Handles))
		}
		for k, v := range in.Handles {
			if _, exists := cur.Handles[k]; !exists {
				cur.Handles[k] = v
			}
		}
	}
	return cur, conflicts
}

func mergeOrgFields(cur, in Organization, key string, conflicts []PersonConflict) (Organization, []PersonConflict) {
	mergeStr := func(name string, cur, in string) (string, *PersonConflict) {
		if cur == "" {
			return in, nil
		}
		if in == "" || cur == in {
			return cur, nil
		}
		return cur, &PersonConflict{IdentityKey: key, Field: name, Existing: cur, Incoming: in}
	}
	type entry struct {
		field string
		cur   *string
		in    string
	}
	for _, e := range []entry{
		{keyName, &cur.Name, in.Name},
		{"alias", &cur.Alias, in.Alias},
		{keyEmail, &cur.Email, in.Email},
		{keyURL, &cur.URL, in.URL},
		{keyOrcid, &cur.Orcid, in.Orcid},
	} {
		v, c := mergeStr(e.field, *e.cur, e.in)
		*e.cur = v
		if c != nil {
			conflicts = append(conflicts, *c)
		}
	}
	if len(in.Handles) > 0 {
		if cur.Handles == nil {
			cur.Handles = make(map[string]any, len(in.Handles))
		}
		for k, v := range in.Handles {
			if _, exists := cur.Handles[k]; !exists {
				cur.Handles[k] = v
			}
		}
	}
	return cur, conflicts
}
