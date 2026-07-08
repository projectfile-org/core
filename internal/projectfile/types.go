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
}

type License struct {
	Spdx   string `toml:"spdx" yaml:"spdx" json:"spdx"`
	Covers string `toml:"covers" yaml:"covers" json:"covers"`
	File   any    `toml:"file" yaml:"file" json:"file"`
}

// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
// Copyright carries the top-level [copyright] block. Year is the default
//
// SPDX-License-Identifier: MIT
type Copyright struct {
	Year int `toml:"year" yaml:"year" json:"year"`
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
}

// Role* are the [[people]].roles values the CLI filters on. Names match
// spec/v1.md §5.5.4. Generators that want a single contact for an artefact
// resolve via projectfile.ContactEmail(doc, role) — specific role first, then
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
}

type Dependencies struct {
	Runtime []string `toml:"runtime" yaml:"runtime" json:"runtime"`
	Build   []string `toml:"build" yaml:"build" json:"build"`
	Test    []string `toml:"test" yaml:"test" json:"test"`
}

type Link struct {
	Type      string           `toml:"type" yaml:"type" json:"type"`
	URL       string           `toml:"url" yaml:"url" json:"url"`
	Label     *LocalizedString `toml:"label" yaml:"label" json:"label"`
	Preferred bool             `toml:"preferred" yaml:"preferred" json:"preferred"`
	Derived   bool             `toml:"derived" yaml:"derived" json:"derived"`
}

type CitationExtension struct {
	DOI       string             `toml:"doi" yaml:"doi" json:"doi"`
	Message   string             `toml:"message" yaml:"message" json:"message"`
	Preferred *PreferredCitation `toml:"preferred" yaml:"preferred" json:"preferred"`
}

// IgnoresExtension binds the proposed `org.projectfile.ignores` namespace.
// See specification/spec/shapes/org.projectfile.ignores.toml.
//
// This namespace is for gitignore-style FILE-PATTERN ignores. Suppressed
// vulnerability IDs (any vuln-db format: CVE, GHSA, GO-*, etc.) live in the
// tool-agnostic `org.projectfile.vulnerabilities` namespace instead, which
// fans the same
// list out to every scanner ignore-file (.trivyignore, .grype.yaml,
// osv-scanner.toml) — see VulnerabilitiesExtension.
type IgnoresExtension struct {
	Generate  []string              `toml:"generate"  yaml:"generate"  json:"generate"`
	Extra     []string              `toml:"extra"     yaml:"extra"     json:"extra"`
	Git       *IgnoreTargetOverride `toml:"git"       yaml:"git"       json:"git"`
	Docker    *IgnoreTargetOverride `toml:"docker"    yaml:"docker"    json:"docker"`
	Npm       *IgnoreTargetOverride `toml:"npm"       yaml:"npm"       json:"npm"`
	Claude    *IgnoreTargetOverride `toml:"claude"    yaml:"claude"    json:"claude"`
	Container *IgnoreTargetOverride `toml:"container" yaml:"container" json:"container"`
}

type IgnoreTargetOverride struct {
	Include []string `toml:"include" yaml:"include" json:"include"`
	Exclude []string `toml:"exclude" yaml:"exclude" json:"exclude"`
}

// VulnerabilitiesExtension binds the `org.projectfile.vulnerabilities`
// namespace — a tool-agnostic list of suppressed vulnerability IDs. Each entry
// fans out to every enabled scanner bridge: .trivyignore (trivy),
// .grype.yaml (grype), osv-scanner.toml (osv). One source of truth, consumed
// by every scanner the project runs. See
// specification/spec/shapes/org.projectfile.vulnerabilities.yaml.
type VulnerabilitiesExtension struct {
	// Generate selects which scanner ignore files to emit. Keys are the
	// scanner extKeys in the bridge target table ("trivy", "grype", "osv").
	// When unset, every scanner that has at least one suppressed ID gets a
	// file — the "specify once, fan out everywhere" default.
	Generate []string `toml:"generate" yaml:"generate" json:"generate"`
	// Suppress is the tool-agnostic list of vulnerability IDs (any format:
	// CVE/GHSA/GO-*, ...) to drop from every scanner's report.
	Suppress []VulnerabilitySuppress `toml:"suppress" yaml:"suppress" json:"suppress"`
}

