<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

# projectfile/core — agent guide

## Purpose

The **projectfile document backend** — a pure Go **library**: read, query,
mutate, convert, and validate `projectfile.{toml,yaml,json}`. External consumers
import the `pkg/*` façades (below); core has **no `package main`**.

> **Bridge Revolution Phase 8 cut (2026-07-07):** the `projectfile` CLI itself —
> `main.go` + `internal/{cmd,validate,usersetup}` — **moved to the sibling
> `projectfile/cli` module** (the `pf-cli` binary), a third library consumer
> alongside `pf-bridge` and `pf-ci`. Core is now backend-only. The commands
> (`get`/`set`/`add`/`del`/`convert`/`validate`/`optimize`/`cache`/`setup`) live
> in [../cli/AGENTS.md](../cli/AGENTS.md); they drive core through the façades.

> **Bridge Revolution Phase 2 cut (2026-07-07):** the projection/detection layer
> — `bridge`/`forge`/`scan`/`init` commands and `internal/{bridge,forge,
> scanners,source,scaffold,derive}` — **moved to the sibling `projectfile/
> bridge` module** (`pf-bridge` binary), which consumes this backend through the
> `pkg/*` façades. See [../bridge/AGENTS.md](../bridge/AGENTS.md) and
> [../bridge-revolution.md](../bridge-revolution.md). The deep bridge-authoring
> sections BELOW this line document `pf-bridge` and are pending migration into
> `bridge/AGENTS.md`; treat them as describing the moved code, not `pf-cli`.

The moved bridge model (round-trip Syncer / derive-only Renderer), for reference: 

- **Syncers** (round-trip): `package.json`, `CITATION.cff`, `pyproject.toml`,
    `composer.json`, `CODEOWNERS`.
- **Renderers** (derive-from-pf): `.gitignore`, `.dockerignore`, `.containerignore`,
    `.npmignore`, `.claudeignore`, `.trivyignore`, `.grype.yaml`, `osv-scanner.toml`,
    `LICENSE`, `FUNDING.yml`, `funding.json`,
    `CODE_OF_CONDUCT.md`, `SECURITY.md`, `CONTRIBUTING.md`, `SUPPORT.md`,
    `README.md`, `.browserslistrc`, `.releaserc.yaml`.

Both groups are looked up by **on-disk filename**: `pf-cli bridge <filename>`.
Direction is a positional preposition, not a `--mode` flag:

```text
pf-cli bridge package.json          # default — syncer: mtime arbitration
pf-cli bridge to package.json       # write pf → external (was --mode from-pf)
pf-cli bridge from composer.json    # read  external → pf (was --mode to-pf)
```

Renderers auto-resolve to `to`; `bridge from <renderer>` errors out with
`bridge %q is write-only (Renderer cannot read external)`.

No entry point — core is a library. The `pf-cli` binary that used to live
here (`main.go` → `internal/cmd`) moved to the sibling `projectfile/cli` module
in the Phase 8 cut. Iterate on core with `go build ./...` / `go test ./...`;
build the binary from `../cli`.

## Repository layout

```text
projectfile/core/
├── main.go               Entry point (Cobra root)
├── go.mod / go.sum       Go module (module path: kiota.ch/projectfile/core)
├── Makefile              Build, lint, fetch-spdx, sync-schema-embed, etc.
├── Dockerfile            m6e container build (make build, NOT build-local)
├── projectfile.yaml      Dogfooding: this project's own projectfile
├── dist/                 Build output (pf-cli binary lands here)
├── docs/                 Design docs: CONFORMANCE.md, CONSUMER-PLAN.md, MAKEFILE.md
├── reports/              Lint/scan reports (gitignored)
├── internal/             All implementation code (not importable externally)
├── pkg/                  Public facade for external consumers
│   └── projectfile/      Thin re-export of internal/projectfile types + Read
└── .makefile/            m6e submodule (build framework)
```

## Package map

```text
internal/                  (backend implementation — not importable externally)
├── projectfile/        Document, Read, Write (format-dispatch), MergePeople, Clone, *Extension helpers, Optimize (StripRedundant), ImageBasename synthetic
├── rawdoc/             Lossless round-trip primitives (OrderedJSON, YAMLNode, OrderedTOML)
├── spdx/               SPDX boilerplate resolver — embedded set → XDG cache → upstream
├── genlog/             Structured log surface (charmbracelet/log): Decision traces, warnings, verbose ops
├── pflock/             File-based locking (gofrs/flock) for concurrent runs on same projectfile
├── userconfig/         XDG config reader ($XDG_CONFIG_HOME/projectfile/cli.toml) — identity + generate defaults
├── selector/           Generic bubbletea picker — reused by the moved bridge picker + scaffold via pkg/selector
└── fieldpath/          Dotted-path + bracket grammar for get/set/add/del

pkg/                    Public façades (zero-cost re-exports of internal/*) — the
│                       three library consumers (cli, bridge, ci-resolver) import
│                       these, never internal/
├── projectfile/        read + write + model (projectfile.go = external read surface; surface.go = promoted)
├── genlog/             structured logging surface (+ SetQuiet/SetVerbose/SetOutput)
├── rawdoc/             lossless round-trip primitives (Document.Rest)
├── userconfig/         XDG config load/path/private-host (+ Config round-trip for setup)
├── spdx/               license text + expression helpers (+ cache Status/WarmAll)
├── selector/           bubbletea picker/fill
├── pflock/             file lock
└── fieldpath/          dotted-path grammar + resolve/mutate
```

