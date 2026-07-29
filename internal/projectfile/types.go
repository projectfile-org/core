// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

type Document struct {
	SpecVersion   string         `toml:"spec_version" yaml:"spec_version" json:"spec_version"`
	Schema        string         `toml:"$schema" yaml:"$schema" json:"$schema"`
	Kind          string         `toml:"kind" yaml:"kind" json:"kind"`
	Identity      Identity       `toml:"identity" yaml:"identity" json:"identity"`
	Repositories  []Repository   `toml:"repositories" yaml:"repositories" json:"repositories"`
	License       *License       `toml:"license" yaml:"license" json:"license"`
	Copyright     *Copyright     `toml:"copyright" yaml:"copyright" json:"copyright"`
	People        []Person       `toml:"people" yaml:"people" json:"people"`
	Organizations []Organization `toml:"organizations" yaml:"organizations" json:"organizations"`
	Keywords      []string       `toml:"keywords" yaml:"keywords" json:"keywords"`
	Stack         []string       `toml:"technologies" yaml:"technologies" json:"technologies"`
	Requirements  *Requirements  `toml:"requirements" yaml:"requirements" json:"requirements"`
	Includes      []string       `toml:"includes" yaml:"includes" json:"includes"`
	Dependencies  *Dependencies  `toml:"dependencies" yaml:"dependencies" json:"dependencies"`
	Links         []Link         `toml:"links" yaml:"links" json:"links"`
	Extensions    map[string]any `toml:"-" yaml:"-" json:"-"`
	Rest          map[string]any `toml:"-" yaml:"-" json:"-"`
}

type Identity struct {
	Namespace   string           `toml:"namespace" yaml:"namespace" json:"namespace"`
	Name        string           `toml:"name" yaml:"name" json:"name"`
	Version     string           `toml:"version" yaml:"version" json:"version"`
	Title       *LocalizedString `toml:"title" yaml:"title" json:"title"`
	Summary     *LocalizedString `toml:"summary" yaml:"summary" json:"summary"`
	Description *LocalizedString `toml:"description" yaml:"description" json:"description"`
	Created     string           `toml:"created" yaml:"created" json:"created"`
	Released    string           `toml:"released" yaml:"released" json:"released"`
	Modified    string           `toml:"modified" yaml:"modified" json:"modified"`
	// Extra preserves keys outside the spec-defined identity set, per §139
	// (§4 mappings accept additional keys; consumers MUST round-trip them).
	Extra map[string]any `toml:"-" yaml:"-" json:"-"`
}

type LocalizedString struct {
	Langs map[string]string `toml:"-" yaml:"-" json:"-"`
	Bare  string            `toml:"-" yaml:"-" json:"-"`
}

// Recommended link types (spec §5.11). Open vocabulary — these are the
// concrete strings every consumer-side resolver keys off. Anything outside
// this list is still a valid Type, but tooling will log "unknown type" at
// info level rather than route it through a known chain.
const (
	LinkHomepage       = "homepage"
	LinkSourceCode     = "source-code"
	LinkBugs           = "bugs"
	LinkDocumentation  = "documentation"
	LinkChangelog      = "changelog"
	LinkChat           = "chat"
	LinkForum          = "forum"
	LinkFAQ            = "faq"
	LinkWiki           = "wiki"
	LinkContact        = "contact"
	LinkTranslate      = "translate"
	LinkDonation       = "donation"
	LinkPackageReg     = "package-registry"
	LinkSecurityPolicy = "security-policy"
)

// Repository role constants. Open vocabulary per spec §5.3a, but these
// three values are the recommended set every consumer should understand.
const (
	RepositoryRoleOrigin  = "origin"
	RepositoryRoleMirror  = "mirror"
	RepositoryRoleArchive = "archive"
)

type Repository struct {
	URL    string `toml:"url" yaml:"url" json:"url"`
	Type   string `toml:"type" yaml:"type" json:"type"`
	Path   string `toml:"path" yaml:"path" json:"path"`
	Branch string `toml:"branch" yaml:"branch" json:"branch"`
	Issues bool   `toml:"issues" yaml:"issues" json:"issues"`
	Role   string `toml:"role" yaml:"role" json:"role"`
	// Extra preserves keys outside the spec-defined repository set, per §139.
	Extra map[string]any `toml:"-" yaml:"-" json:"-"`
}

type License struct {
	Spdx   string `toml:"spdx" yaml:"spdx" json:"spdx"`
	Covers string `toml:"covers" yaml:"covers" json:"covers"`
	File   any    `toml:"file" yaml:"file" json:"file"`
	// Extra preserves keys outside the spec-defined license set, per §139.
	Extra map[string]any `toml:"-" yaml:"-" json:"-"`
}

