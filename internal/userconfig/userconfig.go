// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package userconfig loads the per-user pf-cli configuration file at
// $XDG_CONFIG_HOME/projectfile/cli.{yaml,yml,toml,json} (fallback under
// ~/.config). The schema mirrors the in-pf [org.projectfile.cli] extension
// shape where it overlaps, so the merge precedence is mechanical:
//
//	in-pf [org.projectfile.cli].generate.<filename>.path
//	  → user config ([generate.defaults."<filename>"].path)
//	    → built-in default (each target's Path callback)
//
// Encoding probe order mirrors projectfile.DetectPath:
// yaml → yml → toml → json. First hit wins.
//
// Loading is best-effort and lazy: missing file → empty config; malformed
// file → warning logged via genlog, empty config returned. The CLI never
// fails to launch because of a stray user-config error.
package userconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"

	"kiota.ch/projectfile/core/v2/internal/genlog"
)

const (
	extTOML  = ".toml"
	extYAML  = ".yaml"
	extYAML2 = ".yml"
	extJSON  = ".json"
)

// Config carries every per-user setting pf-cli understands. The structure
// mirrors what could appear in the in-pf [org.projectfile.cli] extension
// where the two overlap — keeping them shape-compatible means migrating a
// setting from user-level to project-level is a copy-paste, not a rewrite.
//
// Field tags cover all three supported encodings (TOML, YAML, JSON) so the
// same struct decodes any of cli.{toml,yaml,yml,json}.
type Config struct {
	Init         InitSection         `toml:"init,omitempty"         yaml:"init,omitempty"         json:"init,omitempty"`
	Identity     IdentitySection     `toml:"identity,omitempty"     yaml:"identity,omitempty"     json:"identity,omitempty"`
	Copyright    CopyrightSection    `toml:"copyright,omitempty"    yaml:"copyright,omitempty"    json:"copyright,omitempty"`
	Funding      FundingSection      `toml:"funding,omitempty"      yaml:"funding,omitempty"      json:"funding,omitempty"`
	Security     SecuritySection     `toml:"security,omitempty"     yaml:"security,omitempty"     json:"security,omitempty"`
	Conventions  ConventionsSection  `toml:"conventions,omitempty"   yaml:"conventions,omitempty"   json:"conventions,omitempty"`
	Contributing ContributingSection `toml:"contributing,omitempty" yaml:"contributing,omitempty" json:"contributing,omitempty"`
	Support      SupportSection      `toml:"support,omitempty"      yaml:"support,omitempty"      json:"support,omitempty"`
	Scan         ScanSection         `toml:"scan,omitempty"         yaml:"scan,omitempty"         json:"scan,omitempty"`
	Generate     GenerateSection     `toml:"generate,omitempty"     yaml:"generate,omitempty"     json:"generate,omitempty"`
}

// InitSection holds defaults the `init` wizard consults before prompting.
// Each field is a "pre-fill" — non-interactive runs use it verbatim; the
// interactive run uses it as the prompt's default value so the user can
// still override per-project.
type InitSection struct {
	Format           string `toml:"format,omitempty"            yaml:"format,omitempty"            json:"format,omitempty"`
	NamespaceDefault string `toml:"namespace-default,omitempty" yaml:"namespace-default,omitempty" json:"namespace-default,omitempty"`
	LicenseDefault   string `toml:"license-default,omitempty"   yaml:"license-default,omitempty"   json:"license-default,omitempty"`
	NoScan           bool   `toml:"no-scan,omitempty"           yaml:"no-scan,omitempty"           json:"no-scan,omitempty"`
}

