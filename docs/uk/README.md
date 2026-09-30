<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
pf-cli-managed: yes
-->

<!-- textlint-disable terminology,common-misspellings -->

[English](../../README.md) · [Español](../es/README.md)

# Projectfile Core Go Library

Центральна бібліотека Go, спільна для всіх інструментів projectfile.org: pf-cli, pf-bridge і pf-ci. Розбирає, розвʼязує та валідує projectfile-файли за вбудованою схемою v1, відкриваючи типізовану модель, на якій побудовано інструменти. Опублікована як Go-модуль для повторного використання в усьому флоті projectfile.

[![Stand with Ukraine](https://raw.githubusercontent.com/vshymanskyy/StandWithUkraine/main/badges/StandWithUkraine.svg)](https://damian-buho.github.io/support-ukraine/) [![Projectfile inside](https://badges.kiota.ch/static/v1?label=projectfile&message=inside&labelColor=0d0d0d&color=8c6723&style=flat-square)](https://projectfile.org) [![License](https://badges.kiota.ch/static/v1?label=license&message=MIT&color=1e5913&style=flat-square)](LICENSE) [![PRs welcome](https://badges.kiota.ch/static/v1?label=PRs&message=welcome&color=1e5913&style=flat-square)](CONTRIBUTING.md) [![REUSE compliance](https://api.reuse.software/badge/codeberg.org/projectfile/core)](https://api.reuse.software/info/codeberg.org/projectfile/core)

![Project status](https://badges.kiota.ch/static/v1?label=status&message=maintained&color=1d63ed&style=flat-square) [![Last commit on kiota.ch](https://badges.kiota.ch/gitea/last-commit/projectfile/core?gitea_url=https://kiota.ch&label=last%20commit%20on%20kiota.ch&style=flat-square)](https://kiota.ch/projectfile/core) [![Last commit on Codeberg](https://badges.kiota.ch/gitea/last-commit/projectfile/core?gitea_url=https://codeberg.org&label=last%20commit%20on%20Codeberg&style=flat-square)](https://codeberg.org/projectfile/core) [![Last commit on GitHub](https://badges.kiota.ch/github/last-commit/projectfile-org/core?label=last%20commit%20on%20GitHub&style=flat-square)](https://github.com/projectfile-org/core)

[![Publish pipeline on GitHub](https://github.com/projectfile-org/core/actions/workflows/published.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/core/actions) [![Vulnerability audit on GitHub](https://github.com/projectfile-org/core/actions/workflows/audited.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/core/actions) [![Dependency freshness on GitHub](https://github.com/projectfile-org/core/actions/workflows/check-outdated.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/core/actions) [![Analysis sweep on GitHub](https://github.com/projectfile-org/core/actions/workflows/analyze.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/core/actions)

[![Publish pipeline on kiota.ch](https://kiota.ch/projectfile/core/badges/workflows/published.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/core/actions) [![Vulnerability audit on kiota.ch](https://kiota.ch/projectfile/core/badges/workflows/audited.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/core/actions) [![Dependency freshness on kiota.ch](https://kiota.ch/projectfile/core/badges/workflows/check-outdated.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/core/actions) [![Analysis sweep on kiota.ch](https://kiota.ch/projectfile/core/badges/workflows/analyze.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/core/actions)

## Збирання

Клонуйте репозиторій разом із підмодулями:

```sh
git clone --recurse-submodules https://codeberg.org/projectfile/core core && cd core
```

- [Довідник із Makefile](../how-to/MAKEFILE.md)

Виконайте `make` без аргументів для типової цілі; виконайте `make help`, щоб переглянути всі цілі.

Точки входу конвеєра:

- `make analyze` — Запускає важкий аналіз (мутаційне тестування, бенчмарки)
- `make audited` — Повторно сканує закріплені залежності й опубліковані артефакти на нові вразливості
- `make check-outdated` — Звітує про кожну закріплену залежність, що відстає від upstream
- `make ready-to-publish` — Запускає псевдо-CI локально — збирає, тестує й сканує без публікації

## Політики

- [Як зробити внесок](CONTRIBUTING.md)
- [Політика безпеки](SECURITY.md)
- [Як отримати підтримку](SUPPORT.md)
- [Кодекс поведінки](CODE_OF_CONDUCT.md)
- [Політика щодо ШІ та LLM](AI_POLICY.md)

## Посилання

- [Специфікація Projectfile](https://projectfile.org)

## Ліцензія

Цей проєкт ліцензовано на умовах MIT — див. файл [LICENSE](LICENSE) для подробиць.

<!-- textlint-enable -->
