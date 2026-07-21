<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

# projectfile/core — agent guide

## Purpose

The **projectfile document backend** — a pure Go **library**: read, query,
mutate, convert, and validate `projectfile.{toml,yaml,json}`. External consumers
import the `pkg/*` façades (below); core has **no `package main`**.

> **Core 2.0 cut (2026-07-20):** the bridge-owned typed shapes of projectfile
> extension namespaces (citation, readme, forge, funding, codeowners,
> contributing, support, security, release, conventions, cli-derive, ignores,
> vulnerabilities, editors) and their accessors moved to the sibling
> `projectfile/bridge` module's `internal/pfmodel` package. The contract for
> staying in core is now strict: every exported symbol is used by ≥2 of the
> three consumers (cli, bridge, ci-resolver), or is a defensible generic
> primitive (the document model, read/write/include/optimize machinery,
> `rawdoc`, `spdx`, `Toggle`+`ParseToggle`). Bridge-only repository/link/people
> helpers (`PrimaryRepository`, `LinkByType`, `ContactEmail`, `ForgePersonHandle`,
> …) moved with them.
>
> Earlier cuts already moved the CLI (`get`/`set`/… — now in
> [../cli/AGENTS.md](../cli/AGENTS.md)) and the projection/detection layer
> (`bridge`/`forge`/`scan`/`init` — now in
> [../bridge/AGENTS.md](../bridge/AGENTS.md)). This file documents the backend
> only; the bridge-authoring, render-algorithm, and CLI-command sections that
> used to live here describe code that has since moved and were trimmed.

No entry point — core is a library. Iterate on core with
`go build ./...` / `go test ./...`; build the binaries from `../cli` and
`../bridge`.

## Repository layout

```text
projectfile/core/
├── go.mod / go.sum       Go module (module path: kiota.ch/projectfile/core)
├── Makefile              m6e bootstrap + lint hook-ins (pure library: no image build)
├── projectfile.yaml      Dogfooding: this project's own projectfile
├── docs/                 Design docs: CONFORMANCE.md, CONSUMER-PLAN.md, MAKEFILE.md
├── reports/              Lint/scan reports (gitignored)
├── internal/             All implementation code (not importable externally)
├── pkg/                  Public façade for external consumers
└── .makefile/            m6e submodule (build framework)
```

No `main.go`, no `dist/`, no `Dockerfile` — core is a pure library. The
binaries that used to live here moved to `../cli` (`pf-cli`) and `../bridge`
(`pf-bridge`) in earlier cuts.

## Package map

```text
internal/                  (backend implementation — not importable externally)
├── projectfile/        Document model, Read, Write (format-dispatch), MergePeople,
│                       Clone, include resolution, Optimize (StripRedundant),
│                       ImageBasename synthetic, the generic extension-namespace
│                       primitives (LookupExtension/SetExtension). The typed shapes
│                       of individual org.projectfile.* namespaces and their
│                       accessors live in bridge/internal/pfmodel, not here.
├── rawdoc/             Lossless round-trip primitives (OrderedJSON, YAMLNode, OrderedTOML)
├── spdx/               SPDX boilerplate resolver — registered corpus → XDG cache → upstream
├── genlog/             Structured log surface (charmbracelet/log): Decision traces, warnings, verbose ops
├── pflock/             File-based locking (gofrs/flock) for concurrent runs on same projectfile
├── userconfig/         XDG config reader ($XDG_CONFIG_HOME/projectfile/cli.toml) — identity + generate defaults
├── selector/           Generic bubbletea picker — reused by cli usersetup + bridge picker/scaffold via pkg/selector
└── fieldpath/          Dotted-path + bracket grammar for get/set/add/del

pkg/                    Public façades (zero-cost re-exports of internal/*) — the
│                       three library consumers (cli, bridge, ci-resolver) import
│                       these, never internal/
├── projectfile/        read + write + document model + generic extension primitives
├── genlog/             structured logging surface (+ SetQuiet/SetVerbose/SetOutput)
├── rawdoc/             lossless round-trip primitives (Document.Rest)
├── userconfig/         XDG config load/path/private-host (+ Config round-trip for setup)
├── spdx/               license text + expression helpers (+ cache Status/WarmAll)
├── selector/           bubbletea picker/fill
├── pflock/             file lock
└── fieldpath/          dotted-path grammar + resolve/mutate
```

