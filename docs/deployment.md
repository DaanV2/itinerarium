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
| `server.database-path` | Path to SQLite file | `data/itinerarium.db` |
| `server.keys-path` | Directory for RS512 JWT signing keys (auto-generated on first start) | `data/keys` |
| `server.token-ttl` | Access token lifetime | `15m` |

Environment variables are derived automatically from key names: `server.database-path` → `SERVER_DATABASE_PATH`. The frontend's port is not a config key — it's set directly on the `web` service in `docker-compose.yml` (`3000`).

## Upgrading

Because all state lives in a single SQLite file (`server.database-path`), upgrading is:

```bash
docker compose pull
docker compose up -d
```

Back up the SQLite file before upgrading in production.
