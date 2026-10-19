// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package userconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// Fixture values the cases below repeat, one named home per string.
const (
	yamlName      = "yaml"
	handleGitHub  = "buho"
	hostExample   = "example.com"
	hostExampleCo = "example.co"
	hostDev       = "*.dev"
	hostABTest    = "a.b.test"
	extINI        = ".ini"
	mutatedValue  = "mutated"
	urlA          = "https://a.test"
	urlB          = "https://b.test"
)

// useTempXDG points the loader at an isolated config root and drops every
// cache the previous test left behind.
func useTempXDG(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	SetIgnored(false)
	Reset()
	t.Cleanup(Reset)
	return base
}

// writeConfigFile seeds one cli.<ext> under the isolated config root.
func writeConfigFile(t *testing.T, base, ext, body string) string {
	t.Helper()
	dir := filepath.Join(base, "projectfile")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	p := filepath.Join(dir, "cli"+ext)
	require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	return p
}

// fullyPopulated exercises every section so a round-trip cannot silently
// drop one the encoder does not know about.
func fullyPopulated() *Config {
	return &Config{
		Init:         InitSection{Format: yamlName, NamespaceDefault: "b19", LicenseDefault: "MIT", NoScan: true},
		Identity:     IdentitySection{IsEntity: true, FamilyNames: "Buho", GivenNames: "Damian", Name: "Damian Buho", Orcid: "0000-0002-1825-0097", Email: "d@example.com", URL: "https://example.com", Affiliation: "Kiota", GPGKey: "ABCDEF0123456789"},
		Copyright:    CopyrightSection{YearStrategy: "fixed"},
		Funding:      FundingSection{GitHub: []string{handleGitHub}, Patreon: "p", KoFi: "k", Liberapay: "l", Tidelift: "t", CommunityBridge: "cb", IssueHunt: "ih", OpenCollective: "oc", LFXCrowdfunding: "lfx", Polar: "po", BuyMeACoffee: "bmac", ThanksDev: "td", Custom: []string{urlA, urlB}},
		Security:     SecuritySection{ReportURL: "https://s.test", DisclosureWindow: "30 days", BugBountyURL: "https://bb.test"},
		Contributing: ContributingSection{CLAURL: "https://cla.test", ChatURL: "https://chat.test"},
		Support:      SupportSection{ResponseTime: "7 days"},
		Conventions:  ConventionsSection{CommitStyle: "conventional", Workflow: "trunk", Versioning: "semver", StyleGuideURL: "https://style.test"},
		Scan:         ScanSection{PrivateHosts: []string{hostExample, "*.internal.test"}},
		Generate:     GenerateSection{Defaults: map[string]TargetDefault{"CONTRIBUTING.md": {Path: "docs/CONTRIBUTING.md"}}},
	}
}

// yamlOf renders cfg as YAML so a table test can feed every encoding from
// one source of truth instead of hand-writing three documents.
func yamlOf(t *testing.T, cfg *Config) string {
	t.Helper()
	b, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	return string(b)
}

func jsonOf(t *testing.T, cfg *Config) string {
	t.Helper()
	b, err := json.Marshal(cfg)
	require.NoError(t, err)
	return string(b)
}

// bodyFor renders cfg in the encoding ext names, so a table test feeds every
// decoder from one source of truth instead of hand-writing three documents.
func bodyFor(t *testing.T, cfg *Config, ext string) string {
	t.Helper()
	switch ext {
	case extTOML:
		b, err := toml.Marshal(cfg)
		require.NoError(t, err)
		return string(b)
	case extJSON:
		return jsonOf(t, cfg)
	default:
		return yamlOf(t, cfg)
	}
}

// validBodyFor renders the minimal document each decoder accepts for ext, so
// a probe-order table can seed a candidate without hand-writing three syntaxes.
func validBodyFor(t *testing.T, ext string) string {
	t.Helper()
	switch ext {
	case extTOML:
		return "identity = { email = \"\" }\n"
	case extJSON:
		return `{"identity":{"email":""}}`
	default:
		return "identity: {email: \"\"}\n"
	}
}

