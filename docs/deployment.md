# Deployment

## Quick Start

```bash
docker compose up -d
```

On first start a setup wizard creates the initial GM account.

## Configuration

Configuration is loaded in priority order: CLI flags → environment variables → `config/config.yaml` → defaults.

| Key | Description | Default |
|-----|-------------|---------|
| `server.port` | API port | `8080` |
| `db.path` | Path to SQLite file | `./data/itinerarium.db` |
| `auth.keys_dir` | Directory for RS512 JWT signing keys (auto-generated on first start) | `./data/keys` |
| `web.port` | Frontend port | `3000` |

Environment variables are derived automatically from key names: `server.port` → `SERVER_PORT`.

## Upgrading

Because all state lives in a single SQLite file (`db.path`), upgrading is:

```bash
docker compose pull
docker compose up -d
```

Back up the SQLite file before upgrading in production.
