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
	auth      *hplmnAuthorization
}

func NewHPLMNSBI(n32 N32, discovery *Discovery) *HPLMNSBI {
	return &HPLMNSBI{n32: n32, discovery: discovery, timeout: hplmnRequestTimeout, auth: newHPLMNAuthorization()}
}

type HPLMNRequest struct {
	Method      string
	URL         string
	Body        wire.Checker
	ContentType string
	Requester   wire.NFType
	Accept      []int
}

type HPLMNAnswer struct {
	Status   int
	Location string
	Body     []byte
	Bearer   bool
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

	h.auth.remember(c, t, requester)

	return c, nil
}

func (h *HPLMNSBI) Forget(partnerID, nfInstanceID string) {
	h.discovery.Drop(partnerID, nfInstanceID)
}

func (h *HPLMNSBI) Do(ctx context.Context, partnerID string, req HPLMNRequest) (HPLMNAnswer, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	target := req.URL
	redirected, retried, refresh := false, false, false

	for {
		token, err := h.authorize(ctx, partnerID, target, req.Requester, refresh)
		if err != nil {
			return HPLMNAnswer{}, err
		}

		refresh = false

		answer, err := h.send(ctx, partnerID, target, token, req)

		h.record(partnerID, target, answer.Status, err)

		if err != nil {
			return HPLMNAnswer{}, err
		}

		if answer.Status == http.StatusUnauthorized && answer.Bearer && !retried && h.auth.known(target) {
			retried, refresh = true, true

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

	_, bearer := wire.BearerChallenge(resp.Header.Values("WWW-Authenticate"))

	return HPLMNAnswer{Status: resp.StatusCode, Location: resp.Header.Get("Location"), Body: data, Bearer: bearer}, nil
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