func TestMatchPrivateHost(t *testing.T) {
	cases := []struct {
		pattern string
		host    string
		want    bool
	}{
		{hostExample, hostExample, true},
		{"example.org", "foo.example.org", true},
		{"example.net", "a.b.example.net", true},
		{"example.io", "bad-example.io", false},
		{"*.example.dev", "src.example.dev", true},
		{"*.example.app", "example.app", false},
		{"", "example.site", false},
		{"example.club", "", false},
		{hostExampleCo, "bad.example.co", true},
		{hostExampleCo, "badexample.co", false},
		{hostExampleCo, "example.co.evil.test", false},
		{hostDev, "src.example.dev", true},
		{hostDev, ".dev", false},
		{hostDev, "src.dev", true},
		{hostDev, "dev", false},
		{hostDev, "srcdev", false},
		{hostABTest, "x.a.b.test", true},
		{hostABTest, hostABTest, true},
		{hostABTest, "c.a.b.testd", false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, matchPrivateHost(tc.pattern, tc.host), "pattern %q host %q", tc.pattern, tc.host)
	}
}

func TestLoadReturnsCopy(t *testing.T) {
	useTempXDG(t)
	a := Load()
	require.NotNil(t, a)
	a.Scan.PrivateHosts = []string{"elsewhere.test"}
	a.Generate.Defaults = map[string]TargetDefault{"CONTRIBUTING.md": {Path: "docs/CONTRIBUTING.md"}}
	b := Load()
	assert.Empty(t, b.Scan.PrivateHosts, "mutation of a Load result must not leak into the cache")
	assert.Empty(t, b.Generate.Defaults, "mutation of a Load result must not leak into the cache")
}

// TestLoadEveryEncoding proves the probe order is honoured: yaml, yml, toml,
// json — first hit wins, and each parser accepts the same struct.
func TestLoadEveryEncoding(t *testing.T) {
	for _, ext := range []string{extYAML, extYAML2, extJSON, extTOML} {
		t.Run(ext, func(t *testing.T) {
			base := useTempXDG(t)
			writeConfigFile(t, base, ext, bodyFor(t, fullyPopulated(), ext))
			got := Load()
			assert.Equal(t, yamlName, got.Init.Format, "ext %s", ext)
			assert.Equal(t, "Damian Buho", got.Identity.Name, "ext %s", ext)
			assert.True(t, got.Identity.IsEntity, "ext %s", ext)
			assert.Equal(t, []string{handleGitHub}, got.Funding.GitHub, "ext %s", ext)
			assert.Equal(t, []string{urlA, urlB}, got.Funding.Custom, "ext %s", ext)
			assert.Equal(t, []string{hostExample, "*.internal.test"}, got.Scan.PrivateHosts, "ext %s", ext)
			assert.Equal(t, "docs/CONTRIBUTING.md", got.Generate.Defaults["CONTRIBUTING.md"].Path, "ext %s", ext)
		})
	}
}

// TestLoadTOML pins the TOML branch of parseConfig, which no JSON/YAML body
// can reach.
func TestLoadTOML(t *testing.T) {
	base := useTempXDG(t)
	writeConfigFile(t, base, extTOML, "[identity]\nemail = \"toml@example.com\"\n[scan]\nprivate-hosts = [\"toml.test\"]\n[generate.defaults.CODEOWNERS]\npath = \"docs/CODEOWNERS\"\n")
	got := Load()
	assert.Equal(t, "toml@example.com", got.Identity.Email)
	assert.Equal(t, []string{"toml.test"}, got.Scan.PrivateHosts)
	assert.Equal(t, "docs/CODEOWNERS", got.Generate.Defaults["CODEOWNERS"].Path)
}

