<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>

SPDX-License-Identifier: MIT
-->

# Conformance and field-coverage report

Audit of the `pf-cli` reference implementation against the projectfile v1
specification (`../specification/spec/v1.md`, schema `spec/schema/v1.json`).

Two concerns are covered:

- **Field coverage** — which reserved/extension fields the CLI *defines* (type structs, field-path logic, derive layer) versus which it actually
    *consumes* (reads, writes, or acts on in a command). Defined-but-unused fields are flagged for human decision, not deleted: trimming them risks breaking lossless round-trip and serialization.
- **Spec conformance** — whether the normative MUST/SHOULD rules most likely to drift are correctly implemented.

Status vocabulary:

- **CONSUMED** — read/written/acted-on by at least one command beyond bare serialization round-trip.
- **ROUND-TRIP-ONLY** — parsed, serialized, cloned, and addressable through
    `get`/`set`/`add`/`del`, but no dedicated behavior derives from it. This is a deliberate baseline (peaceful-cohabitation, §3.7) — not a defect — but fields here are candidates if the CLI is meant to act on them.
- **PASS / VIOLATION / NEEDS-HUMAN** — conformance verdicts.

## Field coverage

### Reserved top-level fields (`internal/projectfile/types.go`)

| Field                               | Defined                 | Consumed by                                                | Status          |
| ----------------------------------- | ----------------------- | ---------------------------------------------------------- | --------------- |
| `spec_version`                      | `Document.SpecVersion`  | `parse.go`, `serialize.go`, validate                       | CONSUMED        |
| `$schema`                           | `Document.Schema`       | discriminator, validate, `write.go` header                 | CONSUMED        |
| `kind`                              | `Document.Kind`         | forge push (Schema.org), default `library`                 | CONSUMED        |
| `identity.namespace`                | `Identity.Namespace`    | init, scan, addressing                                     | CONSUMED        |
| `identity.name`                     | `Identity.Name`         | init, scan, bridges                                        | CONSUMED        |
| `identity.version`                  | `Identity.Version`      | npm/pyproject/composer PURL + version sync                 | CONSUMED        |
| `identity.title`                    | `Identity.Title`        | localized; forge/bridges read                              | CONSUMED        |
| `identity.summary`                  | `Identity.Summary`      | forge push description (`forge/core/push.go`)              | CONSUMED        |
| `identity.description`              | `Identity.Description`  | forge push, bridges                                        | CONSUMED        |
| `identity.created`                  | `Identity.Created`      | Git scanner populates (`scanners/git`)                     | CONSUMED        |
| `identity.released`                 | `Identity.Released`     | CFF `date-released` sync (`bridge/cff`)                    | CONSUMED        |
| `identity.modified`                 | `Identity.Modified`     | Git scanner populates from HEAD (`scanners/git`)           | CONSUMED        |
| `repositories[].url`                | `Repository.URL`        | scan, derive, forge host match                             | CONSUMED        |
| `repositories[].type`               | `Repository.Type`       | scan, bridges                                              | CONSUMED        |
| `repositories[].path`               | `Repository.Path`       | npm `repository.directory` sync                            | CONSUMED        |
| `repositories[].branch`             | `Repository.Branch`     | Git scanner sets default branch                            | CONSUMED        |
| `repositories[].role`               | `Repository.Role`       | role constants; origin canonical; Git scanner; addressing  | CONSUMED        |
| `license.spdx`                      | `License.Spdx`          | LICENSE/REUSE renderers, SPDX resolver                     | CONSUMED        |
| `license.covers`                    | `License.Covers`        | round-trip + serialize only                                | ROUND-TRIP-ONLY |
| `license.file`                      | `License.File`          | round-trip + serialize only                                | ROUND-TRIP-ONLY |
| `copyright.year`                    | `Copyright.Year`        | LICENSE/NOTICE lines, default = current year               | CONSUMED        |
| `people[].*`                        | `PersonOrEntity`        | merge/dedup, CFF/CODEOWNERS/contact resolution             | CONSUMED        |
| `people[].alias`                    | `PersonOrEntity.Alias`  | merged (`people.go`); no dedicated emit                    | ROUND-TRIP-ONLY |
| `keywords`                          | `Document.Keywords`     | forge topics, npm/pyproject keywords sync                  | CONSUMED        |
| `stack`                             | `Document.Stack`        | ignore-snippet selection, scan                             | CONSUMED        |
| `requirements.operating-system`     | `Requirements.OS`       | npm `os` sync; addressing                                  | CONSUMED        |
| `requirements.arch`                 | `Requirements.Arch`     | npm `cpu` sync; addressing                                 | CONSUMED        |
| `requirements.browsers`             | `Requirements.Browsers` | parse/serialize/clone only — no `.browserslistrc` renderer | ROUND-TRIP-ONLY |
| `requirements.runtime`              | `Requirements.Runtime`  | composer platform-req sync                                 | CONSUMED        |
| `dependencies.{runtime,build,test}` | `Dependencies`          | npm/composer/pyproject dep sync                            | CONSUMED        |
| `links[].*`                         | `Link`                  | derive engine, forge homepage, fallback chains             | CONSUMED        |
| `funding[].*`                       | `Funding`               | FUNDING.yml renderer                                       | CONSUMED        |

