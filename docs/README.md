---
layout: default
title: Welcome
nav_order: 1
permalink: /
---

# Runtime documentation

Documentation index for the Runtime monorepo. Start with the project [README](../README.md) for an overview and quick start.

## Guides

| Document | What's in it |
|----------|--------------|
| [architecture.md](architecture.md) | Monorepo layout, apps-vs-packages boundary, dependency direction, the Grid/Strata registry patterns, the plugin model, and the correctness-properties / PBT testing approach |
| [configuration.md](configuration.md) | The shared `BaseConfig` schema, per-OS config file locations, theme files, and live reload |
| [releasing.md](releasing.md) | Cutting releases with `scripts/version.sh` (individual vs coordinated), tag conventions, and what the release pipeline produces |

## Per-app guides

| App | Guide | Summary |
|-----|-------|---------|
| Grid | [grid.md](grid.md) | Tabular data workbench — CSV, TSV, XLSX, Parquet, Arrow |
| Prism | [prism.md](prism.md) | Structured document explorer — JSON, YAML, TOML, XML |
| Pulse | [pulse.md](pulse.md) | Log exploration — live tail, regex filter, error grouping |
| Strata | [strata.md](strata.md) | Database exploration — PostgreSQL, MySQL, SQLite, MongoDB, Cassandra |
| Vault | [vault.md](vault.md) | Secrets explorer — env, JWT, certificate, API key, k8s secret |

## Related

- [../CONTRIBUTING.md](../CONTRIBUTING.md) — setup, workspace, module rules, commit conventions, and CI gates
- [../examples/TESTING.md](../examples/TESTING.md) — exercising each app against the bundled sample data
- [../.github/workflows/README.md](../.github/workflows/README.md) — CI/CD pipeline reference
