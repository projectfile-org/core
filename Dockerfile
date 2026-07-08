# SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
#
# SPDX-License-Identifier: MIT

ARG B19_GO_BASE_IMAGE=registry.invalid/b19/go:latest
ARG B19_UBUNTU_BASE_IMAGE=registry.invalid/b19/ubuntu/resolute:latest
ARG B19_UBUNTU_SERIES=resolute

FROM ${B19_GO_BASE_IMAGE} AS projectfile-cli-builder

ARG B19_VERBOSITY
ARG LANG
ARG M6E_AI=N
ARG M6E_APT_CACHE_HOST
ARG M6E_APT_CACHE_PORT
ARG M6E_BUILD_DEBUG
ARG M6E_NAMESPACE
ARG M6E_NEAR_CACHE_HOST
ARG M6E_PROJECT
ARG M6E_VERSION
ARG SOURCE_DATE_EPOCH
ARG TARGETARCH

COPY --chown=${B19_UID}:${B19_GID} .container/compile-go/           /
COPY --chown=${B19_UID}:${B19_GID} main.go go.mod go.sum            ${B19_HOME}/
COPY --chown=${B19_UID}:${B19_GID} internal/                        ${B19_HOME}/internal/

USER root

WORKDIR ${B19_HOME}

RUN --mount=type=bind,from=fetch,source=.,target=/fetch                                           \
    --mount=type=cache,target=${B19_DOWNLOAD_PATH},sharing=shared                                 \
    --mount=type=cache,target=${GOCACHE},sharing=locked                                           \
    --mount=type=cache,target=${GOMODCACHE},sharing=locked                                        \
    --mount=type=cache,id=apt-cache-${B19_UBUNTU_SERIES},target=/var/cache/apt,sharing=shared     \
    --mount=type=cache,id=apt-lists-${B19_UBUNTU_SERIES},target=/var/lib/apt,sharing=shared       \
    --mount=type=tmpfs,target=${B19_TEMP_PATH}                                                    \
    build-stage compile-go

# hadolint DL3002
USER ${B19_UID}

FROM ${B19_UBUNTU_BASE_IMAGE} AS final

ARG B19_VERBOSITY
ARG LANG
ARG M6E_AI=N
ARG M6E_APT_CACHE_HOST
ARG M6E_APT_CACHE_PORT
ARG M6E_BUILD_DEBUG
ARG M6E_NAMESPACE
ARG M6E_NEAR_CACHE_HOST
ARG M6E_PROJECT
ARG M6E_VERSION
ARG SOURCE_DATE_EPOCH
ARG TARGETARCH

ENV M6E_VERSION=${M6E_VERSION}

COPY --chown=${B19_UID}:${B19_GID} .container/base/ /
COPY --from=projectfile-cli-builder /export /

USER root

WORKDIR ${B19_HOME}

RUN --mount=type=bind,from=fetch,source=.,target=/fetch                                           \
    --mount=type=cache,target=${B19_DOWNLOAD_PATH},sharing=shared                                 \
    --mount=type=cache,id=apt-cache-${B19_UBUNTU_SERIES},target=/var/cache/apt,sharing=shared     \
    --mount=type=cache,id=apt-lists-${B19_UBUNTU_SERIES},target=/var/lib/apt,sharing=shared       \
    --mount=type=tmpfs,target=${B19_TEMP_PATH}                                                    \
    build-stage base

USER ${B19_UID}

COPY --chown=${B19_UID}:${B19_GID} .container/user/ /

RUN --mount=type=bind,from=fetch,source=.,target=/fetch                                             \
    --mount=type=cache,target=${B19_DOWNLOAD_PATH},sharing=shared,uid=${B19_UID},gid=${B19_GID}     \
    --mount=type=tmpfs,target=${B19_TEMP_PATH}                                                      \
    build-stage user

# ENTRYPOINT ["entrypoint.d"] is inherited
# HEALTHCHECK CMD ["healthcheck.d"] is inherited
# CMD ["sleep", "infinity"]
