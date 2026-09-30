<!--
SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
SPDX-License-Identifier: MIT
pf-cli-managed: yes
-->

<!-- textlint-disable terminology,common-misspellings -->

[English](../../README.md) · [Українська](../uk/README.md)

# Projectfile Core Go Library

Biblioteca Go central compartida por todas las herramientas de projectfile.org: pf-cli, pf-bridge y pf-ci. Analiza, resuelve y valida projectfiles contra el esquema v1 embebido, exponiendo el modelo tipado sobre el que se construyen las herramientas. Publicada como módulo de Go para su reutilización en toda la flota de projectfile.

[![Stand with Ukraine](https://raw.githubusercontent.com/vshymanskyy/StandWithUkraine/main/badges/StandWithUkraine.svg)](https://damian-buho.github.io/support-ukraine/) [![Projectfile inside](https://badges.kiota.ch/static/v1?label=projectfile&message=inside&labelColor=0d0d0d&color=8c6723&style=flat-square)](https://projectfile.org) [![License](https://badges.kiota.ch/static/v1?label=license&message=MIT&color=1e5913&style=flat-square)](LICENSE) [![PRs welcome](https://badges.kiota.ch/static/v1?label=PRs&message=welcome&color=1e5913&style=flat-square)](CONTRIBUTING.md) [![REUSE compliance](https://api.reuse.software/badge/codeberg.org/projectfile/core)](https://api.reuse.software/info/codeberg.org/projectfile/core)

![Project status](https://badges.kiota.ch/static/v1?label=status&message=maintained&color=1d63ed&style=flat-square) [![Last commit on kiota.ch](https://badges.kiota.ch/gitea/last-commit/projectfile/core?gitea_url=https://kiota.ch&label=last%20commit%20on%20kiota.ch&style=flat-square)](https://kiota.ch/projectfile/core) [![Last commit on Codeberg](https://badges.kiota.ch/gitea/last-commit/projectfile/core?gitea_url=https://codeberg.org&label=last%20commit%20on%20Codeberg&style=flat-square)](https://codeberg.org/projectfile/core) [![Last commit on GitHub](https://badges.kiota.ch/github/last-commit/projectfile-org/core?label=last%20commit%20on%20GitHub&style=flat-square)](https://github.com/projectfile-org/core)

[![Publish pipeline on GitHub](https://github.com/projectfile-org/core/actions/workflows/published.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/core/actions) [![Vulnerability audit on GitHub](https://github.com/projectfile-org/core/actions/workflows/audited.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/core/actions) [![Dependency freshness on GitHub](https://github.com/projectfile-org/core/actions/workflows/check-outdated.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/core/actions) [![Analysis sweep on GitHub](https://github.com/projectfile-org/core/actions/workflows/analyze.yaml/badge.svg?style=flat-square)](https://github.com/projectfile-org/core/actions)

[![Publish pipeline on kiota.ch](https://kiota.ch/projectfile/core/badges/workflows/published.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/core/actions) [![Vulnerability audit on kiota.ch](https://kiota.ch/projectfile/core/badges/workflows/audited.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/core/actions) [![Dependency freshness on kiota.ch](https://kiota.ch/projectfile/core/badges/workflows/check-outdated.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/core/actions) [![Analysis sweep on kiota.ch](https://kiota.ch/projectfile/core/badges/workflows/analyze.yaml/badge.svg?style=flat-square)](https://kiota.ch/projectfile/core/actions)

## Compilación

Clona el repositorio con sus submódulos:

```sh
git clone --recurse-submodules https://codeberg.org/projectfile/core core && cd core
```

- [Referencia del Makefile](../how-to/MAKEFILE.md)

Ejecuta `make` sin argumentos para el destino predeterminado; ejecuta `make help` para listar todos los destinos.

Puntos de entrada de la canalización:

- `make analyze` — Ejecuta el análisis pesado (pruebas de mutación, benchmarks)
- `make audited` — Vuelve a escanear las dependencias fijadas y los artefactos publicados en busca de vulnerabilidades nuevas
- `make check-outdated` — Informa de cada dependencia fijada que va por detrás de su versión upstream
- `make ready-to-publish` — Ejecuta localmente el pipeline pseudo-CI — compila, prueba y escanea, sin publicar

## Políticas

- [Cómo contribuir](CONTRIBUTING.md)
- [Política de seguridad](SECURITY.md)
- [Cómo obtener ayuda](SUPPORT.md)
- [Código de conducta](CODE_OF_CONDUCT.md)
- [Política sobre IA y LLM](AI_POLICY.md)

## Enlaces

- [Especificación de Projectfile](https://projectfile.org)

## Licencia

Este proyecto se publica bajo la licencia MIT — consulta el archivo [LICENSE](LICENSE) para más detalles.

<!-- textlint-enable -->
