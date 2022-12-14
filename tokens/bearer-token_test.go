package tokens

import (
	"encoding/base64"
	"testing"

	"github.com/TrueCloudLab/frostfs-sdk-go/bearer"
	"github.com/TrueCloudLab/frostfs-sdk-go/user"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

func makeTestCookie(value []byte) *fasthttp.RequestHeader {
	header := new(fasthttp.RequestHeader)
	header.SetCookie(bearerTokenHdr, string(value))
	return header
}

func makeTestHeader(value []byte) *fasthttp.RequestHeader {
	header := new(fasthttp.RequestHeader)
	if value != nil {
		header.Set(fasthttp.HeaderAuthorization, bearerTokenHdr+" "+string(value))
	}
	return header
}

func Test_fromCookie(t *testing.T) {
	cases := []struct {
		name   string
		actual []byte
		expect []byte
	}{
		{name: "empty"},
		{name: "normal", actual: []byte("TOKEN"), expect: []byte("TOKEN")},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expect, BearerTokenFromCookie(makeTestCookie(tt.actual)))
		})
	}
}

func Test_fromHeader(t *testing.T) {
	cases := []struct {
		name   string
		actual []byte
		expect []byte
	}{
		{name: "empty"},
		{name: "normal", actual: []byte("TOKEN"), expect: []byte("TOKEN")},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expect, BearerTokenFromHeader(makeTestHeader(tt.actual)))
		})
	}
}

func Test_fetchBearerToken(t *testing.T) {
	key, err := keys.NewPrivateKey()
	require.NoError(t, err)
	var uid user.ID
	user.IDFromKey(&uid, key.PrivateKey.PublicKey)

	tkn := new(bearer.Token)
	tkn.ForUser(uid)

	t64 := base64.StdEncoding.EncodeToString(tkn.Marshal())
	require.NotEmpty(t, t64)

	cases := []struct {
		name string

		cookie string
		header string

		error  string
		expect *bearer.Token
	}{
		{name: "empty"},

		{name: "bad base64 header", header: "WRONG BASE64", error: "can't base64-decode bearer token"},
		{name: "bad base64 cookie", cookie: "WRONG BASE64", error: "can't base64-decode bearer token"},

		{name: "header token unmarshal error", header: "dGVzdAo=", error: "can't unmarshal bearer token"},
		{name: "cookie token unmarshal error", cookie: "dGVzdAo=", error: "can't unmarshal bearer token"},

		{
			name:   "bad header and cookie",
			header: "WRONG BASE64",
			cookie: "dGVzdAo=",
			error:  "can't unmarshal bearer token",
		},

		{
			name:   "bad header, but good cookie",
			header: "dGVzdAo=",
			cookie: t64,
			expect: tkn,
		},

		{name: "ok for header", header: t64, expect: tkn},
		{name: "ok for cookie", cookie: t64, expect: tkn},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := makeTestRequest(tt.cookie, tt.header)
			actual, err := fetchBearerToken(ctx)

			if tt.error == "" {
				require.NoError(t, err)
				require.Equal(t, tt.expect, actual)

				return
			}

			require.Contains(t, err.Error(), tt.error)
		})
	}
}

func makeTestRequest(cookie, header string) *fasthttp.RequestCtx {
	ctx := new(fasthttp.RequestCtx)

	if cookie != "" {
		ctx.Request.Header.SetCookie(bearerTokenHdr, cookie)
	}

	if header != "" {
		ctx.Request.Header.Set(fasthttp.HeaderAuthorization, bearerTokenHdr+" "+header)
	}
	return ctx
}

func Test_checkAndPropagateBearerToken(t *testing.T) {
	key, err := keys.NewPrivateKey()
	require.NoError(t, err)
	var uid user.ID
	user.IDFromKey(&uid, key.PrivateKey.PublicKey)

	tkn := new(bearer.Token)
	tkn.ForUser(uid)

	t64 := base64.StdEncoding.EncodeToString(tkn.Marshal())
	require.NotEmpty(t, t64)

	ctx := makeTestRequest(t64, "")

	// Expect to see the token within the context.
	require.NoError(t, StoreBearerToken(ctx))

	// Expect to see the same token without errors.
	actual, err := LoadBearerToken(ctx)
	require.NoError(t, err)
	require.Equal(t, tkn, actual)
}
