package projectfile

import (
	"fmt"
	"strings"
	"time"
)

func ExtractLocalizedString(ls *LocalizedString) string {
	if ls == nil {
		return ""
	}
	if ls.Bare != "" {
		return ls.Bare
	}
	if v, ok := ls.Langs["en"]; ok && v != "" {
		return v
	}
	for _, v := range ls.Langs {
		if v != "" {
			return v
		}
	}
	return ""
}

// SetLocalizedEN writes val into the English position of a LocalizedString.
// If the document was using the Bare form (or the field is nil), the result
// stays Bare — matching the file's existing style. If it was using Langs
// already, the "en" key is updated. Without this helper, mappers that wrote
// to Langs["en"] silently lost their write when the field was Bare, because
// the serializer prefers Bare.
func SetLocalizedEN(ls **LocalizedString, val string) {
	if *ls == nil {
		*ls = &LocalizedString{Bare: val}
		return
	}
	if len((*ls).Langs) == 0 {
		(*ls).Bare = val
		return
	}
	(*ls).Langs["en"] = val
	(*ls).Bare = ""
}

// Reverse-DNS namespace constants. Every helper below routes through
// LookupExtension so TOML dotted headers ([org.projectfile.X]) and literal
// flat keys (YAML/JSON, or quoted TOML) both work uniformly.
const (
	CIExtensionNS              = "org.projectfile.ci"
	IgnoresExtensionNS         = "org.projectfile.ignores"
	EditorsExtensionNS         = "org.projectfile.editors"
	VulnerabilitiesExtensionNS = "org.projectfile.vulnerabilities"
	FundingExtensionNS         = "org.projectfile.funding"
	SecurityExtensionNS        = "org.projectfile.security"
	CodeOfConductExtensionNS   = "org.projectfile.code-of-conduct"
	ContributingExtensionNS    = "org.projectfile.contributing"
	CodeOwnersExtensionNS      = "org.projectfile.codeowners"
	CLIExtensionNS             = "org.projectfile.cli"
	ForgeExtensionNS           = "org.projectfile.forge"
	ConventionsExtensionNS     = "org.projectfile.conventions"
	SupportExtensionNS         = "org.projectfile.support"
	ReadmeExtensionNS          = "org.projectfile.readme"
	ReleaseExtensionNS         = "org.projectfile.release"
)

// SetExtension writes value into doc.Extensions for the given reverse-DNS
// namespace, after removing any nested-map form (the parse result of a TOML
// dotted-table header like `[org.projectfile.X]`). Writing under the flat key
// is what every other driver does; without the pre-prune the serialiser
// would emit BOTH the original dotted header AND the new quoted-dotted key
// on first sync — duplicate sections.
//
// Side-effect: a hand-written `[org.projectfile.X]` dotted header is
// rewritten as `["org.projectfile.X"]` on first sync. Both are valid TOML
// and round-trip identically; the visual change is one-time.
func SetExtension(doc *Document, ns string, value any) {
	if doc == nil || ns == "" {
		return
	}
	if doc.Extensions == nil {
		doc.Extensions = map[string]any{}
	}
	pruneNestedExtension(doc.Extensions, ns)
	doc.Extensions[ns] = value
}

// pruneNestedExtension walks the dotted-segment path under extMap and removes
// the leaf, then prunes any intermediate maps left empty. No-op when the
// nested form does not exist (which is the common case after the first sync).
func pruneNestedExtension(extMap map[string]any, ns string) {
	segments := strings.Split(ns, ".")
	if len(segments) < 2 {
		return
	}
	head, ok := extMap[segments[0]].(map[string]any)
	if !ok {
		return
	}
	chain := []map[string]any{head}
	parent := head
	for _, seg := range segments[1 : len(segments)-1] {
		next, ok := parent[seg].(map[string]any)
		if !ok {
			return
		}
		chain = append(chain, next)
		parent = next
	}
	leaf := segments[len(segments)-1]
	if _, has := parent[leaf]; !has {
		return
	}
	delete(parent, leaf)
	for i := len(chain) - 1; i > 0; i-- {
		if len(chain[i]) > 0 {
			return
		}
		delete(chain[i-1], segments[i])
	}
	if len(head) == 0 {
		delete(extMap, segments[0])
	}
}

