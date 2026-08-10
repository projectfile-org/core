<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->
# Makefile Targets

## Analyze

### `gremlins`

Run mutation testing (requires gremlins binary)

> Source: project/50-gremlins.mk

## Bootstrap

### `bootstrap`

First-run actions

> Source: core/workflow/bootstrap.mk

### `custom-bootstrap-script`

Run bootstrap.sh if it exists

> Source: core/workflow/bootstrap.mk

### `m6e-cache-init`

Create host build-mount directories declared in the projectfile

> Source: core/base/035-cache.mk

### `preflight`

Run all preflight probes (host/tooling readiness)

> Source: core/base/080-preflight.mk

### `preflight-registries`

Verify referenced tool-image registry vars are configured (not a *.invalid sentinel)

> Source: core/base/115-registries.mk

## CI

### `act`

Run every generated GHA workflow via act (offline smoke test; per-goal: make act-<goal>)

> Source: core/ci/040-act.mk

### `ci-check`

Verify committed workflows match org.projectfile.ci (drift => non-zero)

> Source: core/ci/030-generate.mk

### `ci-dag`

Run the projectfile CI DAG — its goals (or all sinks); continues past failures

> Source: core/ci/010-select.mk

### `ci-generate`

Regenerate committed GHA + Forgejo workflows from org.projectfile.ci

> Source: core/ci/030-generate.mk

### `install-hooks`

Sync Git hooks (lefthook) to the committed config (dev machines only)

> Source: core/ci/030-generate.mk

### `matrix-sweep`

Pipeline sweep — the DAG once (no matrix) or per product cell (once→cell→join)

> Source: core/workflow/ci.mk

## Git

### `git-bug-clusters`

Show files most touched by bugfix commits (defect clusters)

`.makefile/core/scripts/git-maint.sh git-bug-clusters`

> Image: host runner

### `git-churn`

Show the most changed files in the last year (churn hotspots)

`.makefile/core/scripts/git-maint.sh git-churn`

> Image: host runner

### `git-clean-ignored`

Remove ignored files

`.makefile/core/scripts/git-maint.sh git-clean-ignored`

> Image: host runner

### `git-clean-untracked`

Remove untracked files

`.makefile/core/scripts/git-maint.sh git-clean-untracked`

> Image: host runner

### `git-contributors`

Rank contributors all-time versus the last 6 months (bus factor)

`.makefile/core/scripts/git-maint.sh git-contributors`

> Image: host runner

### `git-crisis`

Show revert/hotfix/rollback commits from the last year (firefighting)

`.makefile/core/scripts/git-maint.sh git-crisis`

> Image: host runner

### `git-disk-usage`

Report repository disk usage

`.makefile/core/scripts/git-maint.sh git-disk-usage`

> Image: host runner

### `git-fetch-prune`

Fetch and prune deleted remote branches

`.makefile/core/scripts/git-maint.sh git-fetch-prune`

> Image: host runner

### `git-fetch-prune-all`

Fetch all remotes and prune

`.makefile/core/scripts/git-maint.sh git-fetch-prune-all`

> Image: host runner

### `git-gc`

Garbage-collect the repository

`.makefile/core/scripts/git-maint.sh git-gc`

> Image: host runner

### `git-list-remotes`

List configured remotes

`.makefile/core/scripts/git-maint.sh git-list-remotes`

> Image: host runner

### `git-optimize`

Run full Git maintenance and repack

`.makefile/core/scripts/git-maint.sh git-optimize`

> Image: host runner

### `git-pack-refs`

Pack loose refs into packed-refs

`.makefile/core/scripts/git-maint.sh git-pack-refs`

> Image: host runner

### `git-prune-tags`

Prune deleted remote tags locally

`.makefile/core/scripts/git-maint.sh git-prune-tags`

> Image: host runner

### `git-pull-all`

Pull all branches

`.makefile/core/scripts/git-maint.sh git-pull-all`

> Image: host runner

### `git-push-all`

Push all branches

`.makefile/core/scripts/git-maint.sh git-push-all`

> Image: host runner

### `git-push-current`

Push the current branch

`.makefile/core/scripts/git-maint.sh git-push-current`

> Image: host runner

### `git-push-force-all`

Force-push all branches

`.makefile/core/scripts/git-maint.sh git-push-force-all`

> Image: host runner

### `git-rebuild-repo`

Rebuild the repository from scratch

`.makefile/core/scripts/git-maint.sh git-rebuild-repo`

