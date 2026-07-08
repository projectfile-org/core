#!/bin/sh

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

set -eu

# =============================================================================
# build-binaries.sh — cross-compile projectfile for one GOOS/GOARCH matrix cell
# =============================================================================
#
# Embeds the SPDX license texts (via download-spdx.sh), injects the release
# version from the git tag, and writes the stripped binary to
# dist/projectfile-<goos>-<goarch>. Called once per matrix axis by the
# build-binaries CI tool.
# =============================================================================

version="${1:?usage: build-binaries.sh <version>}"
goos="${GOOS:?GOOS must be set}"
goarch="${GOARCH:?GOARCH must be set}"
out="dist/projectfile-${goos}-${goarch}"

"$(dirname "$0")/download-spdx.sh"

log() { printf '[build-binaries] %s\n' "$*" >&2; }
log "building projectfile ${version} for ${goos}/${goarch}"

go build -ldflags="-s -w -X kiota.ch/projectfile/core/internal/cmd.version=${version}" \
         -o "${out}" .
