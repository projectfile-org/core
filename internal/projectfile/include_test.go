// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testApacheLicense = "Apache-2.0"
	testAcme          = "Acme"
	testItems         = "items"
	testDocker        = "docker"
	testLinks         = "links"
	testIdentity      = "identity"
	testMIT           = "MIT"
	testAuthor        = "author"
	testEmail         = "damian.buho@proton.me"
	testFrom          = "from"
	testFamilyNames   = "family-names"
	testGivenNames    = "given-names"
	testBuho          = "Búho"
	testDamian        = "Damián"
	testIncludeURL    = "https://include.git"
	testNested        = "nested"
	testExampleURL    = "https://x.example"
	testOrg           = "org"
	testScalar        = "scalar"
	testValue         = "value"
)

func TestDeepMergeMaps(t *testing.T) {
	winner := map[string]any{"a": "1"}
	loser := map[string]any{"b": "2"}
	got := deepMerge(winner, loser)
	assert.Equal(t, "1", got["a"])
	assert.Equal(t, "2", got["b"])
}

func TestDeepMergeWinnerOverridesScalar(t *testing.T) {
	winner := map[string]any{"k": "winner"}
	loser := map[string]any{"k": "loser"}
	got := deepMerge(winner, loser)
	assert.Equal(t, "winner", got["k"])
}

func TestDeepMergeRecursiveMaps(t *testing.T) {
	winner := map[string]any{
		testNested: map[string]any{"a": "w"},
	}
	loser := map[string]any{
		testNested: map[string]any{"b": "l"},
	}
	got := deepMerge(winner, loser)
	nested := got[testNested].(map[string]any)
	assert.Equal(t, "w", nested["a"])
	assert.Equal(t, "l", nested["b"])
}

func TestDeepMergeConcatenatesSlices(t *testing.T) {
	winner := map[string]any{
		testItems: []any{"c"},
	}
	loser := map[string]any{
		testItems: []any{"a", "b"},
	}
	got := deepMerge(winner, loser)
	items := got[testItems].([]any)
	assert.Equal(t, []any{"a", "b", "c"}, items,
		"slices must be concatenated (loser first, winner second)")
}

// TestDeepMergeDedupsSliceScalars proves the generic-slice merge drops
// duplicates: a value present on both sides survives once. This is what keeps
// a diamond (same document reached via two branches) from duplicating list
// values.
func TestDeepMergeDedupsSliceScalars(t *testing.T) {
	winner := map[string]any{"keywords": []any{"go", testDocker}}
	loser := map[string]any{"keywords": []any{testDocker, "rust"}}
	got := deepMerge(winner, loser)
	assert.Equal(t, []any{testDocker, "rust", "go"}, got["keywords"],
		"duplicates dropped, first-occurrence order preserved (loser first, then winner-only)")
}

// TestDeepMergeDedupsSliceOfMaps proves dedup works for map entries (deep
// equality), not just scalars — two identical link objects collapse.
func TestDeepMergeDedupsSliceOfMaps(t *testing.T) {
	link := map[string]any{keyType: LinkHomepage, keyURL: testExampleURL}
	winner := map[string]any{testLinks: []any{link, map[string]any{keyType: "repo", keyURL: testExampleURL + "/r"}}}
	loser := map[string]any{testLinks: []any{link}}
	got := deepMerge(winner, loser)
	links := got[testLinks].([]any)
	assert.Len(t, links, 2, "deep-equal map entry present on both sides collapses to one")
}

// TestDeepMergeDedupKeepsDistinctMaps proves dedup does NOT collapse maps
// that merely share some fields — only deep-equal entries drop.
func TestDeepMergeDedupKeepsDistinctMaps(t *testing.T) {
	winner := map[string]any{testLinks: []any{map[string]any{keyType: LinkHomepage, keyURL: testExampleURL}}}
	loser := map[string]any{testLinks: []any{map[string]any{keyType: "repo", keyURL: testExampleURL}}}
	got := deepMerge(winner, loser)
	assert.Len(t, got[testLinks].([]any), 2, "distinct maps (different type) both kept")
}

func TestDeepMergeNestedSliceConcat(t *testing.T) {
	winner := map[string]any{
		testOrg: map[string]any{
			BaseName: map[string]any{
				"ignores": map[string]any{
					"git": map[string]any{
						"include": []any{".container/foundation/usr/"},
					},
				},
			},
		},
	}
	loser := map[string]any{
		testOrg: map[string]any{
			BaseName: map[string]any{
				"ignores": map[string]any{
					"git": map[string]any{
						"include": []any{".secrets/", "reports/"},
					},
				},
			},
		},
	}
	got := deepMerge(winner, loser)
	outer := got[testOrg].(map[string]any)
	pf := outer[BaseName].(map[string]any)
	ign := pf["ignores"].(map[string]any)
	git := ign["git"].(map[string]any)
	inc := git["include"].([]any)
	assert.Equal(t, []any{".secrets/", "reports/", ".container/foundation/usr/"}, inc)
}

