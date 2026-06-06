# Runtime manual testing guide

A hands-on walkthrough for exercising every app in the monorepo against the
sample data in `examples/`. Pair this with [README.md](./README.md), which
describes what each fixture contains.

> All secret-like fixtures under `examples/vault/` are **synthetic**. See the
> security note in [README.md](./README.md#security-note--these-are-fake-fixtures).

## Prerequisites

- **Go 1.25.x** (the workspace declares `go 1.25.0`; a newer toolchain such as
  1.26 also works).
- **Python 3** (only needed to regenerate `examples/grid/sales.csv`).
- Build all five app binaries into `./bin/`:

  ```sh
  make build
  ```

  This produces `bin/runtime-grid`, `bin/runtime-prism`, `bin/runtime-pulse`,
  `bin/runtime-strata`, and `bin/runtime-vault`.

- (Optional) Regenerate the binary sample files. They are committed, so this is
  only needed if you change the generators:

  ```sh
  go run ./examples/_generators/grid      # people.parquet, people.arrow, people.xlsx
  go run ./examples/_generators/strata    # examples/strata/sample.db
  python3 examples/_generators/gen_sales.py   # examples/grid/sales.csv
  ```

Run all commands below from the repo root.

## Universal key bindings

Every app shares the canonical Runtime key map (`packages/tui`):

| Key | Action |
|-----|--------|
| `↑` / `k` | move up |
| `↓` / `j` | move down |
| `←` / `h` | move left (Prism: collapse) |
| `→` / `l` | move right (Prism: expand) |
| `pgup` / `ctrl+u` | page up |
| `pgdn` / `ctrl+d` | page down |
| `g` | jump to top |
| `G` | jump to bottom |
| `/` | search / filter |
| `?` | toggle help |
| `q` / `ctrl+c` | quit |

App-specific keys are called out per app below.

---

## Grid — `bin/runtime-grid <file>`

Grid imports tabular data and renders it as a navigable, searchable table.

### Launch commands

```sh
./bin/runtime-grid examples/grid/people.csv
./bin/runtime-grid examples/grid/people.tsv
./bin/runtime-grid examples/grid/sales.csv
./bin/runtime-grid examples/grid/people.parquet
./bin/runtime-grid examples/grid/people.arrow
./bin/runtime-grid examples/grid/people.xlsx
```

### What you should see

- A header row of column names (`id, name, email, department, salary,
  start_date, active` for the people files) and one row per record.
- The status line shows `loaded <path> (<n> rows)` — 12 rows for the `people.*`
  files, ~360 for `sales.csv`.
- All six files render the **same kind** of table; this confirms CSV, TSV,
  Parquet, Arrow, and XLSX all decode through Grid's own readers.

### Keys to try

- `↑↓` / `jk` and `←→` / `hl` to move the cursor cell-by-cell.
- `g` / `G` to jump to the first/last row; `pgup` / `pgdn` to page.
- `/` to open search, type a term (e.g. `Research` in `people.csv` or `EMEA` in
  `sales.csv`), then `Enter`. The status line reports `<n> rows match "<term>"`.
  Live filtering updates as you type; `Esc` clears the filter.
- `?` toggles help; `q` quits.

### Fail-closed behavior (Requirement 7.1)

Grid refuses to launch unless **all** required formats (CSV, TSV, XLSX, Parquet,
Arrow) report available — a fail-closed policy enforced by
`datasource.CheckAvailability()` in `cmd/grid/main.go`. To observe the two
branches of that policy:

1. **Normal launch** — every shipped format has a real decoder, so any of the
   commands above starts cleanly.
2. **Refusal path** — pass an unsupported extension to see Grid reject the input
   rather than starting half-functional:

   ```sh
   ./bin/runtime-grid examples/prism/config.json
   ```

   You should get a `runtime-grid: ...unknown file format...` error and a
   non-zero exit, never a partially-working UI.

   The fail-closed *launch* check (a required format failing to initialize) is
   covered directly by the unit tests in
   `apps/grid/internal/model/datasource/datasource_test.go`, which use
   `SetAvailability` to simulate an unavailable format and assert
   `CheckAvailability` returns an error.

---

## Prism — `bin/runtime-prism <file>`

Prism parses a structured document into a navigable tree.

### Launch commands

```sh
./bin/runtime-prism examples/prism/config.json
./bin/runtime-prism examples/prism/config.yaml
./bin/runtime-prism examples/prism/config.toml
./bin/runtime-prism examples/prism/data.xml
```

### What you should see

- A tree rooted at `root`, with the document's top-level keys as children.
- Leaf nodes render inline as `key: value`; container nodes (objects/arrays)
  expand to reveal children.
- Format-specific things worth confirming:
  - **YAML** (`config.yaml`): anchors/aliases are resolved — `deployments.api`
    and `deployments.worker` show the merged `cpu`/`memory`/`replicas` from the
    `&default_resources` anchor; `banner` (literal `|`) preserves line breaks and
    `description` (folded `>`) is joined into one paragraph.
  - **TOML** (`config.toml`): `deployed_at` shows as an RFC3339 datetime,
    `[[endpoints]]` appears as an array of three entries, and
    `max_request_bytes` reads as `1048576` (underscores stripped).
  - **XML** (`data.xml`): attributes appear as `@name: value` leaf children;
    repeated `<method>`/`<endpoint>` siblings are distinct nodes.

### Keys to try

- `↑↓` / `jk` to move between visible nodes.
- `→` / `l` to **expand** the selected node; `←` / `h` to **collapse** it (or
  move to the parent when on a leaf).
- `Enter` or `Space` to **toggle** expand/collapse on the selected node.
- `/` to search; `?` for help; `q` to quit.

### Testing search

With any document open, press `/` and search for a key or value (e.g. `replicas`
in `config.yaml`, or `inventory` is not present so try `endpoints`). Searching
navigates to the matching node and expands its ancestors so the match is
visible. Try a term that does not exist to confirm it is a no-op rather than an
error.

---

## Pulse — `bin/runtime-pulse <file>`

Pulse tails a log source, filters by pattern, and groups similar lines.

### Launch commands

```sh
./bin/runtime-pulse examples/pulse/app.log
./bin/runtime-pulse examples/pulse/structured.log
./bin/runtime-pulse examples/pulse/app.jsonl
```

### What you should see

- The log lines in order, with the newest at the bottom and a cursor you can
  move.
- The status line shows the current mode and `tab: toggle view`; when a filter
  is active it is prefixed with `filter:"<query>"`.

### Keys to try

- `↑↓` / `jk`, `g` / `G`, `pgup` / `pgdn` to navigate.
- `tab` toggles between **log view** (individual lines) and **group view**
  (similar lines collapsed into templates). In `app.log`, group view should
  surface the repeated patterns near the top by count:
  - `db: connection timeout to <HEX> after <NUM>ms` (×5)
  - `upstream: request to inventory-svc failed: <NUM> Service Unavailable` (×4)
  - `auth: token validation failed for user <NUM>: signature mismatch` (×3)
  (Numbers/hosts are normalized to `<NUM>`/`<HEX>` placeholders for grouping.)
- `/` to filter: type a substring (e.g. `ERROR` in `app.log`, or
  `level=error` in `structured.log`), `Enter` to commit, `Esc` to cancel.
- `?` for help; `q` to quit.

### Testing live tail

Pulse polls the file for appended lines. Open it in one terminal:

```sh
./bin/runtime-pulse examples/pulse/app.log
```

…and in a second terminal append lines:

```sh
echo "2024-05-17 09:03:00 ERROR db: connection timeout to db.internal.example.com after 5000ms" >> examples/pulse/app.log
echo "2024-05-17 09:03:01 INFO  request: GET /v2/search status=200 duration=40ms" >> examples/pulse/app.log
```

The new lines appear in Pulse without a restart. In group view, the repeated
`db connection timeout` group's count increments. (To reset the file afterward,
re-run `git checkout examples/pulse/app.log`.)

