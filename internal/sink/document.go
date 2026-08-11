// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package sink

import (
	"fmt"
	"sort"
	"strconv"

	"kiota.ch/projectfile/core/v2/internal/fieldpath"
	"kiota.ch/projectfile/core/v2/internal/genlog"
	"kiota.ch/projectfile/core/v2/internal/projectfile"
)

// The two namespaces this file reads. They are PEERS, never derived from one
// another: `sinks` says where an artifact MAY go, `publish` says which pipeline
// actually sends it there. One fleet-wide include can therefore declare every
// destination once, while each forge decides which of them it feeds — a kiota
// pipeline pushing to kiota and ECR, a GitHub mirror pushing to GHCR.
const (
	ExtensionNS        = "org.projectfile.sinks"
	PublishExtensionNS = "org.projectfile.publish"
)

// Roles. The vocabulary is OPEN — an unlisted role is carried through untouched —
// but these two change how a README introduces the destination, so they are named
// rather than spelled at each use.
//
// Role is also what makes an entry ADDRESSABLE across the map. A bare `{}`
// projection admits no trailing field (a map fans out to PAIRS, so `.ref` has
// nowhere to land), leaving the selector form `{k=v}` as the only way to reach a
// key across every entry — and a set every entry belongs to needs a key every
// entry carries. RolePrimary is that key's default, filled in by the reader
// rather than typed by the author.
const (
	RolePrimary  = "primary"
	RoleFallback = "fallback"
)

// Keys of a publish route.
const (
	keyPush = "push"
	keyPull = "pull"
)

// Sink is one named destination as the document declares it.
//
// The name is a LABEL and nothing reads meaning into it, so two sinks may
// address one host under two accounts. What decides the shape of the reference
// is the entry's `ref` template — see Compose.
type Sink struct {
	// Name is the map key the sink was declared under. Carried on the value so
	// an entry stays self-describing once the map is flattened into a list, and
	// so credentials can key on it: two accounts on one host cannot share a
	// host-keyed secret.
	Name string

	// Entry is the declared keys, VERBATIM. Every one of them is addressable
	// from a template as `${sink.<key>}`, which is what keeps the vocabulary
	// open — an unusual destination costs a template, never a schema change.
	Entry map[string]any
}

// Template is the entry's own `ref`, else the default for its host.
func (s Sink) Template() string {
	if t, _ := s.Entry[KeyRef].(string); t != "" {
		return t
	}
	return defaultTemplate(s.Entry)
}

// Role is the entry's declared role, else primary.
func (s Sink) Role() string {
	if r, _ := s.Entry[KeyRole].(string); r != "" {
		return r
	}
	return RolePrimary
}

// Priority is the entry's rank, higher first. Read through fieldpath so a
// Go-side list of sinks and the resolver's `{}` fan-out over the same map cannot
// disagree about where an unranked entry sits.
func (s Sink) Priority() int { return fieldpath.EntryPriority(s.Entry) }

// Compose renders this sink's reference for the given image coordinates.
func (s Sink) Compose(coords Coords) (string, bool) { return Compose(s.bindings(), coords) }

// ComposeFanOut is Compose for a template whose references name several values.
func (s Sink) ComposeFanOut(coords Coords) ([]string, bool) {
	return ComposeFanOut(s.bindings(), coords)
}

// bindings is the entry the template expands against: the declared keys plus the
// two values the MODEL supplies rather than requires.
//
//   - `ref` gains the default template when the entry declares none, so there is
//     exactly one composition path and a bare `host:` entry is complete.
//   - `owner` falls back to the project's own image namespace, so a template
//     written for an account-forcing registry keeps working on one that forces
//     none instead of collapsing to a double slash. interp re-expands a resolved
//     value, so the substituted `${image.namespace}` resolves in the same pass.
//
// The declared entry is copied rather than edited: it belongs to the document,
// and a reader that quietly wrote defaults into it would persist a value nobody
// typed on the next base write.
func (s Sink) bindings() map[string]any {
	out := make(map[string]any, len(s.Entry)+2)
	for k, v := range s.Entry {
		out[k] = v
	}
	out[KeyRef] = s.Template()
	if owner, _ := out[KeyOwner].(string); owner == "" {
		out[KeyOwner] = addr(fieldpath.AddrImageNamespace)
	}
	return out
}

// Declared reads `org.projectfile.sinks`, ranked by priority descending with the
// name as tiebreak — the same order the resolver's `{}` fan-out uses, so an
// inventory list and the rendered pull lines agree.
//
// Returns nil when the namespace is absent, which is a valid state and not an
// error: a project that declares no destination publishes nowhere, and the
// caller decides what to do about that.
//
// An entry that composes to nothing — neither a `ref` template nor a `host` to
// build the default from — is dropped with a warning. It addresses no authority,
// and composing from it would emit a reference beginning with a slash.
func Declared(doc *projectfile.Document) ([]Sink, error) {
	m, err := namespace(doc, ExtensionNS)
	if err != nil || m == nil {
		return nil, err
	}
	out := make([]Sink, 0, len(m))
	for _, name := range sortedKeys(m) {
		entry, ok := m[name].(map[string]any)
		if !ok {
			genlog.Warn("sink: entry is not a mapping — skipped", "sink", name)
			continue
		}
		s := Sink{Name: name, Entry: entry}
		if s.Template() == "" {
			genlog.Warn("sink: entry declares neither ref nor host — skipped", "sink", name)
			continue
		}
		genlog.Decision("sink", s.Template(), name, "role="+s.Role()+" priority="+strconv.Itoa(s.Priority()))
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Priority() > out[j].Priority() })
	return out, nil
}

