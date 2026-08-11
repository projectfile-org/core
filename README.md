<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
pf-cli-managed: yes
-->

[Español](docs/es/README.md) · [Українська](docs/uk/README.md)

# projectfile/core

Core Go library for projectfile.org tools

[![Stand with Ukraine](https://raw.githubusercontent.com/vshymanskyy/StandWithUkraine/main/badges/StandWithUkraine.svg)](https://damian-buho.github.io/support-ukraine/) [![License](https://img.shields.io/static/v1?label=license&message=MIT&color=4c1&style=flat-square)](LICENSE) ![Commit style](https://img.shields.io/static/v1?label=commits&message=conventional&color=blue&style=flat-square) ![Workflow](https://img.shields.io/static/v1?label=workflow&message=git-flow&color=blue&style=flat-square) ![Versioning](https://img.shields.io/static/v1?label=versioning&message=semantic&color=blue&style=flat-square) [![PRs welcome](https://img.shields.io/static/v1?label=PRs&message=welcome&color=4c1&style=flat-square)](CONTRIBUTING.md) [![Citation](https://img.shields.io/static/v1?label=citation&message=cff&color=blue&style=flat-square)](CITATION.cff) [![REUSE compliance](https://api.reuse.software/badge/codeberg.org/projectfile/core)](https://api.reuse.software/info/codeberg.org/projectfile/core)

![Project status](https://img.shields.io/static/v1?label=status&message=maintained&color=1d63ed&style=flat-square) [![Last commit](https://img.shields.io/gitea/last-commit/projectfile/core?gitea_url=https://codeberg.org&style=flat-square)](https://codeberg.org/projectfile/core)

[![Build status on kiota.ch](https://kiota.ch/projectfile/core/badges/workflows/published.yaml/badge.svg)](https://kiota.ch/projectfile/core/actions)

## Building

- [Makefile reference](docs/MAKEFILE.md)

Pipeline entry points:

- `make analyze` — Run the heavy analysis sweep (mutation testing, benchmarks)
- `make audited` — Re-scan the pinned dependencies and published artifacts for new vulnerabilities
- `make check-outdated` — Report every pinned dependency that lags upstream
- `make ready-to-publish` — Run the pseudo-CI pipeline locally — build, test and scan, without publishing

Run `make` with no arguments for the default target; run `make help` to list every target.

## Documentation

- [Conformance and field-coverage report](docs/CONFORMANCE.md)
- [Per-field consumer implementation plan](docs/CONSUMER-PLAN.md)

## Policies

- [How to contribute](CONTRIBUTING.md)
- [Security policy](SECURITY.md)
- [Getting support](SUPPORT.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)

## Links

### Project

- [projectfile specification](https://projectfile.org)
- [projectfile/core on Codeberg](https://codeberg.org/projectfile/core)
- [projectfile/core on GitHub](https://github.com/damian-buho/projectfile-core)
- [projectfile/core on kiota.ch](https://kiota.ch/projectfile/core)
- [Issues on Codeberg](https://codeberg.org/projectfile/core/issues)
- [Issues on GitHub](https://github.com/damian-buho/projectfile-core/issues)

### Other

- [Projectfile Specification](projectfile.org)

## License

This project is licensed under MIT — see the [LICENSE](LICENSE) file for details.