// LookupExtension resolves a reverse-DNS namespace against the untyped
// Extensions map, transparently handling two on-disk encodings:
//
//  1. literal dotted key (YAML/JSON, or TOML quoted as ["org.projectfile.X"])
//     — landed directly under doc.Extensions[ns]
//  2. dotted-table header (TOML's [org.projectfile.X]) — exploded by go-toml
//     into nested maps, so we walk the segments
//
// Returns (nil, false) when the namespace is not present in either form.
func LookupExtension(doc *Document, ns string) (any, bool) {
	if doc == nil || doc.Extensions == nil || ns == "" {
		return nil, false
	}
	if v, ok := doc.Extensions[ns]; ok {
		return v, true
	}
	segments := strings.Split(ns, ".")
	cur, ok := doc.Extensions[segments[0]]
	if !ok {
		return nil, false
	}
	for _, seg := range segments[1:] {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[seg]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// HasExtension reports whether an extension namespace is present in the
// document. It is a boolean convenience wrapper over LookupExtension.
func HasExtension(doc *Document, ns string) bool {
	_, ok := LookupExtension(doc, ns)
	return ok
}

// GetIgnoresExtension parses the `org.projectfile.ignores` namespace out of
// the untyped Extensions map. Returns (nil, nil) when the namespace is
// absent, which the generator treats as "use defaults".
func GetIgnoresExtension(doc *Document) (*IgnoresExtension, error) {
	raw, ok := LookupExtension(doc, IgnoresExtensionNS)
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", IgnoresExtensionNS)
	}

	ext := &IgnoresExtension{
		Generate: strListVal(m, "generate"),
		Extra:    strListVal(m, "extra"),
	}
	if v, ok := m["git"]; ok {
		ext.Git = parseIgnoreTargetOverride(v)
	}
	if v, ok := m["docker"]; ok {
		ext.Docker = parseIgnoreTargetOverride(v)
	}
	if v, ok := m["npm"]; ok {
		ext.Npm = parseIgnoreTargetOverride(v)
	}
	if v, ok := m["claude"]; ok {
		ext.Claude = parseIgnoreTargetOverride(v)
	}
	if v, ok := m["container"]; ok {
		ext.Container = parseIgnoreTargetOverride(v)
	}
	return ext, nil
}

// GetEditorsExtension parses the `org.projectfile.editors` namespace.
// Returns (nil, nil) when absent, which means "auto-detect from filesystem".
func GetEditorsExtension(doc *Document) (*EditorsExtension, error) {
	raw, ok := LookupExtension(doc, EditorsExtensionNS)
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", EditorsExtensionNS)
	}
	return &EditorsExtension{
		Use: strListVal(m, "use"),
	}, nil
}

// GetVulnerabilitiesExtension parses the `org.projectfile.vulnerabilities`
// namespace — the tool-agnostic source of truth for suppressed vulnerability
// IDs that fan out to every scanner bridge (.trivyignore, .grype.yaml,
// osv-scanner.toml). Returns (nil, nil) when absent, which the bridges treat
// as "nothing to suppress".
func GetVulnerabilitiesExtension(doc *Document) (*VulnerabilitiesExtension, error) {
	raw, ok := LookupExtension(doc, VulnerabilitiesExtensionNS)
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", VulnerabilitiesExtensionNS)
	}
	ext := &VulnerabilitiesExtension{
		Generate: strListVal(m, "generate"),
	}
	if list, ok := m["suppress"].([]any); ok {
		ext.Suppress = parseSuppressList(list)
	}
	return ext, nil
}

// parseSuppressList accepts both the structured form ([{id, reason}]) and the
// shorthand form (a bare list of ID strings) so users can write the common
// case compactly while keeping reason provenance available.
func parseSuppressList(list []any) []VulnerabilitySuppress {
	out := make([]VulnerabilitySuppress, 0, len(list))
	for _, item := range list {
		// Shorthand: a bare string is just the ID.
		if s, ok := item.(string); ok {
			out = append(out, VulnerabilitySuppress{ID: s})
			continue
		}
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		s := VulnerabilitySuppress{
			ID:     strVal(m, "id"),
			Reason: strVal(m, "reason"),
		}
		if s.ID != "" {
			out = append(out, s)
		}
	}
	return out
}

func parseIgnoreTargetOverride(v any) *IgnoreTargetOverride {
	// Shorthand: a bare list is treated as include-only.
	if list, ok := v.([]any); ok {
		return &IgnoreTargetOverride{Include: toStrSlice(list)}
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return &IgnoreTargetOverride{
		Include: strListVal(m, "include"),
		Exclude: strListVal(m, "exclude"),
	}
}

// GetFundingExtension parses `org.projectfile.funding`. Returns (nil, nil)
// when the namespace is absent so callers can fall through to defaults.
func GetFundingExtension(doc *Document) (*FundingExtension, error) {
	raw, ok := LookupExtension(doc, FundingExtensionNS)
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", FundingExtensionNS)
	}
	return &FundingExtension{
		GitHub:          strListVal(m, "github"),
		Patreon:         strVal(m, "patreon"),
		KoFi:            strVal(m, "ko-fi"),
		Liberapay:       strVal(m, "liberapay"),
		Tidelift:        strVal(m, "tidelift"),
		CommunityBridge: strVal(m, "community-bridge"),
		IssueHunt:       strVal(m, "issuehunt"),
		OpenCollective:  strVal(m, "open-collective"),
		LFXCrowdfunding: strVal(m, "lfx-crowdfunding"),
		Polar:           strVal(m, "polar"),
		BuyMeACoffee:    strVal(m, "buy-me-a-coffee"),
		ThanksDev:       strVal(m, "thanks-dev"),
		Custom:          strListVal(m, "custom"),
		Path:            strVal(m, "path"),
		EntityType:      strVal(m, "entity-type"),
		EntityRole:      strVal(m, "entity-role"),
		Channels:        parseFundingChannels(m["channels"]),
		Plans:           parseFundingPlans(m["plans"]),
		History:         parseFundingHistory(m["history"]),
	}, nil
}