func TestDeepMergeSliceVsNonSlice(t *testing.T) {
	winner := map[string]any{"k": []any{"w"}}
	loser := map[string]any{"k": testScalar}
	got := deepMerge(winner, loser)
	assert.Equal(t, []any{"w"}, got["k"],
		"winner's non-matching type should be used verbatim")
}

func TestDeepMergeEmptyWinner(t *testing.T) {
	winner := map[string]any{}
	loser := map[string]any{"a": "1", "b": "2"}
	got := deepMerge(winner, loser)
	assert.Equal(t, map[string]any{"a": "1", "b": "2"}, got)
}

func TestDeepMergeEmptyLoser(t *testing.T) {
	winner := map[string]any{"a": "1"}
	loser := map[string]any{}
	got := deepMerge(winner, loser)
	assert.Equal(t, map[string]any{"a": "1"}, got)
}

func TestDeepMergeBothEmpty(t *testing.T) {
	got := deepMerge(map[string]any{}, map[string]any{})
	assert.Equal(t, map[string]any{}, got)
}

func TestDeepMergeScalarOverridesMap(t *testing.T) {
	winner := map[string]any{"k": testScalar}
	loser := map[string]any{"k": map[string]any{testNested: testValue}}
	got := deepMerge(winner, loser)
	assert.Equal(t, testScalar, got["k"],
		"winner scalar should replace loser map entirely")
}

func TestDeepMergeMapOverridesScalar(t *testing.T) {
	winner := map[string]any{"k": map[string]any{testNested: testValue}}
	loser := map[string]any{"k": testScalar}
	got := deepMerge(winner, loser)
	assert.Equal(t, map[string]any{testNested: testValue}, got["k"],
		"winner map should replace loser scalar entirely")
}

func TestDeepMergeScalarOverridesSlice(t *testing.T) {
	winner := map[string]any{"k": testScalar}
	loser := map[string]any{"k": []any{"a", "b"}}
	got := deepMerge(winner, loser)
	assert.Equal(t, testScalar, got["k"],
		"winner scalar should replace loser slice (types do not match)")
}

func TestDeepMergeNilWinnerValue(t *testing.T) {
	winner := map[string]any{"k": nil}
	loser := map[string]any{"k": "existing"}
	got := deepMerge(winner, loser)
	assert.Nil(t, got["k"], "winner nil should replace loser value")
}

func TestDeepMergeNilLoserValue(t *testing.T) {
	winner := map[string]any{"k": testValue}
	loser := map[string]any{"k": nil}
	got := deepMerge(winner, loser)
	assert.Equal(t, testValue, got["k"])
}

func TestDeepMergeNonStringScalars(t *testing.T) {
	winner := map[string]any{
		"int":   float64(42),
		"bool":  true,
		"float": float64(3.14),
	}
	loser := map[string]any{
		"int":   float64(0),
		"bool":  false,
		"float": float64(0.0),
	}
	got := deepMerge(winner, loser)
	assert.Equal(t, float64(42), got["int"])
	assert.Equal(t, true, got["bool"])
	assert.Equal(t, float64(3.14), got["float"])
}

func TestDeepMergeWinnerOverridesKeyInNestedMap(t *testing.T) {
	winner := map[string]any{
		testIdentity: map[string]any{keyName: "winner-name", "version": "2.0"},
	}
	loser := map[string]any{
		testIdentity: map[string]any{keyName: "loser-name", keyNamespace: "org.example"},
	}
	got := deepMerge(winner, loser)
	ident := got[testIdentity].(map[string]any)
	assert.Equal(t, "winner-name", ident[keyName], "winner scalar overrides loser scalar in nested map")
	assert.Equal(t, "2.0", ident["version"], "winner-only key preserved")
	assert.Equal(t, "org.example", ident[keyNamespace], "loser-only key preserved")
}

