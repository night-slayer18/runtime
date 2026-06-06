# Runtime examples

Sample data for exercising every app in the Runtime monorepo. Each subdirectory
holds inputs for one app; point the app at a file (or, for Strata, a connection
string) and explore.

For a step-by-step manual walkthrough of every app and sample, see
[TESTING.md](./TESTING.md).

## Security note — these are FAKE fixtures

> **Every secret-like value in `examples/` is synthetic and exists only to test
> the tools. None of them are real credentials.**
>
> - `vault/sample.env` — fake DSNs and demo tokens (e.g. `demo_user` /
>   `demo_password`).
> - `vault/token.jwt` — a structurally-valid but **unsigned/fake** HS256 JWT.
>   The signature segment is literally `base64("this-is-a-fake-signature-not-real")`.
> - `vault/cert.pem` — a self-signed certificate generated for a throwaway
>   `demo.example.com` key. Not trusted by anything.
> - `vault/apikeys.txt`, `vault/apikey.txt` — strings that match the *shape* of
>   provider keys (AWS, GitHub, Stripe, …) but are all zeros/`demo` filler.
> - `vault/secret.yaml`, `vault/secret.json` — Kubernetes Secret manifests whose
>   base64 `data` values decode to obvious placeholders, e.g.
>   `ZGVtby11c2Vy` == `base64("demo-user")`.
>
> Never replace these with real secrets. Never commit real credentials to the
> repo.

## What's in each subdirectory

### `grid/` — tabular data for runtime-grid
| File | Format | Static / Generated |
|------|--------|--------------------|
| `people.csv` | CSV (header + 12 rows) | static |
| `people.tsv` | TSV (same data, tab-delimited) | static |
| `sales.csv` | CSV (~360 rows) | generated (`gen_sales.py`) |
| `people.parquet` | Apache Parquet | **generated** (`_generators/grid`) |
| `people.arrow` | Arrow IPC / Feather v2 | **generated** (`_generators/grid`) |
| `people.xlsx` | Office Open XML spreadsheet | **generated** (`_generators/grid`) |

### `prism/` — structured documents for runtime-prism
| File | Notable features |
|------|------------------|
| `config.json` | nested objects, arrays, `null` |
| `config.yaml` | anchors/aliases (`&`/`*`/`<<`), literal (`|`) and folded (`>`) block scalars |
| `config.toml` | offset/local datetimes, multiline basic & literal strings, arrays of tables (`[[endpoints]]`), inline tables |
| `data.xml` | nested elements, attributes, repeated siblings |

All four are static.

### `pulse/` — logs for runtime-pulse
| File | Format | Notes |
|------|--------|-------|
| `app.log` | plain text, `TS LEVEL component: msg` | varied levels; repeated `db connection timeout`, `upstream … 503`, and `auth token validation failed` lines for grouping |
| `structured.log` | logfmt (`key=value`) | structured fields |
| `app.jsonl` | JSON lines | one JSON object per line |

All three are static.

### `strata/` — database for runtime-strata
| File | Format | Static / Generated |
|------|--------|--------------------|
| `sample.db` | SQLite 3 (tables: `departments`, `employees`, `sales`) | **generated** (`_generators/strata`) |

Connect with: `./bin/runtime-strata "sqlite:file:examples/strata/sample.db"`

### `vault/` — secret artifacts for runtime-vault
| File | Artifact kind |
|------|---------------|
| `sample.env` | dotenv file |
| `token.jwt` | JWT (fake HS256) |
| `cert.pem` | PEM X.509 certificate (self-signed) |
| `apikeys.txt` | reference catalog of fake API-key shapes (one per format) |
| `apikey.txt` | a single API key (GitHub-shaped) for a quick single-token run |
| `secret.yaml` | Kubernetes Secret manifest (YAML) |
| `secret.json` | Kubernetes Secret manifest (JSON) |

All are static. (Vault inspects **one** single-line token per run, so
`apikey.txt` is the convenient input; `apikeys.txt` is a documented catalog.)

## Regenerating the binary samples

The binary fixtures (Parquet, Arrow, XLSX, SQLite) are **not** hand-written.
They are produced by small Go programs under `_generators/`, which use the
**same libraries the apps read with** (`parquet-go`, `arrow-go`, the
`packages/export` XLSX writer, and `modernc.org/sqlite`). `sales.csv` is built
by a small Python script.

Run all of them from the repo root:

```sh
# Parquet + Arrow + XLSX  ->  examples/grid/
go run ./examples/_generators/grid

# SQLite                  ->  examples/strata/sample.db
go run ./examples/_generators/strata

# sales.csv (~360 rows)   ->  examples/grid/sales.csv
python3 examples/_generators/gen_sales.py
```

The generators are a self-contained module
(`examples/_generators/go.mod`) that is part of the workspace via
`go work use ./examples/_generators`. It builds without affecting the apps.

## Layout

```
examples/
├── README.md            (this file)
├── TESTING.md           (manual + automated testing guide)
├── _generators/         (generator programs — not sample data)
│   ├── go.mod
│   ├── gen_sales.py
│   ├── grid/main.go     (Parquet, Arrow, XLSX)
│   └── strata/main.go   (SQLite)
├── grid/                (csv, tsv, parquet, arrow, xlsx)
├── prism/               (json, yaml, toml, xml)
├── pulse/               (log, logfmt, jsonl)
├── strata/              (sample.db)
└── vault/               (env, jwt, pem, api keys, k8s secrets)
```