// IdentitySection describes the user-as-person OR the user-as-organization
// — fields auto-attached to a people[] entry whose Email matches the
// configured Email. Mirrors the subset of projectfile.Person (or
// projectfile.Organization) that is stable across all of a user's projects
// (ORCID, affiliation, GPG key — never roles or dates).
//
// Shape discriminator follows the projectfile spec §5.5:
//   - IsEntity=false (default) → person; FamilyNames / GivenNames carry
//     the split name. Affiliation is honoured.
//   - IsEntity=true → organization; Name carries the flat label, and
//     Affiliation / FamilyNames / GivenNames MUST be empty (consumers
//     ignore them).
//
// Name is preserved as a legacy field: pre-existing user configs that
// were written before the split prompts still load, and the wizard
// migrates them on next run.
type IdentitySection struct {
	IsEntity    bool   `toml:"is-entity,omitempty"    yaml:"is-entity,omitempty"    json:"is-entity,omitempty"`
	FamilyNames string `toml:"family-names,omitempty" yaml:"family-names,omitempty" json:"family-names,omitempty"`
	GivenNames  string `toml:"given-names,omitempty"  yaml:"given-names,omitempty"  json:"given-names,omitempty"`
	Name        string `toml:"name,omitempty"         yaml:"name,omitempty"         json:"name,omitempty"`
	Orcid       string `toml:"orcid,omitempty"        yaml:"orcid,omitempty"        json:"orcid,omitempty"`
	Email       string `toml:"email,omitempty"        yaml:"email,omitempty"        json:"email,omitempty"`
	URL         string `toml:"url,omitempty"          yaml:"url,omitempty"          json:"url,omitempty"`
	Affiliation string `toml:"affiliation,omitempty"  yaml:"affiliation,omitempty"  json:"affiliation,omitempty"`
	GPGKey      string `toml:"gpg-key,omitempty"      yaml:"gpg-key,omitempty"      json:"gpg-key,omitempty"`
}

// CopyrightSection — YearStrategy is "current" (use time.Now().Year() at
// LICENSE-generation time) or "fixed" (use the projectfile's [copyright]
// year as-written). Empty = consumer default (current).
type CopyrightSection struct {
	YearStrategy string `toml:"year-strategy,omitempty" yaml:"year-strategy,omitempty" json:"year-strategy,omitempty"`
}

// FundingSection mirrors the in-pf FundingExtension shape exactly so a user
// who funds the same way across projects sets it once. Path is omitted on
// purpose — that's a per-project knob, not a personal one.
type FundingSection struct {
	GitHub          []string `toml:"github,omitempty"           yaml:"github,omitempty"           json:"github,omitempty"`
	Patreon         string   `toml:"patreon,omitempty"          yaml:"patreon,omitempty"          json:"patreon,omitempty"`
	KoFi            string   `toml:"ko-fi,omitempty"            yaml:"ko-fi,omitempty"            json:"ko-fi,omitempty"`
	Liberapay       string   `toml:"liberapay,omitempty"        yaml:"liberapay,omitempty"        json:"liberapay,omitempty"`
	Tidelift        string   `toml:"tidelift,omitempty"         yaml:"tidelift,omitempty"         json:"tidelift,omitempty"`
	CommunityBridge string   `toml:"community-bridge,omitempty" yaml:"community-bridge,omitempty" json:"community-bridge,omitempty"`
	IssueHunt       string   `toml:"issuehunt,omitempty"        yaml:"issuehunt,omitempty"        json:"issuehunt,omitempty"`
	OpenCollective  string   `toml:"open-collective,omitempty"  yaml:"open-collective,omitempty"  json:"open-collective,omitempty"`
	LFXCrowdfunding string   `toml:"lfx-crowdfunding,omitempty" yaml:"lfx-crowdfunding,omitempty" json:"lfx-crowdfunding,omitempty"`
	Polar           string   `toml:"polar,omitempty"            yaml:"polar,omitempty"            json:"polar,omitempty"`
	BuyMeACoffee    string   `toml:"buy-me-a-coffee,omitempty"  yaml:"buy-me-a-coffee,omitempty"  json:"buy-me-a-coffee,omitempty"`
	ThanksDev       string   `toml:"thanks-dev,omitempty"       yaml:"thanks-dev,omitempty"       json:"thanks-dev,omitempty"`
	Custom          []string `toml:"custom,omitempty"           yaml:"custom,omitempty"           json:"custom,omitempty"`
}