> Image: host runner

### `git-reflog-clean`

Expire and clean the reflog

`.makefile/core/scripts/git-maint.sh git-reflog-clean`

> Image: host runner

### `git-remove-merged`

Delete merged local branches

`.makefile/core/scripts/git-maint.sh git-remove-merged`

> Image: host runner

### `git-repack`

Repack objects into fewer packs

`.makefile/core/scripts/git-maint.sh git-repack`

> Image: host runner

### `git-reset-current`

Hard-reset the current branch

`.makefile/core/scripts/git-maint.sh git-reset-current`

> Image: host runner

### `git-stats`

Show repository statistics

`.makefile/core/scripts/git-maint.sh git-stats`

> Image: host runner

### `git-submodules-pull-all`

Pull all Git submodules

> Source: core/tools/git.mk

### `git-velocity`

Show commits per month across all history (project pulse)

`.makefile/core/scripts/git-maint.sh git-velocity`

> Image: host runner

### `git-verify`

Verify repository object integrity

`.makefile/core/scripts/git-maint.sh git-verify`

> Image: host runner

### `pf-bridge-gitattributes-check`

Verify .gitattributes still matches the projectfile

`pf-bridge gitattributes --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-gitattributes-generate`

Generate .gitattributes from the projectfile

`pf-bridge gitattributes --force`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-gitignore-check`

Verify .gitignore still matches the projectfile

`pf-bridge ignore .gitignore --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-gitignore-generate`

Generate .gitignore from the projectfile

`pf-bridge ignore .gitignore --force`

> Image: PF_BRIDGE_IMAGE

### `pf-scan-git`

Import Git metadata into the projectfile

`pf-bridge scan git`

> Image: PF_BRIDGE_IMAGE

## Go

### `check-outdated-go`

List available Go module updates

`go list -u -m all`

> Image: GO_TOOL_IMAGE

### `go-fix`

Apply gofix rewrites to Go code

`go fix ./...`

> Image: GO_TOOL_IMAGE

### `go-fmt`

Format Go source with gofmt

`go fmt ./...`

> Image: GO_TOOL_IMAGE

### `go-test`

Run the Go test suite with coverage

`go test -coverprofile=coverage.out ./...`

> Image: GO_TOOL_IMAGE

### `go-tidy`

Prune and sync go.mod dependencies

`go mod tidy`

> Image: GO_TOOL_IMAGE

### `go-vet`

Report suspicious Go constructs

`go vet ./...`

> Image: GO_TOOL_IMAGE

### `golangci-fmt`

Format Go code via golangci-lint

`auto-golangci-lint-fmt`

> Image: D9T_GO_TOOLS_IMAGE

### `golangci-lint`

Run aggregated Go linters

`auto-golangci-lint`

> Image: D9T_GO_TOOLS_IMAGE

### `gosec`

Scan Go code for security issues

`auto-gosec`

> Image: D9T_GO_TOOLS_IMAGE

### `gsa`

Analyze Go binary size (go-size-analyzer)

`auto-gsa ${org.projectfile.artifacts.go-binary.path}`

> Image: D9T_GO_TOOLS_IMAGE

## Help

### `help`

Show categorized help for all targets

> Source: core/meta/help.mk

### `m6e-commit-docs`

Commit Markdown documentation

> Source: core/meta/help.mk

### `m6e-generate-docs`

Generate Markdown documentation

> Source: core/meta/help.mk

## License

### `pf-bridge-license-check`

Verify LICENSE still matches the projectfile

`pf-bridge license --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-license-generate`

Generate LICENSE from the projectfile

`pf-bridge license --force`

> Image: PF_BRIDGE_IMAGE

### `reuse-annotate`

`reuse annotate`

> Image: D9T_PYTHON_TOOLS_IMAGE

### `reuse-download`

`reuse download`

> Image: D9T_PYTHON_TOOLS_IMAGE

### `reuse-lint`

`reuse lint`

> Image: D9T_PYTHON_TOOLS_IMAGE

### `reuse-spdx`

`reuse spdx`

> Image: D9T_PYTHON_TOOLS_IMAGE

## Lint

### `checkov`

Scan IaC for security misconfigurations

`auto-checkov`

> Image: D9T_PYTHON_TOOLS_IMAGE

### `html-validate`

Validate HTML markup

`auto-html-validate`

> Image: D9T_JS_TOOLS_IMAGE

### `ignorelint-check`

Check ignore files for issues

`ignorelint`

> Image: D9T_IGNORELINT_IMAGE

### `ignorelint-fix`

Autofix ignore-file issues

`ignorelint --fix`

> Image: D9T_IGNORELINT_IMAGE

### `jsonlint`

Lint JSON files

`auto-jsonlint`

> Image: D9T_JS_TOOLS_IMAGE

### `linkinator`

Check for broken links

`auto-linkinator`

> Image: D9T_JS_TOOLS_IMAGE

### `ls-lint`

Enforce file and directory naming conventions

`auto-ls-lint`

> Image: D9T_GO_TOOLS_IMAGE

### `scc`

Count lines of code and complexity

`auto-scc`

> Image: D9T_GO_TOOLS_IMAGE

### `shellcheck`

Lint shell scripts for bugs and pitfalls

`auto-shellcheck`

> Image: D9T_MISC_TOOLS_IMAGE

### `yamllint`

Lint YAML for syntax and style

`auto-yamllint`

> Image: D9T_PYTHON_TOOLS_IMAGE

## Maintenance

### `clean`

Tear down compose stacks for ALL variants (sweeps `cleaned`)

> Source: core/workflow/ci.mk

### `deep-clean`

Tear down stacks+volumes, remove images + fetch cache, ALL variants

> Source: core/workflow/ci.mk

## Meta

### `m6e-commit`

Commit Makefile submodules

> Source: core/meta/m6e.mk

### `m6e-update`

Update Makefile submodules

> Source: core/meta/m6e.mk

### `m6e-update-and-commit`

Update and commit Makefile submodules

> *Internal target*
> Source: core/meta/m6e.mk

## Metadata

### `cffr-validate`

`auto-cffr-validate`

> Image: D9T_R_TOOLS_IMAGE

### `pf-bridge-citation-cff-check`

Verify CITATION.cff and the projectfile agree

`pf-bridge cff CITATION.cff --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-citation-cff-sync`

Sync CITATION.cff with the projectfile

`pf-bridge cff CITATION.cff`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-code-of-conduct-md-check`

