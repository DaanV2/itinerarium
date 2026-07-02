package servers_test

import (
	"net/http"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/servers"
)

func TestOptionsApply(t *testing.T) {
	s := servers.New(
		servers.WithAddr(":9999"),
		servers.WithHandler(http.NewServeMux()),
	)
	if s.Addr() != ":9999" {
		t.Fatalf("expected :9999, got %s", s.Addr())
	}

	if servers.HandlerOf(s) == nil {
		t.Fatal("expected handler to be set")
	}

	if servers.ReadHeaderTimeoutOf(s).Seconds() != 10 {
		t.Fatalf("expected 10s read-header timeout, got %s", servers.ReadHeaderTimeoutOf(s))
	}
}