// SecuritySection — user-stable subset of SecurityExtension. Contact is
// intentionally omitted: it's derived from [identity].email so users never
// repeat themselves. SupportedVersions is per-project and stays out too.
type SecuritySection struct {
	ReportURL        string `toml:"report-url,omitempty"        yaml:"report-url,omitempty"        json:"report-url,omitempty"`
	DisclosureWindow string `toml:"disclosure-window,omitempty" yaml:"disclosure-window,omitempty" json:"disclosure-window,omitempty"`
	BugBountyURL     string `toml:"bug-bounty-url,omitempty"    yaml:"bug-bounty-url,omitempty"    json:"bug-bounty-url,omitempty"`
}

// ContributingSection — user-stable URLs the CONTRIBUTING.md template would
// otherwise force the user to repeat per project. Sections list is omitted
// (per-project shape) and CoCURL is omitted (derived from the generated CoC
// file's location).
type ContributingSection struct {
	CLAURL  string `toml:"cla-url,omitempty"  yaml:"cla-url,omitempty"  json:"cla-url,omitempty"`
	ChatURL string `toml:"chat-url,omitempty" yaml:"chat-url,omitempty" json:"chat-url,omitempty"`
}

// SupportSection — user-stable defaults for org.projectfile.support.
// Response-time is the only per-user field; the EOL table is per-project.
type SupportSection struct {
	ResponseTime string `toml:"response-time,omitempty" yaml:"response-time,omitempty" json:"response-time,omitempty"`
}

// ConventionsSection — user-stable defaults for org.projectfile.conventions.
// These are cross-cutting (commit-style, workflow, style-guide-url) and not
// per-project, so the user-config is the right home for them.
type ConventionsSection struct {
	CommitStyle   string `toml:"commit-style,omitempty"   yaml:"commit-style,omitempty"   json:"commit-style,omitempty"`
	Workflow      string `toml:"workflow,omitempty"        yaml:"workflow,omitempty"        json:"workflow,omitempty"`
	StyleGuideURL string `toml:"style-guide-url,omitempty" yaml:"style-guide-url,omitempty" json:"style-guide-url,omitempty"`
}

// ScanSection — strictly user-only (per design decision: this layer is
// "secret to me, not to the repo", so no in-pf override). PrivateHosts is a
// list of hostnames the git scanner treats as confidential; matching
// origins are omitted entirely rather than emitted with stripped URLs.
type ScanSection struct {
	PrivateHosts []string `toml:"private-hosts,omitempty" yaml:"private-hosts,omitempty" json:"private-hosts,omitempty"`
}

// GenerateSection holds settings that govern `pf-cli generate`. Defaults is
// keyed by canonical target filename ("CONTRIBUTING.md", "FUNDING.yml"), not
// by alias.
type GenerateSection struct {
	Defaults map[string]TargetDefault `toml:"defaults,omitempty" yaml:"defaults,omitempty" json:"defaults,omitempty"`
}

// TargetDefault is the per-target user-level override. Today only Path is
// honoured; new knobs land here as a single new tagged field plus a caller
// switch.
type TargetDefault struct {
	Path string `toml:"path,omitempty" yaml:"path,omitempty" json:"path,omitempty"`
}

var (
	loadOnce sync.Once
	cached   *Config
	// ignored short-circuits Load() to return an empty Config — used by the
	// root --ignore-user-config flag for one-off runs that want pristine
	// "no personal defaults applied" behavior (debugging, reproducible CI,
	// comparing output against a teammate's). Toggling this clears the
	// cache so the next Load() respects the new state.
	ignored bool
)

// SetIgnored toggles the "ignore user config" flag. When true, subsequent
// Load() calls return a zero-value Config regardless of what's on disk.
// Resets the load cache so a SetIgnored(false) right after Load() refreshes
// rather than returning the stale empty Config.
func SetIgnored(b bool) {
	ignored = b
	Reset()
}

