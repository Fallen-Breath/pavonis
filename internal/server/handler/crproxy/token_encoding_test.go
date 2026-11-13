package crproxy

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPackTokenRoundTrip(t *testing.T) {
	tests := []struct {
		name                  string
		user                  string
		password              string
		upstreamAuthorization string
	}{
		{name: "empty"},
		{name: "short", user: "a", password: "b", upstreamAuthorization: "c"},
		{name: "no upstream", user: "alice", password: "pw"},
		{name: "normal", user: "alice", password: "p$w:d", upstreamAuthorization: "Bearer upstream.jwt"},
		{name: "special characters", user: "a\"b\\c", password: "\x00\n\r\t :&<>/", upstreamAuthorization: "Basic dXNlcjpwYXNz"},
		{name: "unicode", user: "用户🙂", password: "密碼é🔑", upstreamAuthorization: "Bearer 上游令牌"},
		{name: "long", user: strings.Repeat("u", 1024), password: strings.Repeat("p", 1024), upstreamAuthorization: "Bearer " + strings.Repeat("x", 64*1024)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			token, ok := unpackToken(packToken(tt.user, tt.password, tt.upstreamAuthorization))
			require.True(t, ok)
			require.Equal(t, packedToken{
				Issuer:                packedTokenIssuer,
				User:                  tt.user,
				Password:              tt.password,
				UpstreamAuthorization: tt.upstreamAuthorization,
			}, *token)
		})
	}
}

func TestTokenBase62RoundTrip(t *testing.T) {
	for _, size := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 15, 16, 17, 31, 32, 33, 1024, 64 * 1024} {
		t.Run(fmt.Sprintf("%d bytes", size), func(t *testing.T) {
			mixed := make([]byte, size)
			for i := range mixed {
				mixed[i] = byte(i * 37)
			}
			for name, data := range map[string][]byte{
				"zeros": make([]byte, size),
				"ones":  bytes.Repeat([]byte{0xff}, size),
				"mixed": mixed,
			} {
				t.Run(name, func(t *testing.T) {
					decoded, ok := decodeTokenBase62(encodeTokenBase62(data))
					require.True(t, ok)
					require.Equal(t, data, decoded)
				})
			}
		})
	}
}
