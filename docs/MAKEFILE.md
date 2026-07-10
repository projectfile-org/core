<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->
# Makefile Targets

## Bootstrap

### `m6e-cache-init`

Create host cache directories declared by loaded stacks

> Source: core/base/035-cache.mk

### `preflight`

Run all preflight probes (host/tooling readiness)

> Source: core/base/080-preflight.mk

### `preflight-registries`

Verify referenced tool-image registry vars are configured (not a *.invalid sentinel)

> Source: core/base/115-registries.mk

### `bootstrap`

First-run actions

> Source: core/workflow/bootstrap.mk

### `custom-bootstrap-script`

Run bootstrap.sh if it exists

> Source: core/workflow/bootstrap.mk

## CI

### `act`

Run every generated GHA workflow via act (offline smoke test; per-goal: make act-<goal>)

> Source: core/ci/act.mk

### `ci-generate`

Regenerate committed GHA + Forgejo workflows from org.projectfile.ci

> Source: core/ci/generate.mk

### `ci-check`

Verify committed workflows match org.projectfile.ci (drift => non-zero)

> Source: core/ci/generate.mk

### `install-hooks`

Sync Git hooks (lefthook) to the committed config (dev machines only)

> Source: core/ci/generate.mk

### `ci-dag`

Run the projectfile CI DAG — its goals (or all sinks); continues past failures

> Source: core/ci/select.mk

### `matrix-sweep`

Pipeline sweep — the DAG once (no matrix) or per product cell (once→cell→join)

> Source: core/workflow/ci.mk

## Help

### `help`

Show categorized help for all targets

> Source: core/meta/help.mk

### `m6e-generate-docs`

Generate Markdown documentation

> Source: core/meta/help.mk

### `m6e-commit-docs`

Commit Markdown documentation

> Source: core/meta/help.mk

## Meta

### `m6e-update`

Update Makefile submodules

> Source: core/meta/m6e.mk

### `m6e-commit`

Commit Makefile submodules

> Source: core/meta/m6e.mk

### `m6e-update-and-commit`

Update and commit Makefile submodules

> *Internal target*
> Source: core/meta/m6e.mk

## Git

### `git-submodules-pull-all`

Pull all Git submodules

> Source: core/tools/git.mk

## Workflow

### `all`

Default goal — redirects to M6E_ALL_GOAL (ci by default)

> Source: core/workflow/ci.mk

### `ci`

Run the pseudo-CI scenario (isolated CI resources; cleans up on exit)

> Source: core/workflow/ci.mk

## Maintenance

### `clean`

Tear down compose stacks for ALL variants (sweeps `cleaned`)

> Source: core/workflow/ci.mk

### `deep-clean`

Tear down stacks+volumes, remove images + fetch cache, ALL variants

> Source: core/workflow/ci.mk

## Analyze

### `gremlins`

Run mutation testing (requires gremlins binary)

> Source: project/50-gremlins.mk

## Manifest Commands

Generated from `org.projectfile.ci.tools` — each is invocable as `make <command>`.

### `alex`

`auto-alex`

> Image: D9T_JS_TOOLS_IMAGE

### `cffr-validate`

`auto-cffr-validate`

> Image: D9T_R_TOOLS_IMAGE

### `checkov`

Scan IaC for security misconfigurations

`auto-checkov`

> Image: D9T_PYTHON_TOOLS_IMAGE

### `fetch-spdx`

`.scripts/download-spdx.sh`

> Image: host runner

### `go-fix`

Apply gofix rewrites to Go code

`go fix ./...`

> Image: GO_TOOL_IMAGE

### `go-fmt`

Format Go source with gofmt

`go fmt ./...`

> Image: GO_TOOL_IMAGE

### `golangci-fmt`

Format Go code via golangci-lint

`auto-golangci-lint-fmt`

> Image: D9T_GO_TOOLS_IMAGE

### `golangci-lint`

Run aggregated Go linters

`auto-golangci-lint`

> Image: D9T_GO_TOOLS_IMAGE

### `go-mod-list`

List available Go module updates

`go list -u -m all`

> Image: GO_TOOL_IMAGE

### `gosec`

Scan Go code for security issues

`auto-gosec`

> Image: D9T_GO_TOOLS_IMAGE

### `go-test`

Run the Go test suite

`go test ./...`

> Image: GO_TOOL_IMAGE

### `go-tidy`

Prune and sync go.mod dependencies