// Load returns the cached user config. First call reads the file; subsequent
// calls return the same pointer. Tests can call Reset() to force a re-read.
// When SetIgnored(true) has been called, returns an empty Config — every
// fallback site treats this identically to "no file on disk", which is the
// correct semantics for --ignore-user-config.
func Load() *Config {
	loadOnce.Do(func() {
		if ignored {
			cached = &Config{}
			return
		}
		cached = readFromDisk()
	})
	return cached
}

// Reset clears the cache so the next Load() re-reads from disk. Tests use
// this when they want to validate the file-read path; not exported beyond
// the package boundary in practice.
func Reset() {
	loadOnce = sync.Once{}
	cached = nil
}

// PathFor returns the per-user override for filename's on-disk path, or "" if
// none is set. Callers pair this with their built-in default — typically:
//
//	rel := userconfig.PathFor("CONTRIBUTING.md")
//	if rel == "" { rel = "CONTRIBUTING.md" }
func PathFor(filename string) string {
	cfg := Load()
	if cfg == nil {
		return ""
	}
	d, ok := cfg.Generate.Defaults[filename]
	if !ok {
		return ""
	}
	return d.Path
}

// IsPrivateHost reports whether host appears in [scan].private-hosts.
// Matching is case-insensitive and supports two shapes:
//
//   - Bare hostname — "example.com" matches the apex AND any direct or
//     nested subdomain on a dot boundary (foo.example.com, a.b.example.com)
//     BUT NOT "bad-example.com". Secure-by-default for a privacy feature:
//     listing a domain implies "anything at this domain is confidential".
//   - Wildcard subdomain — "*.example.com" matches subdomains only, never
//     the bare apex. Use this when the apex is public marketing but the
//     subdomain (e.g. src.example.com) is the private forge.
//
// We deliberately do NOT implement glob-anywhere (`*.dev.*`) — it would let
// a misconfigured entry silently match too much.
//
// Empty host always returns false so callers can pass parser output
// directly without pre-guarding.
func IsPrivateHost(host string) bool {
	if host == "" {
		return false
	}
	cfg := Load()
	if cfg == nil {
		return false
	}
	h := strings.ToLower(host)
	for _, p := range cfg.Scan.PrivateHosts {
		if matchPrivateHost(strings.ToLower(p), h) {
			return true
		}
	}
	return false
}

// matchPrivateHost is the per-pattern comparator factored out of
// IsPrivateHost so the wildcard semantics live in exactly one place.
// pattern and host MUST already be lowercased — case folding is the
// caller's job (it only needs to happen once per host).
func matchPrivateHost(pattern, host string) bool {
	if pattern == "" {
		return false
	}
	if strings.HasPrefix(pattern, "*.") {
		// `*.example.com` — match any host that ends with `.example.com`.
		// The leading dot in the suffix is what prevents `bad-example.com`
		// from matching `*.example.com`. Apex deliberately excluded — when
		// the user wants apex coverage they drop the wildcard prefix.
		suffix := pattern[1:] // ".example.com"
		return strings.HasSuffix(host, suffix) && len(host) > len(suffix)
	}
	// Bare hostname — apex + any subdomain on a dot boundary. The "." +
	// pattern suffix check is what prevents `bad-example.com` from matching
	// `example.com`; equality covers the apex case.
	return host == pattern || strings.HasSuffix(host, "."+pattern)
}

// candidateBasenames is the probe order for cli.<ext>. Mirrors
// projectfile.DetectPath so users get the same "yaml-first" preference
// for both document types.
var candidateBasenames = []string{
	"cli.yaml",
	"cli.yml",
	"cli.toml",
	"cli.json",
}