There is no longer an `internal/target/`, `internal/sync/`, or
`internal/generators/`: `internal/bridge/registry.go` is the single source
of truth for both round-trip and derive-only handlers.

## Filename-keyed bridge registry

`internal/bridge/registry.go` exports four things:

- `Register(core.Bridge)` — bridges self-register in `init()`; panics on empty Filename, duplicate Filename, or alias collision.
- `Lookup(name) (core.Bridge, bool)` — matches Filename or any registered alias.
- `List() []core.Bridge` — every registered bridge, sorted by Filename.
- `Filter(pred) []core.Bridge` — capability-typed filtering (`bridge.IsSyncer`, `bridge.IsRenderer` are the convenience predicates).

`cmd/bridge.go` blank-imports every bridge subpackage so registrations
land before Cobra dispatches.

## Interfaces

```go
// Bridge is the identity surface — every bridge implements this.
type Bridge interface {
    Name() string                                         // stable id ("cff", "license", ...)
    Filename() string                                     // canonical on-disk name (the lookup key)
    Aliases() []string                                    // alternative spellings (may be nil)
    Labels() (extName, pfName string)                     // labels for FieldChange display
    FullPath(dir string, pf *projectfile.Document) string // absolute write path
    Exists(dir string) bool
    Policy() Policy
}

// Syncer extends Bridge with the round-trip surface.
type Syncer interface {
    Bridge
    NewEmpty() any
    Clone(doc any) any
    Read(dir string) (any, error)
    Write(dir string, doc any) error
    BuildMappers(extDoc any, pf *projectfile.Document) MapperList
}

// Renderer extends Bridge with the derive-only surface.
type Renderer interface {
    Bridge
    Render(pf *projectfile.Document, opts Options) (Output, error)
}

// Output is the multi-file emission shape (keys are paths relative to opts.Dir).
// Single-file renderers return a one-entry map.
type Output struct {
    Files map[string][]byte
}

// RequiredFieldsBridge is an optional capability for Syncer OR Renderer —
// declares projectfile fields that must be set before the bridge can run.
// The dispatcher walks Missing in TTY fill-mode.
type RequiredFieldsBridge interface {
    Bridge
    RequiredFields(pf *projectfile.Document) []Missing
}
```

`Policy{Marker, ScaffoldOnce}` is the overwrite gate: at most one is set
(both unset means always overwrite, used for pure-data files with no
user-edit expectation).

## Mode vocabulary

```go
type Mode string

const (
    ModeSync  Mode = "sync"  // newer side authoritative, other side gap-fills
    ModeWrite Mode = "write" // pf → external (was ModeFromPF / --mode from-pf)
    ModeRead  Mode = "read"  // external → pf (was ModeToPF   / --mode to-pf)
)
```

The CLI surface is the positional preposition, not the constant — `to` →
`ModeWrite`, `from` → `ModeRead`, nothing → `ModeSync`. Mapper closures
keep `ToPF` / `FromPF` names because they describe which side of the
mapper writes, not a CLI mode.

## Sync algorithm (`bridge/core/runsync.go`)

`core.RunSync(syn, pf, opts)`:

1. **External missing** → push every mapper’s `FromPF(force=true)` onto a fresh ext doc; mark `Result.Created`; write iff `!opts.DryRun`.
1. **Both present** → resolve direction:
    - Explicit `bridge to` / `bridge from` bypass mtime arbitration.
    - Default `bridge`: newer file is authoritative.
1. **Authoritative-side push + reverse gap-fill**:
    - PF newer → `FromPF(force=true)` then `ToPF(force=false)`.
    - Ext newer → `ToPF(force=true)` then `FromPF(force=false)`.
1. **Dry-run safety**: when `opts.DryRun` is set, work documents are
    `Clone()`d before mappers run — the caller’s pf and extDoc are never mutated.
1. **Person merge conflicts** detected by `projectfile.MergePeople` are stashed on the work-PF via `core.RecordPersonConflicts` and surfaced as one warning line per conflict to `opts.Stderr`.
1. **Persist** via `syn.Write` / `projectfile.Write` only when `!opts.DryRun`.
    **Base reconciliation**: the caller passes the MERGED document (includes resolved) so mappers see every effective value, but the on-disk write targets the BASE document. Before mappers run, `RunSync` snapshots the merged doc (`preSync`). After mappers + derive, it reads the base doc and calls `projectfile.ReconcileBase(basePF, preSync, workPF)` which applies ONLY the fields the sync actually changed. Include-inherited values are never materialised into the base file.

### FieldMapper

```go
type FieldMapper struct {
    ExtKey string                    // label when destination is the external file
    PFKey  string                    // label when destination is projectfile
    ToPF   func(force bool) string   // mutates pf from extDoc
    FromPF func(force bool) string   // mutates extDoc from pf
}
```

`force=true` overwrites the target (authoritative direction or explicit
`bridge to` / `bridge from`); `force=false` is gap-fill (only mutates when
the target is empty — the reverse direction in default sync mode). People
always merge (no push/fill distinction).

## Render algorithm (`bridge/core/runrender.go`)

`core.RunRender(r, pf, opts)`:

1. `r.Render(pf, opts)` returns an `Output{Files: map[relpath][]byte}`.
1. For each file (sorted), apply the policy gate:
    - `ScaffoldOnce && !Force` → refuse with `refused (scaffold-once)`.
    - `Marker && !HasMarker(existing) && !Force` → refuse with `refused (hand-edited)`.
1. `--dry-run` prints the status line and skips the write.
1. Otherwise `mkdir -p` + `os.WriteFile`; one status line per file; at least one refusal turns into a batch-level error.