`go mod tidy`

> Image: GO_TOOL_IMAGE

### `go-vet`

Report suspicious Go constructs

`go vet ./...`

> Image: GO_TOOL_IMAGE

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

### `gsa`

Analyze Go binary size (go-size-analyzer)

`auto-gsa ${org.projectfile.artifacts.go-binary.path}`

> Image: D9T_GO_TOOLS_IMAGE

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

### `markdownlint`

`auto-markdownlint`

> Image: D9T_JS_TOOLS_IMAGE

### `markdownlint-fix`

`auto-markdownlint --fix`

> Image: D9T_JS_TOOLS_IMAGE

### `mdformat`

`auto-mdformat`

> Image: D9T_PYTHON_TOOLS_IMAGE

### `osv-scanner-db-update`

Mirror the offline OSV databases for the fleet’s ecosystems

`osv-db-mirror Go npm PyPI Packagist RubyGems crates.io Maven CRAN`

> Image: D9T_GO_TOOLS_IMAGE

### `osv-scanner-scan-source`

Scan dependencies against the OSV database

`auto-osv-scanner --offline-vulnerabilities`

> Image: D9T_GO_TOOLS_IMAGE

### `pf-bridge-browserslistrc`

Generate .browserslistrc from the projectfile

`pf-bridge browserslist`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-citation-cff`

`pf-bridge cff CITATION.cff`

> Image: PF_CLI_IMAGE

### `pf-bridge-code-of-conduct-md`

Generate CODE_OF_CONDUCT.md from the projectfile

`pf-bridge coc`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-codeowners`

Sync CODEOWNERS from the projectfile

`pf-bridge codeowners`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-composer-json`

Sync composer.json from the projectfile

`pf-bridge composer`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-contributing-md`

Generate CONTRIBUTING.md from the projectfile

`pf-bridge contributing --force`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-dockerignore`

Generate .dockerignore from the projectfile

`pf-bridge ignore .dockerignore`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-funding-json`

Generate funding.json from the projectfile

`pf-bridge fundingjson`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-funding-yml`

Generate FUNDING.yml from the projectfile

`pf-bridge funding`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-gitignore`

Generate .gitignore from the projectfile

`pf-bridge ignore .gitignore`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-grype`

Generate .grype.yaml from the projectfile

`pf-bridge vulnerabilities .grype.yaml`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-license`

Generate LICENSE from the projectfile

`pf-bridge license --force`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-list`

List available projectfile bridges

`pf-bridge --list`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-npmignore`

Generate .npmignore from the projectfile

`pf-bridge ignore .npmignore`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-osv`

Generate osv-scanner.toml from the projectfile

`pf-bridge vulnerabilities osv-scanner.toml`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-package-json`

Sync package.json from the projectfile

`pf-bridge npm`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-pyproject-toml`

Sync pyproject.toml from the projectfile

`pf-bridge pyproject`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-security-md`

Generate SECURITY.md from the projectfile

`pf-bridge security`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-support-md`

Generate SUPPORT.md from the projectfile

`pf-bridge support --force`

> Image: PF_BRIDGE_IMAGE

### `pf-bridge-trivyignore`

Generate .trivyignore from the projectfile

`pf-bridge vulnerabilities .trivyignore`

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

### `pf-scan-git`

Import Git metadata into the projectfile

`pf-bridge scan git`

> Image: PF_BRIDGE_IMAGE

### `pf-scan-stacks`

Detect the tech stack into the projectfile

`pf-bridge scan stacks`

> Image: PF_BRIDGE_IMAGE

### `pf-validate`

Validate the projectfile document

`pf-cli validate`

> Image: PF_CLI_IMAGE

### `proselint`

`auto-proselint`

> Image: D9T_PYTHON_TOOLS_IMAGE

### `scc`

Count lines of code and complexity

`auto-scc`

> Image: D9T_GO_TOOLS_IMAGE

### `shellcheck`

Lint shell scripts for bugs and pitfalls

`auto-shellcheck`

> Image: D9T_MISC_TOOLS_IMAGE

### `textlint`

`auto-textlint`

> Image: D9T_JS_TOOLS_IMAGE

### `textlint-fix`

`auto-textlint --fix`

> Image: D9T_JS_TOOLS_IMAGE

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

### `yamllint`

Lint YAML for syntax and style

`auto-yamllint`

> Image: D9T_PYTHON_TOOLS_IMAGE
