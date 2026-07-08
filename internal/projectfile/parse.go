// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

// FromMap rebuilds a typed Document from a raw map produced by ToMap (or
// by an external mutation step). Mirror of Read's parse stage exposed for
// the fieldpath mutation helpers, which do map-based edits then need a
// typed Document to pass back through Write.
func FromMap(raw map[string]any) *Document {
	return parseRawDocument(raw)
}

func parseRawDocument(raw map[string]any) *Document {
	doc := &Document{}

	doc.SpecVersion = strVal(raw, "spec_version")
	doc.Schema = strVal(raw, "$schema")
	doc.Kind = strVal(raw, keyKind)

	if ident, ok := mapVal(raw, keyIdentity); ok {
		doc.Identity = parseIdentity(ident)
	}

	if lic, ok := mapVal(raw, "license"); ok {
		doc.License = parseLicense(lic)
	}

	if cp, ok := mapVal(raw, "copyright"); ok {
		doc.Copyright = parseCopyright(cp)
	}

	if people, ok := listVal(raw, "people"); ok {
		doc.People = parsePeople(people)
	}

	if orgs, ok := listVal(raw, "organizations"); ok {
		doc.Organizations = parseOrganizations(orgs)
	}

	doc.Keywords = strListVal(raw, keyKeywords)
	doc.Stack = strListVal(raw, "technologies")
	doc.Includes = strListVal(raw, keyIncludes)

	if req, ok := mapVal(raw, "requirements"); ok {
		doc.Requirements = parseRequirements(req)
	}

	if deps, ok := mapVal(raw, "dependencies"); ok {
		doc.Dependencies = parseDependencies(deps)
	}

	if repos, ok := listVal(raw, "repositories"); ok {
		doc.Repositories = parseRepositories(repos)
	}

	if links, ok := listVal(raw, keyLinks); ok {
		doc.Links = parseLinks(links)
	}

	doc.Extensions = extractExtensions(raw)
	return doc
}

func parseIdentity(raw map[string]any) Identity {
	ident := Identity{
		Namespace: strVal(raw, "namespace"),
		Name:      strVal(raw, "name"),
		Version:   strVal(raw, "version"),
		Created:   strVal(raw, "created"),
		Released:  strVal(raw, "released"),
		Modified:  strVal(raw, "modified"),
	}

	if v, ok := raw["title"]; ok {
		ident.Title = parseLocalizedString(v)
	}
	if v, ok := raw["summary"]; ok {
		ident.Summary = parseLocalizedString(v)
	}
	if v, ok := raw["description"]; ok {
		ident.Description = parseLocalizedString(v)
	}

	return ident
}

func parseLocalizedString(v any) *LocalizedString {
	ls := &LocalizedString{}
	switch val := v.(type) {
	case string:
		ls.Bare = val
	case map[string]any:
		ls.Langs = make(map[string]string)
		for k, v := range val {
			if s, ok := v.(string); ok {
				ls.Langs[k] = s
			}
		}
	}
	return ls
}

func parseRepositories(raw []any) []Repository {
	out := make([]Repository, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		r := Repository{
			URL:    strVal(m, "url"),
			Type:   strVal(m, "type"),
			Path:   strVal(m, "path"),
			Branch: strVal(m, "branch"),
			Issues: boolVal(m, "issues"),
			Role:   strVal(m, "role"),
		}
		out = append(out, r)
	}
	return out
}

func parseLicense(raw map[string]any) *License {
	lic := &License{
		Spdx:   strVal(raw, "spdx"),
		Covers: strVal(raw, "covers"),
	}
	if v, ok := raw["file"]; ok {
		lic.File = v
	}
	return lic
}

func parseCopyright(raw map[string]any) *Copyright {
	cp := &Copyright{}
	if v, ok := raw["year"]; ok {
		switch n := v.(type) {
		case int:
			cp.Year = n
		case int64:
			cp.Year = int(n)
		case float64:
			cp.Year = int(n)
		}
	}
	return cp
}