func parseFundingChannels(raw any) []FundingChannel {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]FundingChannel, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, FundingChannel{
			GUID:        strVal(m, "guid"),
			Type:        strVal(m, "type"),
			Address:     strVal(m, "address"),
			Description: strVal(m, "description"),
		})
	}
	return out
}

func parseFundingPlans(raw any) []FundingPlan {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]FundingPlan, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, FundingPlan{
			GUID:        strVal(m, "guid"),
			Status:      strVal(m, "status"),
			Name:        strVal(m, "name"),
			Description: strVal(m, "description"),
			Amount:      floatVal(m, "amount"),
			Currency:    strVal(m, "currency"),
			Frequency:   strVal(m, "frequency"),
			Channels:    strListVal(m, "channels"),
		})
	}
	return out
}

func parseFundingHistory(raw any) []FundingHistory {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]FundingHistory, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, FundingHistory{
			Year:        intVal(m, "year"),
			Income:      floatVal(m, "income"),
			Expenses:    floatVal(m, "expenses"),
			Taxes:       floatVal(m, "taxes"),
			Currency:    strVal(m, "currency"),
			Description: strVal(m, "description"),
		})
	}
	return out
}

// GetReleaseExtension parses `org.projectfile.release`. Returns (nil, nil) when
// absent — the `.releaserc.yaml` renderer refuses rather than emit a release
// config for a project that declared no release intent.
func GetReleaseExtension(doc *Document) (*ReleaseExtension, error) {
	raw, ok := LookupExtension(doc, ReleaseExtensionNS)
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", ReleaseExtensionNS)
	}
	return &ReleaseExtension{
		TagFormat: strVal(m, "tag-format"),
		Changelog: strVal(m, "changelog"),
		Branches:  parseReleaseBranches(m["branches"]),
	}, nil
}

func parseReleaseBranches(raw any) []ReleaseBranch {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]ReleaseBranch, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, ReleaseBranch{
			Pattern:    strVal(m, "pattern"),
			Channel:    strVal(m, "channel"),
			Prerelease: boolVal(m, "prerelease"),
		})
	}
	return out
}

// GetSecurityExtension parses `org.projectfile.security`. Returns (nil, nil)
// when absent — the SECURITY.md template falls back to first-maintainer email.
func GetSecurityExtension(doc *Document) (*SecurityExtension, error) {
	raw, ok := LookupExtension(doc, SecurityExtensionNS)
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", SecurityExtensionNS)
	}
	return &SecurityExtension{
		Contact:           strVal(m, "contact"),
		ReportURL:         strVal(m, "report-url"),
		SupportedVersions: strListVal(m, "supported-versions"),
		DisclosureWindow:  strVal(m, "disclosure-window"),
		GPGKey:            strVal(m, "gpg-key"),
		BugBountyURL:      strVal(m, "bug-bounty-url"),
	}, nil
}

// GetCodeOfConductExtension parses `org.projectfile.code-of-conduct`. Returns
// (nil, nil) when absent — the CoC bridge defaults to Contributor Covenant 2.1.
func GetCodeOfConductExtension(doc *Document) (*CodeOfConductExtension, error) {
	raw, ok := LookupExtension(doc, CodeOfConductExtensionNS)
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", CodeOfConductExtensionNS)
	}
	return &CodeOfConductExtension{
		Covenant: strVal(m, "covenant"),
		Scope:    strVal(m, "scope"),
	}, nil
}

// GetContributingExtension parses `org.projectfile.contributing`. Returns
// (nil, nil) when absent; the CONTRIBUTING.md generator applies its default
// section list at render time.
func GetContributingExtension(doc *Document) (*ContributingExtension, error) {
	raw, ok := LookupExtension(doc, ContributingExtensionNS)
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", ContributingExtensionNS)
	}
	return &ContributingExtension{
		Sections:          strListVal(m, "sections"),
		CLAURL:            strVal(m, "cla-url"),
		ChatURL:           strVal(m, "chat-url"),
		CoCURL:            strVal(m, "coc-url"),
		RecommendToFollow: ParseToggle(m["recommend-to-follow"]),
		RecommendToStar:   ParseToggle(m["recommend-to-star"]),
	}, nil
}

// GetSupportExtension parses `org.projectfile.support`. Returns (nil, nil)
// when absent; the SUPPORT.md generator applies its defaults at render time.
func GetSupportExtension(doc *Document) (*SupportExtension, error) {
	raw, ok := LookupExtension(doc, SupportExtensionNS)
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", SupportExtensionNS)
	}
	ext := &SupportExtension{
		ResponseTime: strVal(m, "response-time"),
	}
	if items, ok := m["eol"].([]any); ok {
		for _, item := range items {
			em, ok := item.(map[string]any)
			if !ok {
				continue
			}
			ext.EOL = append(ext.EOL, EOLEntry{
				Version: strVal(em, "version"),
				Date:    strVal(em, "date"),
			})
		}
	}
	return ext, nil
}

