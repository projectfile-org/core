// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fieldpath

import (
	"fmt"

	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

const (
	keyKeywords     = "keywords"
	keyRepositories = "repositories"
	keyLinks        = "links"
)

// IdentityFunc returns the canonical identity-key for one list item, used
// by Add to deduplicate. The ok flag is false when the item shape is not
// what the rule expects (e.g. a non-map value handed to urlIdentity); the
// caller treats such cases as "no dedup applicable" so Add still proceeds.
type IdentityFunc func(item any) (key string, ok bool)

// identityFuncs binds dotted list addresses to their dedup rules. Lookup
// is exact-match on the address text — there is no prefix descent — so the
// caller is forced to address the list precisely (`requirements.operating-system`, not
// `requirements`). This keeps "what counts as a duplicate" predictable.
var identityFuncs = map[string]IdentityFunc{
	keyKeywords:                     stringIdentity,
	"technologies":                  stringIdentity,
	"requirements.operating-system": stringIdentity,
	"requirements.arch":             stringIdentity,
	keyRepositories:                 urlIdentity,
	keyLinks:                        typeURLIdentity,
	"people":                        personIdentityKey,
	"organizations":                 organizationIdentityKey,
	"includes":                      stringIdentity,
}

// IdentityFor returns the IdentityFunc registered for an exact list path.
// Returns nil when the path has no registered rule — Add treats nil as
// "no dedup, always append".
func IdentityFor(path string) IdentityFunc {
	return identityFuncs[path]
}

// stringIdentity is the identity rule for scalar-string lists (keywords,
// stack, requirements.operating-system/arch, includes). The string value IS the
// identity key.
func stringIdentity(item any) (string, bool) {
	s, ok := item.(string)
	if !ok {
		return "", false
	}
	return s, true
}

// urlIdentity is the identity rule for object lists keyed solely by URL
// (repositories). Two repositories with the same URL are considered the
// same entry regardless of role/branch — change behaviour by editing the
// existing entry, not by appending a clashing one.
func urlIdentity(item any) (string, bool) {
	m, ok := item.(map[string]any)
	if !ok {
		return "", false
	}
	url, _ := m["url"].(string)
	if url == "" {
		return "", false
	}
	return "url:" + url, true
}

// typeURLIdentity is the identity rule for object lists keyed by the
// (type, url) tuple — links and funding. Two links with the same type and
// URL collapse; differing labels are merged via the existing helpers
// (SetLink, etc.) not via this dedup path.
func typeURLIdentity(item any) (string, bool) {
	m, ok := item.(map[string]any)
	if !ok {
		return "", false
	}
	t, _ := m["type"].(string)
	url, _ := m["url"].(string)
	if t == "" && url == "" {
		return "", false
	}
	return fmt.Sprintf("type=%s|url=%s", t, url), true
}

// personIdentityKey re-uses projectfile.PersonIdentityKey so person dedup
// inside Add behaves exactly like MergePeople — orcid → email → name in
// canonical form. Items arrive as map[string]any (post-ToMap shape); we
// project them into a Person so the existing helper applies.
func personIdentityKey(item any) (string, bool) {
	m, ok := item.(map[string]any)
	if !ok {
		return "", false
	}
	p := projectfile.Person{
		FamilyNames:  strField(m, "family-names"),
		GivenNames:   strField(m, "given-names"),
		NameParticle: strField(m, "name-particle"),
		NameSuffix:   strField(m, "name-suffix"),
		DisplayName:  strField(m, "display-name"),
		Email:        strField(m, "email"),
		Orcid:        strField(m, "orcid"),
	}
	return projectfile.PersonIdentityKey(p), true
}

func organizationIdentityKey(item any) (string, bool) {
	m, ok := item.(map[string]any)
	if !ok {
		return "", false
	}
	o := projectfile.Organization{
		Name:  strField(m, "name"),
		Email: strField(m, "email"),
		Orcid: strField(m, "orcid"),
	}
	return projectfile.OrganizationIdentityKey(o), true
}

func strField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}
