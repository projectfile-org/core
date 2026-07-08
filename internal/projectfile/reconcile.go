// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import "reflect"

// ReconcileBase applies only the mutations captured between preSync and
// postSync to base, so include-inherited values are never materialised into
// the base file on write-back.
//
// The bridge sync, derive, and fill-required-fields commands read the MERGED
// document (includes resolved) so mappers see every effective value.  But
// when they write the projectfile back, they must write the BASE document —
// otherwise every field an include contributed gets baked into the base file,
// defeating the purpose of includes.
//
// preSync  — snapshot of the merged document BEFORE the operation ran.
// postSync — the same merged document AFTER  the operation mutated it.
// base     — the base-only document (ReadBaseFromPath) to be written to disk.
//
// Principle: a field that changed between preSync and postSync was touched by
// the operation (the external file or derive engine provided a value).  That
// value is local data and lands in base.  A field that did NOT change was
// either base-local already or include-inherited — either way it stays
// untouched in base so include data is never duplicated.
func ReconcileBase(base, preSync, postSync *Document) {
	if base == nil || preSync == nil || postSync == nil {
		return
	}

	reconcileIdentity(&base.Identity, preSync.Identity, postSync.Identity)
	reconcileLicense(base, preSync.License, postSync.License)
	reconcileCopyright(base, preSync.Copyright, postSync.Copyright)
	reconcileRequirements(base, preSync.Requirements, postSync.Requirements)
	reconcileDependencies(base, preSync.Dependencies, postSync.Dependencies)

	if !reflect.DeepEqual(preSync.Keywords, postSync.Keywords) {
		base.Keywords = postSync.Keywords
	}
	if !reflect.DeepEqual(preSync.Stack, postSync.Stack) {
		base.Stack = postSync.Stack
	}

	base.People = reconcilePeople(base.People, preSync.People, postSync.People)
	base.Organizations = reconcileOrgs(base.Organizations, preSync.Organizations, postSync.Organizations)
	base.Repositories = reconcileRepos(base.Repositories, preSync.Repositories, postSync.Repositories)
	base.Links = reconcileLinks(base.Links, preSync.Links, postSync.Links)

	reconcileExtensions(base, preSync.Extensions, postSync.Extensions)
}

// --- Identity ---

func reconcileIdentity(dst *Identity, pre, post Identity) {
	setIfChanged(&dst.Namespace, pre.Namespace, post.Namespace)
	setIfChanged(&dst.Name, pre.Name, post.Name)
	setIfChanged(&dst.Version, pre.Version, post.Version)
	setIfChanged(&dst.Created, pre.Created, post.Created)
	setIfChanged(&dst.Released, pre.Released, post.Released)
	setIfChanged(&dst.Modified, pre.Modified, post.Modified)
	if !reflect.DeepEqual(pre.Title, post.Title) {
		dst.Title = post.Title
	}
	if !reflect.DeepEqual(pre.Summary, post.Summary) {
		dst.Summary = post.Summary
	}
	if !reflect.DeepEqual(pre.Description, post.Description) {
		dst.Description = post.Description
	}
}

func setIfChanged(dst *string, pre, post string) {
	if pre != post {
		*dst = post
	}
}

// --- License / Copyright ---

func reconcileLicense(base *Document, pre, post *License) {
	if !reflect.DeepEqual(pre, post) && post != nil {
		base.License = post
	}
}

func reconcileCopyright(base *Document, pre, post *Copyright) {
	if !reflect.DeepEqual(pre, post) && post != nil {
		base.Copyright = post
	}
}

// --- Requirements / Dependencies (per sub-field) ---

func reconcileRequirements(base *Document, pre, post *Requirements) {
	if reflect.DeepEqual(pre, post) {
		return
	}
	if post == nil {
		return
	}
	if base.Requirements == nil {
		base.Requirements = &Requirements{}
	}
	if !reflect.DeepEqual(reqOS(pre), post.OS) {
		base.Requirements.OS = post.OS
	}
	if !reflect.DeepEqual(reqArch(pre), post.Arch) {
		base.Requirements.Arch = post.Arch
	}
	if !reflect.DeepEqual(reqBrowsers(pre), post.Browsers) {
		base.Requirements.Browsers = post.Browsers
	}
	if !reflect.DeepEqual(reqRuntime(pre), post.Runtime) {
		base.Requirements.Runtime = post.Runtime
	}
}

func reconcileDependencies(base *Document, pre, post *Dependencies) {
	if reflect.DeepEqual(pre, post) {
		return
	}
	if post == nil {
		return
	}
	if base.Dependencies == nil {
		base.Dependencies = &Dependencies{}
	}
	if !reflect.DeepEqual(depRuntime(pre), post.Runtime) {
		base.Dependencies.Runtime = post.Runtime
	}
	if !reflect.DeepEqual(depBuild(pre), post.Build) {
		base.Dependencies.Build = post.Build
	}
	if !reflect.DeepEqual(depTest(pre), post.Test) {
		base.Dependencies.Test = post.Test
	}
}

func reqOS(r *Requirements) []string {
	if r == nil {
		return nil
	}
	return r.OS
}