// GetReadmeExtension parses `org.projectfile.readme`. Returns (nil, nil)
// when absent; the README bridge applies its default block list at render
// time.
func GetReadmeExtension(doc *Document) (*ReadmeExtension, error) {
	raw, ok := LookupExtension(doc, ReadmeExtensionNS)
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", ReadmeExtensionNS)
	}
	ext := &ReadmeExtension{
		Blocks: strListVal(m, "blocks"),
	}
	if items, ok := m["extras"].([]any); ok {
		for _, item := range items {
			em, ok := item.(map[string]any)
			if !ok {
				continue
			}
			ext.Extras = append(ext.Extras, ReadmeExtra{
				Name:    strVal(em, "name"),
				Content: extractLocalizedVal(em, "content"),
			})
		}
	}
	if items, ok := m["shields"].([]any); ok {
		for _, item := range items {
			em, ok := item.(map[string]any)
			if !ok {
				continue
			}
			ext.Shields = append(ext.Shields, Shield{
				Name: strVal(em, "name"),
				Img:  strVal(em, "img"),
				Href: strVal(em, "href"),
				Alt:  strVal(em, "alt"),
			})
		}
	}
	return ext, nil
}

// extractLocalizedVal extracts a localized-string value from a map entry.
// The value may be a bare string or a map of lang→text; for maps it
// prefers "en", then the first non-empty value.
func extractLocalizedVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	langMap, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	if s, _ := langMap["en"].(string); s != "" {
		return s
	}
	for _, val := range langMap {
		if s, _ := val.(string); s != "" {
			return s
		}
	}
	return ""
}

// GetConventionsExtension parses `org.projectfile.conventions`. Returns
// (nil, nil) when absent. Per-language sub-maps (keys matching stack tags)
// are parsed into Languages; scalar keys are parsed directly.
func GetConventionsExtension(doc *Document) (*ConventionsExtension, error) {
	raw, ok := LookupExtension(doc, ConventionsExtensionNS)
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", ConventionsExtensionNS)
	}
	ext := &ConventionsExtension{
		CommitStyle:   strVal(m, keyCommitStyle),
		Workflow:      strVal(m, "workflow"),
		StyleGuideURL: strVal(m, "style-guide-url"),
	}
	for k, v := range m {
		if k == keyCommitStyle || k == "workflow" || k == "style-guide-url" {
			continue
		}
		sub, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if ext.Languages == nil {
			ext.Languages = map[string]LangConventions{}
		}
		ext.Languages[k] = LangConventions{
			StyleGuideURL: strVal(sub, "style-guide-url"),
		}
	}
	return ext, nil
}

// ConventionsStyleGuideURL resolves the effective style-guide-url for a project:
// top-level conventions.style-guide-url wins; otherwise the first per-language
// entry whose key matches a stack tag. Returns "" when no URL is found.
func ConventionsStyleGuideURL(conv *ConventionsExtension, stack []string) string {
	if conv == nil {
		return ""
	}
	if conv.StyleGuideURL != "" {
		return conv.StyleGuideURL
	}
	for _, tag := range stack {
		if lang, ok := conv.Languages[tag]; ok && lang.StyleGuideURL != "" {
			return lang.StyleGuideURL
		}
	}
	return ""
}

// GetCLIExtension parses `org.projectfile.cli`. The derive toggles default
// to true: when the extension is absent the engine runs every pass, and when
// the extension is present but a toggle is omitted that pass still runs. The
// only way to disable a pass is to set the toggle to false explicitly.
func GetCLIExtension(doc *Document) (*CLIExtension, error) {
	raw, ok := LookupExtension(doc, CLIExtensionNS)
	if !ok {
		// Absent: every pass runs by default. Engine callers treat a nil
		// extension as "both toggles true, derived is empty".
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", CLIExtensionNS)
	}
	ext := &CLIExtension{
		// Read from extension map for backward compat with old format.
		Derived: strListVal(m, "derived"),
		Derive: CLIDeriveToggles{
			// Explicit-false is the only "off" — implicit absence stays on.
			Forges:     boolValDefaultTrue(m, "derive", "forges"),
			Registries: boolValDefaultTrue(m, "derive", "registries"),
		},
	}
	// Also collect paths from links with Derived=true (new per-link format).
	for _, l := range doc.Links {
		if l.Derived {
			ext.Derived = append(ext.Derived, "links[type="+l.Type+",url="+l.URL+"]")
		}
	}
	return ext, nil
}

// SetCLIExtension serialises ext into doc.Extensions under the cli namespace.
// Marshal cost is two map allocations; the engine only calls this once per
// run after Apply() collects every Change.
func SetCLIExtension(doc *Document, ext *CLIExtension) {
	if doc == nil || ext == nil {
		return
	}
	m := map[string]any{}
	derive := map[string]any{}
	if !ext.Derive.Forges {
		derive["forges"] = false
	}
	if !ext.Derive.Registries {
		derive["registries"] = false
	}
	if len(derive) > 0 {
		m["derive"] = derive
	}
	SetExtension(doc, CLIExtensionNS, m)
}