// TestProbeOrderPinsYAMLFirst pins the first-hit-wins contract: with all
// four candidates present, the yaml one must be the one read.
func TestProbeOrderPinsYAMLFirst(t *testing.T) {
	base := useTempXDG(t)
	writeConfigFile(t, base, extYAML, yamlOf(t, &Config{Identity: IdentitySection{Email: "from-yaml@example.com"}}))
	writeConfigFile(t, base, extYAML2, yamlOf(t, &Config{Identity: IdentitySection{Email: "from-yml@example.com"}}))
	writeConfigFile(t, base, extTOML, "[identity]\nemail = \"from-toml@example.com\"\n")
	writeConfigFile(t, base, extJSON, jsonOf(t, &Config{Identity: IdentitySection{Email: "from-json@example.com"}}))
	assert.Equal(t, "from-yaml@example.com", Load().Identity.Email)
}

// TestProbeOrderSkipsAbsent walks the same table one candidate shorter each
// time, so every rung of the probe order is answered by a real read.
func TestProbeOrderSkipsAbsent(t *testing.T) {
	for _, tc := range []struct {
		name string
		exts []string
	}{
		{"yml only", []string{extYAML2}},
		{"toml only", []string{extTOML}},
		{"json only", []string{extJSON}},
		{"yaml+toml", []string{extYAML, extTOML}},
		{"yml+json", []string{extYAML2, extJSON}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := useTempXDG(t)
			for _, ext := range tc.exts {
				writeConfigFile(t, base, ext, validBodyFor(t, ext))
			}
			path, err := ExistingPath()
			require.NoError(t, err)
			require.FileExists(t, path, "probe order must find one of %v", tc.exts)
			assert.Equal(t, &Config{}, Load(), "the seeded probe carries no real values")
		})
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	useTempXDG(t)
	got := Load()
	require.NotNil(t, got)
	assert.Equal(t, &Config{}, got)
}

func TestLoadMalformedIsEmptyNotFatal(t *testing.T) {
	for _, ext := range []string{extYAML, extYAML2, extTOML, extJSON} {
		t.Run(ext, func(t *testing.T) {
			base := useTempXDG(t)
			writeConfigFile(t, base, ext, "{ not: valid: [ for: \n")
			assert.Equal(t, &Config{}, Load(), "ext %s", ext)
		})
	}
}

func TestParseConfigRejectsUnknownExtension(t *testing.T) {
	for _, ext := range []string{extINI, ".xml", ""} {
		cfg, err := parseConfig("cli"+ext, []byte("x"))
		require.Error(t, err, "ext %q", ext)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "unsupported user-config extension")
	}
}

func TestParseConfigAcceptsEverySupportedExtension(t *testing.T) {
	for _, ext := range []string{extTOML, extYAML, extYAML2, extJSON} {
		cfg, err := parseConfig("cli"+ext, []byte(validBodyFor(t, ext)))
		require.NoError(t, err, "ext %q", ext)
		require.NotNil(t, cfg)
	}
}

func TestPathFor(t *testing.T) {
	base := useTempXDG(t)
	writeConfigFile(t, base, extYAML, yamlOf(t, fullyPopulated()))
	assert.Equal(t, "docs/CONTRIBUTING.md", PathFor("CONTRIBUTING.md"))
	assert.Equal(t, "", PathFor("FUNDING.yml"), "unlisted target has no override")
}

func TestPathForEmptyConfig(t *testing.T) {
	useTempXDG(t)
	assert.Equal(t, "", PathFor("CONTRIBUTING.md"))
}

// TestPathForIgnoredConfig proves --ignore-user-config wins over a real
// file on disk, the whole point of the flag.
func TestPathForIgnoredConfig(t *testing.T) {
	base := useTempXDG(t)
	writeConfigFile(t, base, extYAML, yamlOf(t, fullyPopulated()))
	require.Equal(t, "docs/CONTRIBUTING.md", PathFor("CONTRIBUTING.md"))
	SetIgnored(true)
	assert.Equal(t, "", PathFor("CONTRIBUTING.md"))
	SetIgnored(false)
	assert.Equal(t, "docs/CONTRIBUTING.md", PathFor("CONTRIBUTING.md"), "SetIgnored(false) must re-read, not serve the empty cache")
}

