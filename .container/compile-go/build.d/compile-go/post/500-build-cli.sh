#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

  set -euo pipefail

  # shellcheck source=/dev/null
  . b19-i18n

  LDFLAGS="-s -w -X projectfile.org/projectfile/cli/internal/cmd.version=${M6E_VERSION:-dev}"

  b19-run "CLI" "$(_ 'Download modules')" --      \
    go mod download

  b19-run "CLI" "$(_ 'Build pf-cli')" --      \
    go build -ldflags="${LDFLAGS}" -o pf-cli .

  b19-strip "CLI" pf-cli

  b19-run "CLI" "$(_ 'Copy binary to export')" --     \
    mkdir -p /export/usr/local/bin

  b19-run "CLI" "$(_p 'Copy %s to %s' "pf-cli" "/export/usr/local/bin")" --     \
    cp pf-cli /export/usr/local/bin