// boolValDefaultTrue reads m[outer][inner] as a bool, defaulting to true when
// the path is missing or the leaf is non-bool. The "convention over
// configuration" implication of CLIDeriveToggles: a present extension without
// the toggle still leaves the inference pass on.
func boolValDefaultTrue(m map[string]any, outer, inner string) bool {
	sub, ok := m[outer].(map[string]any)
	if !ok {
		return true
	}
	v, ok := sub[inner]
	if !ok {
		return true
	}
	b, ok := v.(bool)
	if !ok {
		return true
	}
	return b
}

// boolVal reads m[key] as a bool, defaulting to false when missing or non-bool.
func boolVal(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	if !ok {
		return false
	}
	return b
}

// GetForgeExtension parses `org.projectfile.forge`. All toggles default to
// true: when the extension is absent every field is pushed to every host;
// when present-but-incomplete, omitted toggles still default to "on" so a
// projectfile that mentions the extension only to disable one host doesn't
// accidentally turn the others off. Returns (nil, nil) when the namespace
// is absent — callers treat nil as "every default applies".
func GetForgeExtension(doc *Document) (*ForgeExtension, error) {
	raw, ok := LookupExtension(doc, ForgeExtensionNS)
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", ForgeExtensionNS)
	}
	ext := &ForgeExtension{
		// Top-level push toggle defaults to true so a [org.projectfile.forge]
		// block that only carries field-level overrides still allows the
		// push to run at all.
		Push: boolFieldDefaultTrue(m, "push"),
		Fields: ForgeFieldsToggles{
			Description: boolValDefaultTrue(m, "fields", "description"),
			Homepage:    boolValDefaultTrue(m, "fields", "homepage"),
			Topics:      boolValDefaultTrue(m, "fields", "topics"),
		},
	}
	if hostsRaw, ok := m["hosts"].(map[string]any); ok && len(hostsRaw) > 0 {
		ext.Hosts = map[string]bool{}
		for host, v := range hostsRaw {
			b, ok := v.(bool)
			if !ok {
				// Non-bool value here is a user typo (e.g. "true" as a string).
				// Treating it as the safe default (push allowed) avoids
				// silently suppressing a push the user expected to happen.
				ext.Hosts[host] = true
				continue
			}
			ext.Hosts[host] = b
		}
	}
	if kindsRaw, ok := m["kinds"].(map[string]any); ok && len(kindsRaw) > 0 {
		ext.Kinds = map[string]string{}
		for host, v := range kindsRaw {
			// Non-string value is a typo — skip silently rather than coerce.
			// A bad kind ("forfejo") will fail later at driver-lookup with a
			// clear "no driver registered for kind X" warning.
			s, _ := v.(string)
			if s == "" {
				continue
			}
			ext.Kinds[host] = s
		}
	}
	return ext, nil
}

// boolFieldDefaultTrue is the single-level analogue of boolValDefaultTrue
// for a top-level toggle. Used by GetForgeExtension for the namespace-wide
// `push` switch which has no nesting layer.
func boolFieldDefaultTrue(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return true
	}
	b, ok := v.(bool)
	if !ok {
		return true
	}
	return b
}

// GetCodeOwnersExtension parses `org.projectfile.codeowners`. The entries
// array is order-significant — CODEOWNERS pattern resolution is "last match
// wins per path", so reordering changes semantics.
func GetCodeOwnersExtension(doc *Document) (*CodeOwnersExtension, error) {
	raw, ok := LookupExtension(doc, CodeOwnersExtensionNS)
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", CodeOwnersExtensionNS)
	}
	entries, _ := m["entries"].([]any)
	ext := &CodeOwnersExtension{}
	for _, e := range entries {
		em, ok := e.(map[string]any)
		if !ok {
			continue
		}
		ext.Entries = append(ext.Entries, CodeOwnersEntry{
			Pattern: strVal(em, "pattern"),
			Owners:  strListVal(em, "owners"),
		})
	}
	return ext, nil
}

// PrimaryRepository returns the entry in doc.Repositories that is the
// canonical source location: the one with role == origin, or the sole
// entry (implicitly origin), or nil when no repositories are recorded. See spec §5.3a.
func PrimaryRepository(doc *Document) *Repository {
	if doc == nil {
		return nil
	}
	repos := doc.Repositories
	if len(repos) == 0 {
		return nil
	}
	if len(repos) == 1 {
		return &doc.Repositories[0]
	}
	for i := range repos {
		if repos[i].Role == RepositoryRoleOrigin {
			return &doc.Repositories[i]
		}
	}
	return nil
}

// IssuesRepository returns the repository entry that should be used for
// issue-tracker URL derivation. Resolution order (spec §4.3a, "issues" key):
//
//  1. The entry with Issues == true — the author explicitly chose this forge
//     as the canonical bug tracker.
//  2. PrimaryRepository — fallback when no entry carries Issues == true,
//     matching the pre-issues behaviour.
//
// Returns nil when no repositories are recorded.
func IssuesRepository(doc *Document) *Repository {
	if doc == nil {
		return nil
	}
	repos := doc.Repositories
	for i := range repos {
		if repos[i].Issues {
			return &doc.Repositories[i]
		}
	}
	return PrimaryRepository(doc)
}