// Route is one pipeline's publish dogma: which sinks a forge pushes to, and
// which one it pulls its own base images from.
type Route struct {
	// Forge is the map key — the pipeline this route belongs to, by forge slug.
	Forge string

	// Push is the sinks this pipeline sends an artifact to, in declared order.
	Push []string

	// Pull is the sink this pipeline reads base images FROM. It is declared,
	// never derived from priority: a kiota pipeline builds from kiota because
	// kiota is local to it, even when a higher-priority sink exists.
	Pull string
}

// Routes reads `org.projectfile.publish`, sorted by forge slug. Returns nil when
// the namespace is absent — a project that declares no route falls back to
// whatever the build plane's own defaults are, which is the state of the fleet
// before this namespace existed.
func Routes(doc *projectfile.Document) ([]Route, error) {
	m, err := namespace(doc, PublishExtensionNS)
	if err != nil || m == nil {
		return nil, err
	}
	out := make([]Route, 0, len(m))
	for _, forge := range sortedKeys(m) {
		entry, ok := m[forge].(map[string]any)
		if !ok {
			genlog.Warn("sink: publish route is not a mapping — skipped", "forge", forge)
			continue
		}
		pull, _ := entry[keyPull].(string)
		route := Route{Forge: forge, Push: projectfile.AsStringList(entry[keyPush]), Pull: pull}
		genlog.Decision("publish_route", pull, forge, "push="+strconv.Itoa(len(route.Push)))
		out = append(out, route)
	}
	return out, nil
}

// ByName finds a sink by its label.
func ByName(sinks []Sink, name string) (Sink, bool) {
	for _, s := range sinks {
		if s.Name == name {
			return s, true
		}
	}
	return Sink{}, false
}

// Select resolves sink NAMES — a route's `push` list — against the declared
// sinks, keeping the order the route named them in.
//
// A name no sink answers to is dropped LOUDLY and ok is false. A route naming a
// destination that does not exist is a push to nowhere, and a caller that
// publishes must be able to refuse rather than quietly send to one fewer place
// than the document promised.
func Select(sinks []Sink, names []string) (selected []Sink, ok bool) {
	ok = true
	for _, name := range names {
		s, found := ByName(sinks, name)
		if !found {
			genlog.Warn("sink: publish route names an undeclared sink", "sink", name,
				"remedy", "declare it under "+ExtensionNS+" or remove it from the route")
			ok = false
			continue
		}
		selected = append(selected, s)
	}
	return selected, ok
}

// defaultTemplate is what an entry declaring no `ref` composes through: the
// host, an owner segment ONLY where the entry declares an owner, then the
// project's own image path and tag.
//
// That asymmetry is the whole rule, and it reproduces the single-registry
// behaviour the fleet had before this namespace existed. `image.basename`
// already carries the project's namespace, so defaulting the owner SEGMENT to
// `${image.namespace}` would publish `kiota.ch/b19/b19/ubuntu`. The `${sink.owner}`
// FALLBACK still applies (see bindings) — but only for a template that names the
// variable itself.
//
// The default is a TEMPLATE rather than a concatenation because the scratch
// document carries the entry under `sink`: `${sink.host}` is an ordinary address,
// so a declared ref and a defaulted one travel the same single code path.
//
// Returns "" for an entry with no host — there is no authority to address.
func defaultTemplate(entry map[string]any) string {
	host, _ := entry[KeyHost].(string)
	if host == "" {
		return ""
	}
	tail := addr(fieldpath.AddrImageBasename) + ":" + addr(fieldpath.AddrImageTag)
	if owner, _ := entry[KeyOwner].(string); owner != "" {
		return addr(scopeSink+"."+KeyHost) + "/" + addr(scopeSink+"."+KeyOwner) + "/" + tail
	}
	return addr(scopeSink+"."+KeyHost) + "/" + tail
}

// addr wraps a field address as a `${…}` reference, so the addresses a default
// template is built from are spelled from constants rather than as literals.
func addr(path string) string { return "${" + path + "}" }

// namespace resolves one extension namespace to its entry map. A present but
// non-map namespace is an ERROR rather than an empty result: it is a typo in a
// document that meant to declare destinations, and reporting nothing would look
// identical to declaring nothing.
func namespace(doc *projectfile.Document, ns string) (map[string]any, error) {
	raw, ok := projectfile.LookupExtension(doc, ns)
	if !ok {
		return nil, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a map", ns)
	}
	return m, nil
}

// sortedKeys is the deterministic starting order every reader here uses. The
// sinks are re-ranked by priority afterwards with a STABLE sort, so entries of
// equal rank keep this name order instead of swapping between runs.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