Verify CODE_OF_CONDUCT.md still matches the projectfile

`pf-bridge coc --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-code-of-conduct-md-generate`

Generate CODE_OF_CONDUCT.md from the projectfile

`pf-bridge coc --force`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-codeowners-check`

Verify CODEOWNERS and the projectfile agree

`pf-bridge codeowners --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-codeowners-sync`

Sync CODEOWNERS with the projectfile

`pf-bridge codeowners`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-contributing-md-check`

Verify CONTRIBUTING.md still matches the projectfile

`pf-bridge contributing --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-contributing-md-generate`

Generate CONTRIBUTING.md from the projectfile

`pf-bridge contributing --force`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-support-md-check`

Verify SUPPORT.md still matches the projectfile

`pf-bridge support --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-support-md-generate`

Generate SUPPORT.md from the projectfile

`pf-bridge support --force`

> Image: PF_BRIDGE_IMAGE

## Projectfile

### `pf-bridge-claudeignore-check`

Verify .claudeignore still matches the projectfile

`pf-bridge ignore .claudeignore --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-claudeignore-generate`

Generate .claudeignore from the projectfile

`pf-bridge ignore .claudeignore --force`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-fragments-check`

Verify FEATURES.md/ROADMAP.md still match the docs/*.d fragments

`pf-bridge fragments --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-fragments-generate`

Assemble FEATURES.md/ROADMAP.md from docs/*.d fragments

`pf-bridge fragments`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-fragments-refresh`

Re-read what the parent projects publish and update the inherited copies

`pf-bridge fragments --refresh`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-list`

List available projectfile bridges

`pf-bridge --list`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-readme-check`

Verify the committed readme still matches the projectfile

`pf-bridge readme --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-readme-generate`

Generate README.md from the projectfile

`pf-bridge readme --force`

> Image: PF_BRIDGE_IMAGE

### `pf-forge-list`

List forge metadata fields

`pf-bridge forge list`

> Image: PF_BRIDGE_IMAGE

### `pf-forge-push`

Push projectfile metadata to the forge

`pf-bridge forge push`

> Image: PF_BRIDGE_IMAGE

### `pf-optimize`

Optimize and rewrite the projectfile

`pf-cli optimize`

> Image: PF_CLI_IMAGE

### `pf-optimize-sorted`

Optimize the projectfile with sorted keys

`pf-cli optimize --sorted`

> Image: PF_CLI_IMAGE

### `pf-scan`

Scan the project and update the projectfile

`pf-bridge scan all`

> Image: PF_BRIDGE_IMAGE

### `pf-scan-stacks`

Detect the tech stack into the projectfile

`pf-bridge scan stacks`

> Image: PF_BRIDGE_IMAGE

### `pf-validate`

Validate the projectfile document

`pf-cli validate`

> Image: PF_CLI_IMAGE

## Security

### `grype-db-update`

Update the grype vulnerability database

`grype db update`

> Image: D9T_GO_TOOLS_IMAGE

### `grype-scan-image`

Scan the live built image for vulnerabilities (grype)

`auto-grype image $(M6E_IMAGE_FULLNAME)`

> Image: D9T_GO_TOOLS_IMAGE

### `grype-scan-source`

Scan project source for vulnerabilities (grype)

`auto-grype`

> Image: D9T_GO_TOOLS_IMAGE

### `osv-scanner-db-update`

Mirror the offline OSV databases for the fleet’s ecosystems

`osv-db-mirror Go npm PyPI Packagist RubyGems crates.io Maven CRAN`

> Image: D9T_GO_TOOLS_IMAGE

### `osv-scanner-scan-source`

Scan dependencies against the OSV database

`auto-osv-scanner --offline-vulnerabilities`

> Image: D9T_GO_TOOLS_IMAGE

### `pf-bridge-grype-check`

Verify .grype.yaml still matches the projectfile

`pf-bridge vulnerabilities .grype.yaml --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-grype-generate`

Generate .grype.yaml from the projectfile

`pf-bridge vulnerabilities .grype.yaml --force`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-osv-check`

Verify osv-scanner.toml still matches the projectfile

`pf-bridge vulnerabilities osv-scanner.toml --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-osv-generate`

Generate osv-scanner.toml from the projectfile

`pf-bridge vulnerabilities osv-scanner.toml --force`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-security-md-check`

Verify SECURITY.md still matches the projectfile

`pf-bridge security --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-security-md-generate`

Generate SECURITY.md from the projectfile

`pf-bridge security --force`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-trivyignore-check`

Verify .trivyignore still matches the projectfile

`pf-bridge vulnerabilities .trivyignore --check`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-trivyignore-generate`

Generate .trivyignore from the projectfile

`pf-bridge vulnerabilities .trivyignore --force`

> Image: PF_BRIDGE_IMAGE

### `trivy-db-update`

Update the trivy vulnerability database

`auto-trivy db-update`

> Image: D9T_GO_TOOLS_IMAGE

### `trivy-scan-image`

Scan the live built image for vulnerabilities (trivy)

`auto-trivy image $(M6E_IMAGE_FULLNAME)`

> Image: D9T_GO_TOOLS_IMAGE

### `trivy-scan-source`

Scan project source for vulnerabilities (trivy)

`auto-trivy fs`

> Image: D9T_GO_TOOLS_IMAGE

## Workflow

### `all`

Default goal — redirects to M6E_ALL_GOAL (ci by default)

> Source: core/workflow/ci.mk

### `ci`

Run the pseudo-CI scenario (isolated CI resources; cleans up on exit)

> Source: core/workflow/ci.mk

## Pipeline Targets

Generated from `org.projectfile.ci.nodes` — each is invocable as `make <target>`.

### `analyze`

Run the heavy analysis sweep (mutation testing, benchmarks)

> Goal — lowered to its own CI workflow.

### `audited`

Re-scan the pinned dependencies and published artifacts for new vulnerabilities

> Goal — lowered to its own CI workflow.

### `check-outdated`

Report every pinned dependency that lags upstream

> Goal — lowered to its own CI workflow.

### `fragments-refreshed`

Update the inherited copies from what the parent projects publish

### `pre-commit`

Run the commit gate — the fast static checks

### `pre-push`

Run the push gate — every static check plus the vulnerability scans

### `projectfile-is-synced`

Verify every projectfile-derived file still matches the projectfile

### `projectfile-sync`

Regenerate every projectfile-derived file

### `published`

Build, test, scan and publish the release artifacts

> Goal — lowered to its own CI workflow.

### `ready-to-publish`

Run the pseudo-CI pipeline locally — build, test and scan, without publishing

### `static-passes`

Run every static check — lint, documentation, licence compliance and vulnerability scans

### `static-passes-quick`

Run the fast static checks — lint, documentation and licence compliance
