<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
pf-cli-managed: yes
-->

<!-- textlint-disable terminology -->

[English](README.md) · [Українська](docs/uk/README.md)

# projectfile/core

Core Go library for projectfile.org tools

[![Stand with Ukraine](https://raw.githubusercontent.com/vshymanskyy/StandWithUkraine/main/badges/StandWithUkraine.svg)](https://damian-buho.github.io/support-ukraine/) [![License](https://img.shields.io/static/v1?label=license&message=MIT&color=4c1&style=flat-square)](LICENSE) ![Commit style](https://img.shields.io/static/v1?label=commits&message=conventional&color=blue&style=flat-square) ![Workflow](https://img.shields.io/static/v1?label=workflow&message=git-flow&color=blue&style=flat-square) ![Versioning](https://img.shields.io/static/v1?label=versioning&message=semantic&color=blue&style=flat-square) [![PRs welcome](https://img.shields.io/static/v1?label=PRs&message=welcome&color=4c1&style=flat-square)](CONTRIBUTING.md) [![Citation](https://img.shields.io/static/v1?label=citation&message=cff&color=blue&style=flat-square)](CITATION.cff) [![REUSE compliance](https://api.reuse.software/badge/codeberg.org/projectfile/core)](https://api.reuse.software/info/codeberg.org/projectfile/core)

![Project status](https://img.shields.io/static/v1?label=status&message=maintained&color=1d63ed&style=flat-square) [![Last commit](https://img.shields.io/gitea/last-commit/projectfile/core?gitea_url=https://codeberg.org&style=flat-square)](https://codeberg.org/projectfile/core)

[![Build status on kiota.ch](https://kiota.ch/projectfile/core/badges/workflows/published.yaml/badge.svg)](https://kiota.ch/projectfile/core/actions)

## Compilación

- [Referencia del Makefile](docs/MAKEFILE.md)

Puntos de entrada de la canalización:

- `make analyze` — Run the heavy analysis sweep (mutation testing, benchmarks)
- `make audited` — Re-scan the pinned dependencies and published artifacts for new vulnerabilities
- `make check-outdated` — Report every pinned dependency that lags upstream
- `make ready-to-publish` — Run the pseudo-CI pipeline locally — build, test and scan, without publishing

Ejecuta `make` sin argumentos para el destino predeterminado; ejecuta `make help` para listar todos los destinos.

## Documentación

- [Conformance and field-coverage report](docs/CONFORMANCE.md)
- [Per-field consumer implementation plan](docs/CONSUMER-PLAN.md)

## Políticas

- [Cómo contribuir](docs/es/CONTRIBUTING.md)
- [Política de seguridad](docs/es/SECURITY.md)
- [Cómo obtener ayuda](docs/es/SUPPORT.md)
- [Código de conducta](docs/es/CODE_OF_CONDUCT.md)

## Enlaces

### Proyecto

- [especificación de projectfile](https://projectfile.org)
- [projectfile/core on Codeberg](https://codeberg.org/projectfile/core)
- [projectfile/core on GitHub](https://github.com/damian-buho/projectfile-core)
- [projectfile/core on kiota.ch](https://kiota.ch/projectfile/core)
- [Issues on Codeberg](https://codeberg.org/projectfile/core/issues)
- [Issues on GitHub](https://github.com/damian-buho/projectfile-core/issues)

### Otros

- [Projectfile Specification](projectfile.org)

## Licencia

Este proyecto se publica bajo la licencia MIT — consulta el archivo [LICENSE](LICENSE) para más detalles.

<!-- textlint-enable -->