## Renderers: two shapes behind one interface

### Snippet-stitch (`bridge/ignore/`)

Per-stack snippet bundles assembled into a single ignore-file body. One
Bridge per ignore-file filename (`.gitignore`, `.dockerignore`,
`.containerignore`, `.npmignore`, `.claudeignore`); the single `bridge.go`
holds the assemble pipeline + Renderer implementation; `register.go` carries
the target table and the `init()` registration loop. Each ignore filename is
a regular `bridge <filename>` target — there is no sibling command.

### Vulnerability suppress-list (`bridge/vulnerabilities/`)

Tool-agnostic fan-out of suppressed vulnerability IDs. One source —
`org.projectfile.vulnerabilities.suppress` — renders to three scanner ignore
files, so trivy/grype/osv never disagree on whether a known-unfixable vuln
gates publish: `.trivyignore` (one ID per line; reason dropped),
`.grype.yaml` (`ignore[].vulnerability`; reason as trailing comment), and
`osv-scanner.toml` (`[[IgnoredVulns]]` id/reason). The single `bridge.go`
holds the per-scanner render arms; `register.go` carries the target table.
`generate` opts a project out of scanners it does not run (default: all).
grype and osv auto-discover their files from the cwd; trivy is fed via
`--ignorefile` by the `auto-trivy` wrapper. This is intentionally separate
from `bridge/ignore/` (gitignore-style file patterns) — the data model and
per-scanner formats are unrelated to stack snippet-stitching.

### Template-substitution (`bridge/{license,funding,security,contributing,coc,readme}/`)

Each bridge owns a `text/template` at `templates/<filename>.tmpl` plus a
sibling `bridge.go` with the Bridge impl, the view struct (`view.go`),
optional `RequiredFields`, and the `init()` registration. File-internal
order:

```text
1. document.go    format types + Rest canvas      (round-trip only)
2. read.go        parse from disk                  (round-trip only)
3. write.go       paint typed view onto Rest       (round-trip only)
4. view.go        template view struct             (renderer only)
5. bridge.go      Bridge methods + Syncer/Renderer impl
6. mapper.go      BuildMappers + per-field closures (round-trip only)
7. helpers.go     anything that doesn't fit elsewhere
8. register.go    init() → bridge.Register(...) and embed.FS wiring
```

Templates are loaded by the shared `core.Render(dir, name, data)` helper.
Before consulting the embedded template, `Render` checks whether the
project has a local override at `.projectfile/templates/<name>`. When
that file exists, it takes precedence over the integrated template.
Parsing is lazy (first use) and cached per (dir, name) pair for the
process lifetime. This applies to all current and future template-using
bridges — no per-bridge wiring is needed.

### LICENSE and multi-file output

`LICENSE` returns multi-file `Output` when the SPDX expression is
compound (`"MIT OR Apache-2.0"`, `"MIT AND Apache-2.0"`): a top-level
`LICENSE` rendered from `LICENSE.tmpl` plus per-term `LICENSE-<id>`
files. Single-term expressions skip the template entirely and emit the
SPDX text directly, keyed as `LICENSE`. The two-call (`Body` + `Files`)
split from the pre-bridge model is gone — `Render` returns one map.

`SplitCompound` handles both `OR` (disjunctive — "any one of") and `AND`
(conjunctive — "you must comply with ALL") at the top level; the
overview wording adapts via the `Conjunction` template field. `WITH`
exceptions (e.g. `GPL-2.0-only WITH Classpath-exception-2.0`) stay
opaque during splitting and are stripped by `StripException` before each
per-term `Text` call.

## Project-local template overrides

All template-using bridges (current and future) support project-local
template overrides without any per-bridge wiring. When a file named
`<name>` exists under `.projectfile/templates/` in the project directory,
it takes precedence over the embedded template:

```text
my-project/
├── .projectfile/
│   └── templates/
│       ├── CODE_OF_CONDUCT.md.tmpl    # replaces the built-in CoC template
│       └── SECURITY.md.tmpl           # replaces the built-in SECURITY template
└── projectfile.toml
```

Resolution order inside `core.Render(dir, name, data)`:

1. **Project-local**: `<dir>/.projectfile/templates/<name>` — checked first when `dir` is non-empty.
1. **Embedded**: the `text/template` registered by the bridge at `init()` time via `core.RegisterTemplates`.

Templates are parsed on first use and cached per `(dir, name)` pair for
the process lifetime. A symlink or file move that changes the inode does
NOT invalidate the cache — the content is captured at parse time.

The override path is exported as `core.LocalTemplatesDir` (= `.projectfile/templates`).

New template-using bridges inherit this behaviour automatically — they call
`core.Render(opts.Dir, name, data)` and the override is handled centrally.

## SPDX boilerplate (`internal/spdx/`)

`spdx.Text(id, opts)` resolves an SPDX ID with lookup order:

1. Embedded set under `internal/spdx/embedded/<id>.txt`.
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

License texts are **committed** (`internal/spdx/embedded/*.txt`) — core is a
consumed Go library, so its `go:embed` assets must ship in the module (a
consumer building `pkg/spdx` from the immutable module cache has no build step
to fetch them). `make fetch-spdx` (idempotent, used as `build-local`
prerequisite) refreshes missing IDs; `make sync-spdx-embed` re-downloads all
unconditionally; commit the result so the module stays self-contained. Note the SPDX ID correction: `BUSL-1.1` is canonical
for the Business Source License — `BSL-1.1` 404s upstream.

## Offline mode (`--offline`)

Global persistent flag. When set, all network fetches are refused:

- **SPDX**: skips upstream fetch; uses embedded set + XDG cache only. Returns `ErrOffline` when the ID is in neither.
- **Includes**: HTTP(S) URLs in `includes` are resolved from XDG cache only. Uncached includes are skipped with a warning (partial data > hard stop).
- **Forge push**: refuses immediately with an error.
- **Forge list**: works fine (reads env vars + projectfile data only).

### Missing local includes (`--fail-on`)

A local include that does not exist on disk is a **soft failure**, not a hard
one: the file may be transiently absent (e.g. an m6e include being fixed in
parallel while m6e-sync consumes it). The persistent `--fail-on` flag sets the
minimum include-resolution severity that aborts a read:

- `--fail-on error` (default) — a missing local include is **warned and skipped** `WARN include skipped (not found)`, visible even without
    `--verbose`); partial data still resolves. This is what keeps m6e-sync unblocked while an include is being fixed upstream.
- `--fail-on warning` — a missing local include **aborts** the command, restoring the pre-lenient strict behaviour.

Other include failures (parse errors, HTTP errors, cycles, permission denied)
are always hard errors regardless of the flag. `ReadOptions.FailOn`
(`IncludeFailLevel`: `FailOnError` | `FailOnWarning`) carries the threshold
into the resolver; `fetchLocalInclude` classifies an `os.IsNotExist` read as
the soft case. Offline-uncached HTTP includes remain warn-and-skip always.

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

1. **Embedded set** (compile-time, zero I/O).
1. **XDG cache**: `${XDG_CACHE_HOME:-~/.cache}/projectfile-cli/<spdx|includes>/`.
1. **Network fetch** → write to cache → return. Skipped when `--offline`.

### HTTP fetch defenses (includes)

The include network tier refuses to silently serve wrong content so users
see the real HTTP cause instead of a downstream YAML/JSON parse error:

- **Status hints**: non-2xx surfaces as `HTTP <code> (hint) for <url>`. Hints: 401 → authentication required, 403 → forbidden (repository may be private), 404 → not found, 5xx → server error.
- **Cross-host redirect refusal**: a forge should never bounce raw content to another host; doing so is treated as an SSO/auth gateway.
- **HTML content-type refusal**: an include document is never HTML. A 200 OK with `text/html` is the signature of an auth/login wall reached after a same-host redirect (Forgejo/Gitea private-repo raw → `/user/login`).
- **Poisoned-cache self-heal**: cache entries that begin with `<!DOCTYPE` or `<html>` (written by an older pf-cli build before the guards above) are discarded on read and replaced by a fresh fetch.

### Cache management

- `pf-cli cache status` — show embedded/cached counts for SPDX and includes.
- `pf-cli cache warm [dir]` — prefetch all SPDX licenses + HTTP includes from the projectfile. Flags: `--spdx-only`, `--includes-only`.
- Cache is immutable (URL→content is deterministic). No TTL, no eviction. Manual `rm -rf ${XDG_CACHE_HOME:-~/.cache}/projectfile-cli/` to clear.

### `ReadOptions` propagation

`projectfile.ReadWithOptions(dir, ReadOptions{Offline: true})` threads the
offline flag through include resolution. All commands use this when
`--offline` is set. `ReadBase`/`ReadBaseFromPath` are unaffected — they
never resolve includes.

## `pf-cli-managed:` marker

Sentinel pf-cli writes at the top of every file with `Policy.Marker: true`.
Two forms — both recognised by `core.HasMarker`:

- `# pf-cli-managed: yes` (exported as `core.Marker`) — `.gitignore` family, `FUNDING.yml`.
- `<!-- pf-cli-managed: yes -->` (exported as `core.MarkerHTML`) — Markdown documents where `#` would render as an H1 heading (SECURITY.md, CODE_OF_CONDUCT.md).

`CONTRIBUTING.md` and `LICENSE` use `Policy.ScaffoldOnce: true` instead —
`--force` is required to overwrite.

## Snippet authoring (`bridge/ignore/snippets/<extKey>/`)

- `<extKey>` is the `[org.projectfile.ignores.<extKey>]` sub-namespace pointer (today: `git`, `docker`, `container`, `npm`, `claude`). On-disk filename is `.<extKey>ignore`, also the snippet file extension.
- One file per stack tag (`go.gitignore`, `python.gitignore`, ...). Tags from
    `specification/spec/stack-tags.yaml`.
- Every pattern in the output has provenance: stack snippet, editor snippet, or user-declared include/extra. There is no shared/universal block — patterns belong to their source.
- **Comment-only** snippets are deliberate no-ops; the assembler’s
    `hasContent` filter drops them. Use for tags like `react`/`vue` whose useful patterns already come from `node`/`typescript`.
- The embed directive is `//go:embed all:snippets`, NOT `//go:embed snippets` — default `embed` silently excludes `_*` / `.*` files.
- **Editor snippets** live in `snippets/editors/` (e.g. `idea.gitignore`, `vscode.gitignore`). They are keyed by editor tag, not stack tag, and emit as `editor:<tag>` blocks. Only applies to `.gitignore`.

### Editor ignore rules (`org.projectfile.editors`)

Editors are distinct from stack — they describe developer tooling preferences, not project technology. Two resolution modes:

1. **Explicit**: `[org.projectfile.editors] use = ["idea", "vscode"]` in projectfile. That list is authoritative.
1. **Auto-detect**: When no `use` list is set, the bridge probes the project directory for known markers (`.idea/`, `.vscode/`, `.vimrc`, `.emacs`, `*.sublime-project`). Detected editors are emitted in sorted order.