There is no longer an `internal/target/`, `internal/sync/`,
`internal/generators/`, or `internal/bridge/` in core — the projection layer,
the bridge registry, and every per-bridge handler live in `../bridge`. The
typed shapes of projectfile extension namespaces (citation, readme, forge,
funding, …) and their accessors live in `../bridge/internal/pfmodel`.

## SPDX boilerplate (`internal/spdx/`)

`spdx.Text(id, opts)` resolves an SPDX ID with lookup order:

1. Embedded set: `<id>.txt` in the `fs.FS` a consumer registered via `spdx.SetEmbedded`. Skipped when none is registered.
1. XDG cache: `${XDG_CACHE_HOME:-~/.cache}/projectfile-cli/spdx/<id>.txt`.
1. Upstream fetch from
    `raw.githubusercontent.com/spdx/license-list-data/main/text/<id>.txt` (10 s timeout, single retry with bounded jitter). Skipped when `opts.Offline`.

`spdx.Substitute` is best-effort for the common placeholder families:
`[year]`, `<year>`, `[fullname]`, `[name of copyright owner]`,
`[name of author]`, plus the angle-bracket MIT/BSD variants
(`<copyright holders>`, `<copyright holder>`, `<owner>`). Exotic
templates may need post-edits.

Error sentinels (use `errors.Is`): `ErrOffline`, `ErrCompound`, `ErrUnknown`.

Compound-expression helpers:

- `SplitCompound(id) (terms []string, conjunction string)` — splits on `OR` and `AND` (WITH stays opaque). Returns `("", "")` for a single ID. OR takes precedence over AND when both are present.
- `StripException(id) string` — removes trailing `WITH <exception>` from a single SPDX ID; no-op for plain IDs.

CFF license mapping (CFF 1.2.0):

- Single SPDX ID → scalar `license: MIT`
- OR compound (`MIT OR Apache-2.0`) → array `license: [MIT, Apache-2.0]` (CFF multi-license = OR semantics)
- AND compound / WITH → skipped (CFF cannot represent conjunctive licenses or exceptions)
- `LicenseToString(any) string` normalises both scalar and array shapes back to an SPDX expression string

**Core ships no licence texts.** The corpus is DATA the consumer registers with
`spdx.SetEmbedded(fsys)`; core owns only the algorithm. Tier 1 reads `<id>.txt`
at the root of the registered `fs.FS`, and a consumer that registers nothing
starts at the cache tier — `EmbeddedIDs()` then honestly reports zero rather
than failing.

The rule that forces this: a licence corpus is a **build artifact** of whoever
renders LICENSE files, and core is a consumed library whose consumers compile it
from the immutable module cache, where no build step could fetch one. Vendoring
it in core would mean tracking ~2000 lines of third-party text in a repository
that never reads them — so the corpus lives with its only consumer,
[`../bridge`](../bridge/AGENTS.md), which is a **binary** and therefore has a
build that can fetch it. (Historic trap: `cbf230e` untracked the texts while the
embed still lived here, which shipped an EMPTY `embedded/` in `core/v2@v2.0.0`
and broke every offline consumer. Untracking was right; the embed was in the
wrong repo.)

## Offline mode + include resolution

Core's network-facing resolvers (SPDX boilerplate, HTTP includes) honour an
`Offline` toggle carried on `ReadOptions`/`spdx.Options`. When set, all
network fetches are refused:

- **SPDX**: skips upstream fetch; uses embedded set + XDG cache only. Returns `ErrOffline` when the ID is in neither.
- **Includes**: HTTP(S) URLs in `includes` are resolved from XDG cache only. Uncached includes are skipped with a warning (partial data > hard stop).

