// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import "strings"

// keyURL is the canonical map/struct key for any "url" field — repository,
// link, funding, person — shared by serialize.go and people.go.
const keyURL = "url"

const keySchema = "$schema"

const (
	keyKind          = "kind"
	keyIdentity      = "identity"
	keyKeywords      = "keywords"
	keyIncludes      = "includes"
	keyLinks         = "links"
	keyFamilyNames   = "family-names"
	keyGivenNames    = "given-names"
	keyEmail         = "email"
	keyFrom          = "from"
	keyCommitStyle   = "commit-style"
	keyName          = "name"
	keyNamespace     = "namespace"
	keyLicense       = "license"
	keyTechnologies  = "technologies"
	keyRoles         = "roles"
	keyOrcid         = "orcid"
	keySpdx          = "spdx"
	keyType          = "type"
	keySpecVersion   = "spec_version"
	keyPeople        = "people"
	keyOrganizations = "organizations"
)

func extractExtensions(raw map[string]any) map[string]any {
	ext := make(map[string]any)
	for k, v := range raw {
		if !ReservedKeys[k] {
			ext[k] = v
		}
	}
	if len(ext) == 0 {
		return nil
	}
	return ext
}

// ReservedKeys is the set of top-level keys owned by the spec. Used by
// extractExtensions to separate extension namespaces, and by writeYAMLFile
// to paint typed fields onto a rawdoc.YAMLNode without destroying extensions.
var ReservedKeys = map[string]bool{
	keySpecVersion: true, keySchema: true, keyKind: true,
	keyIdentity: true, "repositories": true, keyLicense: true,
	"copyright": true, keyPeople: true, keyOrganizations: true, keyKeywords: true,
	keyTechnologies: true, "requirements": true, keyIncludes: true,
	"dependencies": true, keyLinks: true,
}

func (doc *Document) ToMap() map[string]any {
	m := map[string]any{}

	if doc.SpecVersion != "" {
		m[keySpecVersion] = doc.SpecVersion
	}
	if doc.Schema != "" {
		m[keySchema] = doc.Schema
	}
	if doc.Kind != "" {
		m[keyKind] = doc.Kind
	}

	m[keyIdentity] = identityToMap(doc.Identity)

	if len(doc.Repositories) > 0 {
		m["repositories"] = repositoriesToMapList(doc.Repositories)
	}

	if doc.License != nil {
		m[keyLicense] = licenseToMap(doc.License)
	}

	if doc.Copyright != nil {
		m["copyright"] = copyrightToMap(doc.Copyright)
	}

	if len(doc.People) > 0 {
		m[keyPeople] = peopleToMapList(doc.People)
	}

	if len(doc.Organizations) > 0 {
		m[keyOrganizations] = organizationsToMapList(doc.Organizations)
	}

	if len(doc.Keywords) > 0 {
		m[keyKeywords] = doc.Keywords
	}

	if len(doc.Stack) > 0 {
		m[keyTechnologies] = doc.Stack
	}

	if doc.Requirements != nil {
		m["requirements"] = requirementsToMap(doc.Requirements)
	}

	if len(doc.Includes) > 0 {
		m[keyIncludes] = doc.Includes
	}

	if doc.Dependencies != nil {
		m["dependencies"] = dependenciesToMap(doc.Dependencies)
	}

	if len(doc.Links) > 0 {
		m[keyLinks] = linksToMapList(doc.Links)
	}

	// Prune del-orphaned `{}` containers in NATIVE structure only, THEN graft the
	// opaque extension namespaces. An empty map inside an `org.*` extension can be a
	// deliberate presence signal pf-cli does not own (e.g. org.projectfile.ci's
	// `dispatch: {}` — a manual-run button with zero inputs, which ci-resolver reads
	// as button-present); pruning it would silently delete another tool's data.
	pruneEmptyMaps(m)

	for k, v := range doc.Extensions {
		nestExtension(m, strings.Split(k, "."), v)
	}

	return m
}

// nestExtension explodes a dotted reverse-DNS namespace key into nested
// mappings under m, merging into any existing branch when two namespaces
// share a prefix (e.g. "org.projectfile.cli" + "org.projectfile.codeowners"
// both land under m["org"]). The v1 schema rejects flat
// dotted top-level keys (additionalProperties:false against single-segment
// patternProperties: ^[a-z][a-z0-9-]*$), so writing the literal dotted
// string here would produce a document the same CLI then refuses to
// validate. Single-segment keys (no dots) are written verbatim — that is
// the on-spec encoding for reserved-but-unknown top-level keys (spec §4.7).
func nestExtension(m map[string]any, segments []string, value any) {
	head := segments[0]
	if len(segments) == 1 {
		m[head] = value
		return
	}
	child, ok := m[head].(map[string]any)
	if !ok {
		child = map[string]any{}
		m[head] = child
	}
	nestExtension(child, segments[1:], value)
}

// pruneEmptyMaps removes map entries whose value is an empty map, recursively.
// Applied to the NATIVE structure before extensions are grafted, so a
// deleted-last-key scenario (e.g. `del requirements.runtime` when runtime was
// the only sub-key) does not leave an orphaned `{}` container in the serialised
// output. Extension namespaces are intentionally excluded (see ToMap): an empty
// map there may be a meaningful presence signal pf-cli must not judge.
func pruneEmptyMaps(m map[string]any) {
	for k, v := range m {
		if sub, ok := v.(map[string]any); ok {
			pruneEmptyMaps(sub)
			if len(sub) == 0 {
				delete(m, k)
			}
		}
	}
}

