package authentication_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
	"github.com/stretchr/testify/require"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := authentication.HashPassword("correct horse battery staple")
	require.NoError(t, err, "HashPassword")

	require.True(t, authentication.VerifyPassword(hash, "correct horse battery staple"))
	require.False(t, authentication.VerifyPassword(hash, "wrong password"))
}
