package cmd_test

import (
	"path/filepath"
	"testing"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/cmd"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitBootstrapsFirstGMThenRefuses exercises the init command's wiring
// (SetupDatabase → repositories → authentication → services → CreateInitialGM)
// against a temp-file SQLite database that survives the two runs: the first
// bootstraps the initial GM, the second refuses because an account now exists.
func TestInitBootstrapsFirstGMThenRefuses(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATABASE_TYPE", "sqlite")
	t.Setenv("DATABASE_PATH", filepath.Join(dir, "itinerarium.db"))
	t.Setenv("AUTH_KEYS_PATH", dir)

	// First run bootstraps the initial GM account.
	err := cmd.RunInit(newInitCmd(t, "gm@example.com", "password123"), nil)
	require.NoError(t, err)

	// Second run refuses: setup already completed.
	err = cmd.RunInit(newInitCmd(t, "other@example.com", "password123"), nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, application.ErrAlreadySetUp)
}

// newInitCmd builds a throwaway command carrying just the email/password flags
// runInit reads, with the test's context attached.
func newInitCmd(t *testing.T, email, password string) *cobra.Command {
	t.Helper()

	c := &cobra.Command{}
	c.Flags().String("email", "", "")
	c.Flags().String("password", "", "")
	require.NoError(t, c.Flags().Set("email", email))
	require.NoError(t, c.Flags().Set("password", password))
	c.SetContext(t.Context())

	return c
}
