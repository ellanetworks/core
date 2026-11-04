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
	"golang.org/x/sync/singleflight"
)

const (
	tokenRefreshMargin   = 30 * time.Second
	tokenDefaultLifetime = time.Minute
	maxTokenAnswer       = 64 << 10
)

var ErrHPLMNUnauthorized = errors.New("roaming: the home NRF did not grant an access token")

type authTarget struct {
	partnerID string
	home      wire.PlmnID
	serving   wire.PlmnID
	producer  wire.NFType
	requester wire.NFType
	service   wire.ServiceName
	required  bool
}

type rootKey struct {
	root      string
	requester wire.NFType
}

type tokenKey struct {
	partnerID string
	home      wire.PlmnID
	serving   wire.PlmnID
	producer  wire.NFType
	requester wire.NFType
	scope     string
}

func (k tokenKey) String() string {
	return strings.Join([]string{k.partnerID, k.home.String(), k.serving.String(), string(k.producer), string(k.requester), k.scope}, "|")
}

type cachedToken struct {
	value   string
	expires time.Time
}

type hplmnAuthorization struct {
	now func() time.Time

	mu     sync.Mutex
	roots  map[rootKey]authTarget
	tokens map[tokenKey]cachedToken
	flight singleflight.Group
}

func newHPLMNAuthorization() *hplmnAuthorization {
	return &hplmnAuthorization{now: time.Now, roots: map[rootKey]authTarget{}, tokens: map[tokenKey]cachedToken{}}
}

func (a *hplmnAuthorization) remember(c wire.Candidate, t HPLMNTarget, requester wire.NFType) {
	if c.APIRoot == "" {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.roots[rootKey{root: strings.TrimSuffix(c.APIRoot, "/"), requester: requester}] = authTarget{
		partnerID: t.PartnerID,
		home:      t.PLMN,
		serving:   t.Serving,
		producer:  c.NfType,
		requester: requester,
		service:   c.Service.ServiceName,
		required:  c.OAuth2Required,
	}
}

func (a *hplmnAuthorization) lookup(target string, requester wire.NFType) (rootKey, authTarget, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var (
		best  rootKey
		found authTarget
		ok    bool
	)

	for k, at := range a.roots {
		if k.requester != requester || !underRoot(target, k.root) || len(k.root) <= len(best.root) {
			continue
		}

		best, found, ok = k, at, true
	}

	return best, found, ok
}

func (a *hplmnAuthorization) known(target string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	for k := range a.roots {
		if underRoot(target, k.root) {
			return true
		}
	}

	return false
}

func (a *hplmnAuthorization) demand(k rootKey) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if at, ok := a.roots[k]; ok {
		at.required = true
		a.roots[k] = at
	}
}

func (a *hplmnAuthorization) cached(k tokenKey) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	t, ok := a.tokens[k]
	if !ok || !a.now().Before(t.expires) {
		delete(a.tokens, k)
		return "", false
	}

	return t.value, true
}

func (a *hplmnAuthorization) store(k tokenKey, value string, lifetime time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.tokens[k] = cachedToken{value: value, expires: a.now().Add(max(lifetime-tokenRefreshMargin, lifetime/2))}
}

func (a *hplmnAuthorization) drop(k tokenKey) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.tokens, k)
}

func underRoot(target, root string) bool {
	if !strings.HasPrefix(target, root) {
		return false
	}

	rest := target[len(root):]

	return rest == "" || rest[0] == '/' || rest[0] == '?'
}

func (h *HPLMNSBI) authorize(ctx context.Context, partnerID, target string, requester wire.NFType, force bool) (string, error) {
	rk, at, ok := h.auth.lookup(target, requester)
	if !ok || at.partnerID != partnerID {
		return "", nil
	}

	if force {
		h.auth.demand(rk)
	} else if !at.required {
		return "", nil
	}

	key := tokenKey{partnerID: partnerID, home: at.home, serving: at.serving, producer: at.producer, requester: requester, scope: string(at.service)}

	if force {
		h.auth.drop(key)
	} else if token, ok := h.auth.cached(key); ok {
		return token, nil
	}

	v, err, _ := h.auth.flight.Do(key.String(), func() (any, error) {
		if token, ok := h.auth.cached(key); ok && !force {
			return token, nil
		}

		return h.fetchToken(ctx, key)
	})
	if err != nil {
		return "", err
	}

	token, _ := v.(string)

	return token, nil
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
		Scope:         key.scope,
		RequesterPlmn: &serving,
		TargetPlmn:    &home,
	}.Values()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrHPLMNUnauthorized, err)
	}

	endpoint := JoinAPIRoot(nrf, wire.AccessTokenPath())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrHPLMNUnauthorized, err)
	}

	req.Header.Set("Content-Type", wire.ContentTypeForm)
	req.Header.Set("Accept", wire.ContentTypeJSON+", "+wire.ContentTypeProblem)
	req.Header.Set("User-Agent", string(key.requester))

	resp, err := h.n32.Client(key.partnerID).Do(req)
	if err != nil {
		h.record(key.partnerID, endpoint, 0, err)
		return "", hplmnTransportError(fmt.Errorf("access token request: %w", err))
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenAnswer))

	h.record(key.partnerID, endpoint, resp.StatusCode, err)

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

	lifetime, ok := answer.Lifetime(h.auth.now())
	if !ok {
		lifetime = tokenDefaultLifetime
	}

	if lifetime <= 0 {
		return "", fmt.Errorf("%w: the access token has already expired", ErrHPLMNUnauthorized)
	}

	h.auth.store(key, answer.AccessToken, lifetime)

	return answer.AccessToken, nil
}

func tokenRefusal(status int, body []byte) string {
	if e, err := wire.Decode[wire.AccessTokenErr](body); err == nil {
		return e.String()
	}

	return wire.ParseProblem(status, body).Error()
}

type seppIdentity interface {
	SEPPID() int
}

func (h *HPLMNSBI) consumerInstanceID(requester wire.NFType, serving wire.PlmnID) string {
	seppID := 0
	if s, ok := h.n32.(seppIdentity); ok {
		seppID = s.SEPPID()
	}

	nf := sepp.CallbackAMF
	if requester == wire.NFTypeSMF {
		nf = sepp.CallbackSMF
	}

	return NFInstanceID(sepp.CallbackHost(nf, seppID, serving))
}
