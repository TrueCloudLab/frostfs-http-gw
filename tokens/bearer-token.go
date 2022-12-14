package tokens

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/TrueCloudLab/frostfs-sdk-go/bearer"
	"github.com/valyala/fasthttp"
)

type fromHandler = func(h *fasthttp.RequestHeader) []byte

const (
	bearerTokenHdr = "Bearer"
	bearerTokenKey = "__context_bearer_token_key"
)

// BearerToken usage:
//
// if err = storeBearerToken(ctx); err != nil {
// 	log.Error("could not fetch bearer token", zap.Error(err))
// 	c.Error("could not fetch bearer token", fasthttp.StatusBadRequest)
// 	return
// }

// BearerTokenFromHeader extracts a bearer token from Authorization request header.
func BearerTokenFromHeader(h *fasthttp.RequestHeader) []byte {
	auth := h.Peek(fasthttp.HeaderAuthorization)
	if auth == nil || !bytes.HasPrefix(auth, []byte(bearerTokenHdr)) {
		return nil
	}
	if auth = bytes.TrimPrefix(auth, []byte(bearerTokenHdr+" ")); len(auth) == 0 {
		return nil
	}
	return auth
}

// BearerTokenFromCookie extracts a bearer token from cookies.
func BearerTokenFromCookie(h *fasthttp.RequestHeader) []byte {
	auth := h.Cookie(bearerTokenHdr)
	if len(auth) == 0 {
		return nil
	}

	return auth
}

// StoreBearerToken extracts a bearer token from the header or cookie and stores
// it in the request context.
func StoreBearerToken(ctx *fasthttp.RequestCtx) error {
	tkn, err := fetchBearerToken(ctx)
	if err != nil {
		return err
	}
	// This is an analog of context.WithValue.
	ctx.SetUserValue(bearerTokenKey, tkn)
	return nil
}

// LoadBearerToken returns a bearer token stored in the context given (if it's
// present there).
func LoadBearerToken(ctx context.Context) (*bearer.Token, error) {
	if tkn, ok := ctx.Value(bearerTokenKey).(*bearer.Token); ok && tkn != nil {
		return tkn, nil
	}
	return nil, errors.New("found empty bearer token")
}

func fetchBearerToken(ctx *fasthttp.RequestCtx) (*bearer.Token, error) {
	// ignore empty value
	if ctx == nil {
		return nil, nil
	}
	var (
		lastErr error

		buf []byte
		tkn = new(bearer.Token)
	)
	for _, parse := range []fromHandler{BearerTokenFromHeader, BearerTokenFromCookie} {
		if buf = parse(&ctx.Request.Header); buf == nil {
			continue
		} else if data, err := base64.StdEncoding.DecodeString(string(buf)); err != nil {
			lastErr = fmt.Errorf("can't base64-decode bearer token: %w", err)
			continue
		} else if err = tkn.Unmarshal(data); err != nil {
			lastErr = fmt.Errorf("can't unmarshal bearer token: %w", err)
			continue
		}

		return tkn, nil
	}

	return nil, lastErr
}