// VulnerabilitySuppress is one suppressed vulnerability. Reason is OPTIONAL
// and surfaced in the scanner formats that support it (grype, osv); it is
// dropped from the plain-line .trivyignore dialect.
type VulnerabilitySuppress struct {
	ID     string `toml:"id" yaml:"id" json:"id"`
	Reason string `toml:"reason" yaml:"reason" json:"reason"`
}

// EditorsExtension binds the `org.projectfile.editors` namespace.
// Editors are distinct from stack — they describe developer tooling
// preferences, not project technology.
type EditorsExtension struct {
	Use []string `toml:"use" yaml:"use" json:"use"`
}

// FundingChannel carries a payment channel for FundingJSON bridges.
// Maps to fundingjson.org channels[] items.
type FundingChannel struct {
	GUID        string `toml:"guid" yaml:"guid" json:"guid"`
	Type        string `toml:"type" yaml:"type" json:"type"`
	Address     string `toml:"address" yaml:"address" json:"address"`
	Description string `toml:"description" yaml:"description" json:"description"`
}

// FundingPlan carries a funding plan for FundingJSON bridges.
// Maps to fundingjson.org plans[] items.
type FundingPlan struct {
	GUID        string   `toml:"guid" yaml:"guid" json:"guid"`
	Status      string   `toml:"status" yaml:"status" json:"status"`
	Name        string   `toml:"name" yaml:"name" json:"name"`
	Description string   `toml:"description" yaml:"description" json:"description"`
	Amount      float64  `toml:"amount" yaml:"amount" json:"amount"`
	Currency    string   `toml:"currency" yaml:"currency" json:"currency"`
	Frequency   string   `toml:"frequency" yaml:"frequency" json:"frequency"`
	Channels    []string `toml:"channels" yaml:"channels" json:"channels"`
}

// FundingHistory carries a yearly funding summary for FundingJSON bridges.
// Maps to fundingjson.org history[] items.
type FundingHistory struct {
	Year        int     `toml:"year" yaml:"year" json:"year"`
	Income      float64 `toml:"income" yaml:"income" json:"income"`
	Expenses    float64 `toml:"expenses" yaml:"expenses" json:"expenses"`
	Taxes       float64 `toml:"taxes" yaml:"taxes" json:"taxes"`
	Currency    string  `toml:"currency" yaml:"currency" json:"currency"`
	Description string  `toml:"description" yaml:"description" json:"description"`
}

