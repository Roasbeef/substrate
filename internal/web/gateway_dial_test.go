package web

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDialEndpoint asserts that a "localhost" host is rewritten to the IPv4
// loopback while every other form of address is passed through untouched. The
// rewrite keeps the gateway from being captured by an unrelated process bound
// to the IPv6 loopback or wildcard on the gRPC port, which accepts the TCP
// connection but never speaks gRPC.
func TestDialEndpoint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		addr string
		want string
	}{
		{
			name: "localhost rewritten to ipv4 loopback",
			addr: "localhost:10009",
			want: "127.0.0.1:10009",
		},
		{
			name: "ipv4 loopback left alone",
			addr: "127.0.0.1:10009",
			want: "127.0.0.1:10009",
		},
		{
			name: "explicit ipv6 loopback preserved",
			addr: "[::1]:10009",
			want: "[::1]:10009",
		},
		{
			name: "remote host preserved",
			addr: "grpc.internal:10009",
			want: "grpc.internal:10009",
		},
		{
			name: "wildcard host preserved",
			addr: ":10009",
			want: ":10009",
		},
		{
			name: "port only preserved on other port",
			addr: "localhost:8080",
			want: "127.0.0.1:8080",
		},
		{
			name: "malformed address passed through",
			addr: "localhost",
			want: "localhost",
		},
		{
			name: "empty address passed through",
			addr: "",
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, dialEndpoint(tc.addr))
		})
	}
}

// TestDialEndpointNeverResolvesToIPv6 is the invariant that actually matters:
// whatever dialEndpoint returns for a loopback endpoint must not resolve to an
// IPv6 address, since that is the resolution order bug being defended against.
func TestDialEndpointNeverResolvesToIPv6(t *testing.T) {
	t.Parallel()

	host, _, err := net.SplitHostPort(dialEndpoint("localhost:10009"))
	require.NoError(t, err)

	ip := net.ParseIP(host)
	require.NotNil(t, ip, "host must be a literal IP, not a name to resolve")
	require.NotNil(t, ip.To4(), "host must be an IPv4 address")
}
