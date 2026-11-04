// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
)

const (
	tokenRefreshMargin   = 30 * time.Second
	tokenDefaultLifetime = time.Minute
	maxTokenAnswer       = 64 << 10
)

var ErrHPLMNUnauthorized = errors.New("roaming: the home NRF did not grant an access token")

type tokenKey struct {
	partnerID string
	home      wire.PlmnID
	serving   wire.PlmnID
	requester wire.NFType
	producer  wire.NFType
	service   wire.ServiceName
}

type cachedToken struct {
	value   string
	expires time.Time
}

type tokenCache struct {
	now func() time.Time

	mu     sync.Mutex
	tokens map[tokenKey]cachedToken
}

func newTokenCache() *tokenCache {
	return &tokenCache{now: time.Now, tokens: map[tokenKey]cachedToken{}}
}

func (c *tokenCache) get(k tokenKey) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	t, ok := c.tokens[k]
	if !ok || !c.now().Before(t.expires) {
		delete(c.tokens, k)
		return "", false
	}

	return t.value, true
}

func (c *tokenCache) store(k tokenKey, value string, lifetime time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tokens[k] = cachedToken{value: value, expires: c.now().Add(max(lifetime-tokenRefreshMargin, lifetime/2))}
}

func (c *tokenCache) drop(k tokenKey) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.tokens, k)
}

func (h *HPLMNSBI) fetchToken(ctx context.Context, key tokenKey) (string, error) {
	nrf, err := h.discovery.NRF(key.partnerID, key.home)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrHPLMNUnauthorized, err)
	}

	home, serving := key.home, key.serving

	values, err := wire.AccessTokenReq{
		NfInstanceID:  h.consumerInstanceID(key.requester, serving),
		NfType:        key.requester,
		TargetNfType:  key.producer,
		Scope:         string(key.service),
		RequesterPlmn: &serving,
		TargetPlmn:    &home,
	}.Values()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrHPLMNUnauthorized, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, JoinAPIRoot(nrf, wire.AccessTokenPath()), strings.NewReader(values.Encode()))
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrHPLMNUnauthorized, err)
	}

	req.Header.Set("Content-Type", wire.ContentTypeForm)
	req.Header.Set("Accept", wire.ContentTypeJSON+", "+wire.ContentTypeProblem)
	req.Header.Set("User-Agent", string(key.requester))

	resp, err := h.n32.Client(key.partnerID).Do(req)
	if err != nil {
		return "", hplmnTransportError(fmt.Errorf("access token request: %w", err))
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenAnswer))
	if err != nil {
		return "", hplmnTransportError(fmt.Errorf("read the access token answer: %w", err))
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: status %d: %s", ErrHPLMNUnauthorized, resp.StatusCode, tokenRefusal(resp.StatusCode, body))
	}

	answer, err := wire.Decode[wire.AccessTokenRsp](body)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrHPLMNUnauthorized, err)
	}

	lifetime, ok := answer.Lifetime(h.tokens.now())
	if !ok {
		lifetime = tokenDefaultLifetime
	}

	if lifetime <= 0 {
		return "", fmt.Errorf("%w: the access token has already expired", ErrHPLMNUnauthorized)
	}

	h.tokens.store(key, answer.AccessToken, lifetime)

	return answer.AccessToken, nil
}

func tokenRefusal(status int, body []byte) string {
	if e, err := wire.Decode[wire.AccessTokenErr](body); err == nil {
		return e.String()
	}

	return wire.ParseProblem(status, body).Error()
}

func (h *HPLMNSBI) consumerInstanceID(requester wire.NFType, serving wire.PlmnID) string {
	nf := sepp.CallbackAMF
	if requester == wire.NFTypeSMF {
		nf = sepp.CallbackSMF
	}

	return NFInstanceID(sepp.CallbackHost(nf, h.n32.SEPPID(), serving))
}