func TestDeepMergeThreeWayAccumulation(t *testing.T) {
	inc1 := map[string]any{
		keyLicense:      map[string]any{keySpdx: testMIT},
		keyTechnologies: []any{"go"},
	}
	inc2 := map[string]any{
		keyLicense:      map[string]any{"covers": "source"},
		keyTechnologies: []any{testDocker},
	}
	acc := deepMerge(inc1, map[string]any{})
	acc = deepMerge(inc2, acc)

	lic := acc[keyLicense].(map[string]any)
	assert.Equal(t, testMIT, lic[keySpdx], "first include spdx preserved")
	assert.Equal(t, "source", lic["covers"], "second include covers added")

	stack := acc[keyTechnologies].([]any)
	assert.Equal(t, []any{"go", testDocker}, stack,
		"slices accumulated across includes (first, then second)")
}

func TestDeepMergeThreeWayBaseOverIncludes(t *testing.T) {
	acc := map[string]any{
		keyLicense:      map[string]any{keySpdx: testApacheLicense},
		keyTechnologies: []any{"go"},
	}
	base := map[string]any{
		keyLicense:      map[string]any{keySpdx: testMIT},
		keyTechnologies: []any{testDocker},
	}
	got := deepMerge(base, acc)

	lic := got[keyLicense].(map[string]any)
	assert.Equal(t, testMIT, lic[keySpdx], "base scalar overrides include scalar in nested map")

	stack := got[keyTechnologies].([]any)
	assert.Equal(t, []any{"go", testDocker}, stack,
		"base slice concatenated after include slice")
}

func TestDeepMergeSliceOfMaps(t *testing.T) {
	// Generic slice-of-maps behaviour: entries are concatenated, not merged.
	// Uses a non-entity key (`items`) because `people` and `organizations`
	// have identity-aware merging — see TestDeepMergePeopleByIdentity.
	winner := map[string]any{
		testItems: []any{
			map[string]any{keyName: "Alice", keyRoles: []any{testAuthor}},
		},
	}
	loser := map[string]any{
		testItems: []any{
			map[string]any{keyName: "Bob", keyRoles: []any{RoleMaintainer}},
		},
	}
	got := deepMerge(winner, loser)
	items := got[testItems].([]any)
	assert.Equal(t, 2, len(items), "slice-of-map entries are concatenated, not merged")
	bob := items[0].(map[string]any)
	alice := items[1].(map[string]any)
	assert.Equal(t, "Bob", bob[keyName])
	assert.Equal(t, "Alice", alice[keyName])
}

// TestDeepMergePeopleByIdentity reproduces the include-vs-base scenario the
// raw people merge exists for: base (winner) carries the project-scoped
// `from`; include (loser) carries identity + contact fields. Same email
// must fold into a single entry so the schema's required `family-names`
// is satisfied by the include's contribution.
func TestDeepMergePeopleByIdentity(t *testing.T) {
	winner := map[string]any{
		keyPeople: []any{
			map[string]any{
				keyEmail: testEmail,
				testFrom: "2026-06-23",
			},
		},
	}
	loser := map[string]any{
		keyPeople: []any{
			map[string]any{
				testFamilyNames: testBuho,
				testGivenNames:  testDamian,
				keyEmail:        testEmail,
				keyRoles:        []any{testAuthor, RoleMaintainer},
			},
		},
	}
	got := deepMerge(winner, loser)
	people := got[keyPeople].([]any)
	require.Len(t, people, 1, "same-email entries must merge into one")
	p := people[0].(map[string]any)
	assert.Equal(t, testEmail, p[keyEmail])
	assert.Equal(t, testBuho, p[testFamilyNames], "loser fills fields the winner lacks")
	assert.Equal(t, "2026-06-23", p[testFrom], "winner's project-scoped field is preserved")
	assert.Equal(t, []any{testAuthor, RoleMaintainer}, p[keyRoles], "roles are unioned")
}

// TestDeepMergePeopleDistinctIdentities verifies the fall-through contract
// of samePerson at the raw level: two different emails with no name-tier
// match must stay as two entries (the include contributed a different
// person, not a duplicate of the base one).
func TestDeepMergePeopleDistinctIdentities(t *testing.T) {
	winner := map[string]any{
		keyPeople: []any{
			map[string]any{keyEmail: "a@x.example", keyRoles: []any{testAuthor}},
		},
	}
	loser := map[string]any{
		keyPeople: []any{
			map[string]any{keyEmail: "b@x.example", keyRoles: []any{RoleMaintainer}},
		},
	}
	got := deepMerge(winner, loser)
	people := got[keyPeople].([]any)
	assert.Len(t, people, 2, "different identities stay distinct")
}

