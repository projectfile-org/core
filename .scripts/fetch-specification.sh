#!/bin/sh

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

# Clones or fast-forwards projectfile/specification into _specification/ for the conformance tests.
set -eu

REPO="${PROJECTFILE_SPECIFICATION_REPO:-https://kiota.ch/projectfile/specification.git}"
REF="${PROJECTFILE_SPECIFICATION_REF:-main}"
DEST="_specification"

log() { printf '[fetch-specification] %s\n' "$*" >&2; }

# A hook-exported GIT_DIR would make every git -C below act on this repository.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_PREFIX GIT_COMMON_DIR

assert_dest_is_repo_root() {
    _got=$(git -C "${DEST}" rev-parse --absolute-git-dir 2>/dev/null || true)
    _want=$(cd "${DEST}" && pwd -P)/.git
    if [ "${_got}" != "${_want}" ]; then
        log "ERROR: Git resolves ${DEST} to '${_got:-none}', want '${_want}'"
        return 1
    fi
    return 0
}

if [ -d "${DEST}/.git" ]; then
    log "existing clone at ${DEST}, syncing to ref=${REF} repo=${REPO}"
    assert_dest_is_repo_root || exit 1
    git -C "${DEST}" remote set-url origin "${REPO}"
    git -C "${DEST}" fetch --quiet --force origin "${REF}"
    git -C "${DEST}" reset --quiet --hard "origin/${REF}"
elif [ -d "${DEST}" ]; then
    log "ERROR: ${DEST} exists but is not a git clone; refusing to overwrite"
    exit 1
else
    log "cloning repo=${REPO} ref=${REF} into ${DEST}"
    git clone --quiet --no-tags --branch "${REF}" "${REPO}" "${DEST}"
fi

set -- "${DEST}"/spec/conformance/interpolation/*.yaml
[ -e "$1" ] || shift
log "ok HEAD=$(git -C "${DEST}" rev-parse --short HEAD) ref=${REF} vectors=$#"