// Copyright carries the top-level [copyright] block. Year is the default
// start year for SPDX-style copyright lines when a person entry has no `from`.
type Copyright struct {
	Year int `toml:"year" yaml:"year" json:"year"`
	// Extra preserves keys outside the spec-defined copyright set, per §139.
	Extra map[string]any `toml:"-" yaml:"-" json:"-"`
}

type Person struct {
	FamilyNames  string         `toml:"family-names" yaml:"family-names" json:"family-names"`
	GivenNames   string         `toml:"given-names" yaml:"given-names" json:"given-names"`
	NameParticle string         `toml:"name-particle" yaml:"name-particle" json:"name-particle"`
	NameSuffix   string         `toml:"name-suffix" yaml:"name-suffix" json:"name-suffix"`
	Alias        string         `toml:"alias" yaml:"alias" json:"alias"`
	DisplayName  string         `toml:"display-name" yaml:"display-name" json:"display-name"`
	Email        string         `toml:"email" yaml:"email" json:"email"`
	URL          string         `toml:"url" yaml:"url" json:"url"`
	Handles      map[string]any `toml:"handles" yaml:"handles" json:"handles"`
	Affiliation  string         `toml:"affiliation" yaml:"affiliation" json:"affiliation"`
	Orcid        string         `toml:"orcid" yaml:"orcid" json:"orcid"`
	From         string         `toml:"from" yaml:"from" json:"from"`
	To           string         `toml:"to" yaml:"to" json:"to"`
	Roles        []string       `toml:"roles" yaml:"roles" json:"roles"`
	// Extra preserves keys outside the spec-defined person set, per §139.
	Extra map[string]any `toml:"-" yaml:"-" json:"-"`
}

type Organization struct {
	Name    string         `toml:"name" yaml:"name" json:"name"`
	Alias   string         `toml:"alias" yaml:"alias" json:"alias"`
	Email   string         `toml:"email" yaml:"email" json:"email"`
	URL     string         `toml:"url" yaml:"url" json:"url"`
	Handles map[string]any `toml:"handles" yaml:"handles" json:"handles"`
	Orcid   string         `toml:"orcid" yaml:"orcid" json:"orcid"`
	From    string         `toml:"from" yaml:"from" json:"from"`
	To      string         `toml:"to" yaml:"to" json:"to"`
	Roles   []string       `toml:"roles" yaml:"roles" json:"roles"`
	// Extra preserves keys outside the spec-defined organization set, per §139.
	Extra map[string]any `toml:"-" yaml:"-" json:"-"`
}

// Role* are the [[people]].roles values the CLI filters on. Names match
// spec/v1.md §5.5.4. Generators that want a single contact for an artefact
// resolve via pfmodel.ContactEmail(doc, role) — specific role first, then
// maintainer, then any person with email.
const (
	RoleCopyright  = "copyright"  // LICENSE / REUSE copyright lines
	RoleMaintainer = "maintainer" // generic fallback for single-contact artefacts
	RoleSecurity   = "security"   // SECURITY.md contact
	RoleCommunity  = "community"  // CODE_OF_CONDUCT.md enforcement contact
)

type Requirements struct {
	OS       []string          `toml:"operating-system" yaml:"operating-system" json:"operating-system"`
	Arch     []string          `toml:"arch" yaml:"arch" json:"arch"`
	Browsers any               `toml:"browsers" yaml:"browsers" json:"browsers"`
	Runtime  map[string]string `toml:"runtime" yaml:"runtime" json:"runtime"`
	// Extra preserves keys outside the spec-defined requirements set, per §139.
	Extra map[string]any `toml:"-" yaml:"-" json:"-"`
}

type Dependencies struct {
	Runtime []string `toml:"runtime" yaml:"runtime" json:"runtime"`
	Build   []string `toml:"build" yaml:"build" json:"build"`
	Test    []string `toml:"test" yaml:"test" json:"test"`
	// Extra preserves keys outside the spec-defined dependencies set, per §139.
	Extra map[string]any `toml:"-" yaml:"-" json:"-"`
}

type Link struct {
	Type      string           `toml:"type" yaml:"type" json:"type"`
	URL       string           `toml:"url" yaml:"url" json:"url"`
	Label     *LocalizedString `toml:"label" yaml:"label" json:"label"`
	Preferred bool             `toml:"preferred" yaml:"preferred" json:"preferred"`
	Derived   bool             `toml:"derived" yaml:"derived" json:"derived"`
	// Extra preserves keys outside the spec-defined link set, per §139.
	Extra map[string]any `toml:"-" yaml:"-" json:"-"`
}