// TestDeepMergePeopleOrcidDecisive verifies the ORCID tier is decisive:
// even when emails differ, two records sharing an ORCID collapse, and two
// records with different ORCIDs stay distinct (no email/name fallthrough).
func TestDeepMergePeopleOrcidDecisive(t *testing.T) {
	t.Run("same ORCID collapses despite different email", func(t *testing.T) {
		winner := map[string]any{
			keyPeople: []any{
				map[string]any{keyOrcid: "0000-0001-0002-0003", keyEmail: "old@x", testFrom: "2020"},
			},
		}
		loser := map[string]any{
			keyPeople: []any{
				map[string]any{keyOrcid: "https://orcid.org/0000-0001-0002-0003", keyEmail: "new@x", testFamilyNames: "Y"},
			},
		}
		got := deepMerge(winner, loser)
		people := got[keyPeople].([]any)
		require.Len(t, people, 1)
		p := people[0].(map[string]any)
		assert.Equal(t, "old@x", p[keyEmail], "winner email wins; ORCID tier matched but email conflict resolves to winner")
		assert.Equal(t, "Y", p[testFamilyNames], "loser fills the field winner lacked")
		assert.Equal(t, "2020", p[testFrom])
	})
	t.Run("different ORCIDs stay distinct despite same email", func(t *testing.T) {
		winner := map[string]any{
			keyPeople: []any{
				map[string]any{keyOrcid: "0000-0001-0002-0003", keyEmail: "shared@x"},
			},
		}
		loser := map[string]any{
			keyPeople: []any{
				map[string]any{keyOrcid: "0000-0001-0002-9999", keyEmail: "shared@x"},
			},
		}
		got := deepMerge(winner, loser)
		people := got[keyPeople].([]any)
		assert.Len(t, people, 2, "decisive ORCID mismatch must NOT fall through to email tier")
	})
}

// TestDeepMergeOrganizationsByIdentity mirrors TestDeepMergePeopleByIdentity
// for the organizations list (identity tier 3 uses the single `name` field).
func TestDeepMergeOrganizationsByIdentity(t *testing.T) {
	winner := map[string]any{
		keyOrganizations: []any{
			map[string]any{keyName: testAcme, testFrom: "2020"},
		},
	}
	loser := map[string]any{
		keyOrganizations: []any{
			map[string]any{keyName: testAcme, keyURL: "https://acme.example"},
		},
	}
	got := deepMerge(winner, loser)
	orgs := got[keyOrganizations].([]any)
	require.Len(t, orgs, 1)
	o := orgs[0].(map[string]any)
	assert.Equal(t, testAcme, o[keyName])
	assert.Equal(t, "2020", o[testFrom], "winner's project-scoped field is preserved")
	assert.Equal(t, "https://acme.example", o[keyURL], "loser fills fields the winner lacks")
}

func TestDeepMergeDeepNesting(t *testing.T) {
	winner := map[string]any{
		testOrg: map[string]any{
			BaseName: map[string]any{
				"ci": map[string]any{
					"nodes": map[string]any{
						"build": map[string]any{"needs": map[string]any{"fetch": true}},
					},
				},
			},
		},
	}
	loser := map[string]any{
		testOrg: map[string]any{
			BaseName: map[string]any{
				"ci": map[string]any{
					"nodes": map[string]any{
						"test": map[string]any{"needs": map[string]any{"build": true}},
					},
				},
			},
		},
	}
	got := deepMerge(winner, loser)
	nodes := got[testOrg].(map[string]any)[BaseName].(map[string]any)["ci"].(map[string]any)["nodes"].(map[string]any)
	assert.Contains(t, nodes, "build", "winner node preserved")
	assert.Contains(t, nodes, "test", "loser node preserved")
}

func TestDeepMergeDoesNotMutateInputs(t *testing.T) {
	winner := map[string]any{
		testNested: map[string]any{"a": "1"},
		"slice":    []any{"x"},
	}
	loser := map[string]any{
		testNested: map[string]any{"b": "2"},
		"slice":    []any{"y"},
	}
	deepMerge(winner, loser)

	assert.Equal(t, map[string]any{"a": "1"}, winner[testNested],
		"winner nested map should not be mutated")
	assert.Equal(t, []any{"x"}, winner["slice"],
		"winner slice should not be mutated")
	assert.Equal(t, map[string]any{"b": "2"}, loser[testNested],
		"loser nested map should not be mutated")
	assert.Equal(t, []any{"y"}, loser["slice"],
		"loser slice should not be mutated")
}

// isolateCache points XDG_CACHE_HOME at a fresh temp dir so network-fetch
// tests never hit a pre-existing include cache from a previous run.
func isolateCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	return dir
}

