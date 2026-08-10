<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
pf-cli-managed: yes
-->

<!-- textlint-disable terminology -->

[English](README.md) · [Español](docs/es/README.md)

# projectfile/core

Core Go library for projectfile.org tools

[![Stand with Ukraine](https://raw.githubusercontent.com/vshymanskyy/StandWithUkraine/main/badges/StandWithUkraine.svg)](https://damian-buho.github.io/support-ukraine/) [![License](https://img.shields.io/static/v1?label=license&message=MIT&color=4c1&style=flat-square)](LICENSE) ![Commit style](https://img.shields.io/static/v1?label=commits&message=conventional&color=blue&style=flat-square) ![Workflow](https://img.shields.io/static/v1?label=workflow&message=git-flow&color=blue&style=flat-square) ![Versioning](https://img.shields.io/static/v1?label=versioning&message=semantic&color=blue&style=flat-square) [![PRs welcome](https://img.shields.io/static/v1?label=PRs&message=welcome&color=4c1&style=flat-square)](CONTRIBUTING.md) [![Citation](https://img.shields.io/static/v1?label=citation&message=cff&color=blue&style=flat-square)](CITATION.cff) [![REUSE compliance](https://api.reuse.software/badge/codeberg.org/projectfile/core)](https://api.reuse.software/info/codeberg.org/projectfile/core)

![Project status](https://img.shields.io/static/v1?label=status&message=maintained&color=1d63ed&style=flat-square) [![Last commit](https://img.shields.io/gitea/last-commit/projectfile/core?gitea_url=https://codeberg.org&style=flat-square)](https://codeberg.org/projectfile/core)

[![Build status on kiota.ch](https://kiota.ch/projectfile/core/badges/workflows/published.yaml/badge.svg)](https://kiota.ch/projectfile/core/actions)

## Збирання

- [Довідник із Makefile](docs/MAKEFILE.md)

Точки входу конвеєра:

- `make analyze` — Run the heavy analysis sweep (mutation testing, benchmarks)
- `make audited` — Re-scan the pinned dependencies and published artifacts for new vulnerabilities
- `make check-outdated` — Report every pinned dependency that lags upstream

Виконайте `make` без аргументів для типової цілі; виконайте `make help`, щоб переглянути всі цілі.

## Документація

- [Conformance and field-coverage report](docs/CONFORMANCE.md)
- [Per-field consumer implementation plan](docs/CONSUMER-PLAN.md)

## Політики

- [Як зробити внесок](docs/uk/CONTRIBUTING.md)
- [Політика безпеки](docs/uk/SECURITY.md)
- [Як отримати підтримку](docs/uk/SUPPORT.md)
- [Кодекс поведінки](docs/uk/CODE_OF_CONDUCT.md)

## Посилання

### Проєкт

- [специфікація projectfile](https://projectfile.org)
- [projectfile/core on Codeberg](https://codeberg.org/projectfile/core)
- [projectfile/core on GitHub](https://github.com/damian-buho/projectfile-core)
- [projectfile/core on kiota.ch](https://kiota.ch/projectfile/core)
- [Issues on Codeberg](https://codeberg.org/projectfile/core/issues)
- [Issues on GitHub](https://github.com/damian-buho/projectfile-core/issues)

### Інше

- [Projectfile Specification](projectfile.org)

## Ліцензія

Цей проєкт ліцензовано на умовах MIT — див. файл [LICENSE](LICENSE) для подробиць.

<!-- textlint-enable -->
