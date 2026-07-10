# Deployment

## Quick Start

```bash
docker compose up -d
```

On first start a setup wizard creates the initial GM account. For headless deployments, the same bootstrap can be run from the command line instead of the web wizard:

```bash
docker compose exec api /app/itinerarium init --email gm@example.com --password <password>
```

Fails once any account already exists, same as the web wizard.

## Configuration

Configuration is loaded in priority order: CLI flags → environment variables → `config/config.yaml` (or any path via `--config`) → defaults. See `config/config.example.yaml` for a copyable file.

| Key | Description | Default |
|-----|-------------|---------|
| `server.address` | Address the API listens on | `:8080` |
| `server.database-type` | Database backend: `sqlite`, `memory`, `postgres`, `mysql` | `sqlite` |
| `server.database-dsn` | Connection string for postgres/mysql; overrides `database-path` for sqlite | _(unset)_ |
| `server.database-path` | Path to the SQLite file (sqlite backend) | `data/itinerarium.db` |
| `server.database-max-idle-conns` | Max idle connections in the pool | `2` |
| `server.database-max-open-conns` | Max open connections (`0` = unlimited) | `0` |
| `server.database-conn-max-lifetime` | Max time a connection may be reused | `1h` |
| `server.keys-path` | Directory for RS512 JWT signing keys (auto-generated on first start) | `data/keys` |
| `server.token-ttl` | Access token lifetime | `15m` |
| `server.catalog-path` | Optional JSON/YAML file seeding the currency & item catalog on startup (see [currency-configuration.md](currency-configuration.md)) | _(unset)_ |

Environment variables are derived automatically from key names: `server.database-path` → `SERVER_DATABASE_PATH`. The frontend's port is not a config key — it's set directly on the `web` service in `docker-compose.yml` (`3000`).

## Database backends

SQLite (the default) keeps the "one file, no dependencies" self-hosting story. Larger installs can point at PostgreSQL or MySQL instead — the schema and migrations are identical across backends (pure-Go drivers, no cgo).

**SQLite (default)** — the file lives at `server.database-path`:

```yaml
server:
  database-type: sqlite
  database-path: /data/itinerarium.db
```

**PostgreSQL** — set a [DSN](https://pkg.go.dev/github.com/jackc/pgx/v5#hdr-Establishing_a_Connection):

```yaml
server:
  database-type: postgres
  database-dsn: "host=localhost user=itinerarium password=secret dbname=itinerarium port=5432 sslmode=disable"
```

**MySQL** — set a [DSN](https://github.com/go-sql-driver/mysql#dsn-data-source-name):

```yaml
server:
  database-type: mysql
  database-dsn: "itinerarium:secret@tcp(localhost:3306)/itinerarium?charset=utf8mb4&parseTime=True&loc=Local"
```

`memory` is an ephemeral SQLite used for tests — all data is lost on shutdown. Postgres and MySQL require `database-dsn`; starting without one is a configuration error.

## Upgrading

State lives in the configured database. With the default SQLite backend it is a single file (`server.database-path`); with postgres/mysql it is your managed server. Upgrading the app is:

```bash
docker compose pull
docker compose up -d
```

Back up the database before upgrading in production.