// TestFetchHTTPInclude_StatusErrors verifies that non-2xx responses surface
// as a clear "HTTP <code> (hint) for <url>" error instead of being parsed
// as content. Regression guard for the private-repo-gateway confusion where
// the real cause (403/404) was hidden behind a YAML parse failure.
func TestFetchHTTPInclude_StatusErrors(t *testing.T) {
	codes := []int{http.StatusNotFound, http.StatusForbidden, http.StatusUnauthorized, 500}
	for _, code := range codes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			isolateCache(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
			}))
			t.Cleanup(srv.Close)

			_, _, err := fetchHTTPInclude(srv.URL+"/include.yaml", false)
			require.Error(t, err)
			// Message must mention the code AND the requested URL.
			assert.Contains(t, err.Error(), "HTTP")
			assert.Contains(t, err.Error(), srv.URL)
			// Hint must appear for codes we have advice on (404/403/401/5xx).
			if hint := statusHint(code); hint != "" {
				assert.Contains(t, err.Error(), strings.TrimSpace(hint),
					"expected status hint for HTTP %d in error: %v", code, err)
			}
		})
	}
}

// TestFetchHTTPInclude_HTMLLoginWall reproduces the exact bug report: a
// forge returns HTTP 200 with an HTML sign-in page (private repo / wrong
// path). The fetcher must refuse the HTML instead of letting the YAML
// parser choke on it.
func TestFetchHTTPInclude_HTMLLoginWall(t *testing.T) {
	isolateCache(t)
	const loginHTML = "<!DOCTYPE html><html><head><title>Sign in</title></head><body>login</body></html>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(loginHTML))
	}))
	t.Cleanup(srv.Close)

	_, _, err := fetchHTTPInclude(srv.URL+"/include.yaml", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTML", "error must name the HTML/login cause")
	assert.NotContains(t, err.Error(), "mapping values are not allowed",
		"must not leak the raw YAML parser error")
}

// TestFetchHTTPInclude_SameHostRedirectToLogin reproduces the Forgejo
// private-repo gateway: the raw URL 302-redirects (same host) to
// /user/login which returns 200 HTML. The content-type guard catches it.
func TestFetchHTTPInclude_SameHostRedirectToLogin(t *testing.T) {
	isolateCache(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/raw/include.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/user/login", http.StatusFound)
	})
	mux.HandleFunc("/user/login", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><title>Sign in</title></html>"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	_, _, err := fetchHTTPInclude(srv.URL+"/raw/include.yaml", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTML", "error must name the HTML/login cause")
}

// TestFetchHTTPInclude_CrossHostRedirect verifies that a redirect to a
// different host (typical SSO/auth gateway) is refused outright rather
// than followed into an HTML body.
func TestFetchHTTPInclude_CrossHostRedirect(t *testing.T) {
	isolateCache(t)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>sso login</html>"))
	}))
	t.Cleanup(target.Close)

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/login", http.StatusFound)
	}))
	t.Cleanup(origin.Close)

	_, _, err := fetchHTTPInclude(origin.URL+"/include.yaml", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cross-host redirect",
		"error must name the cross-host redirect")
}

// TestFetchHTTPInclude_ValidYAML is the positive control: a 200 OK with
// real YAML content (plain text content-type) succeeds and is cached.
func TestFetchHTTPInclude_ValidYAML(t *testing.T) {
	cacheDir := isolateCache(t)
	const body = "identity:\n  name: test\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	data, hint, err := fetchHTTPInclude(srv.URL+"/include.yaml", false)
	require.NoError(t, err)
	assert.Equal(t, []byte(body), data)
	assert.Equal(t, "include.yaml", hint)
	// Result must be cached for the next call to hit cache tier.
	entries, err := os.ReadDir(filepath.Join(cacheDir, "projectfile-cli", "includes"))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "include should be cached after a successful fetch")
}

// TestFetchHTTPInclude_SelfHealsPoisonedCache verifies that a cache entry
// containing HTML (written by an older pf-cli build before content-type
// validation) is discarded on read and replaced by a fresh fetch.
func TestFetchHTTPInclude_SelfHealsPoisonedCache(t *testing.T) {
	cacheDir := isolateCache(t)
	incDir := filepath.Join(cacheDir, "projectfile-cli", "includes")
	require.NoError(t, os.MkdirAll(incDir, 0o755))

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte("identity:\n  name: fresh\n"))
	}))
	t.Cleanup(srv.Close)

	// Plant a poisoned entry (HTML) for the URL we are about to fetch.
	const poison = "<!DOCTYPE html><html><title>Sign in</title></html>"
	cp, err := includeCachePath(srv.URL + "/include.yaml")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cp, []byte(poison), 0o644))

	data, _, err := fetchHTTPInclude(srv.URL+"/include.yaml", false)
	require.NoError(t, err)
	assert.Equal(t, []byte("identity:\n  name: fresh\n"), data, "poisoned entry must be replaced by fresh fetch")
	assert.Equal(t, 1, calls, "must have performed a network fetch to replace the poison")

	// Cache file must now hold the fresh content, not the poison.
	fresh, err := os.ReadFile(cp)
	require.NoError(t, err)
	assert.Equal(t, "identity:\n  name: fresh\n", string(fresh))
	assert.NotContains(t, string(fresh), "<html", "cache must no longer contain HTML")
}