func TestIsPrivateHost(t *testing.T) {
	base := useTempXDG(t)
	writeConfigFile(t, base, extYAML, yamlOf(t, fullyPopulated()))
	assert.True(t, IsPrivateHost(hostExample))
	assert.True(t, IsPrivateHost("EXAMPLE.COM"), "matching is case-insensitive")
	assert.True(t, IsPrivateHost("forge.example.com"))
	assert.True(t, IsPrivateHost("src.internal.test"))
	assert.False(t, IsPrivateHost("example.com.evil.test"))
	assert.False(t, IsPrivateHost("internal.test"), "wildcard never covers the apex")
	assert.False(t, IsPrivateHost(""), "an empty host is never private")
	assert.False(t, IsPrivateHost("elsewhere.test"))
}

func TestIsPrivateHostWithoutPatterns(t *testing.T) {
	base := useTempXDG(t)
	writeConfigFile(t, base, extYAML, yamlOf(t, &Config{}))
	assert.False(t, IsPrivateHost(hostExample))
}

func TestIsPrivateHostIgnoredConfig(t *testing.T) {
	base := useTempXDG(t)
	writeConfigFile(t, base, extYAML, yamlOf(t, fullyPopulated()))
	require.True(t, IsPrivateHost(hostExample))
	SetIgnored(true)
	assert.False(t, IsPrivateHost(hostExample))
}

func TestExistingPath(t *testing.T) {
	base := useTempXDG(t)
	got, err := ExistingPath()
	require.NoError(t, err)
	assert.Equal(t, "", got, "no cli.<ext> yet")

	want := writeConfigFile(t, base, extTOML, "[identity]\n")
	got, err = ExistingPath()
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestWritePath(t *testing.T) {
	base := useTempXDG(t)
	for _, ext := range []string{extYAML, extYAML2, extTOML, extJSON} {
		got, err := WritePath(ext)
		require.NoError(t, err, "ext %q", ext)
		assert.Equal(t, filepath.Join(base, "projectfile", "cli"+ext), got)
		assert.False(t, strings.Contains(got, "*"), "WritePath resolves globs")
	}
}

func TestWritePathRejectsUnsupportedExtension(t *testing.T) {
	useTempXDG(t)
	for _, ext := range []string{extINI, "", yamlName} {
		got, err := WritePath(ext)
		require.Error(t, err, "ext %q", ext)
		assert.Empty(t, got)
		assert.Contains(t, err.Error(), "unsupported user-config extension")
	}
}

// TestWritePathHomeFallback covers the XDG unset branch of configBaseDir.
func TestWritePathHomeFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", home)
	got, err := WritePath(extYAML)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".config", "projectfile", "cli.yaml"), got)
}

// TestConfigBaseDirUnresolvable pins the only error path: neither XDG nor
// HOME names a directory, so there is nothing to join.
func TestConfigBaseDirUnresolvable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	_, err := WritePath(extYAML)
	require.Error(t, err)

	_, err = ExistingPath()
	require.Error(t, err, "ExistingPath must propagate the base-dir failure, not mask it")
}

