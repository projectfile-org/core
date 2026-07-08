#!/bin/sh

# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

set -eu

# =============================================================================
# download-spdx.sh — SPDX license-text downloader
# =============================================================================
#
# Downloads SPDX boilerplate texts from the SPDX license-list-data
# repository.  Every call unconditionally re-fetches every id.
# Called by the fetch-spdx CI manifest tool and by build prerequisites.
# =============================================================================

embed_dir="internal/spdx/embedded"
base_url="https://raw.githubusercontent.com/spdx/license-list-data/main/text"

# BUSL-1.1 (Business Source License) is the canonical SPDX id; "BSL-1.1" 404s
# upstream. BSL-1.0 (Boost Software License) is a separate, unrelated licence.
ids="MIT Apache-2.0 GPL-3.0-or-later GPL-2.0-or-later LGPL-3.0-or-later \
      BSD-2-Clause BSD-3-Clause MPL-2.0 ISC AGPL-3.0-or-later             \
      Unlicense CC0-1.0 BSL-1.0 EUPL-1.2 EPL-2.0                          \
      CDDL-1.0 Zlib OFL-1.1 BUSL-1.1 WTFPL"

log() { printf '[download-spdx] %s\n' "$*" >&2; }

mkdir -p "${embed_dir}"

for id in ${ids}; do
    log "fetch ${id}"
    curl --fail --silent --show-error --max-time 15 \
         --output "${embed_dir}/${id}.txt" "${base_url}/${id}.txt"
done
