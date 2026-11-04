// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

const (
	maxHPLMNAnswer      = 1 << 20
	hplmnRequestTimeout = 10 * time.Second
)

var (
	ErrHPLMNUnreachable = errors.New("roaming: the home network cannot be reached")
	ErrHPLMNAnswer      = errors.New("roaming: unexpected answer from the home network")
)

type HPLMNTarget struct {
	PartnerID        string
	PLMN             wire.PlmnID
	Serving          wire.PlmnID
	RoutingIndicator string
	Supi             string
	UDMGroupID       string
	UDMInstanceID    string
}

type HPLMNSBI struct {
	n32       N32
	discovery *Discovery
	timeout   time.Duration
	tokens    *tokenCache
}

func NewHPLMNSBI(n32 N32, discovery *Discovery) *HPLMNSBI {
	return &HPLMNSBI{n32: n32, discovery: discovery, timeout: hplmnRequestTimeout, tokens: newTokenCache()}
}

type HPLMNRequest struct {
	Method      string
	URL         string
	Body        wire.Checker
	ContentType string
	Requester   wire.NFType
	Producer    wire.NFType
	Service     wire.ServiceName
	OAuth2      bool
	Accept      []int
}

type HPLMNAnswer struct {
	Status   int
	Location string
	Body     []byte
}

func (h *HPLMNSBI) APIRoot(ctx context.Context, t HPLMNTarget, target, requester wire.NFType, service wire.ServiceName) (wire.Candidate, error) {
	req := DiscoveryRequest{
		PartnerID:        t.PartnerID,
		HomePLMN:         t.PLMN,
		ServingPLMN:      t.Serving,
		Target:           target,
		Requester:        requester,
		Service:          service,
		RoutingIndicator: t.RoutingIndicator,
		Supi:             t.Supi,
	}

	if target == wire.NFTypeUDM {
		req.GroupID = t.UDMGroupID
		req.Pinned = t.UDMInstanceID
	}

	c, err := h.discovery.Discover(ctx, req)
	if err != nil {
		return wire.Candidate{}, hplmnTransportError(fmt.Errorf("discover the home %s: %w", target, err))
	}

	return c, nil
}

func (h *HPLMNSBI) Forget(partnerID, nfInstanceID string) {
	h.discovery.Forget(partnerID, nfInstanceID)
}

func (h *HPLMNSBI) WaitEstablished(ctx context.Context, partnerID string, timeout time.Duration) error {
	return h.n32.WaitEstablished(ctx, partnerID, timeout)
}

func (h *HPLMNSBI) Do(ctx context.Context, t HPLMNTarget, req HPLMNRequest) (HPLMNAnswer, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	key := tokenKey{partnerID: t.PartnerID, home: t.PLMN, serving: t.Serving, requester: req.Requester, producer: req.Producer, service: req.Service}
	scoped := req.Producer != "" && req.Service != ""

	token := ""

	if scoped {
		cached, ok := h.tokens.get(key)

		switch {
		case ok:
			token = cached
		case req.OAuth2:
			fetched, err := h.fetchToken(ctx, key)
			if err != nil {
				return HPLMNAnswer{}, err
			}

			token = fetched
		}
	}

	target := req.URL
	redirected, retried := false, false

	for {
		answer, err := h.send(ctx, t.PartnerID, target, token, req)
		if err != nil {
			return HPLMNAnswer{}, err
		}

		if answer.Status == http.StatusUnauthorized && scoped && !retried {
			retried = true

			h.tokens.drop(key)

			token, err = h.fetchToken(ctx, key)
			if err != nil {
				return HPLMNAnswer{}, err
			}

			continue
		}

		if (answer.Status == http.StatusTemporaryRedirect || answer.Status == http.StatusPermanentRedirect) && answer.Location != "" && !redirected {
			next, err := resolveReference(target, answer.Location)
			if err != nil {
				return HPLMNAnswer{}, fmt.Errorf("%w: redirect to %q: %w", ErrHPLMNAnswer, answer.Location, err)
			}

			target = next
			redirected = true

			continue
		}

		if !slices.Contains(req.Accept, answer.Status) {
			return answer, wire.ParseProblem(answer.Status, answer.Body)
		}

		answer.Location = resolveOrKeep(target, answer.Location)

		return answer, nil
	}
}

func (h *HPLMNSBI) send(ctx context.Context, partnerID, target, token string, req HPLMNRequest) (HPLMNAnswer, error) {
	var body io.Reader

	if req.Body != nil {
		data, err := wire.Encode(req.Body)
		if err != nil {
			return HPLMNAnswer{}, fmt.Errorf("encode the %s %s request: %w", req.Method, target, err)
		}

		body = bytes.NewReader(data)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, target, body)
	if err != nil {
		return HPLMNAnswer{}, fmt.Errorf("build the %s request: %w", req.Method, err)
	}

	if req.Body != nil {
		contentType := req.ContentType
		if contentType == "" {
			contentType = wire.ContentTypeJSON
		}

		httpReq.Header.Set("Content-Type", contentType)
	}

	httpReq.Header.Set("Accept", wire.ContentTypeJSON+", "+wire.ContentTypeProblem)
	httpReq.Header.Set("User-Agent", string(req.Requester))

	if token != "" {
		httpReq.Header.Set("Authorization", wire.TokenTypeBearer+" "+token)
	}

	resp, err := h.n32.Client(partnerID).Do(httpReq)
	if err != nil {
		return HPLMNAnswer{}, hplmnTransportError(err)
	}

	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxHPLMNAnswer))
	if err != nil {
		return HPLMNAnswer{}, hplmnTransportError(fmt.Errorf("read the answer: %w", err))
	}

	return HPLMNAnswer{Status: resp.StatusCode, Location: resp.Header.Get("Location"), Body: data}, nil
}

func hplmnTransportError(err error) error {
	if errors.Is(err, ErrHPLMNUnreachable) {
		return err
	}

	return fmt.Errorf("%w: %w", ErrHPLMNUnreachable, err)
}

func resolveReference(base, ref string) (string, error) {
	b, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	r, err := url.Parse(ref)
	if err != nil {
		return "", err
	}

	return b.ResolveReference(r).String(), nil
}

func resolveOrKeep(base, ref string) string {
	if ref == "" {
		return ""
	}

	out, err := resolveReference(base, ref)
	if err != nil {
		return ref
	}

	return out
}

func DecodeAnswer[T any, P interface {
	*T
	wire.Checker
}](a HPLMNAnswer) (T, error) {
	v, err := wire.Decode[T, P](a.Body)
	if err != nil {
		return v, fmt.Errorf("%w: %w", ErrHPLMNAnswer, err)
	}

	return v, nil
}

func ProblemOf(err error) (wire.ProblemDetails, bool) {
	var pd wire.ProblemDetails
	if errors.As(err, &pd) {
		return pd, true
	}

	return wire.ProblemDetails{}, false
}

func JoinAPIRoot(root, path string) string {
	return strings.TrimSuffix(root, "/") + path
}