// CitableRepositoryURL returns the first repository URL whose scheme is
// http(s) or ftp(s) — the schemes acceptable in CITATION.cff repository-code,
// SPDX VCS expressions, and most public package metadata. Resolution order:
//
//  1. links[type=source-code] (preferred entry, sole, or first) — the spec's
//     dedicated slot for the forge landing page (§5.11).
//  2. primary repository when its URL is already http-style.
//  3. any other repository entry whose URL is http-style (mirror, archive).
//
// Returns "" when no usable URL is found. Repositories[].url may be ssh
// (the truthful clone endpoint); links[type=source-code] is where the
// http forge page lives.
func CitableRepositoryURL(doc *Document) string {
	if doc == nil {
		return ""
	}
	if l := LinkByType(doc, LinkSourceCode); l != nil && isPublicURL(l.URL) {
		return l.URL
	}
	if p := PrimaryRepository(doc); p != nil && isPublicURL(p.URL) {
		return p.URL
	}
	for i := range doc.Repositories {
		r := &doc.Repositories[i]
		if r.Role == RepositoryRoleOrigin {
			continue
		}
		if isPublicURL(r.URL) {
			return r.URL
		}
	}
	return ""
}

// EnsureCitableSourceLink returns a pointer to the links[type=source-code]
// entry the citation chain would emit, creating one when absent. Resolution
// order: preferred-or-sole-or-first existing source-code link, else append
// a new entry. Used by sync drivers that need to push a foreign manifest's
// public URL into pf without clobbering the ssh clone endpoint in
// repositories[].
func EnsureCitableSourceLink(doc *Document) *Link {
	if doc == nil {
		return nil
	}
	if l := LinkByType(doc, LinkSourceCode); l != nil {
		return l
	}
	doc.Links = append(doc.Links, Link{Type: LinkSourceCode})
	return &doc.Links[len(doc.Links)-1]
}

func isPublicURL(u string) bool {
	return strings.HasPrefix(u, "https://") ||
		strings.HasPrefix(u, "http://") ||
		strings.HasPrefix(u, "ftp://") ||
		strings.HasPrefix(u, "sftp://")
}

// EnsurePrimaryRepository returns a pointer to the primary repository entry,
// creating one (role: origin) when none exist. Used by sync drivers that
// receive a single repository URL from a foreign manifest (package.json,
// pyproject.toml, composer.json, CITATION.cff) and need a stable slot to
// write into without losing existing mirror/archive entries.
func EnsurePrimaryRepository(doc *Document) *Repository {
	if doc == nil {
		return nil
	}
	repos := doc.Repositories
	if len(repos) == 0 {
		doc.Repositories = []Repository{{Role: RepositoryRoleOrigin}}
		return &doc.Repositories[0]
	}
	for i := range repos {
		if repos[i].Role == RepositoryRoleOrigin {
			return &doc.Repositories[i]
		}
	}
	return &doc.Repositories[0]
}

// LinkByType returns the canonical entry for a given link type from doc.Links,
// applying the §5.11 selection rule: the entry with Preferred=true if any,
// else the sole entry of that type, else the first entry of that type in
// document order. Returns nil when no entry of the type exists.
//
// Mutating the returned pointer mutates the document. To add a new entry,
// use SetLink (gap-fill) or AddLink (always append) instead.
func LinkByType(doc *Document, linkType string) *Link {
	if doc == nil || linkType == "" {
		return nil
	}
	var first, sole *Link
	count := 0
	for i := range doc.Links {
		l := &doc.Links[i]
		if l.Type != linkType {
			continue
		}
		if l.Preferred {
			return l
		}
		if first == nil {
			first = l
		}
		sole = l
		count++
	}
	if count == 1 {
		return sole
	}
	return first
}

// LinkURL is the read-only sugar over LinkByType — returns the URL of the
// canonical entry, or "" when no entry of the type exists. Use this in
// generator templates and any read-path that only cares about the URL string.
func LinkURL(doc *Document, linkType string) string {
	if l := LinkByType(doc, linkType); l != nil {
		return l.URL
	}
	return ""
}

// LinksByType returns all entries of a given link type in document order.
// Use this when a consumer needs every entry (e.g., SUPPORT.md showing
// multiple paid-support or chat channels). Returns nil when no entries match.
func LinksByType(doc *Document, linkType string) []Link {
	if doc == nil || linkType == "" {
		return nil
	}
	var out []Link
	for _, l := range doc.Links {
		if l.Type == linkType {
			out = append(out, l)
		}
	}
	return out
}

// SetLink writes url into the canonical links[type=linkType] entry: updates
// the entry LinkByType would return, or appends a new entry when none exists.
// force=true overwrites a non-empty URL; force=false only writes when the
// canonical entry is missing or empty (gap-fill semantics matching the sync
// FromPF/ToPF closures). Returns true when the document was mutated.
func SetLink(doc *Document, linkType, url string, force bool) bool {
	if doc == nil || linkType == "" || url == "" {
		return false
	}
	if l := LinkByType(doc, linkType); l != nil {
		if l.URL == url {
			return false
		}
		if l.URL != "" && !force {
			return false
		}
		l.URL = url
		return true
	}
	doc.Links = append(doc.Links, Link{Type: linkType, URL: url})
	return true
}

