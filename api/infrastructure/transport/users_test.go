package transport_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/repositories"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/stretchr/testify/require"
)

type usersTestEnv struct {
	router      *transport.Router
	gmToken     string
	playerToken string
}

func newUsersTestEnv(t *testing.T) usersTestEnv {
	t.Helper()

	db, err := persistence.New(persistence.WithInMemory())
	require.NoError(t, err, "persistence.New")
	require.NoError(t, db.Migrate(), "Migrate")

	keys, err := authentication.NewKeyStore(authentication.WithKeysDir(t.TempDir()))
	require.NoError(t, err, "NewKeyStore")

	tokens := authentication.NewTokenService(keys, repositories.NewRevokedTokens(db))
	users := repositories.NewUsers(db)
	authSvc := application.NewAuthService(tokens, users)
	userSvc := application.NewUserService(users)
	requireAuth := transport.RequireAuth(authSvc)

	ctx := t.Context()

	gm := &models.User{Email: "gm@example.com", PasswordHash: "hash", Role: models.RoleGM}
	require.NoError(t, users.Create(ctx, gm), "Create gm")

	player := &models.User{Email: "player@example.com", PasswordHash: "hash", Role: models.RolePlayer}
	require.NoError(t, users.Create(ctx, player), "Create player")

	gmToken, err := tokens.Issue(gm.ID)
	require.NoError(t, err, "Issue(gm)")

	playerToken, err := tokens.Issue(player.ID)
	require.NoError(t, err, "Issue(player)")

	router := transport.NewRouter(
		transport.WithHandle("GET /api/admin/users", requireAuth(transport.ListAccountsHandler(userSvc))),
		transport.WithHandle("POST /api/admin/users", requireAuth(transport.CreateAccountHandler(userSvc))),
		transport.WithHandle(
			"POST /api/admin/users/{id}/reset-password",
			requireAuth(transport.ResetPasswordHandler(userSvc)),
		),
	)

	return usersTestEnv{router: router, gmToken: gmToken, playerToken: playerToken}
}

func (e usersTestEnv) doJSON(t *testing.T, method, path, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	if payload != nil {
		require.NoError(t, json.NewEncoder(&body).Encode(payload), "encoding request")
	}

	req := httptest.NewRequestWithContext(t.Context(), method, path, &body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)

	return rec
}

func TestCreateAccount_RequiresAuth(t *testing.T) {
	env := newUsersTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/admin/users", "", map[string]string{
		"email": "new@example.com", "role": "player",
	})

	require.Equal(t, http.StatusUnauthorized, rec.Code, "body: %s", rec.Body.String())
}

func TestCreateAccount_RejectsPlayer(t *testing.T) {
	env := newUsersTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/admin/users", env.playerToken, map[string]string{
		"email": "new@example.com", "role": "player",
	})

	require.Equal(t, http.StatusForbidden, rec.Code, "player caller body: %s", rec.Body.String())
}

func TestCreateAccount_GMCreatesAccount(t *testing.T) {
	env := newUsersTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/admin/users", env.gmToken, map[string]string{
		"email": "new-player@example.com", "role": "player",
	})

	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var created struct {
		ID                string `json:"id"`
		Email             string `json:"email"`
		Role              string `json:"role"`
		TemporaryPassword string `json:"temporary_password"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created), "decoding body")
	require.Equal(t, "player", created.Role)
	require.NotEmpty(t, created.TemporaryPassword, "expected a non-empty temporary password")
}

func TestListAccounts_RejectsPlayer(t *testing.T) {
	env := newUsersTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/admin/users", env.playerToken, nil)

	require.Equal(t, http.StatusForbidden, rec.Code, "player caller body: %s", rec.Body.String())
}

func TestListAccounts_GM(t *testing.T) {
	env := newUsersTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/admin/users", env.gmToken, nil)

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var accounts []struct {
		Email string `json:"email"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &accounts), "decoding body")
	require.Len(t, accounts, 2, "want gm + player")
}

func TestResetPassword_RejectsPlayer(t *testing.T) {
	env := newUsersTestEnv(t)

	rec := env.doJSON(t, http.MethodGet, "/api/admin/users", env.gmToken, nil)

	var accounts []struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &accounts), "decoding body")

	targetID := accounts[0].ID

	resetRec := env.doJSON(t, http.MethodPost, "/api/admin/users/"+targetID+"/reset-password", env.playerToken, nil)
	require.Equal(t, http.StatusForbidden, resetRec.Code, "player caller body: %s", resetRec.Body.String())
}

func TestResetPassword_GM(t *testing.T) {
	env := newUsersTestEnv(t)

	listRec := env.doJSON(t, http.MethodGet, "/api/admin/users", env.gmToken, nil)

	var accounts []struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &accounts), "decoding body")

	var targetID string
	for _, a := range accounts {
		if a.Email == "player@example.com" {
			targetID = a.ID
		}
	}

	require.NotEmpty(t, targetID, "could not find player account in list")

	rec := env.doJSON(t, http.MethodPost, "/api/admin/users/"+targetID+"/reset-password", env.gmToken, nil)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var body struct {
		TemporaryPassword string `json:"temporary_password"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "decoding body")
	require.NotEmpty(t, body.TemporaryPassword, "expected a non-empty temporary password")
}

func TestResetPassword_UnknownAccount(t *testing.T) {
	env := newUsersTestEnv(t)

	rec := env.doJSON(t, http.MethodPost, "/api/admin/users/does-not-exist/reset-password", env.gmToken, nil)
	require.Equal(t, http.StatusNotFound, rec.Code, "body: %s", rec.Body.String())
}