Recognised editor tags: `idea`, `vscode`, `vim`, `emacs`, `sublimetext`. Adding a new editor is one snippet file in `snippets/editors/` plus one entry in `editorProbes` in `bridge.go`.

### Generated-file layout (ignore)

```text
<banner with # pf-cli-managed: yes marker>

# >>> stack:<tag>            (one block per stack tag, sorted, content-only)
<tag snippet body>
# <<< stack:<tag>

# >>> editor:<tag>           (one block per editor tag, sorted, git target only)
<editor snippet body>
# <<< editor:<tag>

# >>> user-include           (from ext.<target>.include, if any)
<lines>
# <<< user-include

# >>> user-extra             (from ext.extra, if any)
<lines>
# <<< user-extra
```

Each block’s content lines are **sorted and deduplicated** so the output is stable and deterministic.

## CODEOWNERS-specific notes

- Bridge probes `CODEOWNERS`, `.github/CODEOWNERS`, `docs/CODEOWNERS` in that order — first hit wins.
- `[org.projectfile.codeowners].entries` is an ordered array of
    `{pattern, owners}`. Order is semantically significant — CODEOWNERS pattern resolution is "last match wins per path".
- Comments and empty lines in a hand-written CODEOWNERS do NOT survive the first to-pf sync: pf has no slot for them. After that first normalisation the file is byte-stable across alternating round-trips.

## fundingjson-specific notes

