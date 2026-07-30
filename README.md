<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
-->

<!-- pf-cli-managed: yes -->
# projectfile/core

Core Go library for projectfile.org tools

[![License](https://img.shields.io/badge/license-MIT-4c1?style=flat-square)](LICENSE) ![Project status](https://img.shields.io/badge/status-maintained-1d63ed?style=flat-square) [![Last commit](https://img.shields.io/gitea/last-commit/projectfile/core?gitea_url=https://codeberg.org&style=flat-square)](https://codeberg.org/projectfile/core) [![PRs welcome](https://img.shields.io/badge/PRs-welcome-4c1?style=flat-square)](CONTRIBUTING.md) [![REUSE compliance](https://api.reuse.software/badge/codeberg.org/projectfile/core)](https://api.reuse.software/info/codeberg.org/projectfile/core)

## Building

- [Makefile reference](docs/MAKEFILE.md)

Pipeline entry points:

- `make analyze` — Run the heavy analysis sweep (mutation testing, benchmarks)
- `make audited` — Re-scan the pinned dependencies and published artifacts for new vulnerabilities
- `make check-outdated` — Report every pinned dependency that lags upstream
- `make published` — Build, test, scan and publish the release artifacts

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

- [projectfile/core on Codeberg](https://codeberg.org/projectfile/core)
- [projectfile/core on GitHub](https://github.com/damian-buho/projectfile-core)
- [projectfile/core on kiota.ch](https://kiota.ch/projectfile/core)
- [Issues on Codeberg](https://codeberg.org/projectfile/core/issues)
- [Issues on GitHub](https://github.com/damian-buho/projectfile-core/issues)

### Other

- [Projectfile Specification](projectfile.org)

## License

This project is licensed under MIT — see the [LICENSE](LICENSE) file for details.