---

## Strata — `bin/runtime-strata <connection-string>`

Strata connects to a database and lets you browse tables and run queries. SQLite
is the pure-Go, offline backend (`modernc.org/sqlite`) — no server required.

### Launch command

```sh
./bin/runtime-strata "sqlite:file:examples/strata/sample.db"
```

The `sqlite:` scheme selects the SQLite backend; the remainder (`file:...`) is
the driver DSN. An in-memory database also works: `./bin/runtime-strata
"sqlite::memory:"` (but it starts empty).

### What you should see

- A successful connection to `sample.db`, which contains three tables:
  - `departments` (4 rows)
  - `employees` (10 rows)
  - `sales` (8 rows)
- Browse a table to see its columns and rows; the schema is derived from the
  result set of the focused table.

### Connect + query

Try queries against the seeded data, for example:

```sql
SELECT name, location FROM departments ORDER BY id;
SELECT e.name, d.name AS dept, e.salary
  FROM employees e JOIN departments d ON e.dept_id = d.id
  WHERE e.active = 1
  ORDER BY e.salary DESC;
SELECT region, SUM(net_total) AS revenue FROM sales GROUP BY region;
```

### Keys to try

- `↑↓` / `jk` to move through tables/rows; `/` to search; `?` for help; `q` to
  quit.