func reqArch(r *Requirements) []string {
	if r == nil {
		return nil
	}
	return r.Arch
}

func reqBrowsers(r *Requirements) any {
	if r == nil {
		return nil
	}
	return r.Browsers
}

func reqRuntime(r *Requirements) map[string]string {
	if r == nil {
		return nil
	}
	return r.Runtime
}

func depRuntime(d *Dependencies) []string {
	if d == nil {
		return nil
	}
	return d.Runtime
}

func depBuild(d *Dependencies) []string {
	if d == nil {
		return nil
	}
	return d.Build
}

func depTest(d *Dependencies) []string {
	if d == nil {
		return nil
	}
	return d.Test
}

// --- Slice reconciliation (people, orgs, repos, links) ---
//
// MergePeople / EnsurePrimaryRepository / SetLink operate on the merged
// document, so postSync may differ from preSync in two ways:
//
//  1. Existing entries were modified (e.g. a role was added, a URL updated).
//  2. New entries were appended (incoming-only identities from the ext file).
//
// For (1) we search base by the PRE-sync identity key — if the entry exists in
// base it gets the updated value; if it was include-only it is silently
// skipped (the include still carries it).
// For (2) we append the new entries to base, deduplicating by identity key
// so an entry that matches a base-only person is not duplicated.

func reconcilePeople(base, pre, post []Person) []Person {
	if reflect.DeepEqual(pre, post) {
		return base
	}
	out := append([]Person(nil), base...)

	// Modified entries: indices 0..minLen-1 that differ.
	minLen := len(pre)
	if len(post) < minLen {
		minLen = len(post)
	}
	for i := range minLen {
		if reflect.DeepEqual(pre[i], post[i]) {
			continue
		}
		// Search by the PRE-sync identity — the sync may have added an
		// email or ORCID that changes the key, but the base entry still
		// carries the old identity.  samePerson handles tiered matching.
		for j := range out {
			if samePerson(out[j], pre[i]) || samePerson(out[j], post[i]) {
				out[j] = post[i]
				break
			}
		}
	}

	// New entries: indices len(pre).. — appended by MergePeople.
	if len(post) > len(pre) {
		merged, _ := MergePeople(out, post[len(pre):])
		out = merged
	}
	return out
}

func reconcileOrgs(base, pre, post []Organization) []Organization {
	if reflect.DeepEqual(pre, post) {
		return base
	}
	out := append([]Organization(nil), base...)

	minLen := len(pre)
	if len(post) < minLen {
		minLen = len(post)
	}
	for i := range minLen {
		if reflect.DeepEqual(pre[i], post[i]) {
			continue
		}
		for j := range out {
			if sameOrg(out[j], pre[i]) || sameOrg(out[j], post[i]) {
				out[j] = post[i]
				break
			}
		}
	}

	if len(post) > len(pre) {
		merged, _ := MergeOrganizations(out, post[len(pre):])
		out = merged
	}
	return out
}

func reconcileRepos(base, pre, post []Repository) []Repository {
	if reflect.DeepEqual(pre, post) {
		return base
	}
	out := append([]Repository(nil), base...)

	minLen := len(pre)
	if len(post) < minLen {
		minLen = len(post)
	}
	for i := range minLen {
		if reflect.DeepEqual(pre[i], post[i]) {
			continue
		}
		// Search base by the PRE-sync URL so a URL change is detected.
		for j := range out {
			if out[j].URL == pre[i].URL {
				out[j] = post[i]
				break
			}
		}
	}

	for _, r := range post[len(pre):] {
		if !repoURLExists(out, r.URL) {
			out = append(out, r)
		}
	}
	return out
}

func reconcileLinks(base, pre, post []Link) []Link {
	if reflect.DeepEqual(pre, post) {
		return base
	}
	out := append([]Link(nil), base...)

	minLen := len(pre)
	if len(post) < minLen {
		minLen = len(post)
	}
	for i := range minLen {
		if reflect.DeepEqual(pre[i], post[i]) {
			continue
		}
		for j := range out {
			if out[j].Type == pre[i].Type && out[j].URL == pre[i].URL {
				out[j] = post[i]
				break
			}
		}
	}

	for _, l := range post[len(pre):] {
		if !linkExists(out, l.Type, l.URL) {
			out = append(out, l)
		}
	}
	return out
}

// --- Extensions ---

func reconcileExtensions(base *Document, pre, post map[string]any) {
	if reflect.DeepEqual(pre, post) {
		return
	}
	for ns, postVal := range post {
		preVal, existed := pre[ns]
		if !existed || !reflect.DeepEqual(preVal, postVal) {
			if base.Extensions == nil {
				base.Extensions = map[string]any{}
			}
			// We use SetExtension to prune any nested-map form so the
			// serialiser does not emit duplicate sections.
			SetExtension(base, ns, postVal)
		}
	}
}

// --- helpers ---

func repoURLExists(repos []Repository, url string) bool {
	for _, r := range repos {
		if r.URL == url {
			return true
		}
	}
	return false
}

func linkExists(links []Link, linkType, url string) bool {
	for _, l := range links {
		if l.Type == linkType && l.URL == url {
			return true
		}
	}
	return false
}
