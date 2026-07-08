# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

# =============================================================================
# Mutation testing via go-gremlins
# Install: go install github.com/go-gremlins/gremlins/cmd/gremlins@latest
# =============================================================================

#@ Analyze | Run mutation testing (requires gremlins binary)
gremlins:
	gremlins unleash ./...

ANALYZE_TARGETS += gremlins

.PHONY: gremlins