// FundingExtension binds `[org.projectfile.funding]`. The set of providers
// follows GitHub's FUNDING.yml schema 1:1 — including the later additions
// (`lfx-crowdfunding`, `polar`, `buy-me-a-coffee`, `thanks-dev`) — so the
// generator can emit anything the spec §5.7 vocabulary names. Path is the
// on-disk override (default `.github/FUNDING.yml`) so users on non-GitHub
// forges can drop the same file under `docs/` or wherever their CI looks.
//
// Channels, Plans, and History extend the extension for FundingJSON-compatible
// bridges. Bridges targeting formats that don't support plans (GitHub
// FUNDING.yml, package.json funding, etc.) ignore these fields.
type FundingExtension struct {
	GitHub          []string         `toml:"github"           yaml:"github"           json:"github"`
	Patreon         string           `toml:"patreon"          yaml:"patreon"          json:"patreon"`
	KoFi            string           `toml:"ko-fi"            yaml:"ko-fi"            json:"ko-fi"`
	Liberapay       string           `toml:"liberapay"        yaml:"liberapay"        json:"liberapay"`
	Tidelift        string           `toml:"tidelift"         yaml:"tidelift"         json:"tidelift"`
	CommunityBridge string           `toml:"community-bridge" yaml:"community-bridge" json:"community-bridge"`
	IssueHunt       string           `toml:"issuehunt"        yaml:"issuehunt"        json:"issuehunt"`
	OpenCollective  string           `toml:"open-collective"  yaml:"open-collective"  json:"open-collective"`
	LFXCrowdfunding string           `toml:"lfx-crowdfunding" yaml:"lfx-crowdfunding" json:"lfx-crowdfunding"`
	Polar           string           `toml:"polar"            yaml:"polar"            json:"polar"`
	BuyMeACoffee    string           `toml:"buy-me-a-coffee"  yaml:"buy-me-a-coffee"  json:"buy-me-a-coffee"`
	ThanksDev       string           `toml:"thanks-dev"       yaml:"thanks-dev"       json:"thanks-dev"`
	Custom          []string         `toml:"custom"           yaml:"custom"           json:"custom"`
	Path            string           `toml:"path"             yaml:"path"             json:"path"`
	EntityType      string           `toml:"entity-type"      yaml:"entity-type"      json:"entity-type"`
	EntityRole      string           `toml:"entity-role"      yaml:"entity-role"      json:"entity-role"`
	Channels        []FundingChannel `toml:"channels" yaml:"channels" json:"channels"`
	Plans           []FundingPlan    `toml:"plans" yaml:"plans" json:"plans"`
	History         []FundingHistory `toml:"history" yaml:"history" json:"history"`
}

// SecurityExtension binds `[org.projectfile.security]`. Empty strings/slices
// signal "section absent in output" — the template omits the corresponding
// block rather than emitting an empty stub.
type SecurityExtension struct {
	Contact           string   `toml:"contact"            yaml:"contact"            json:"contact"`
	ReportURL         string   `toml:"report-url"         yaml:"report-url"         json:"report-url"`
	SupportedVersions []string `toml:"supported-versions" yaml:"supported-versions" json:"supported-versions"`
	DisclosureWindow  string   `toml:"disclosure-window"  yaml:"disclosure-window"  json:"disclosure-window"`
	GPGKey            string   `toml:"gpg-key"            yaml:"gpg-key"            json:"gpg-key"`
	BugBountyURL      string   `toml:"bug-bounty-url"     yaml:"bug-bounty-url"     json:"bug-bounty-url"`
}

// CodeOfConductExtension binds `[org.projectfile.code-of-conduct]`.
// Covenant names which covenant the project adopts (default Contributor
// Covenant 2.1 when the namespace is absent); Scope narrows where it applies.
// Enforcement contacts stay in the top-level people array (role: community).
type CodeOfConductExtension struct {
	Covenant string `toml:"covenant" yaml:"covenant" json:"covenant"`
	Scope    string `toml:"scope"    yaml:"scope"    json:"scope"`
}

// ContributingExtension binds `[org.projectfile.contributing]`. Sections is
// the ordered section list the template loops over; the default set (see
// helpers.GetContributingExtension) covers the standard overview/setup/...
// flow that contributors expect.
//
// recommend-to-star and recommend-to-follow share the Toggle grammar
// (bool | map[string]bool), documented on projectfile.Toggle. Both default
// OFF (unset -> nothing is recommended):
//
//   - recommend-to-star: set true to show a star line for every forge, or a
//     per-host map to show only the named forges (keys are link hosts:
//     github.com, codeberg.org, ...). Unset shows no star lines.
//   - recommend-to-follow: set true to show all author follow/site/social
//     and funding lines, or a per-platform map to show only the named
//     contacts (keys are handle platforms: mastodon, github, codeberg,
//     bluesky, linkedin, rss; plus "site" for author websites and
//     "funding" for the funding line). Unset shows none.
type ContributingExtension struct {
	Sections          []string `toml:"sections" yaml:"sections" json:"sections"`
	CLAURL            string   `toml:"cla-url"  yaml:"cla-url"  json:"cla-url"`
	ChatURL           string   `toml:"chat-url" yaml:"chat-url" json:"chat-url"`
	CoCURL            string   `toml:"coc-url"  yaml:"coc-url"  json:"coc-url"`
	RecommendToFollow Toggle   `toml:"recommend-to-follow" yaml:"recommend-to-follow" json:"recommend-to-follow"`
	RecommendToStar   Toggle   `toml:"recommend-to-star"   yaml:"recommend-to-star"   json:"recommend-to-star"`
}