// readFromDisk performs the file lookup + parse. Missing file: empty config
// returned. Parse error: warning logged, empty config returned (the CLI
// continues with built-in defaults rather than aborting).
func readFromDisk() *Config {
	path, err := resolveExistingConfigPath()
	if err != nil || path == "" {
		return &Config{}
	}
	b, err := os.ReadFile(path) // #nosec G304 -- path derived from XDG envvar; user-owned
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			genlog.Warn("userconfig: cannot read", "path", path, "err", err)
		}
		return &Config{}
	}
	cfg, err := parseConfig(path, b)
	if err != nil {
		genlog.Warn("userconfig: malformed, ignoring", "path", path, "err", err)
		return &Config{}
	}
	return cfg
}

// parseConfig dispatches on the file extension. Mirrors
// projectfile.ReadRawFromPath — same encoding set, same precedence.
func parseConfig(path string, data []byte) (*Config, error) {
	cfg := &Config{}
	switch ext := filepath.Ext(path); ext {
	case extTOML:
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	case extYAML, extYAML2:
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	case extJSON:
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported user-config extension: %s", ext)
	}
	return cfg, nil
}

// resolveExistingConfigPath returns the first existing cli.<ext> under the
// XDG config dir, or "" if none exist. Missing files are not an error —
// they just mean "use built-in defaults". A real error (e.g. unreadable
// home dir) propagates so the caller can fall back to an empty config
// without silently masking platform issues.
func resolveExistingConfigPath() (string, error) {
	base, err := configBaseDir()
	if err != nil {
		return "", err
	}
	for _, name := range candidateBasenames {
		p := filepath.Join(base, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", nil
}

// ExistingPath returns the resolved on-disk path of the user-config file the
// loader would read, or "" if no cli.<ext> exists yet. Exposed so the
// `setup` command can show users where they are editing.
func ExistingPath() (string, error) { return resolveExistingConfigPath() }

// WritePath returns the absolute destination path for a cli.<ext> file in
// the canonical config dir, creating no directories. Caller is responsible
// for `os.MkdirAll` before Write; we keep that side-effect out of pure
// path resolution so callers can dry-run safely.
func WritePath(ext string) (string, error) {
	if !isSupportedExt(ext) {
		return "", fmt.Errorf("unsupported user-config extension: %s", ext)
	}
	base, err := configBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "cli"+ext), nil
}

// Write marshals cfg into the encoding implied by ext (`.yaml`, `.yml`,
// `.toml`, `.json`) and persists it under the canonical config dir. Returns
// the on-disk path so callers can report it. Side effects: creates the
// config dir if missing; clears the load cache so the next Load() sees the
// write.
func Write(cfg *Config, ext string) (string, error) {
	path, err := WritePath(ext)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	data, err := marshalConfig(cfg, ext)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	Reset()
	return path, nil
}

// marshalConfig is the inverse of parseConfig — same ext-keyed dispatch.
// Pure: no I/O, no logging.
func marshalConfig(cfg *Config, ext string) ([]byte, error) {
	switch ext {
	case extTOML:
		return toml.Marshal(cfg)
	case extYAML, extYAML2:
		return yaml.Marshal(cfg)
	case extJSON:
		// Indented for human-edit friendliness, matches the rest of pf-cli's
		// JSON output conventions (`projectfile.Write`'s JSON branch).
		return marshalJSONIndented(cfg)
	default:
		return nil, fmt.Errorf("unsupported user-config extension: %s", ext)
	}
}

// marshalJSONIndented wraps json.MarshalIndent with a trailing newline so
// the file matches POSIX text-file expectations (and our other writers).
func marshalJSONIndented(cfg *Config) ([]byte, error) {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// isSupportedExt is a one-line predicate the WritePath / Write entry
// points share. New encodings land here plus parseConfig/marshalConfig.
func isSupportedExt(ext string) bool {
	switch ext {
	case extTOML, extYAML, extYAML2, extJSON:
		return true
	}
	return false
}

// configBaseDir resolves the XDG config root for pf-cli. Mirrors the XDG
// convention pf-cli uses everywhere else (see internal/spdx/spdx.go
// cachePath for the same pattern with XDG_CACHE_HOME).
func configBaseDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "projectfile"), nil
}
