---
layout: default
title: Strata (Database)
nav_order: 7
---

# runtime-strata

Database exploration and administration. Strata connects to a database from a single connection string, reads its schema, and renders query results in a virtualized table.

## Supported backends

Strata is built around a driver registry: each backend registers a connection factory under a stable name, and the app connects purely by name + DSN. All five backends ship as real, driver-backed connectors.

| Backend | Schemes | Backing driver | Mechanism |
|---------|---------|----------------|-----------|
| PostgreSQL | `postgres://`, `postgresql://` | [`github.com/lib/pq`](https://github.com/lib/pq) | `database/sql` |
| MySQL | `mysql://` | [`github.com/go-sql-driver/mysql`](https://github.com/go-sql-driver/mysql) | `database/sql` |
| SQLite | `sqlite:`, `sqlite3:` | [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no cgo) | `database/sql` |
| MongoDB | `mongodb://` (and `mongodb+srv://`) | [`go.mongodb.org/mongo-driver/v2`](https://github.com/mongodb/mongo-go-driver) | native `DataSource` adapter |
| Cassandra | `cassandra:` | [`github.com/gocql/gocql`](https://github.com/gocql/gocql) | native `DataSource` adapter |

PostgreSQL and MongoDB receive the full connection URL (their drivers parse it directly). For MySQL, SQLite, and Cassandra the scheme is stripped and the bare DSN/URI/host list is handed to the driver.

## Launch

```sh
runtime-strata <connection-string>
```

The scheme selects the backend; the remainder is the driver's DSN. Examples:

```sh
runtime-strata sqlite:examples/strata/sample.db
runtime-strata sqlite::memory:
runtime-strata postgres://user:pass@localhost:5432/app
runtime-strata mysql://user:pass@tcp(127.0.0.1:3306)/app
runtime-strata mongodb://localhost:27017/app
runtime-strata cassandra:127.0.0.1
```

Launched with no connection string, Strata starts disconnected. A bad DSN, an unreachable server, or an unknown scheme is surfaced in the footer status line rather than crashing the app.

## Keymap

Universal bindings (see the [main keymap](../README.md#universal-keymap)) plus:

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j`, `←`/`h`, `→`/`l` | Navigate the result table |
| `pgup`/`ctrl+u`, `pgdown`/`ctrl+d` | Page up / down |
| `g` / `G` | Jump to top / bottom |
| `?` | Toggle help |
| `q` / `ctrl+c` | Quit |

## Features

- Pluggable driver registry — adding a backend is a single `Register` call, with no changes to the UI or model
- One connection-string grammar across every backend
- Schema summary line derived from the connected source via the shared `schema` package
- Virtualized result table via the shared `table` package

## Configuration

Strata loads its config on startup from the standard per-user location:

- Linux/macOS: `$XDG_CONFIG_HOME/runtime/strata/config.json` (fallback `~/.config/runtime/strata/config.json`)
- Windows: `%APPDATA%\runtime\strata\config.json`

The `theme` field selects the palette. A missing config is fine; a malformed config never prevents launch. See [docs/configuration.md](configuration.md).

## Limitations

- **SQLite works fully offline** (pure-Go driver, including `:memory:` databases). The bundled `examples/strata/sample.db` is the easiest way to try Strata.
- **PostgreSQL, MySQL, MongoDB, and Cassandra need a reachable server.** Without one, the connection attempt fails and the error is shown in the status line.
- The SQL backends are thin adapters over `database/sql`; MongoDB and Cassandra are not `database/sql`-compatible and use dedicated adapters over their native clients.