// SupportExtension binds `[org.projectfile.support]`. Community support URLs
// (docs, wiki, faq, forum, chat, bugs) live in top-level links[]; the
// extension carries only the fields that have no reserved-field home:
// response-time SLA and the end-of-life version table. Paid-support and
// status-page URLs go in links[type=paid-support] and
// links[type=status-page] respectively.
type SupportExtension struct {
	ResponseTime string     `toml:"response-time" yaml:"response-time" json:"response-time"`
	EOL          []EOLEntry `toml:"eol"           yaml:"eol"           json:"eol"`
}

// EOLEntry carries an end-of-life date for a version series.
type EOLEntry struct {
	Version string `toml:"version" yaml:"version" json:"version"`
	Date    string `toml:"date"    yaml:"date"    json:"date"`
}

// ReleaseExtension binds `[org.projectfile.release]`. It carries engine-agnostic
// release INTENT — tag format, changelog style, and the branch-to-channel map —
// consumed by semantic-release, release-please, and similar automation. The
// pf-cli `.releaserc.yaml` renderer lowers it to a semantic-release config; the
// engine-specific plugin scaffolding lives in that renderer, never here (the
// model stays abstract — no "semantic-release" vocabulary in the spec layer).
type ReleaseExtension struct {
	TagFormat string          `toml:"tag-format" yaml:"tag-format" json:"tag-format"`
	Changelog string          `toml:"changelog"  yaml:"changelog"  json:"changelog"`
	Branches  []ReleaseBranch `toml:"branches"   yaml:"branches"   json:"branches"`
}

// ReleaseBranch is one entry in the release branch-to-channel map. Pattern is
// the branch name or glob that triggers a release; Channel is the distribution
// channel (dist-tag / release label); Prerelease marks the channel as a
// pre-release stream.
type ReleaseBranch struct {
	Pattern    string `toml:"pattern"    yaml:"pattern"    json:"pattern"`
	Channel    string `toml:"channel"    yaml:"channel"    json:"channel"`
	Prerelease bool   `toml:"prerelease" yaml:"prerelease" json:"prerelease"`
}

// ConventionsExtension binds `[org.projectfile.conventions]`. Cross-cutting
// workflow conventions read by multiple generators (contributing, release).
// Per-language overrides live under the matching stack-tag key.
type ConventionsExtension struct {
	CommitStyle   string
	Workflow      string
	StyleGuideURL string
	Languages     map[string]LangConventions
}

// LangConventions holds per-stack-tag convention overrides.
type LangConventions struct {
	StyleGuideURL string
}

// CLIExtension binds `[org.projectfile.cli]`. Derive toggles control per-source
// inference passes (forges, registries). All default to true; setting forges =
// false (or registries = false) disables that whole pass.
//
// Derived is kept for internal ownership tracking only — it is no longer
// serialised to the output document. Each derived link carries derived:true
// directly on the link entry instead.
//
// See spec/shapes/org.projectfile.cli.yaml for the on-disk shape.
type CLIExtension struct {
	Derived []string         `toml:"-" yaml:"-" json:"-"`
	Derive  CLIDeriveToggles `toml:"derive" yaml:"derive" json:"derive"`
}