// --- recursive include resolution (spec §4.9a) ----------------------------

// writeInc writes body to dir/name (creating parent dirs) and returns its path.
func writeInc(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	return p
}

// readInc parses the file at p into a raw map (no include resolution).
func readInc(t *testing.T, p string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(p)
	require.NoError(t, err)
	raw, err := ReadRawFromBytes(p, data)
	require.NoError(t, err)
	return raw
}

func kwSlice(t *testing.T, doc map[string]any) []any {
	t.Helper()
	v, ok := doc["keywords"]
	require.True(t, ok, "expected keywords in resolved doc")
	return v.([]any)
}

// TestResolveIncludes_RecursiveLocal proves an include's own includes are
// resolved transitively: base → a → b, and b's value surfaces in the base.
func TestResolveIncludes_RecursiveLocal(t *testing.T) {
	dir := t.TempDir()
	writeInc(t, dir, "b.yaml", "keywords:\n  - from-b\n")
	writeInc(t, dir, "a.yaml", "includes:\n  - b.yaml\nkeywords:\n  - from-a\n")
	basePath := writeInc(t, dir, "base.yaml",
		"includes:\n  - a.yaml\nidentity:\n  name: root\n")

	resolved, err := resolveIncludes(readInc(t, basePath), dir, basePath, ReadOptions{})
	require.NoError(t, err)
	kw := kwSlice(t, resolved)
	assert.Contains(t, kw, "from-b", "transitive include (b via a) must be resolved")
	assert.Contains(t, kw, "from-a", "direct include (a) preserved")
	ident := resolved[testIdentity].(map[string]any)
	assert.Equal(t, "root", ident[keyName], "base value wins")
}

// TestResolveIncludes_NestedBaseDir proves a relative path declared inside an
// included document resolves against THAT document's directory, not the root.
// a.yaml lives in sub/ and includes sibling b.yaml (also in sub/); the root
// does not know about sub/b.yaml directly.
func TestResolveIncludes_NestedBaseDir(t *testing.T) {
	dir := t.TempDir()
	writeInc(t, dir, "sub/b.yaml", "keywords:\n  - from-b\n")
	writeInc(t, dir, "sub/a.yaml", "includes:\n  - b.yaml\nkeywords:\n  - from-a\n")
	basePath := writeInc(t, dir, "base.yaml", "includes:\n  - sub/a.yaml\n")

	resolved, err := resolveIncludes(readInc(t, basePath), dir, basePath, ReadOptions{})
	require.NoError(t, err, "nested relative include must resolve against include's dir, not root")
	assert.Contains(t, kwSlice(t, resolved), "from-b",
		"b.yaml resolved as sibling of a.yaml inside sub/")
}

