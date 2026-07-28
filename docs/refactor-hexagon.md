# Refactor: restructured `api/` layout

Status: completed (branch `refactor/structure-and-tests`).

## What changed

Two groups of files moved:

### Handlers → `api/v1/{domain}/`

`api/handlers/` was a flat package mixing every endpoint. Each file moved into its
own domain sub-package under `api/api/v1/`:

| Old path | New path |
|---|---|
| `handlers/activity.go` | `api/v1/activities/` |
| `handlers/auth.go` | `api/v1/authenication/` |
| `handlers/characters.go` | `api/v1/characters/` |
| `handlers/catalog.go` | `api/v1/currencies/` |
| `handlers/inventory.go` | `api/v1/inventory/` |
| `handlers/documents.go`, `journal_entries.go`, `repositories.go`, `search.go`, `vault_import.go` | `api/v1/knowledge/` |
| `handlers/locations.go` | `api/v1/locations/` |
| `handlers/groups.go`, `sessions.go` | `api/v1/sessions/` |
| `handlers/setup.go` | `api/v1/setup/` |
| `handlers/users.go` | `api/v1/users/` |

Each sub-package gets its own Go package name (e.g. `package charactersv1`).
`components/router.go` imports each domain package directly.

### Transport → `infrastructure/transport/`

`api/transport/` (and `api/transport/server/`) moved into the `infrastructure/`
namespace:

| Old path | New path |
|---|---|
| `transport/*.go` | `infrastructure/transport/` |
| `transport/server/` | `infrastructure/transport/server/` |

The package names and exported symbols are unchanged; only the import paths
changed.

## Layer rules after the refactor

Request flow: `api/v1/{domain}` → `application` → `repositories` → `models`.

| Layer | Directory |
|---|---|
| Handlers | `api/v1/{domain}/` — one package per domain |
| Transport (mechanism) | `infrastructure/transport/` |
| Services | `application/` |
| Repositories | `infrastructure/persistence/repositories/` |
| Models | `infrastructure/persistence/models/` |
| Domain rules | `domain/` |
