package servers_test

import (
	"net/http"
	"testing"

	"github.com/DaanV2/itinerarium/api/infrastructure/servers"
	"github.com/stretchr/testify/require"
)

func TestOptionsApply(t *testing.T) {
	s := servers.New(
		servers.WithAddr(":9999"),
		servers.WithHandler(http.NewServeMux()),
	)
	require.Equal(t, ":9999", s.Addr())
	require.NotNil(t, servers.Handler(s))
	require.InDelta(t, 10, servers.ReadHeaderTimeout(s).Seconds(), 0)
}
