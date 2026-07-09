# api/ — Go API server

Read the root `CLAUDE.md` first for domain rules. This file tells you exactly where code goes and gives a copyable template for every layer.

## Commands

Preferred: `just api-verify` from the repo root (build + vet + lint + test), `just api` to run the server, `just api-fmt` to format. Raw commands from `api/`:

```bash
go run . serve          # start the API on :8080
go run . init --email gm@example.com --password <password>  # CLI first-run bootstrap, alternative to the web setup wizard
go build ./...          # compile everything
go vet ./...            # static checks
go test ./...           # all tests
gofmt -w .              # format (run before finishing)
golangci-lint run ./... # lint — CI fails on any issue (install: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest)
```

CI additionally runs `go test -race ./...` on Linux. The race detector needs cgo, so it may not run on a Windows machine without gcc — plain `go test ./...` locally is fine.

```bash
```

## Layer rules — where code goes

Request flow: `transport` → `application` → `repositories` → `models`. Never skip a layer, never import upward.

| Layer | Directory | Owns | Never contains |
|-------|-----------|------|----------------|
| Transport | `infrastructure/transport/` | Routes, request decode, response encode, auth middleware | Business logic, GORM queries |
| Services | `application/` | Business logic, **all permission rules** (game-day gating, GM-only stripping, existence hiding) | HTTP types, GORM queries |
| Repositories | `infrastructure/persistence/repositories/` | All GORM queries, one file per entity | Permission decisions, HTTP types |
| Models | `infrastructure/persistence/models/` | GORM structs + tags | Query methods, logic |

Permission checks live in **services**, not repositories and not handlers. A repository returns what it is asked for; the service decides what the caller may see.

## Recipe: adding an entity end-to-end

Follow these six steps in order. Each has an in-repo example to copy.

### 1. Model — `infrastructure/persistence/models/<entity>.go`

Embed `Model` (UUID, timestamps, soft delete — see `models/model.go`):

```go
type Character struct {
	Model
	Name           string `gorm:"not null" json:"name"`
	CurrentGameDay int    `gorm:"not null;default:0" json:"current_game_day"`
	UserID         string `gorm:"type:uuid;index;not null" json:"user_id"`
	User           User   `json:"-"`
}
```

Many-to-many uses a named junction table: `` gorm:"many2many:group_members" ``.

### 2. Register in `infrastructure/persistence/migrations.go`

Add `&models.Character{}` to `allModels()`. **Skipping this means no table is created.**

### 3. Repository — `infrastructure/persistence/repositories/<entities>.go`

One file per entity, plural filename. Struct around `*persistence.Database`, methods take `context.Context`:

```go
type Characters struct{ db *persistence.Database }

func NewCharacters(db *persistence.Database) *Characters { return &Characters{db: db} }

func (r *Characters) GetByID(ctx context.Context, id string) (*models.Character, error) {
	var c models.Character
	if err := r.db.DB().WithContext(ctx).First(&c, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Characters) Create(ctx context.Context, c *models.Character) error {
	return r.db.DB().WithContext(ctx).Create(c).Error
}
```

### 4. Service — `application/<entities>.go`

Constructor takes repositories; methods enforce the domain rules:

```go
type CharacterService struct {
	characters *repositories.Characters
}

func NewCharacterService(characters *repositories.Characters) *CharacterService {
	return &CharacterService{characters: characters}
}

// Get returns the character only if the requester owns it or is a GM —
// otherwise ErrNotFound, never ErrForbidden (existence must not leak).
func (s *CharacterService) Get(ctx context.Context, requester Requester, id string) (*models.Character, error) {
	c, err := s.characters.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !requester.IsGM() && c.UserID != requester.UserID() {
		return nil, ErrNotFound // hidden means invisible: not 403
	}
	return c, nil
}
```

### 5. Route — handler in `infrastructure/transport/`, wired in `components/router.go`

Handlers decode the request, call one service method, encode the response:

```go
// transport/characters.go
func GetCharacterHandler(svc *application.CharacterService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := svc.Get(r.Context(), requesterFrom(r), r.PathValue("id"))
		// ... map error, write JSON
	})
}
```

Wire it into `CreateRouter` in `components/router.go` with the existing options pattern (services come off the `*Services` bundle):

