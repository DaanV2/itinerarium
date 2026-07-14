# Deployment

Itinerarium deploys as **one container running one binary**: the SvelteKit frontend is compiled to a static SPA and embedded into the Go server via `go:embed`, so the same process serves the web UI and the API on `:8080`.

## Quick Start

```bash
docker compose up -d
```

Open http://localhost:8080. On first start a setup wizard creates the initial GM account. For headless deployments, the same bootstrap can be run from the command line instead of the web wizard:

```bash
docker compose exec itinerarium /app/itinerarium init --email gm@example.com --password <password>
```

Fails once any account already exists, same as the web wizard.

## Without Docker: a single binary

`just build` produces a self-contained executable at `api/itinerarium` (`.exe` on Windows): it builds the frontend into `api/infrastructure/webapp/dist`, then compiles the server with the `embedweb` build tag so the site is baked in. Copy the binary anywhere and run `itinerarium serve` — no Node, no cgo, no other files needed (the SQLite database and JWT keys are created next to it under `data/`). A build *without* the tag (plain `go build`, what dev and CI use) serves the API only.

## Configuration

Configuration is loaded in priority order: CLI flags → environment variables → `config/config.yaml` (or any path via `--config`) → defaults. See `config/config.example.yaml` for a copyable file.

| Key | Description | Default |
|-----|-------------|---------|
| `server.address` | Address the API listens on | `:8080` |
| `database.type` | Database backend: `sqlite`, `memory`, `postgres`, `mysql` | `sqlite` |
| `database.dsn` | Connection string for postgres/mysql; overrides `database.path` for sqlite | _(unset)_ |
| `database.path` | Path to the SQLite file (sqlite backend) | `data/itinerarium.db` |
| `database.max-idle-conns` | Max idle connections in the pool | `2` |
| `database.max-open-conns` | Max open connections (`0` = unlimited) | `0` |
| `database.conn-max-lifetime` | Max time a connection may be reused | `1h` |
| `auth.keys-path` | Directory for RS512 JWT signing keys (auto-generated on first start) | `data/keys` |
| `auth.token-ttl` | Access token lifetime | `15m` |
| `catalog.path` | Optional JSON/YAML file seeding the currency & item catalog on startup (see [currency-configuration.md](currency-configuration.md)) | _(unset)_ |
| `log.level` | Log level: `debug`, `info`, `warn`, `error`, `fatal` | `info` |
| `log.format` | Log format: `text`, `json`, `logfmt` | `text` |

Each key doubles as its CLI flag and env var: `database.path` = `--database.path` = `DATABASE_PATH`. There is no separate frontend port — the embedded web UI is served on `server.address` alongside the API.

## Database backends

SQLite (the default) keeps the "one file, no dependencies" self-hosting story. Larger installs can point at PostgreSQL or MySQL instead — the schema and migrations are identical across backends (pure-Go drivers, no cgo).

**SQLite (default)** — the file lives at `database.path`:

```yaml
database:
  type: sqlite
  path: /data/itinerarium.db
```

**PostgreSQL** — set a [DSN](https://pkg.go.dev/github.com/jackc/pgx/v5#hdr-Establishing_a_Connection):

```yaml
database:
  type: postgres
  dsn: "host=localhost user=itinerarium password=secret dbname=itinerarium port=5432 sslmode=disable"
```

**MySQL** — set a [DSN](https://github.com/go-sql-driver/mysql#dsn-data-source-name):

```yaml
server:
  type: mysql
  dsn: "itinerarium:secret@tcp(localhost:3306)/itinerarium?charset=utf8mb4&parseTime=True&loc=Local"
```

`memory` is an ephemeral SQLite used for tests — all data is lost on shutdown. Postgres and MySQL require `database.dsn`; starting without one is a configuration error.

## Upgrading

State lives in the configured database. With the default SQLite backend it is a single file (`database.path`); with postgres/mysql it is your managed server. Upgrading the app is:

```bash
docker compose pull
docker compose up -d
```

Back up the database before upgrading in production.