// AddLink appends a new entry to doc.Links unconditionally. Use this when
// emitting multiple entries of the same type (mirrors of source-code, several
// chat channels); use SetLink for the "one canonical URL per type" case.
func AddLink(doc *Document, linkType, url string) {
	if doc == nil || linkType == "" || url == "" {
		return
	}
	doc.Links = append(doc.Links, Link{Type: linkType, URL: url})
}

// DisplayName returns the best human-readable label for the project:
// identity.title.en → namespace/name → name → "this project". Used by
// every template generator so projects with a localized title don't get
// rendered as their lowercase DNS-label name in CoC / CONTRIBUTING / SECURITY.
func DisplayName(doc *Document) string {
	if doc == nil {
		return "this project"
	}
	if t := ExtractLocalizedString(doc.Identity.Title); t != "" {
		return t
	}
	if doc.Identity.Namespace != "" && doc.Identity.Name != "" {
		return doc.Identity.Namespace + "/" + doc.Identity.Name
	}
	if doc.Identity.Name != "" {
		return doc.Identity.Name
	}
	return "this project"
}

// CopyrightHolders assembles SPDX-style copyright lines from the
// `copyright`-role entries under [[people]]. Order matches the people array.
// Per-person `from` supplies the start year; absent, the top-level
// [copyright].year is used; absent both, the current year is used. Per-person
// `to` becomes the upper bound; absent → open-ended ("YYYY"). The output
// is the deterministic ordered slice the LICENSE generator and SPDX
// substitution previously read from License.Holders — single source of truth.
func CopyrightHolders(doc *Document) []string {
	if doc == nil {
		return nil
	}
	defaultYear := 0
	if doc.Copyright != nil && doc.Copyright.Year != 0 {
		defaultYear = doc.Copyright.Year
	}
	if defaultYear == 0 {
		defaultYear = time.Now().Year()
	}
	var out []string
	for _, p := range doc.People {
		if !roleContains(p.Roles, RoleCopyright) {
			continue
		}
		line := copyrightLine(p.From, p.To, defaultYear, FlatPersonName(p), p.Email)
		if line != "" {
			out = append(out, line)
		}
	}
	for _, o := range doc.Organizations {
		if !roleContains(o.Roles, RoleCopyright) {
			continue
		}
		line := copyrightLine(o.From, o.To, defaultYear, o.Name, o.Email)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func copyrightLine(fromStr, toStr string, defaultYear int, name, email string) string {
	fromYear := yearOf(fromStr)
	if fromYear == 0 {
		fromYear = defaultYear
	}
	toYear := yearOf(toStr)
	years := fmt.Sprintf("%d", fromYear)
	if toYear != 0 && toYear != fromYear {
		years = fmt.Sprintf("%d-%d", fromYear, toYear)
	}
	if name == "" {
		return ""
	}
	line := fmt.Sprintf("Copyright %s %s", years, name)
	if email != "" {
		line = fmt.Sprintf("%s <%s>", line, email)
	}
	return line
}

// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
// Copyright YYYY"
// Copyright [year]" prefix and only the holder name(s) go into the
//
// SPDX-License-Identifier: MIT
func CopyrightHolderNames(doc *Document) []string {
	if doc == nil {
		return nil
	}
	var out []string
	for _, p := range doc.People {
		if !roleContains(p.Roles, RoleCopyright) {
			continue
		}
		if name := FlatPersonName(p); name != "" {
			out = append(out, name)
		}
	}
	for _, o := range doc.Organizations {
		if !roleContains(o.Roles, RoleCopyright) {
			continue
		}
		if o.Name != "" {
			out = append(out, o.Name)
		}
	}
	return out
}

// FlatPersonName flattens a Person to the single-name string consumers emit
// to formats with no structured name slot. Implements the synthesis rule of
// spec §5.5.3 for persons.
func FlatPersonName(p Person) string {
	if p.DisplayName != "" {
		return p.DisplayName
	}
	parts := make([]string, 0, 4)
	if p.GivenNames != "" {
		parts = append(parts, p.GivenNames)
	}
	if p.NameParticle != "" {
		parts = append(parts, p.NameParticle)
	}
	if p.FamilyNames != "" {
		parts = append(parts, p.FamilyNames)
	}
	flat := strings.Join(parts, " ")
	if p.NameSuffix != "" {
		if flat != "" {
			flat = flat + ", " + p.NameSuffix
		} else {
			flat = p.NameSuffix
		}
	}
	return flat
}

func FlatOrgName(o Organization) string {
	return o.Name
}

func roleContains(roles []string, want string) bool {
	for _, r := range roles {
		if r == want {
			return true
		}
	}
	return false
}

// yearOf extracts the leading YYYY out of an ISO-8601 date string. Returns 0
// when the input is empty or malformed (the schema enforces ISO format, but
// the helper is defensive for hand-edited / partial documents).
func yearOf(iso string) int {
	if len(iso) < 4 {
		return 0
	}
	y := 0
	for i := range 4 {
		c := iso[i]
		if c < '0' || c > '9' {
			return 0
		}
		y = y*10 + int(c-'0')
	}
	return y
}

// ContactEmail picks the most appropriate contact email from pf.People for a
// single-contact artefact (SECURITY.md, CODE_OF_CONDUCT.md, future siblings).
// Walk order, first non-empty wins:
//
//  1. People whose roles contain primaryRole — the artefact-specific role
//     (RoleSecurity, RoleCommunity, ...). Spec v1 §5.5.4 SHOULD-filter rule.
//  2. People whose roles contain RoleMaintainer — generic fallback when no
//     artefact-specific contact is registered.
//  3. Any person with an email — last-resort so a project with [[people]] but
//     no curated roles still produces a usable file.
//
// Returns ("", "") when no person carries an email so the caller can surface
// an actionable "no contact available" error. The string returned is the
// email; the source label names which step matched (used by the genlog
// decision trace so the reader can see why a particular address was chosen).
// Pass primaryRole = "" or RoleMaintainer to skip step 1.
func ContactEmail(doc *Document, primaryRole string) (email, source string) {
	if doc == nil {
		return "", ""
	}
	if primaryRole != "" && primaryRole != RoleMaintainer {
		if e := firstEmailByRole(doc, primaryRole); e != "" {
			return e, fmt.Sprintf("first [[people]] or [[organizations]] with role %q", primaryRole)
		}
	}
	if e := firstEmailByRole(doc, RoleMaintainer); e != "" {
		return e, "first maintainer in [[people]] or [[organizations]]"
	}
	for _, p := range doc.People {
		if p.Email != "" {
			return p.Email, "first [[people]] entry with email"
		}
	}
	for _, o := range doc.Organizations {
		if o.Email != "" {
			return o.Email, "first [[organizations]] entry with email"
		}
	}
	return "", ""
}

func firstEmailByRole(doc *Document, role string) string {
	for _, p := range doc.People {
		if p.Email != "" && roleContains(p.Roles, role) {
			return p.Email
		}
	}
	for _, o := range doc.Organizations {
		if o.Email != "" && roleContains(o.Roles, role) {
			return o.Email
		}
	}
	return ""
}

func GetCitationExtension(doc *Document) (*CitationExtension, error) {
	if doc.Extensions == nil {
		return nil, nil
	}

	raw, ok := doc.Extensions["org.projectfile.citation"]
	if !ok {
		return nil, nil
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("org.projectfile.citation is not a map")
	}

	ext := &CitationExtension{}

	if v, ok := m["doi"]; ok {
		ext.DOI, _ = v.(string)
	}
	if v, ok := m["message"]; ok {
		ext.Message, _ = v.(string)
	}

	if pref, ok := m["preferred"]; ok {
		if pm, ok := pref.(map[string]any); ok {
			ext.Preferred = &PreferredCitation{}
			if v, ok := pm["type"]; ok {
				ext.Preferred.Type, _ = v.(string)
			}
			if v, ok := pm["title"]; ok {
				ext.Preferred.Title, _ = v.(string)
			}
			if v, ok := pm["journal"]; ok {
				ext.Preferred.Journal, _ = v.(string)
			}
			if v, ok := pm["volume"]; ok {
				switch n := v.(type) {
				case int:
					ext.Preferred.Volume = n
				case int64:
					ext.Preferred.Volume = int(n)
				case float64:
					ext.Preferred.Volume = int(n)
				}
			}
			if v, ok := pm["issue"]; ok {
				switch n := v.(type) {
				case int:
					ext.Preferred.Issue = n
				case int64:
					ext.Preferred.Issue = int(n)
				case float64:
					ext.Preferred.Issue = int(n)
				}
			}
			if v, ok := pm["pages"]; ok {
				ext.Preferred.Pages, _ = v.(string)
			}
			if v, ok := pm["year"]; ok {
				switch n := v.(type) {
				case int:
					ext.Preferred.Year = n
				case int64:
					ext.Preferred.Year = int(n)
				case float64:
					ext.Preferred.Year = int(n)
				}
			}
			if v, ok := pm["doi"]; ok {
				ext.Preferred.DOI, _ = v.(string)
			}
		}
	}

	return ext, nil
}

// AsStringList normalises a string-or-sequence-of-string value into a flat
// []string. Several spec fields are typed `any` because the encoding admits
// either a single scalar or a sequence (`license.file` per spec §4.4,
// `requirements.browsers`). Empty / whitespace-only entries are dropped so a
// caller can treat a zero-length result as "field absent". The single-string
// and sequence forms both round-trip; this is the read-side normaliser.
func AsStringList(v any) []string {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		if s := strings.TrimSpace(x); s != "" {
			return []string{s}
		}
		return nil
	case []string:
		out := make([]string, 0, len(x))
		for _, s := range x {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				if s = strings.TrimSpace(s); s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	}
	return nil
}