The `--offline` / `--fail-on` CLI flags themselves live in the consumer
modules (cli, bridge); core exposes the plumbing (`ReadOptions.Offline`,
`ReadOptions.FailOn` — `IncludeFailLevel`: `FailOnError` | `FailOnWarning`)
they thread in.

### Missing local includes (`FailOn`)

A local include that does not exist on disk is a **soft failure**, not a hard
one: the file may be transiently absent (e.g. an m6e include being fixed in
parallel while m6e-sync consumes it). `ReadOptions.FailOn` sets the minimum
include-resolution severity that aborts a read:

- `FailOnError` (default) — a missing local include is **warned and skipped** (`WARN include skipped (not found)`); partial data still resolves. This is what keeps m6e-sync unblocked while an include is being fixed upstream.
- `FailOnWarning` — a missing local include **aborts** the read, restoring the pre-lenient strict behaviour.

Other include failures (parse errors, HTTP errors, cycles, permission denied)
are always hard errors regardless of the flag. `fetchLocalInclude` classifies
an `os.IsNotExist` read as the soft case. Offline-uncached HTTP includes
remain warn-and-skip always.

### Recursive includes

Includes resolve **recursively** (spec §4.9a): an included document MAY itself
declare `includes`, which are resolved before the including document is merged.
A relative path inside an included document resolves against THAT document’s
directory (a fragment pulled from a subdirectory can reference its siblings),
not the root. `resolveIncludes` walks the chain to a fixed point via the shared
`resolveIncludesChain` helper; `ResolveIncludesOnly` (optimize) and
`AllHTTPIncludes` (cache warm) reuse it, so transitive contributions reach
every consumer.

Cycle detection uses an **ancestor stack** (the set of document identity keys
on the current resolution path), not a visited set: a key is added on descent
and removed on return. A **cycle** (a document on its own ancestor path —
direct or indirect, local or HTTP) is rejected with a clear error; a
**diamond** (the same document reached through two independent branches) is
NOT a cycle and resolves on each branch. `deepMerge` **deduplicates** the
generic-slice merge by deep equality, so a diamond (or a base + an include
declaring the same list value) surfaces the value once, not twice. Identity
key for cycles: the URL for HTTP includes, the absolute resolved path for
local includes.

### 3-tier lookup pattern (SPDX + includes)

Both the SPDX resolver and the HTTP include fetcher follow the same pattern:

1. **Embedded set** (the consumer's registered corpus, zero I/O; SPDX only).
1. **XDG cache**: `${XDG_CACHE_HOME:-~/.cache}/projectfile-cli/<spdx|includes>/`.
1. **Network fetch** → write to cache → return. Skipped when `Offline` is set.

### HTTP fetch defenses (includes)

The include network tier refuses to silently serve wrong content so users
see the real HTTP cause instead of a downstream YAML/JSON parse error:

- **Status hints**: non-2xx surfaces as `HTTP <code> (hint) for <url>`. Hints: 401 → authentication required, 403 → forbidden (repository may be private), 404 → not found, 5xx → server error.
- **Cross-host redirect refusal**: a forge should never bounce raw content to another host; doing so is treated as an SSO/auth gateway.
- **HTML content-type refusal**: an include document is never HTML. A 200 OK with `text/html` is the signature of an auth/login wall reached after a same-host redirect (Forgejo/Gitea private-repo raw → `/user/login`).
- **Poisoned-cache self-heal**: cache entries that begin with `<!DOCTYPE` or `<html>` (written by an older build before the guards above) are discarded on read and replaced by a fresh fetch.

### Cache management

The cache is immutable (URL→content is deterministic). No TTL, no eviction.
Manual `rm -rf ${XDG_CACHE_HOME:-~/.cache}/projectfile-cli/` to clear. The
`pf-cli cache status` / `pf-cli cache warm` commands (cli module) drive
`spdx.Status`/`spdx.WarmAll` and `AllHTTPIncludes`/`WarmInclude` from this
package.

### `ReadOptions` propagation

`projectfile.ReadWithOptions(dir, ReadOptions{Offline: true})` threads the
offline flag through include resolution. `ReadBase`/`ReadBaseFromPath` are
unaffected — they never resolve includes.

## Lossless round-trip

The rawdoc primitives carry a parsed source in key-order-preserving form so
the round-trip syncer bridges (in `../bridge`) can paint typed fields back
onto the original canvas without losing unknown keys. Each bridge's
`Document` carries a `Rest *rawdoc.X` field; `Read` populates both the typed
view and `Rest`, `Write` repaints typed fields onto `Rest`. Unknown keys
survive a full round-trip including freshly-created files when the in-memory
doc has been carrying its `Rest` through `Clone`.

| Format | Primitive     | Preserves                                      |
| ------ | ------------- | ---------------------------------------------- |
| JSON   | `OrderedJSON` | keys + key order                               |
| YAML   | `YAMLNode`    | keys + order + comments                        |
| TOML   | `OrderedTOML` | keys (no in-table comments — go-toml/v2 limit) |

## People dedup + merge

Identity key precedence via `projectfile.PersonIdentityKey`:

1. `orcid:` + normalized ORCID (strip `https://orcid.org/`, lowercase)
1. `email:` + lowercased email
1. `name:` + canonical display name

Merge rules (`projectfile.MergePeople`):

- **Roles**: set-union (existing-first, then incoming-only).
- **Non-role fields**: existing wins; if existing is empty, incoming fills.
- **Hard conflict** (both sides non-empty, different): record a
    `PersonConflict`, keep existing — caller emits one warning line to stderr.

### Include resolution path (`MergePeopleRaw` / `MergeOrganizationsRaw`)

`deepMerge` (used by `resolveIncludes`) exempts the reserved entity-list
keys `people` and `organizations` from the generic slice merge. Both are
merged by identity via the raw-map twins of `MergePeople` /
`MergeOrganizations`. All other slices are concatenated (loser first, then
winner) and **deduplicated by deep equality** — so a value a base and an
include both declare (or a document reached through two include branches of
a diamond) appears once. What the entity exemption enables: a base document
can carry project-scoped fields (`[[people]].from`, `[[people]].to`) on a
person whose identity, contact, and role fields arrive via an include — the
two records fold into one instead of duplicating and tripping the schema’s
required-field checks.

- Identity tiers are the same as `samePerson`/`sameOrg` (orcid decisive when both present → email match when both present → canonical name).
- Direction is `deepMerge`'s: **winner wins, loser fills**. Winner is the base document; losers are includes (in chain order, then base overlays).
- Roles are unioned **loser-first** (consistent with `deepMerge`'s general slice contract), not existing-first like the bridge sync `MergePeople`.
- Conflicts are **silent** — include resolution is a transparent composition step with no caller to receive a `[]PersonConflict`.
- Raw maps are used (no round-trip through typed `[]Person`) so unknown keys survive — include resolution runs before validate and must not mask the real schema error by silently dropping data.

To extend identity-aware merging to a new reserved list, add one case to
`mergeEntitySliceKey` in `include.go` and a `MergeXxxRaw` helper next to
`MergePeopleRaw` in `people.go`.

## Conventions

- `core.Trunc(s)` caps display strings at 60 chars for `FieldChange` output. (Lives in `../bridge/internal/bridge/core`.)
- PURL format per ecosystem (the per-bridge mappers own these):
    - npm: `pkg:npm/{name}@{version}`
    - pypi: `pkg:pypi/{normalized-name}@{version}`
    - composer: `pkg:composer/{vendor}/{package}@{constraint}` — operators (`^`, `~`, `>=`, `||`, ...) preserved verbatim.
- **Round-trip writes**: never call `os.WriteFile` on `package.json` /
    `CITATION.cff` / `pyproject.toml` / `composer.json` / `projectfile.*` /
    `CODEOWNERS` directly — go through the format package’s `Write` /
    `projectfile.Write` so round-trip semantics and schema-header injection
    are preserved. (The format-specific `Write` functions live in
    `../bridge`; `projectfile.Write` is core's.)
- **Base-write invariant**: every command that mutates and writes the projectfile MUST write the BASE document (no includes resolved), never the merged one. `set`/`add`/`del`/`scan` use `ReadBaseFromPath`; `convert` uses `ReadRawBaseFromPath`; `bridge` sync / derive / fill-required-fields read merged for mapper context but call `projectfile.ReconcileBase(basePF, preSync, postSync)` before writing so only the fields the operation actually changed land on disk.
- Reverse-DNS extension namespaces park ecosystem-specific fields under
    `pf.Extensions[<ns>]` so they round-trip without polluting native pf
    slots. The typed shapes of each namespace and their accessors live in
    `../bridge/internal/pfmodel`; core only owns the generic lookup primitives.
- Extension lookup goes through `projectfile.LookupExtension(doc, ns)` — never read `doc.Extensions[ns]` directly. The helper handles both encodings (flat key for YAML/JSON and quoted TOML, dotted TOML header that go-toml/v2 explodes into nested maps).
- Extension writes go through `projectfile.SetExtension(doc, ns, value)` — it prunes any pre-existing nested-map form before writing the flat key.

## Structured logging (`internal/genlog/`)

Every status line, decision trace, and warning the CLI emits goes through
`genlog`. Wraps `charmbracelet/log`:

- `Decision(field, value, source, override)` — per-field decision trace emitted by each bridge. Column-aligned for scanability.
- `Quiet` — suppresses Decision/Section/Plain output. Warnings and errors are NEVER suppressed.
- `Verbose` — gates operational log lines (file detection, include resolution, lock acquisition). Off by default; enabled by `--verbose` or `PF_CLI_VERBOSE=1`.

## File locking (`internal/pflock/`)

Concurrent `pf-cli` runs on the same projectfile are serialised with
`gofrs/flock`. `WithLock(pfPath, fn)` acquires `<pfPath>.lock`, waits up
to 5 seconds with 100ms retry interval, then runs `fn`. Used by the
sync dispatcher and the write path.

## User config (`internal/userconfig/`)

Reads `$XDG_CONFIG_HOME/projectfile/cli.toml` for user-level defaults:

- `[identity]` — name, email (fallback for bridge RequiredFields prompts).
- `[generate.defaults]` — per-bridge path overrides and scaffold defaults.

Loaded lazily, cached for process lifetime. The `usersetup/` package
provides the interactive first-run wizard that writes this file.

## Public façade (`pkg/*`)

The `pkg/*` packages are zero-cost reexports of the `internal/*` implementations.
They exist so the three library consumers — `projectfile/cli`,
`projectfile/bridge`, `projectfile/ci-resolver` — reach core **only** through
its public API, never `internal/`. Each façade is a thin `surface.go` of type
aliases (`= internal.X`, carrying full method sets), value-aliased functions
(`var F = internal.F`, one implementation), and reexported constants. Curated
to real consumers — do NOT widen without one. **Mutable package vars cross as
setters** (`SetQuiet`/`SetVerbose`/`SetYAMLOutputSorted`/`SetIgnored`), never
value aliases — a `var X = internal.X` copies, so a consumer’s write would
not reach core.

| façade            | promotes                          | consumer uses it for                                                                                                                                                                                                |
| ----------------- | --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `pkg/projectfile` | the document model                | read+write+model + `ReadOptions`/`IncludeFailLevel`/`FailOn*`/`SplitGitName` + generic extension primitives (`LookupExtension`/`SetExtension`/`HasExtension`/`ParseToggle`/`RoleMaintainer`). `projectfile.go` keeps the external read surface (`Read`/`DetectPath`/`Stack`/`Extension`) for `ci-resolver`. |
| `pkg/genlog`      | structured logging                | `Decision`/`Info`/`Warn`/`Error`/`Section`/`Plain` + `SetQuiet`/`SetVerbose`/`SetOutput` (the cli + pf-bridge roots drive the toggles; a mutable var must cross as a setter, not a value alias)                     |
| `pkg/rawdoc`      | lossless round-trip primitives    | `OrderedJSON`/`YAMLNode`/`OrderedTOML` + constructors (bridge `Document.Rest`)                                                                                                                                      |
| `pkg/userconfig`  | XDG config                        | `Load`/`PathFor`/`IsPrivateHost` + `SetIgnored` + `Config`/`ExistingPath`/`Write` (cli setup wizard)                                                                                                                |
| `pkg/spdx`        | license text + expression helpers | `Text`/`Substitute`/`Split`/`StripException` (license + cff bridges) + `Status`/`WarmAll` (cli cache) + `SetEmbedded` (bridge registers the corpus)                                                                 |
| `pkg/selector`    | bubbletea picker/fill             | `Run`/`Choices`/`Fill`/`FillField`/`MultiInput` (cli usersetup + bridge picker/scaffold)                                                                                                                            |
| `pkg/pflock`      | file lock                         | `WithLock`/`WithLockTimeout` (cli + bridge/forge write paths)                                                                                                                                                       |
| `pkg/fieldpath`   | dotted-path grammar               | `Parse`/`Path`/`Segment` (derive selectors) + `Resolve`/`Set`/`Add`/`Delete`/`Result`/`Pair`/`LookupDefault` (cli get/set/add/del)                                                                                  |

`pkg/projectfile` also grew a Phase 8 block (`WriteClean`, `ReadRaw*`,
`ReadFromPath*`, `FromMap`, `ResolveIncludesOnly`/`StripRedundant`/`SortIncludes`
for optimize, `AllHTTPIncludes`/`WarmInclude`/`XDGCacheDir` for cache,
`YAMLOutputSortedEnabled` getter) — the surface the `projectfile` CLI reads.

The bridge-owned typed shapes of `org.projectfile.*` extension namespaces
(citation, readme, forge, funding, codeowners, contributing, support,
security, release, conventions, cli-derive, ignores, vulnerabilities,
editors) and their accessors are NOT in this façade — they live in
`../bridge/internal/pfmodel`. Core-internal packages import each other
directly (no façade needed among siblings); a `pkg` façade and its `internal`
twin never cycle: façades only alias downward.

## i18n note

Per workspace AGENTS.md, user-facing strings ship in `es_CL` and `uk_UA`.
Scope split across the modules:

- **CLI messages** (status lines, errors, hints) live in the consumer modules
    (`cli`, `bridge`) and go through their `.container/.../locale/*.po` flow.
- **Generated file bodies** (CONTRIBUTING.md, SECURITY.md, CoC, FUNDING.yml,
    LICENSE) are English-only in v1, deliberately — these are external-reader
    artefacts (GitHub viewers, contributors) and are conventionally English in
    OSS. The renderer templates themselves live in `../bridge`. A follow-up
    plan will add per-locale template files when concretely requested.

## Build

Core is a pure library — no `package main`, no binary, no Docker image. The
`make` targets that matter here are the linters; the binary build/publish/install
targets live in `../cli` and `../bridge`, and the SPDX corpus fetch moved to
`../bridge` with the corpus itself.

```sh
make help            # categorised list of every target (always up to date)
make lint            # golangci-lint + gosec + shellcheck + markdownlint + textlint
make format          # gofmt + golangci-fmt + markdownlint-fix + textlint-fix
```

For quick correctness checks during iteration, prefer per-package
`go vet ./internal/<pkg>` over the full `make lint` (the latter spawns many
parallel compiles and saturates CPU). `go build ./...` + `go test ./...` from
this directory are the canonical smoke checks.
