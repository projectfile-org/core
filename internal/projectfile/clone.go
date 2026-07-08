// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

// Clone returns a deep-copy of the document so callers (notably the dry-run
// planning path in sync/core) can mutate without touching the original.
func (doc *Document) Clone() *Document {
	if doc == nil {
		return nil
	}
	cp := *doc
	cp.Identity = cloneIdentity(doc.Identity)
	cp.Repositories = cloneRepositories(doc.Repositories)
	cp.License = cloneLicense(doc.License)
	cp.Copyright = cloneCopyright(doc.Copyright)
	cp.People = clonePeople(doc.People)
	cp.Organizations = cloneOrganizations(doc.Organizations)
	cp.Keywords = cloneStrings(doc.Keywords)
	cp.Stack = cloneStrings(doc.Stack)
	cp.Requirements = cloneRequirements(doc.Requirements)
	cp.Dependencies = cloneDependencies(doc.Dependencies)
	cp.Links = cloneLinks(doc.Links)
	cp.Extensions = cloneAnyMap(doc.Extensions)
	cp.Rest = cloneAnyMap(doc.Rest)
	return &cp
}

func cloneIdentity(id Identity) Identity {
	id.Title = cloneLocalizedString(id.Title)
	id.Summary = cloneLocalizedString(id.Summary)
	id.Description = cloneLocalizedString(id.Description)
	return id
}

func cloneLocalizedString(ls *LocalizedString) *LocalizedString {
	if ls == nil {
		return nil
	}
	cp := *ls
	if ls.Langs != nil {
		cp.Langs = make(map[string]string, len(ls.Langs))
		for k, v := range ls.Langs {
			cp.Langs[k] = v
		}
	}
	return &cp
}

func cloneRepositories(in []Repository) []Repository {
	if in == nil {
		return nil
	}
	out := make([]Repository, len(in))
	copy(out, in)
	return out
}

func cloneLicense(l *License) *License {
	if l == nil {
		return nil
	}
	cp := *l
	cp.File = cloneAny(l.File)
	return &cp
}

func cloneCopyright(c *Copyright) *Copyright {
	if c == nil {
		return nil
	}
	cp := *c
	return &cp
}

func clonePeople(in []Person) []Person {
	if in == nil {
		return nil
	}
	out := make([]Person, len(in))
	for i, p := range in {
		cp := p
		cp.Roles = cloneStrings(p.Roles)
		cp.Handles = cloneAnyMap(p.Handles)
		out[i] = cp
	}
	return out
}

func cloneOrganizations(in []Organization) []Organization {
	if in == nil {
		return nil
	}
	out := make([]Organization, len(in))
	for i, o := range in {
		cp := o
		cp.Roles = cloneStrings(o.Roles)
		cp.Handles = cloneAnyMap(o.Handles)
		out[i] = cp
	}
	return out
}

func cloneRequirements(r *Requirements) *Requirements {
	if r == nil {
		return nil
	}
	cp := *r
	cp.OS = cloneStrings(r.OS)
	cp.Arch = cloneStrings(r.Arch)
	cp.Browsers = cloneAny(r.Browsers)
	if r.Runtime != nil {
		cp.Runtime = make(map[string]string, len(r.Runtime))
		for k, v := range r.Runtime {
			cp.Runtime[k] = v
		}
	}
	return &cp
}

func cloneDependencies(d *Dependencies) *Dependencies {
	if d == nil {
		return nil
	}
	return &Dependencies{
		Runtime: cloneStrings(d.Runtime),
		Build:   cloneStrings(d.Build),
		Test:    cloneStrings(d.Test),
	}
}

func cloneLinks(in []Link) []Link {
	if in == nil {
		return nil
	}
	out := make([]Link, len(in))
	for i, l := range in {
		cp := l
		cp.Label = cloneLocalizedString(l.Label)
		out[i] = cp
	}
	return out
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return cloneAnyMap(x)
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = cloneAny(vv)
		}
		return out
	default:
		return v
	}
}
