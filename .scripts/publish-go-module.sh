#!/bin/sh

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

set -eu

# =============================================================================
# publish-go-module.sh — publish core's module zip to the Forgejo Go registry
# =============================================================================
#
# Runs on a pushed semver tag (module-published node). Builds the canonical
# module zip for kiota.ch/projectfile/core@<tag> straight from VCS, then PUTs
# it to the Forgejo Go package registry so consumers resolve the module via the
# chained GOPROXY.
#
# The build MUST bypass the chained GOPROXY: at publish time the tag is not in
# the registry yet (that is what this does), and the r8e mirror 502s for
# kiota.ch without falling through to direct. GOPRIVATE forces direct VCS AND
# skips the checksum DB for this module.
# =============================================================================

version="${1:?usage: publish-go-module.sh <version>}"
module="kiota.ch/projectfile/core"
upload_url="https://kiota.ch/api/packages/projectfile/go/upload"

: "${FORGEJO_TOKEN:?FORGEJO_TOKEN must be set}"

log() { printf '[publish-go-module] %s\n' "$*" >&2; }

# 1) Build the canonical zip from the pushed tag via direct VCS.
log "building module zip for ${module}@${version} via direct VCS"
GOPRIVATE="${module}" GOPROXY=direct go mod download -x "${module}@${version}"

# 2) Locate the zip go placed in the module cache.
zip="$(go env GOMODCACHE)/cache/download/${module}/@v/${version}.zip"
[ -f "${zip}" ] || { log "expected zip not found: ${zip}"; exit 1; }
log "built zip ${zip} ($(wc -c < "${zip}") bytes)"

# 3) Upload to the Forgejo Go registry. curl retries transient failures with
#    exponential backoff; 409 (already published — re-run/race) is success.
log "uploading ${version}.zip to ${upload_url}"
status="$(curl --silent --show-error --output /dev/null             \
               --write-out '%{http_code}'                           \
               --header "Authorization: token ${FORGEJO_TOKEN}"     \
               --upload-file "${zip}"                               \
               --max-time 120                                       \
               --retry 5 --retry-delay 2 --retry-max-time 120       \
               --retry-all-errors                                   \
               "${upload_url}")"

case "${status}" in
    201) log "published ${module}@${version} (HTTP ${status})" ;;
    409) log "already published ${module}@${version} (HTTP ${status}) — ok" ;;
    *)   log "upload failed for ${module}@${version} (HTTP ${status})"; exit 1 ;;
esac