- Bridge renders `funding.json` conforming to FundingJSON v1.1.0 (<https://fundingjson.org>). Output is validated against the embedded schema before writing — invalid generation is impossible.
- `Policy{}` (no marker, no scaffold-once): JSON has no comment syntax so the pf-cli-managed marker cannot be embedded. The file is always overwritten to stay in sync with projectfile.
- Entity resolution: prefers `[[organizations]]` with maintainer/owner role, falls back to `[[people]]` with maintainer role, then any person with email. Override with `[org.projectfile.funding].entity-type` / `entity-role`.
- Required data that must exist in projectfile or the bridge refuses:
    - Entity: at least one person/org with email (for entity.name, entity.email)
    - `links[type=homepage]` (for entity.webpageUrl)
    - `[org.projectfile.funding].channels` (at least one)
    - `[org.projectfile.funding].plans` (at least one)
- Projects section is auto-populated from identity + links + license + keywords. Keywords are filtered to FundingJSON tag pattern `^[a-z0-9-]+$`, capped at 10.
- No REUSE header prepended (JSON has no comment syntax).

## Lossless round-trip

`npm.Document`, `cff.Document`, `pyproject.Document`, and `composer.Document`
all carry a `Rest *rawdoc.X` field — the parsed source kept in a
key-order-preserving form. `Read` populates both the typed view and `Rest`;
`Write` paints the typed fields back onto `Rest` as the canvas. Unknown keys
(`scripts`, `volta`, `workspaces`, `autoload`, `extra`, `repositories`, ...)
survive a full round-trip including freshly-created files when the in-memory
doc has been carrying its Rest through `Clone`.

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

- `core.Trunc(s)` caps display strings at 60 chars for `FieldChange` output.
- PURL format per ecosystem:
    - npm: `pkg:npm/{name}@{version}`
    - pypi: `pkg:pypi/{normalized-name}@{version}`
    - composer: `pkg:composer/{vendor}/{package}@{constraint}` — operators (`^`, `~`, `>=`, `||`, ...) preserved verbatim.
- Never call `os.WriteFile` on `package.json` / `CITATION.cff` /
    `pyproject.toml` / `composer.json` / `projectfile.*` / `CODEOWNERS` directly — go through the format package’s `Write` / `projectfile.Write` so round-trip semantics and schema-header injection are preserved.
- **Base-write invariant**: every command that mutates and writes the projectfile MUST write the BASE document (no includes resolved), never the merged one. `set`/`add`/`del`/`scan` use `ReadBaseFromPath`;
    `convert` uses `ReadRawBaseFromPath`; `bridge` sync / derive / fill-required-fields read merged for mapper context but call
    `projectfile.ReconcileBase(basePF, preSync, postSync)` before writing so only the fields the operation actually changed land on disk.
- Reverse-DNS extension namespaces park ecosystem-only fields under
    `pf.Extensions[<ns>]` so they round-trip without polluting native pf slots. In use:
    - `org.python.pep621` — readme, scripts, gui-scripts, entry-points, optional-dependencies, dynamic, classifiers
    - `org.packagist.composer` — type, minimum-stability, prefer-stable, abandoned
    - `org.projectfile.citation` — DOI, message, preferred citation (CFF mirror)
    - `org.projectfile.ignores` — generate config (generate list, extra, per-target include/exclude) for gitignore-style FILE-PATTERN ignores
    - `org.projectfile.vulnerabilities` — tool-agnostic suppressed vulnerability IDs (CVE/GHSA) fanned out to .trivyignore / .grype.yaml / osv-scanner.toml (suppress list, generate opt-out)
    - `org.projectfile.editors` — editor/IDE ignore rules (`use` list; auto-detect from filesystem when absent)
    - `org.projectfile.funding` — GitHub-style FUNDING.yml providers + `path` override + FundingJSON channels/plans/history + entity overrides
    - `org.projectfile.security` — contact, report-url, supported-versions, disclosure-window, gpg-key, bug-bounty-url
    - `org.projectfile.contributing` — sections, cla-url, chat-url, coc-url, recommend-to-star (bool|map host→bool, default off), recommend-to-follow (bool|map platform→bool, default off)
    - `org.projectfile.support` — response-time, eol table
    - `org.projectfile.conventions` — commit-style, workflow, style-guide-url (top-level + per-stack-tag)
    - `org.projectfile.codeowners` — `entries = [{pattern, owners}, ...]`
    - `org.projectfile.readme` — `blocks` (ordered block names), `extras` (inline content blocks)
- Extension lookup goes through `projectfile.LookupExtension(doc, ns)` — never read `doc.Extensions[ns]` directly. The helper handles both encodings (flat key for YAML/JSON and quoted TOML, dotted TOML header that go-toml/v2 explodes into nested maps).
- Extension writes go through `projectfile.SetExtension(doc, ns, value)` — it prunes any pre-existing nested-map form before writing the flat key.

## Composer-specific notes

- composer’s `require` map is a union of two pf concepts: keys *with* `/` are library deps (→ `dependencies.runtime` as `pkg:composer/...` PURLs); keys
    *without* `/` are platform requirements (`php`, `ext-*`, `lib-*`,
    `composer-*` → `requirements.runtime`). The slash-as-discriminator rule is exact for valid composer.json input.
- `authors[].role` is freeform per spec and is dropped on the way into projectfile; all entries flow in with role `author`. Unmapped composer fields (`autoload`, `scripts`, `bin`, `extra`, `config`, `repositories`, `conflict`, `replace`, `provide`, `suggest`, `archive`, `support.{email,wiki,irc,rss,security}`) round-trip through the Rest canvas without explicit mappers.
- Name mapping: `vendor/package` ↔ `composer.<vendor>` / `<package>`. The
    `composer.` namespace prefix is the round-trip marker.

## How to add a new bridge

### Syncer (round-trip; e.g. `Cargo.toml`, `Gemfile`, `go.mod`)

1. Add a directory `internal/bridge/<name>/` with the standard layout:
    - `document.go` — `Document` struct + `Rest *rawdoc.X` field.
    - `read.go` — populates both typed view and `Rest`.
    - `write.go` — paints typed fields back onto `Rest`.
    - `bridge.go` — `Bridge` struct implementing `core.Syncer` (Name, Filename, Aliases, Labels, FullPath, Exists, Policy, Read, NewEmpty, Write, Clone, BuildMappers).
    - `mapper.go` — `buildMappers(extDoc, pf) core.MapperList` — one entry per synced field. Templates by shape:
        - JSON, flat field set: `bridge/npm/mapper.go`
        - JSON, sub-objects: `bridge/composer/mapper.go`
        - TOML with foreign tables: `bridge/pyproject/mapper.go`
        - YAML with comments: `bridge/cff/mapper.go`
        - Line-oriented text: `bridge/codeowners/mapper.go`
    - `register.go` — `func init() { bridge.Register(Bridge{}) }`.
1. For ecosystem-only fields with no native pf slot, use the extension-namespace passthrough pattern (`mapExtField` in composer, `mapExtensionField` in pyproject). Pick a reverse-DNS namespace key and add it to the "in use" list above.
1. Blank-import the package from `internal/cmd/bridge.go`.
1. (Optional) Add a `Source` to `internal/source/` so `pf-cli init` can auto-detect the ecosystem.

### Renderer (derive-only; e.g. `README.md`, `NOTICE`, `EditorConfig`)

1. Add `internal/bridge/<name>/templates/<filename>.tmpl`.
1. Add `internal/bridge/<name>/{view.go, bridge.go, register.go}` following the file-order convention. `bridge.go` implements `core.Renderer` and optionally `core.RequiredFieldsBridge`.
1. `register.go` declares `//go:embed all:templates`, calls
    `core.RegisterTemplates("<filename>.tmpl", ...)`, and ends with
    `bridge.Register(Bridge{})`. Local template overrides in
    `.projectfile/templates/` are handled automatically by `core.Render` — no per-bridge wiring needed.
1. For per-filename user-config overrides on the write path, use
    `core.PathOrDefault(dir, filename, defaultRel)` in `FullPath`.

### Ignore-file target

The ignore subpackage is multi-target by design. Adding a FILE-PATTERN ignore
target is purely additive (for vuln-ignore scanner targets, see
`bridge/vulnerabilities/` instead):

1. Create `internal/bridge/ignore/snippets/<extKey>/`.
1. Drop per-tag snippet files (`python.dockerignore`, `node.npmignore`, ...).
1. Append a row to the `targets` table in `bridge/ignore/register.go`:
    `{filename: ".<extKey>ignore", extKey: "<extKey>"}`. The `init()` loop turns it into a `bridge.Register` call automatically.
1. If the target needs per-target user overrides (`[org.projectfile.ignores.<extKey>] include / exclude`), add the field to
    `IgnoresExtension` in `internal/projectfile/types.go`, populate it in
    `GetIgnoresExtension` (`helpers.go`), and add the switch arm in
    `overrideFor` inside `bridge/ignore/bridge.go`.

### Vulnerability scanner target

The vulnerabilities subpackage fans the single `org.projectfile.vulnerabilities`
suppress list out to scanner ignore files. Adding a scanner is purely additive:

1. Append a row to the `targets` table in `bridge/vulnerabilities/register.go`:
    `{scanner: "<extKey>", filename: "<file>"}`.
1. Add a render arm to the `switch b.scanner` in `bridge/vulnerabilities/bridge.go` emitting that scanner’s ignore format. Each entry is a `VulnerabilitySuppress{ID, Reason}`; sort/dedup is already applied upstream.

## `get` addressing grammar

The `get`/`set`/`add`/`del` verbs share the dotted-path + bracket grammar
defined in `internal/fieldpath/`. Four bracket shapes:

| Form           | Segment kind    | Meaning                                         |
| -------------- | --------------- | ----------------------------------------------- |
| `key[N]`       | `SegIndex`      | List positional index, negative counts from end |
| `key[k=v,...]` | `SegSelector`   | First list item whose every predicate matches   |
| `key[]`        | `SegProject`    | Project remaining path over every list item     |
| `key{}`        | `SegMapProject` | Fan out a map as `(key, value)` pairs           |

### Synthetic (derived) addresses

A few `get` addresses are **computed** from other fields rather than read from
the document. `internal/cmd/derived.go` (`derivedFields`) wires them for `get`
and `resolveEntry` consults the map *before* document resolution, so a synthetic
address wins. The rules themselves live in `internal/projectfile` (e.g.
`ImageBasename`) and are re-exported via `pkg/projectfile`, so a rule several
consumers would each re-derive has ONE home — read by the `get` synthetic AND by
library consumers (ci-resolver) alike:

| Address          | Computes                                                                            |
| ---------------- | ----------------------------------------------------------------------------------- |
| `image.basename` | `org.projectfile.ci.image`, else `<last-label(identity.namespace)>/<identity.name>` |

`image.basename` is the single source of the container-image basename rule
(`projectfile.ImageBasename`): m6e’s `M6E_IMAGE_BASENAME` (via
`projectfile-read.sh`) reads `projectfile get image.basename` and ci-resolver’s
`Load` reads `pkg/projectfile.ImageBasename` directly — one rule, so a
cloud-built ref and an m6e-built ref of one project can never disagree. A synthetic that cannot be determined (e.g.
no identity name and no override) is treated as a normal missing value, so
`--default` / `--or-default` still apply.

Map projection (`{}`) is the only addressing form that produces *pairs*.
The walker carries the result back through `Result.IsPairs`; downstream
formatters render it as:

- `--format=raw` — `KEY=VALUE` line per pair (line-oriented, shell-friendly)
- `--format=sh` — `export KEY=VALUE` line per pair, with `shellKey` normalisation (`.` and `-` → `_`, uppercase)
- `--format=json` — a single JSON object

Iteration order is **sorted-by-key**. Only `key{}.keys` and `key{}.values`
are legal trailers past `{}`.

### `--expand-env`

`get --expand-env` substitutes `${VAR}` occurrences (braced form only) in
the source bytes with environment values before parsing. Unknown variables
expand to empty. **Braced form only** is a deliberate divergence: bare
`$IDENTIFIER` would obliterate `$schema` (the YAML/JSON projectfile
discriminator). Writes (`set`, `add`, `del`) do not honour `--expand-env` —
expanding on the way back to disk would corrupt the source.

## `optimize` command

`pf-cli optimize [dir]` removes local fields that duplicate values already
provided by includes. Algorithm:

1. Read the base projectfile WITHOUT resolving includes (`ReadRawBaseFromPath`).
1. Build the merged includes only (`ResolveIncludesOnly`): fetch every include and accumulate with `deepMerge` (later wins), but do NOT overlay the base. Resolution is **transitive** — an include’s own includes are folded in via the shared `resolveIncludesChain`, so a base value duplicating a transitive contribution is stripped too.
1. `StripRedundant(base, includes)` walks the base map recursively; any key whose value is `reflect.DeepEqual` to the corresponding include value is deleted from the base. Keys `includes`, `$schema`, and `spec_version` are always preserved (local-file concerns, not inherited data). The reserved entity-list keys `people` and `organizations` are also preserved as whole units: their entries merge by identity in `deepMerge` (see "Include resolution path" above), so a base entry’s identity fields (email / orcid / name) are the LINK that attaches project-scoped data (`from` / `to`) to the include’s full record — stripping that link would orphan the entry and re-introduce the schema violation the include-merge path exists to prevent.
1. Parse the stripped raw map back into a `Document` and write.

Flags: `--dry-run` / `-n` (report without writing), `--path-file` / `-f`
(explicit path). Alias: `opt`.

The command reports each removed dotted path and a total count. When no
includes are declared or no redundancy is found, it exits 0 with a
diagnostic message.

## Derive engine (`internal/derive/`)

Turns the primary repository URL and detected stack into structured field
values that would otherwise need hand-typing. Two inference passes:

- **forges/** — repository URL host → tracker URL pattern (`/issues`, GitLab `/-/issues`, Codeberg/Gitea `/issues`, sourcehut tracker).
- **registries/** — detected stack + `identity.name` → package-registry landing page (npmjs.com, pypi.org, packagist.org, crates.io). Output lands as `[[links]]` entries with `type = "package-registry"`.

The engine runs from `internal/cmd/bridge.go` after person-conflict
emission and before write. `pf-cli bridge` (no args) runs ONLY the
derivation pass — useful when a user edited the repository URL by hand and
wants the issue tracker link to follow.

Output is `[]Change` so the caller can fold each derived field into the
existing `Result.Changes` log and `[org.projectfile.cli]` bookkeeping
survives idempotent re-runs.

## Forge push (`internal/forge/`)

Pushes metadata to forge APIs (GitHub, GitLab, Forgejo). Structure:

- **core/** — `PushOptions`, `Field` constants, `Push()` dispatcher,
    `Resolver` (resolves owner/repository from remote URL).
- **drivers/** — per-forge API clients: `github/`, `gitlab/`, `forgejo/`. Each implements the `Pusher` interface from `core/`.
- **hostmatch/** — host→driver matching with opt-out/override via
    `[org.projectfile.forge]`.

Controlled by `[org.projectfile.forge]` extension:
`push` (bool), `hosts` (opt-out list), `kinds` (host→platform map).
Refused entirely in `--offline` mode.

## Validation (`internal/validate/`)

Runs the projectfile v1 JSON Schema against a parsed document. The schema
is embedded at build time (`embedded/v1.json`); refresh with
`make sync-schema-embed`. Uses `santhosh-tekuri/jsonschema/v6` with
`dlclark/regexp2` for pattern validation. The schema is compiled once and
cached for the process lifetime.

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

The `pkg/*` packages are zero-cost re-exports of the `internal/*` implementations
(Bridge Revolution Phase 1 — [bridge-revolution.md](../bridge-revolution.md)).
They exist so the **movable trio** — `internal/{bridge,forge,scanners}` — depends
only on core's public API, never on core `internal/`. That decoupling is the
whole point of Phase 1: once the trio reaches into no core `internal/` package,
it can extract to its own module (`projectfile/bridge`) in Phase 2.

Each façade is a thin `surface.go` of type aliases (`= internal.X`, carrying
full method sets), value-aliased functions (`var F = internal.F`, one
implementation), and re-exported constants. Curated to real consumers (the
`pf-bridge` module + `d9t/ci-resolve`) — do NOT widen without one. **Mutable
package vars cross as setters** (`SetQuiet`/`SetVerbose`/`SetYAMLOutputSorted`/
`SetIgnored`), never value aliases — a `var X = internal.X` copies, so a
consumer's write would not reach core.

| façade | promotes | consumer uses it for |
| --- | --- | --- |
| `pkg/projectfile` | the document model | read+write+model + `ReadOptions`/`IncludeFailLevel`/`FailOn*`/`SplitGitName`/link+CLI-ext helpers. `projectfile.go` keeps the external read surface (`Read`/`DetectPath`/`Stack`/`Extension`) for `d9t/ci-resolve`. |
| `pkg/genlog` | structured logging | `Decision`/`Info`/`Warn`/`Error`/`Section`/`Plain` + `SetQuiet`/`SetVerbose`/`SetOutput` (the cli + pf-bridge roots drive the toggles; a mutable var must cross as a setter, not a value alias) |
| `pkg/rawdoc` | lossless round-trip primitives | `OrderedJSON`/`YAMLNode`/`OrderedTOML` + constructors (bridge `Document.Rest`) |
| `pkg/userconfig` | XDG config | `Load`/`PathFor`/`IsPrivateHost` + `SetIgnored` + `Config`/`ExistingPath`/`Write` (cli setup wizard) |
| `pkg/spdx` | license text + expression helpers | `Text`/`Substitute`/`Split`/`StripException` (license bridge) + `Status`/`WarmAll` (cli cache) |
| `pkg/selector` | bubbletea picker/fill | `Run`/`Choices`/`Fill`/`FillField`/`MultiInput` (bridge picker, scaffold) |
| `pkg/pflock` | file lock | `WithLock`/`WithLockTimeout` (cli + bridge/forge write paths) |
| `pkg/fieldpath` | dotted-path grammar | `Parse`/`Path`/`Segment` (derive selectors) + `Resolve`/`Set`/`Add`/`Delete`/`Result`/`Pair`/`LookupDefault` (cli get/set/add/del) |

`pkg/projectfile` also grew a Phase 8 block (`WriteClean`, `ReadRaw*`,
`ReadFromPath*`, `FromMap`, `ResolveIncludesOnly`/`StripRedundant`/`SortIncludes`
for optimize, `AllHTTPIncludes`/`WarmInclude`/`XDGCacheDir` for cache,
`YAMLOutputSortedEnabled` getter) — the surface the moved `projectfile` CLI reads.

`pkg/{derive,source}` were retired in the Phase 2 cut — their callers (derive,
scanners) moved into `projectfile/bridge`, so caller and callee reunited there
and the cross-`internal` façade is no longer needed.

The three library consumers — `projectfile/cli` (the `pf-cli` binary),
`projectfile/bridge` (`pf-bridge`), and `d9t/ci-resolve` (`pf-ci`) — reach core
**only** through these façades. Core-internal packages import each other
directly (no façade needed among siblings); a `pkg` façade and its `internal`
twin never cycle: façades only alias downward.

## i18n note for renderer templates

Per workspace AGENTS.md, user-facing strings ship in `es_CL` and `uk_UA`.
Scope split:

- **CLI messages** (status lines, errors, hints) go through the existing
    `.container/.../locale/*.po` flow.
- **Generated file bodies** (CONTRIBUTING.md, SECURITY.md, CoC, FUNDING.yml, LICENSE) are English-only in v1, deliberately. These are external-reader artefacts (GitHub viewers, contributors) and are conventionally English in OSS. A follow-up plan will add per-locale template files when concretely requested.

## Build

```sh
make build-local     # compile host binary to dist/pf-cli (Go toolchain)
make install-local   # build-local + copy into ~/.local/bin
make build           # m6e container build — produces a Docker image, NOT a host binary
make lint            # golangci-lint + gosec + shellcheck + markdownlint + textlint
make format          # gofmt + golangci-fmt + markdownlint-fix + textlint-fix
make sync-spdx-embed # (re)download all SPDX license texts into internal/spdx/embedded/
make fetch-spdx      # download missing SPDX texts (idempotent, runs before build-local)
make help            # categorised list of every target
```

`make lint`/`make gosec` spawn many parallel compiles and saturate CPU; for
quick correctness checks prefer per-package `go vet ./internal/<pkg>`.
