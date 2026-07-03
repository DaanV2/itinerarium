package authentication_test

import (
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/authentication"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := authentication.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if !authentication.VerifyPassword(hash, "correct horse battery staple") {
		t.Fatal("expected the correct password to verify")
	}

	if authentication.VerifyPassword(hash, "wrong password") {
		t.Fatal("expected an incorrect password to fail verification")
	}
}
