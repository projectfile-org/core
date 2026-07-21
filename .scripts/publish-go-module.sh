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
# Go semantic-import-versioning: a module at major >=2 carries the major in its
# path (…/core/v2). Derive the suffix from the tag so this never hardcodes a
# major again; v0/v1 stay unsuffixed.
major="${version#v}"; major="${major%%.*}"
case "${major}" in
    0 | 1) suffix="" ;;
    *)     suffix="/v${major}" ;;
esac
module="kiota.ch/projectfile/core${suffix}"
upload_url="https://kiota.ch/api/packages/projectfile/go/upload"

: "${FORGEJO_TOKEN:?FORGEJO_TOKEN must be set}"

log() { printf '[publish-go-module] %s\n' "$*" >&2; }

# 1) Build the canonical zip from the pushed tag via direct VCS.
#    Clear go's VCS cache for this module first: GOPROXY=direct keeps a bare
#    clone under GOMODCACHE/cache/vcs and trusts its refs within a freshness
#    window. A stale clone (left over from a prior tag) makes the new tag an
#    "unknown revision" — go never refetches. Removing the clone forces a clean
#    fetch, so the just-pushed tag is always visible.
log "building module zip for ${module}@${version} via direct VCS"
vcs_root="$(go env GOMODCACHE)/cache/vcs"
if [ -d "${vcs_root}" ]; then
    # go hashes the repo URL to a 64-hex dir name; match by the remote it holds.
    for clone in "${vcs_root}"/*; do
        [ -d "${clone}" ] || continue
        if git -C "${clone}" config --get remote.origin.url 2>/dev/null | grep -q "projectfile/core.git$"; then
            rm -rf "${clone}"
            log "cleared stale VCS cache clone ${clone}"
        fi
    done
fi
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
