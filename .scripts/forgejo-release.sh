#!/bin/sh

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

set -eu

# =============================================================================
# forgejo-release.sh — attach one matrix binary to the Forgejo release
# =============================================================================
#
# Registers the CI token as a tea login, then creates the release tagged
# <version> with the projectfile-<goos>-<goarch> asset attached. Runs once per
# matrix axis; binaries-released fans the per-cell uploads into a release.
# =============================================================================

version="${1:?usage: forgejo-release.sh <version>}"
goos="${GOOS:?GOOS must be set}"
goarch="${GOARCH:?GOARCH must be set}"
asset="dist/projectfile-${goos}-${goarch}"

tea login add --name ci \
              --url "${GITHUB_SERVER_URL:?GITHUB_SERVER_URL must be set}" \
              --token "${FORGEJO_TOKEN:?FORGEJO_TOKEN must be set}"
tea release create --repo "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY must be set}" \
                   --tag "${version}" \
                   --asset "${asset}"