// TestResolveIncludes_DirectSelfCycle proves a document that includes itself
// directly is rejected rather than looping.
func TestResolveIncludes_DirectSelfCycle(t *testing.T) {
	dir := t.TempDir()
	basePath := writeInc(t, dir, "base.yaml", "includes:\n  - base.yaml\n")

	_, err := resolveIncludes(readInc(t, basePath), dir, basePath, ReadOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle", "error must name the cycle")
}

// TestResolveIncludes_IndirectCycle proves a transitive cycle
// (base → a → base) is rejected.
func TestResolveIncludes_IndirectCycle(t *testing.T) {
	dir := t.TempDir()
	writeInc(t, dir, "a.yaml", "includes:\n  - base.yaml\n")
	basePath := writeInc(t, dir, "base.yaml", "includes:\n  - a.yaml\n")

	_, err := resolveIncludes(readInc(t, basePath), dir, basePath, ReadOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

// TestResolveIncludes_DiamondAllowed proves a diamond (base → a → d and
// base → b → d) is NOT treated as a cycle: d is resolved on each branch.
func TestResolveIncludes_DiamondAllowed(t *testing.T) {
	dir := t.TempDir()
	writeInc(t, dir, "d.yaml", "keywords:\n  - from-d\n")
	writeInc(t, dir, "a.yaml", "includes:\n  - d.yaml\nkeywords:\n  - from-a\n")
	writeInc(t, dir, "b.yaml", "includes:\n  - d.yaml\nkeywords:\n  - from-b\n")
	basePath := writeInc(t, dir, "base.yaml", "includes:\n  - a.yaml\n  - b.yaml\n")

	resolved, err := resolveIncludes(readInc(t, basePath), dir, basePath, ReadOptions{})
	require.NoError(t, err, "diamond must not be flagged as a cycle")
	kw := kwSlice(t, resolved)
	assert.Contains(t, kw, "from-d", "shared diamond target resolved")
	assert.Contains(t, kw, "from-a")
	assert.Contains(t, kw, "from-b")
	// Dedup contract: the diamond target reached via two branches must
	// surface ONCE, not twice (deepMerge dedups the generic-slice merge).
	assert.Equal(t, 1, countValues(kw, "from-d"), "diamond target must not be duplicated")
}

// countValues counts how many times v appears in slice (deep-equal).
func countValues(slice []any, v any) int {
	n := 0
	for _, x := range slice {
		if reflect.DeepEqual(x, v) {
			n++
		}
	}
	return n
}

// TestResolveIncludes_RecursiveHTTP proves an HTTP include that itself
// includes another HTTP include is resolved transitively.
func TestResolveIncludes_RecursiveHTTP(t *testing.T) {
	isolateCache(t)
	mux := http.NewServeMux()
	var aBody, bBody string
	bBody = "keywords:\n  - from-b\n"
	mux.HandleFunc("/b.yaml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(bBody))
	})
	mux.HandleFunc("/a.yaml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(aBody))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	aBody = "includes:\n  - " + srv.URL + "/b.yaml\nkeywords:\n  - from-a\n"

	dir := t.TempDir()
	basePath := writeInc(t, dir, "base.yaml",
		"includes:\n  - "+srv.URL+"/a.yaml\n")

	resolved, err := resolveIncludes(readInc(t, basePath), dir, basePath, ReadOptions{})
	require.NoError(t, err)
	kw := kwSlice(t, resolved)
	assert.Contains(t, kw, "from-b", "transitive HTTP include (b via a) resolved")
	assert.Contains(t, kw, "from-a")
}

// TestResolveIncludes_HTTPCycle proves a cycle across HTTP includes is caught.
func TestResolveIncludes_HTTPCycle(t *testing.T) {
	isolateCache(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Derive this server's base URL from the request host so the handler
		// can reference its own address without capturing srv (which does not
		// exist yet at closure-creation time).
		base := "http://" + r.Host
		other := "/b.yaml"
		if r.URL.Path == "/b.yaml" {
			other = "/a.yaml"
		}
		_, _ = w.Write([]byte("includes:\n  - " + base + other + "\n"))
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	basePath := writeInc(t, dir, "base.yaml", "includes:\n  - "+srv.URL+"/a.yaml\n")
	_, err := resolveIncludes(readInc(t, basePath), dir, basePath, ReadOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

// TestAllHTTPIncludes_Transitive proves cache-warm discovery walks a local
// fragment to find an HTTP include declared inside it.
func TestAllHTTPIncludes_Transitive(t *testing.T) {
	isolateCache(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/remote.yaml", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("keywords:\n  - remote\n"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	writeInc(t, dir, "sub/local.yaml", "includes:\n  - "+srv.URL+"/remote.yaml\n")
	basePath := writeInc(t, dir, "base.yaml", "includes:\n  - sub/local.yaml\n")

	urls := AllHTTPIncludes(readInc(t, basePath), dir, basePath, ReadOptions{})
	assert.Equal(t, []string{srv.URL + "/remote.yaml"}, urls,
		"transitive HTTP include reached via a local fragment must be discovered")
}

// --- missing local include: warn vs fail (spec §4.9a) ----------------------

// TestResolveIncludes_MissingLocalSkippedByDefault proves the lenient default:
// a base referencing a local file that does not exist on disk resolves without
// error — the missing include is skipped (warned), partial data still merges.
// This is what keeps m6e-sync unblocked while an include is fixed upstream.
func TestResolveIncludes_MissingLocalSkippedByDefault(t *testing.T) {
	dir := t.TempDir()
	// present.yaml contributes data; missing.yaml does not exist on disk.
	writeInc(t, dir, "present.yaml", "keywords:\n  - from-present\n")
	basePath := writeInc(t, dir, "base.yaml",
		"includes:\n  - missing.yaml\n  - present.yaml\nidentity:\n  name: root\n")

	resolved, err := resolveIncludes(readInc(t, basePath), dir, basePath, ReadOptions{})
	require.NoError(t, err, "missing local include must not abort under the FailOnError default")
	kw := kwSlice(t, resolved)
	assert.Contains(t, kw, "from-present", "present include must still merge")
	ident := resolved[testIdentity].(map[string]any)
	assert.Equal(t, "root", ident[keyName], "base value wins")
}

// TestResolveIncludes_MissingLocalFailsOnWarning proves --fail-on=warning
// restores hard-fail strictness: a missing local include aborts resolution
// with an error that names the unresolved path.
func TestResolveIncludes_MissingLocalFailsOnWarning(t *testing.T) {
	dir := t.TempDir()
	basePath := writeInc(t, dir, "base.yaml",
		"includes:\n  - missing.yaml\nidentity:\n  name: root\n")

	_, err := resolveIncludes(readInc(t, basePath), dir, basePath, ReadOptions{FailOn: FailOnWarning})
	require.Error(t, err, "missing local include must abort under FailOnWarning")
	assert.Contains(t, err.Error(), "missing.yaml", "error must name the missing include")
}

// TestResolveIncludes_MissingLocalTransitiveSkipped proves the lenient skip
// applies on every level of the chain: an include that is itself present may
// declare a nested include that does not exist — the nested miss is skipped
// and the present fragment's own data still merges.
func TestResolveIncludes_MissingLocalTransitiveSkipped(t *testing.T) {
	dir := t.TempDir()
	writeInc(t, dir, "a.yaml", "includes:\n  - ghost.yaml\nkeywords:\n  - from-a\n")
	basePath := writeInc(t, dir, "base.yaml", "includes:\n  - a.yaml\n")

	resolved, err := resolveIncludes(readInc(t, basePath), dir, basePath, ReadOptions{})
	require.NoError(t, err)
	assert.Contains(t, kwSlice(t, resolved), "from-a", "present fragment still merges despite nested miss")
}

// TestRedundantIncludes_Transitive proves the astro→node case: a base that
// lists both a framework fragment and the language fragment the framework
// already pulls in flags the language as redundant-via-framework.
func TestRedundantIncludes_Transitive(t *testing.T) {
	dir := t.TempDir()
	writeInc(t, dir, "node.yaml", "technologies:\n  - node\n")
	writeInc(t, dir, "astro.yaml", "includes:\n  - node.yaml\ntechnologies:\n  - astro\n")
	basePath := writeInc(t, dir, "base.yaml",
		"includes:\n  - astro.yaml\n  - node.yaml\nidentity:\n  name: app\n")

	got := RedundantIncludes(readInc(t, basePath), dir, basePath, ReadOptions{})
	require.Len(t, got, 1)
	assert.Equal(t, "node.yaml", got[0].Ref)
	assert.Equal(t, "astro.yaml", got[0].Via)
	assert.Equal(t, redundantTransitive, got[0].Reason)
}

// TestRedundantIncludes_Duplicate flags the same target listed twice.
func TestRedundantIncludes_Duplicate(t *testing.T) {
	dir := t.TempDir()
	writeInc(t, dir, "node.yaml", "technologies:\n  - node\n")
	basePath := writeInc(t, dir, "base.yaml", "includes:\n  - node.yaml\n  - node.yaml\n")

	got := RedundantIncludes(readInc(t, basePath), dir, basePath, ReadOptions{})
	require.Len(t, got, 1)
	assert.Equal(t, "node.yaml", got[0].Ref)
	assert.Equal(t, redundantDuplicate, got[0].Reason)
}

// TestRedundantIncludes_Clean proves two independent includes (neither reaches
// the other) and a single-framework include are both reported as clean.
func TestRedundantIncludes_Clean(t *testing.T) {
	dir := t.TempDir()
	writeInc(t, dir, "node.yaml", "technologies:\n  - node\n")
	writeInc(t, dir, "python.yaml", "technologies:\n  - python\n")
	writeInc(t, dir, "astro.yaml", "includes:\n  - node.yaml\ntechnologies:\n  - astro\n")

	independent := writeInc(t, dir, "independent.yaml",
		"includes:\n  - node.yaml\n  - python.yaml\n")
	assert.Empty(t, RedundantIncludes(readInc(t, independent), dir, independent, ReadOptions{}),
		"two unrelated includes are not redundant")

	single := writeInc(t, dir, "single.yaml", "includes:\n  - astro.yaml\n")
	assert.Empty(t, RedundantIncludes(readInc(t, single), dir, single, ReadOptions{}),
		"a lone framework include has no sibling to be redundant against")
}