### Extension namespaces

Every extension struct has a `Get…Extension` helper *and* a concrete
consumer — none are dead.

| Namespace                      | Struct                        | Consumed by                          | Status                        |
| ------------------------------ | ----------------------------- | ------------------------------------ | ----------------------------- |
| `org.projectfile.citation`     | `CitationExtension`           | `bridge/cff`                         | CONSUMED                      |
| `org.projectfile.ignores`      | `IgnoresExtension`            | `bridge/ignore`                      | CONSUMED (partial — see note) |
| `org.projectfile.funding`      | `FundingExtension`            | `bridge/funding`                     | CONSUMED                      |
| `org.projectfile.security`     | `SecurityExtension`           | `bridge/security`                    | CONSUMED                      |
| `org.projectfile.contributing` | `ContributingExtension`       | `bridge/contributing`                | CONSUMED                      |
| `org.projectfile.codeowners`   | `CodeOwnersExtension`         | `bridge/codeowners`                  | CONSUMED                      |
| `org.projectfile.forge`        | `ForgeExtension`              | `forge/core/push.go`, `cmd/forge.go` | CONSUMED                      |
| `org.projectfile.cli`          | `CLIExtension`                | `derive/engine.go`                   | CONSUMED                      |
| `org.python.pep621`            | (via composer/pyproject Rest) | `bridge/pyproject`                   | CONSUMED                      |
| `org.packagist.composer`       | (via composer Rest)           | `bridge/composer`                    | CONSUMED                      |

Plus the read-only consumers `coc` (`bridge/coc`) which reads
`org.projectfile.contributing.coc_url` and `org.projectfile.security`.

### Defined-but-unused candidates (HUMAN DECISION)

These survive round-trip and are addressable, but no command derives
behavior from them. Do **not** delete without confirming nothing external
(future bridges, Schema.org export) is expected to grow into them.

- `requirements.browsers` — CONSUMED: the `.browserslistrc` renderer (`bridge/browserslist`) emits one query line per entry.
- `people[].alias` — CONSUMED: CODEOWNERS owner-token derivation prefers it as a forge handle (`bridge/codeowners`, `projectfile.ForgeHandle`).

### Partial-coverage note: `org.projectfile.ignores`

`IgnoresExtension` (`types.go`) now carries per-target override slots for
all four registered targets — `Git`, `Docker`, `Npm`, `Trivy` — so
`overrideFor` (`bridge/ignore/bridge.go`) honours `include`/`exclude` for
`.gitignore`, `.dockerignore`, `.npmignore`, and `.trivyignore` symmetrically.

## Spec conformance