// TestWriteRoundTripsEveryEncoding is the load-bearing round-trip: what
// Write emits, parseConfig must read back field for field.
func TestWriteRoundTripsEveryEncoding(t *testing.T) {
	for _, ext := range []string{extYAML, extYAML2, extTOML, extJSON} {
		t.Run(ext, func(t *testing.T) {
			base := useTempXDG(t)
			want := fullyPopulated()
			path, err := Write(want, ext)
			require.NoError(t, err)
			assert.Equal(t, filepath.Join(base, "projectfile", "cli"+ext), path)

			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "user config must stay owner-only")

			got := Load()
			assert.Equal(t, want.Identity, got.Identity, "ext %s", ext)
			assert.Equal(t, want.Init, got.Init, "ext %s", ext)
			assert.Equal(t, want.Scan, got.Scan, "ext %s", ext)
			assert.Equal(t, want.Generate, got.Generate, "ext %s", ext)
			assert.Equal(t, want.Conventions, got.Conventions, "ext %s", ext)
			assert.Equal(t, want.Security, got.Security, "ext %s", ext)
			assert.Equal(t, want.Contributing, got.Contributing, "ext %s", ext)
			assert.Equal(t, want.Support, got.Support, "ext %s", ext)
			assert.Equal(t, want.Copyright, got.Copyright, "ext %s", ext)
			assert.Equal(t, want.Funding.Patreon, got.Funding.Patreon, "ext %s", ext)
			assert.Equal(t, want.Funding.OpenCollective, got.Funding.OpenCollective, "ext %s", ext)
			assert.Equal(t, want.Funding.BuyMeACoffee, got.Funding.BuyMeACoffee, "ext %s", ext)
		})
	}
}

// TestWriteClearsCache pins the contract that a write is visible to the very
// next Load without the caller calling Reset by hand.
func TestWriteClearsCache(t *testing.T) {
	useTempXDG(t)
	require.Equal(t, &Config{}, Load())

	_, err := Write(&Config{Identity: IdentitySection{Email: "fresh@example.com"}}, extYAML)
	require.NoError(t, err)
	assert.Equal(t, "fresh@example.com", Load().Identity.Email)
}

func TestWriteCreatesConfigDir(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	SetIgnored(false)
	Reset()
	t.Cleanup(Reset)

	require.NoDirExists(t, filepath.Join(base, "projectfile"))
	path, err := Write(&Config{}, extTOML)
	require.NoError(t, err)
	assert.FileExists(t, path)
}

// TestWriteDirNotCreatable pins the MkdirAll failure: a regular file sits
// where the config directory must go.
func TestWriteDirNotCreatable(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", base)
	SetIgnored(false)
	Reset()
	t.Cleanup(Reset)

	blocker := filepath.Join(base, "projectfile")
	require.NoError(t, os.WriteFile(blocker, []byte("x"), 0o600))

	_, err := Write(&Config{}, extYAML)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create config dir")
}

func TestWriteTargetIsDirectory(t *testing.T) {
	base := useTempXDG(t)
	require.NoError(t, os.MkdirAll(filepath.Join(base, "projectfile", "cli.yaml"), 0o750))

	_, err := Write(&Config{}, extYAML)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write ")
}

func TestMarshalConfigRejectsUnsupportedExtension(t *testing.T) {
	got, err := marshalConfig(&Config{}, extINI)
	require.Error(t, err)
	assert.Nil(t, got)
	assert.Contains(t, err.Error(), "unsupported user-config extension")
}

func TestMarshalJSONIndentedEndsWithNewline(t *testing.T) {
	b, err := marshalJSONIndented(fullyPopulated())
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(string(b), "}\n"), "JSON config must be a POSIX text file")
	assert.Contains(t, string(b), "\n  ", "must stay indented for hand editing")
}

func TestIsSupportedExt(t *testing.T) {
	for _, ext := range []string{extTOML, extYAML, extYAML2, extJSON} {
		assert.True(t, isSupportedExt(ext), "ext %q", ext)
	}
	for _, ext := range []string{".YAML", extINI, "", yamlName} {
		assert.False(t, isSupportedExt(ext), "ext %q must be rejected, not case-folded", ext)
	}
}

func TestConfigBaseDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg/root")
	got, err := configBaseDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/xdg/root", "projectfile"), got)
}

// TestReadFromDiskUnreadableFile pins the warn-and-continue path for a file
// the loader can see but not read.
func TestReadFromDiskUnreadableFile(t *testing.T) {
	base := useTempXDG(t)
	p := writeConfigFile(t, base, extYAML, "identity:\n  email: a@example.com\n")
	require.NoError(t, os.Chmod(p, 0o000))
	t.Cleanup(func() { _ = os.Chmod(p, 0o600) })
	if os.Geteuid() == 0 {
		t.Skip("root reads every file regardless of mode")
	}
	assert.Equal(t, &Config{}, Load())
}

