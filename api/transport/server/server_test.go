package server_test

import (
	"net/http"
	"testing"

	"github.com/DaanV2/itinerarium/api/transport/server"
	"github.com/stretchr/testify/require"
)

func TestOptionsApply(t *testing.T) {
	s := server.New(
		server.WithAddr(":9999"),
		server.WithHandler(http.NewServeMux()),
	)
	require.Equal(t, ":9999", s.Addr())
	require.NotNil(t, server.Handler(s))
	require.InDelta(t, 10, server.ReadHeaderTimeout(s).Seconds(), 0)
}