```go
transport.WithHandle("GET /api/characters/{id}", requireAuth(transport.GetCharacterHandler(services.Characters))),
```

If the handler needs a new service or repository, add it to the `Services` / `Repositories` bundle in `components/services.go` / `components/repositories.go` — that is the composition root.

Route patterns use Go 1.22+ `http.ServeMux` syntax: `"METHOD /path/{param}"`, read params with `r.PathValue("param")`.

### 6. Tests — alongside the code, `_test.go`

- Repositories/services: real in-memory DB via `persistence.New(persistence.WithInMemory())` + `db.Migrate()` — no mocks.
- Handlers: `httptest.NewRecorder()` against a `transport.NewRouter(...)` (see `transport/router_test.go`).
- **Every permission rule needs a negative test**: prove the wrong character gets `404`/empty — not just that the right one succeeds. A feature touching visibility is not done without these.

## Conventions

- **Functional options** for every constructor that has configuration: `New(opts ...Option)` — see `servers/server.go`, `persistence/database.go`.
- **Config**: read via `config.GetContext("component")`; bind command flags with `config.MustBindFlags` (see `cmd/serve.go`). Key `server.database-path` = flag `--database-path` = env `SERVER_DATABASE_PATH` = YAML `server.database-path`.
- **Shutdown**: anything holding resources implements a `lifecycle` interface (`Shutdown(ctx) error` usually) and is joined by `lifecycle.ShutdownAll` in `ServerComponents.Shutdown` (`components/build.go`).
- **Composition root**: `components/` wires config → database → auth → repositories → services → router → server. `BuildServer(ctx)` returns a `*ServerComponents`; commands (`cmd/serve.go`, `cmd/init.go`) stay thin and reuse the smaller builders (`SetupDatabase`, `NewRepositories`, `SetupAuthentication`, `NewServices`).
- **SQLite driver**: `github.com/glebarez/sqlite` (pure Go, no cgo, FTS5-capable). Do not switch to `gorm.io/driver/sqlite` — it needs cgo and breaks the static Docker build.
- **Errors**: wrap with `fmt.Errorf("doing thing: %w", err)`; sentinel errors (`ErrNotFound`) live in the service layer.
- **Logging**: `github.com/charmbracelet/log` (`log.Default()`, `logger.Info("msg", "key", value)`). The linter rejects the stdlib `log` package (and `log/slog` — same import prefix). `infrastructure/logging` configures the global logger from the `log` config component (`--level`/`LOG_LEVEL`, `--format`/`LOG_FORMAT` — text/json/logfmt, `--report-caller`/`LOG_REPORT_CALLER`) and carries request-scoped loggers through `context.Context` via `logging.Context`/`logging.From` — request handlers get one via `logging.From(r.Context())` after `transport.Logging` middleware has run.

## Lint rules that trip people up (`.golangci.yml`, enforced in CI)

- **`nlreturn`**: a `return` needs a blank line above it unless it's the only statement in the block.
- **`testpackage`**: tests go in an external package (`package servers_test`, not `package servers`). To reach unexported internals, add accessors in an `export_test.go` inside the package (see `servers/export_test.go`).
- **`noctx`**: build test requests with `httptest.NewRequestWithContext(t.Context(), ...)`, never `httptest.NewRequest`; use `http.NoBody` instead of `nil` bodies.
- **`cyclop`**: max complexity 20 per function, package average 10 — split big functions into phase helpers (see `lifecycle/lifecycle.go`).
- **`gosec`**: directory permissions ≤ `0o750`, file permissions ≤ `0o600` for anything sensitive (keys!).
- **`depguard`**: no `log`/`log/slog` (use charmbracelet), no `github.com/pkg/errors` (stdlib `errors`), no `gorilla/websocket` (use `coder/websocket`).

When the linter flags something, fix the code — never add `//nolint` without a specific linter and an explanation (the config requires both).

## Definition of done for an API feature

1. `gofmt -w .`, `go vet ./...`, `golangci-lint run ./...`, `go test ./...` all clean
2. New models registered in `migrations.go`
3. Permission rules enforced in the service layer **with negative tests**
4. Anything GM-only stripped server-side (grep the response path, not the client)
5. Roadmap checkbox ticked in `docs/roadmap.md`; `docs/architecture.md` updated if the domain model changed