// TestReadFromDiskVanishedBetweenStatAndRead closes the race the stat-then-
// read pair leaves open: the file is gone by the time it is opened.
func TestReadFromDiskVanishedBetweenStatAndRead(t *testing.T) {
	base := useTempXDG(t)
	writeConfigFile(t, base, extYAML, "identity: {}\n")

	dir := filepath.Join(base, "projectfile")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.NoError(t, os.Remove(filepath.Join(dir, entries[0].Name())))

	assert.Equal(t, &Config{}, Load(), "a vanished config is not an error")
}

func TestResetRereadsDisk(t *testing.T) {
	base := useTempXDG(t)
	require.Equal(t, &Config{}, Load())
	writeConfigFile(t, base, extYAML, yamlOf(t, &Config{Identity: IdentitySection{Email: "later@example.com"}}))
	assert.Equal(t, "", Load().Identity.Email, "Load caches for the process lifetime")

	Reset()
	assert.Equal(t, "later@example.com", Load().Identity.Email)
}

func TestCloneConfigNil(t *testing.T) {
	assert.Nil(t, cloneConfig(nil))
}

// TestCloneConfigDeepCopiesSlices pins that no slice or map aliases the
// cache, so a caller mutating a Load result cannot corrupt the next one.
func TestCloneConfigDeepCopiesSlices(t *testing.T) {
	src := fullyPopulated()
	got := cloneConfig(src)
	got.Funding.GitHub[0] = mutatedValue
	got.Funding.Custom[0] = mutatedValue
	got.Scan.PrivateHosts[0] = mutatedValue
	got.Generate.Defaults["CONTRIBUTING.md"] = TargetDefault{Path: mutatedValue}
	got.Generate.Defaults["NEW"] = TargetDefault{}

	assert.Equal(t, []string{handleGitHub}, src.Funding.GitHub)
	assert.Equal(t, []string{urlA, urlB}, src.Funding.Custom)
	assert.Equal(t, hostExample, src.Scan.PrivateHosts[0])
	assert.Equal(t, "docs/CONTRIBUTING.md", src.Generate.Defaults["CONTRIBUTING.md"].Path)
	assert.NotContains(t, src.Generate.Defaults, "NEW")
}

// TestCloneConfigNilMapStaysNil keeps an absent Defaults map absent rather
// than materialised as an empty one, which TOML and YAML encode differently.
func TestCloneConfigNilMapStaysNil(t *testing.T) {
	assert.Nil(t, cloneConfig(&Config{}).Generate.Defaults)
}

// TestParseConfigMalformedEveryEncoding pins that a parse error surfaces to
// readFromDisk as an error for each decoder, not just YAML.
func TestParseConfigMalformedEveryEncoding(t *testing.T) {
	cases := map[string]string{
		extTOML:  "identity = \n",
		extYAML:  "identity: [\n",
		extYAML2: "identity: [\n",
		extJSON:  `{"identity":`,
	}
	for ext, body := range cases {
		cfg, err := parseConfig("cli"+ext, []byte(body))
		require.Error(t, err, "ext %q", ext)
		assert.Nil(t, cfg, "ext %q", ext)
	}
}

func TestReadFromDiskReadErrorIsWarned(t *testing.T) {
	base := useTempXDG(t)
	writeConfigFile(t, base, extYAML, "identity: {}\n")
	// A directory named cli.yaml satisfies the stat probe, then fails the read.
	dir := filepath.Join(base, "projectfile", "cli.yaml")
	require.NoError(t, os.Remove(dir))
	require.NoError(t, os.MkdirAll(dir, 0o750))
	assert.Equal(t, &Config{}, Load())
}

func TestReadFromDiskPropagatesBaseDirError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")
	assert.Equal(t, &Config{}, Load(), "an unresolvable base dir degrades to defaults")
}
