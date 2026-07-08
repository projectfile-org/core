#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

  set -euo pipefail

  # shellcheck source=/dev/null
  . b19-i18n

  LDFLAGS="-s -w -X main.version=${M6E_VERSION:-dev}"

  b19-run "CITATION" "$(_ 'Download modules')" --     \
    go mod download

  b19-run "CITATION" "$(_ 'Build pf-citation')" --      \
    go build -ldflags="${LDFLAGS}" -o pf-citation .

  b19-strip "CITATION" pf-citation

  b19-run "CITATION" "$(_ 'Copy binary to export')" --      \
    mkdir -p /export/usr/local/bin

  b19-run "CITATION" "$(_p 'Copy %s to %s' "pf-citation" "/export/usr/local/bin")" --     \
    cp pf-citation /export/usr/local/bin