func identityToMap(id Identity) map[string]any {
	m := map[string]any{}
	if id.Namespace != "" {
		m[keyNamespace] = id.Namespace
	}
	if id.Name != "" {
		m[keyName] = id.Name
	}
	if id.Version != "" {
		m["version"] = id.Version
	}
	if id.Title != nil {
		m["title"] = localizedStringToMap(id.Title)
	}
	if id.Summary != nil {
		m["summary"] = localizedStringToMap(id.Summary)
	}
	if id.Description != nil {
		m["description"] = localizedStringToMap(id.Description)
	}
	if id.Created != "" {
		m["created"] = id.Created
	}
	if id.Released != "" {
		m["released"] = id.Released
	}
	if id.Modified != "" {
		m["modified"] = id.Modified
	}
	return m
}

func localizedStringToMap(ls *LocalizedString) any {
	if ls == nil {
		return nil
	}
	if ls.Bare != "" {
		return ls.Bare
	}
	if len(ls.Langs) > 0 {
		m := make(map[string]any)
		for k, v := range ls.Langs {
			m[k] = v
		}
		return m
	}
	return nil
}

func repositoriesToMapList(repos []Repository) []map[string]any {
	out := make([]map[string]any, len(repos))
	for i, r := range repos {
		rm := map[string]any{keyURL: r.URL}
		if r.Type != "" {
			rm["type"] = r.Type
		}
		if r.Path != "" {
			rm["path"] = r.Path
		}
		if r.Branch != "" {
			rm["branch"] = r.Branch
		}
		if r.Issues {
			rm["issues"] = true
		}
		if r.Role != "" {
			rm["role"] = r.Role
		}
		out[i] = rm
	}
	return out
}

func licenseToMap(lic *License) map[string]any {
	m := map[string]any{keySpdx: lic.Spdx}
	if lic.Covers != "" {
		m["covers"] = lic.Covers
	}
	if lic.File != nil {
		m["file"] = lic.File
	}
	return m
}

func copyrightToMap(cp *Copyright) map[string]any {
	m := map[string]any{}
	if cp.Year != 0 {
		m["year"] = cp.Year
	}
	return m
}

func peopleToMapList(people []Person) []map[string]any {
	out := make([]map[string]any, len(people))
	for i, p := range people {
		m := map[string]any{}
		if p.FamilyNames != "" {
			m[keyFamilyNames] = p.FamilyNames
		}
		if p.GivenNames != "" {
			m[keyGivenNames] = p.GivenNames
		}
		if p.NameParticle != "" {
			m["name-particle"] = p.NameParticle
		}
		if p.NameSuffix != "" {
			m["name-suffix"] = p.NameSuffix
		}
		if p.Alias != "" {
			m["alias"] = p.Alias
		}
		if p.DisplayName != "" {
			m["display-name"] = p.DisplayName
		}
		if p.Email != "" {
			m[keyEmail] = p.Email
		}
		if p.URL != "" {
			m[keyURL] = p.URL
		}
		if p.Affiliation != "" {
			m["affiliation"] = p.Affiliation
		}
		if p.Orcid != "" {
			m[keyOrcid] = p.Orcid
		}
		if p.From != "" {
			m[keyFrom] = p.From
		}
		if p.To != "" {
			m["to"] = p.To
		}
		if len(p.Roles) > 0 {
			m[keyRoles] = p.Roles
		}
		if len(p.Handles) > 0 {
			m["handles"] = p.Handles
		}
		out[i] = m
	}
	return out
}

func organizationsToMapList(orgs []Organization) []map[string]any {
	out := make([]map[string]any, len(orgs))
	for i, o := range orgs {
		m := map[string]any{}
		if o.Name != "" {
			m[keyName] = o.Name
		}
		if o.Alias != "" {
			m["alias"] = o.Alias
		}
		if o.Email != "" {
			m[keyEmail] = o.Email
		}
		if o.URL != "" {
			m[keyURL] = o.URL
		}
		if o.Orcid != "" {
			m[keyOrcid] = o.Orcid
		}
		if o.From != "" {
			m[keyFrom] = o.From
		}
		if o.To != "" {
			m["to"] = o.To
		}
		if len(o.Roles) > 0 {
			m[keyRoles] = o.Roles
		}
		if len(o.Handles) > 0 {
			m["handles"] = o.Handles
		}
		out[i] = m
	}
	return out
}

func requirementsToMap(req *Requirements) map[string]any {
	m := map[string]any{}
	if len(req.OS) > 0 {
		m["operating-system"] = req.OS
	}
	if len(req.Arch) > 0 {
		m["arch"] = req.Arch
	}
	if req.Browsers != nil {
		m["browsers"] = req.Browsers
	}
	if len(req.Runtime) > 0 {
		m["runtime"] = req.Runtime
	}
	return m
}

func dependenciesToMap(deps *Dependencies) map[string]any {
	m := map[string]any{}
	if len(deps.Runtime) > 0 {
		m["runtime"] = deps.Runtime
	}
	if len(deps.Build) > 0 {
		m["build"] = deps.Build
	}
	if len(deps.Test) > 0 {
		m["test"] = deps.Test
	}
	return m
}

func linksToMapList(links []Link) []map[string]any {
	out := make([]map[string]any, len(links))
	for i, l := range links {
		m := map[string]any{
			keyType: l.Type,
			keyURL:  l.URL,
		}
		if l.Label != nil {
			m["label"] = localizedStringToMap(l.Label)
		}
		if l.Preferred {
			m["preferred"] = true
		}
		if l.Derived {
			m["derived"] = true
		}
		out[i] = m
	}
	return out
}