func parsePeople(raw []any) []Person {
	out := make([]Person, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		p := Person{
			FamilyNames:  strVal(m, keyFamilyNames),
			GivenNames:   strVal(m, keyGivenNames),
			NameParticle: strVal(m, "name-particle"),
			NameSuffix:   strVal(m, "name-suffix"),
			Alias:        strVal(m, "alias"),
			DisplayName:  strVal(m, "display-name"),
			Email:        strVal(m, keyEmail),
			URL:          strVal(m, "url"),
			Affiliation:  strVal(m, "affiliation"),
			Orcid:        strVal(m, "orcid"),
			From:         strVal(m, keyFrom),
			To:           strVal(m, "to"),
			Roles:        strListVal(m, "roles"),
		}
		if v, ok := m["handles"].(map[string]any); ok {
			p.Handles = make(map[string]any, len(v))
			for k, val := range v {
				p.Handles[k] = val
			}
		}
		out = append(out, p)
	}
	return out
}

func parseOrganizations(raw []any) []Organization {
	out := make([]Organization, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		o := Organization{
			Name:  strVal(m, "name"),
			Alias: strVal(m, "alias"),
			Email: strVal(m, keyEmail),
			URL:   strVal(m, "url"),
			Orcid: strVal(m, "orcid"),
			From:  strVal(m, keyFrom),
			To:    strVal(m, "to"),
			Roles: strListVal(m, "roles"),
		}
		if v, ok := m["handles"].(map[string]any); ok {
			o.Handles = make(map[string]any, len(v))
			for k, val := range v {
				o.Handles[k] = val
			}
		}
		out = append(out, o)
	}
	return out
}

func parseRequirements(raw map[string]any) *Requirements {
	req := &Requirements{
		OS:   strListVal(raw, "operating-system"),
		Arch: strListVal(raw, "arch"),
	}
	if v, ok := raw["browsers"]; ok {
		req.Browsers = v
	}
	if runtime, ok := mapVal(raw, "runtime"); ok {
		req.Runtime = make(map[string]string)
		for k, v := range runtime {
			if s, ok := v.(string); ok {
				req.Runtime[k] = s
			}
		}
	}
	return req
}

func parseDependencies(raw map[string]any) *Dependencies {
	return &Dependencies{
		Runtime: strListVal(raw, "runtime"),
		Build:   strListVal(raw, "build"),
		Test:    strListVal(raw, "test"),
	}
}

func parseLinks(raw []any) []Link {
	out := make([]Link, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		l := Link{
			Type: strVal(m, "type"),
			URL:  strVal(m, "url"),
		}
		if v, ok := m["label"]; ok {
			l.Label = parseLocalizedString(v)
		}
		if v, ok := m["preferred"]; ok {
			if b, ok := v.(bool); ok {
				l.Preferred = b
			}
		}
		out = append(out, l)
	}
	return out
}

func strVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func strListVal(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	return toStrSlice(v)
}

// toStrSlice converts a parsed value to []string. Accepts both the
// parse-time shape ([]any from JSON/YAML/TOML decoders) and the
// serialize-time shape ([]string from ToMap).
func toStrSlice(v any) []string {
	if list, ok := v.([]any); ok {
		out := make([]string, 0, len(list))
		for _, item := range list {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	if list, ok := v.([]string); ok {
		out := make([]string, len(list))
		copy(out, list)
		return out
	}
	return nil
}

func floatVal(m map[string]any, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

func intVal(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func mapVal(m map[string]any, key string) (map[string]any, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	sub, ok := v.(map[string]any)
	return sub, ok
}

func listVal(m map[string]any, key string) ([]any, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	if list, ok := v.([]any); ok {
		return list, true
	}
	// Mirrors strListVal: accept ToMap's []map[string]any shape so the
	// in-memory round-trip works for object lists (repositories, links,
	// people, funding) the same way it does for string lists.
	if list, ok := v.([]map[string]any); ok {
		out := make([]any, len(list))
		for i, m := range list {
			out[i] = m
		}
		return out, true
	}
	return nil, false
}