// CLIDeriveToggles enables/disables individual inference passes. Defaults
// matter: when the extension is absent altogether, both passes run. When
// the extension is present but a toggle is omitted, the toggle is implicitly
// true (this matches "convention over configuration").
type CLIDeriveToggles struct {
	Forges     bool `toml:"forges"     yaml:"forges"     json:"forges"`
	Registries bool `toml:"registries" yaml:"registries" json:"registries"`
}

// ForgeExtension binds `[org.projectfile.forge]`. Four responsibilities:
//
//   - Push is the global kill-switch (default true). Setting it false skips
//     every forge in a single shot; useful for repos hosted on forges whose
//     web UI is the source of truth.
//   - Fields toggles per-field push at the namespace level (description,
//     homepage, topics). All default to true — convention over configuration.
//   - Hosts maps a literal hostname to a per-host opt-out (default true). The
//     entry need not be removed from top-level repositories[]; flipping the
//     toggle to false skips that host without losing the URL.
//   - Kinds maps a literal hostname to the forge kind running on it. This is
//     how a self-hosted Forgejo / Gitea on a bare hostname (e.g.
//     code.example.com, no "gitea." / "forgejo." prefix) declares itself —
//     hostmatch.Resolve can't guess. Takes precedence over hostmatch on
//     conflict, so it doubles as a manual override for misclassified hosts.
//
// See spec/shapes/org.projectfile.forge.toml for the on-disk shape.
type ForgeExtension struct {
	Push   bool               `toml:"push"   yaml:"push"   json:"push"`
	Fields ForgeFieldsToggles `toml:"fields" yaml:"fields" json:"fields"`
	Hosts  map[string]bool    `toml:"hosts"  yaml:"hosts"  json:"hosts"`
	Kinds  map[string]string  `toml:"kinds"  yaml:"kinds"  json:"kinds"`
}

// ForgeFieldsToggles enables/disables individual fields the push command
// would otherwise sync. All default to true: a present extension without
// the toggle still leaves the field on.
type ForgeFieldsToggles struct {
	Description bool `toml:"description" yaml:"description" json:"description"`
	Homepage    bool `toml:"homepage"    yaml:"homepage"    json:"homepage"`
	Topics      bool `toml:"topics"      yaml:"topics"      json:"topics"`
}

// CodeOwnersExtension binds `[org.projectfile.codeowners]`. Entries is an
// ordered list because CODEOWNERS semantics are "last match wins per path";
// reordering changes meaning.
type CodeOwnersExtension struct {
	Entries []CodeOwnersEntry `toml:"entries" yaml:"entries" json:"entries"`
}

type CodeOwnersEntry struct {
	Pattern string   `toml:"pattern" yaml:"pattern" json:"pattern"`
	Owners  []string `toml:"owners"  yaml:"owners"  json:"owners"`
}

// ReadmeExtension binds `[org.projectfile.readme]`. Blocks is the ordered
// list of named template blocks that compose the README; when absent the
// bridge uses its built-in default list. Extras carries inline content
// blocks that do not warrant a dedicated template file.
type ReadmeExtension struct {
	Blocks []string
	Extras []ReadmeExtra
}

// ReadmeExtra is an inline content block referenced by name in Blocks.
type ReadmeExtra struct {
	Name    string
	Content string
}

type PreferredCitation struct {
	Type    string `toml:"type" yaml:"type" json:"type"`
	Title   string `toml:"title" yaml:"title" json:"title"`
	Journal string `toml:"journal" yaml:"journal" json:"journal"`
	Volume  int    `toml:"volume" yaml:"volume" json:"volume"`
	Issue   int    `toml:"issue" yaml:"issue" json:"issue"`
	Pages   string `toml:"pages" yaml:"pages" json:"pages"`
	Year    int    `toml:"year" yaml:"year" json:"year"`
	DOI     string `toml:"doi" yaml:"doi" json:"doi"`
}
