# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

# M6E MAKEFILE T3

# Rules
all: .makefile/core/initialize.mk

.makefile/core/initialize.mk:
	git submodule update --init --recursive
	$(MAKE) bootstrap

# Includes — core is a pure library: only core + languages, no b19/container
# (no image build). Matches the m11s/shared-* library profile.
-include .makefile/core/initialize.mk