### Error path

Point Strata at an unknown scheme to confirm the clear error message:

```sh
./bin/runtime-strata "wat:file:nope.db"
```

You should get `runtime-strata: datasource: unknown scheme "wat" ...` listing
the supported schemes.

---

## Vault — `bin/runtime-vault <file>`

Vault classifies and inspects a secret artifact and reports **non-sensitive
metadata only** — values are always masked (`•••• (n chars)`).

### Launch commands (one per artifact type)

```sh
./bin/runtime-vault examples/vault/sample.env      # dotenv
./bin/runtime-vault examples/vault/token.jwt       # JWT
./bin/runtime-vault examples/vault/cert.pem        # X.509 certificate
./bin/runtime-vault examples/vault/apikey.txt      # single API key (GitHub-shaped)
./bin/runtime-vault examples/vault/secret.yaml     # k8s Secret (YAML)
./bin/runtime-vault examples/vault/secret.json     # k8s Secret (JSON)
```

### What you should see per artifact

- **`sample.env`** — detected as *env file*. Each key is listed with its value
  masked (e.g. `DATABASE_URL  ••••  (52 chars)`). No raw values are printed.
- **`token.jwt`** — detected as *JWT token*. Shows `alg: HS256`, `typ: JWT`, the
  `iss`/`sub`/`aud` claims, and `iat`/`nbf`/`exp` rendered as RFC3339 times. The
  signature is reported as a byte length only. The token's `exp` is far in the
  future, so it should report **valid** (no "expired" issue). The signature is
  fake — Vault inspects, it does **not** verify, by design.
- **`cert.pem`** — detected as *certificate*. Shows `subject`/`issuer`
  (`CN=demo.example.com`), serial, `not_before`/`not_after`, signature
  algorithm, `is_ca`, and any DNS names. It is self-signed and valid for ~10
  years, so no expiry issue.
- **`apikey.txt`** — detected as *API key*, format `GitHub Token`, with the
  length and a masked value. Swap in other lines from `apikeys.txt` (one token
  per file) to see other formats detected — `AKIA...` → AWS, `sk_test_...` →
  Stripe, the UUID → UUID Token, etc.
- **`secret.yaml` / `secret.json`** — detected as *kubernetes secret*. Shows the
  `name`, `type`, and each `data.<key>` / `stringData.<key>` with masked values.
  The base64 `data` values decode internally (e.g. `ZGVtby11c2Vy` → `demo-user`)
  but are never displayed.

### Keys to try

- `↑↓` / `jk` to scroll fields; `/` to search field names; `?` for help; `q` to
  quit.

### Inspecting the API-key catalog

`apikeys.txt` is a **reference catalog** — Vault inspects one single-line token
per run, so the multi-line catalog is for reading, not direct inspection. To
test a specific format, write one line to a temp file:

```sh
grep -m1 '^AKIA' examples/vault/apikeys.txt > /tmp/aws.key
./bin/runtime-vault /tmp/aws.key      # → format: AWS Access Key ID
```

---

## Running the automated tests

The repo root is not a Go module, so tests run **per-module** (see the
`Makefile`).

```sh
# Run every module's tests (apps + shared packages).
make test

# Compile-only smoke test across the whole workspace.
make build-smoke

# Test a single module.
cd apps/pulse && go test ./...
cd packages/export && go test ./...
```

### Where the property-based tests live

Property-based tests (Go's `testing/quick`-style and table-driven generative
tests) are colocated with the code they cover, in files named `*_property_test.go`:

- `apps/pulse/internal/model/streaming_property_test.go` — log streaming/tailing.
- `packages/export/export_property_test.go` — exporter round-trips.
- `packages/table/streaming_property_test.go`, `performance_property_test.go`.
- `packages/tui/tui_property_test.go` — universal keyboard navigation.
- `packages/validation/validation_property_test.go`.
- `packages/config/property_test.go`, `ecosystem_property_test.go`.
- `packages/theme/theme_property_test.go`, `ecosystem_property_test.go`.
- `packages/plugin/consistency_property_test.go`, `sandbox_property_test.go`.

They run as part of `make test` and each module's `go test ./...`.

### The generators module

`examples/_generators` is its own module, wired into the workspace via
`go work use ./examples/_generators`. It has no tests of its own; verify it with:

```sh
go build ./examples/_generators/...
```