| Rule (spec ref)                                                          | Verdict          | Citation                                                                |
| ------------------------------------------------------------------------ | ---------------- | ----------------------------------------------------------------------- |
| §4.5 — at most one `projectfile.*`; 2+ MUST fail (no mtime side-channel) | **VIOLATION**    | `internal/projectfile/read.go:35-78`                                    |
| §4.9 — `requirements.operating-system` (renamed from `os`)               | **PASS** (fixed) | `internal/projectfile/types.go:130`, `parse.go:180`, `serialize.go:257` |
| §3.3 — format-agnostic YAML/TOML/JSON read                               | **PASS**         | `internal/projectfile/read.go:121-130`                                  |
| §3.3 — format-agnostic YAML/TOML/JSON write                              | **PASS**         | `internal/projectfile/write.go:21-26`                                   |
| §3.4 — discriminator (`$schema` / `#:schema` + `spec_version`)           | **PASS**         | `parse.go:14-16`, `write.go:84`, `validate.go`                          |
| §3.6 — unknown reserved key preserved on round-trip                      | **PASS**         | `Document.Rest` (`types.go:19`), serialize.go                           |
| §3.7 — unknown extension namespace preserved (peaceful cohabitation)     | **PASS**         | `Document.Extensions` + `LookupExtension`/`SetExtension`                |
| §3.3 — dotted-string extension keys forbidden; nested mappings only      | **PASS**         | `SetExtension` prunes nested form, writes flat key (`helpers.go:69`)    |
| Namespace hygiene — `org.projectfile.cli` restored from `org.pf-cli`     | **PASS**         | `helpers.go:55` (`CLIExtensionNS = "org.projectfile.cli"`)              |

### VIOLATION detail — §4.5 multi-document selection

Spec §4.5 (v1.md:104-109) is now normative and explicit:

> A project root MUST contain at most one `projectfile.*` file. If two or
> more are present … Consumers MUST fail and SHOULD emit a diagnostic
> naming all the `projectfile.*` files found. … rather than silently
> picking one file by a side-channel signal (such as modification time), a
> consumer surfaces the ambiguity so a human resolves it.

The mtime tie-break the older spec carried was **removed**. `DetectPath`
(`internal/projectfile/read.go:35-78`) still implements the old behavior:
it collects every sibling, sorts by `info.ModTime()` (newest wins), logs
`"multiple projectfiles present; newest mtime wins"` citing "spec §4.5",
and returns the chosen file. This is now a direct contradiction of the
MUST-fail rule.

**NEEDS-HUMAN before fix** — this is a behavior change with blast radius:
every command that reads a projectfile goes through `DetectPath`, and
some workflows (e.g. a format-conversion step that briefly leaves two
files) may rely on the lenient behavior. Recommended fix once approved:
when `len(hits) > 1`, return an error naming all siblings (e.g.
`multiple projectfiles in %s: %s — remove all but one`) instead of
selecting newest. The diagnostic text and candidate-collection loop can
be reused verbatim; only the "sort + pick + Info-log" tail changes to an
error return. The `genlog.Info` reference to "spec §4.5" should be dropped
or repointed.

### NEEDS-HUMAN — orphaned `requirements.operating-system` default

`internal/fieldpath/defaults.go` carries a conditional default: when
`requirements.arch` is set but `operating-system` is not, the latter
defaults to `["linux"]`. The comment cited "spec §5.6", but no §5.6 exists
in v1 and §4.9 states "Absent = unconstrained" for `operating-system` with
no arch-coupled default. The rename left this default in place (now keyed
`requirements.operating-system`) and flagged it in-code, but the rule
itself appears to have **no spec basis in v1**. Recommend the human decide:
remove the default entry, or get the rule added back to the spec. Removing
it is a behavior change for `get --or-default requirements.operating-system`.

## Self-identity

The CLI’s own `projectfile.toml` declares:

```toml
[identity]
name = 'cli'
namespace = 'org.projectfile'
```

i.e. reverse-DNS coordinates `org.projectfile.cli`.

**Recommendation: change the CLI’s self-coordinates (do not edit here —
this is a user judgment call, flagged per instructions).**

Reasoning:

- The settled D-NS decision reserves `org.projectfile.*` for *first-party, spec-blessed* artifacts, and frames this CLI as "one implementation among many." Self-identifying under `org.projectfile` claims the reserved first-party namespace for what is, by that same decision, just one reference implementation.
- The project restored its extension namespace to `org.projectfile.cli`, reversing the earlier rename to `org.pf-cli`. The reserved `org.projectfile` root is appropriate here because this extension is defined by the projectfile specification itself.
- A coherent option is `namespace = 'org.projectfile.cli'` (matching the extension namespace) with `name = 'cli'`. Either keeps the namespace consistent with the spec-defined extension.
- This only affects the CLI’s self-description; it has no runtime behavior impact, so it is safe to defer to the user.
