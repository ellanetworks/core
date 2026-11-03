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
	maxHomeAnswer      = 1 << 20
	homeRequestTimeout = 10 * time.Second
)

var (
	ErrHomeUnreachable = errors.New("roaming: the home network cannot be reached")
	ErrHomeAnswer      = errors.New("roaming: unexpected answer from the home network")
)

type HomeTarget struct {
	PartnerID        string
	PLMN             wire.PlmnID
	Serving          wire.PlmnID
	RoutingIndicator string
}

type HomeSBI struct {
	n32       N32
	discovery *Discovery
	timeout   time.Duration
}

func NewHomeSBI(n32 N32, discovery *Discovery) *HomeSBI {
	return &HomeSBI{n32: n32, discovery: discovery, timeout: homeRequestTimeout}
}

type HomeRequest struct {
	Method      string
	URL         string
	Body        wire.Checker
	ContentType string
	Requester   wire.NFType
	Accept      []int
}

type HomeAnswer struct {
	Status   int
	Location string
	Body     []byte
}

func (h *HomeSBI) APIRoot(ctx context.Context, t HomeTarget, target, requester wire.NFType, service wire.ServiceName) (wire.Candidate, error) {
	c, err := h.discovery.Discover(ctx, DiscoveryRequest{
		PartnerID:        t.PartnerID,
		HomePLMN:         t.PLMN,
		ServingPLMN:      t.Serving,
		Target:           target,
		Requester:        requester,
		Service:          service,
		RoutingIndicator: t.RoutingIndicator,
	})
	if err != nil {
		return wire.Candidate{}, homeTransportError(fmt.Errorf("discover the home %s: %w", target, err))
	}

	return c, nil
}

func (h *HomeSBI) Forget(partnerID, nfInstanceID string) {
	h.discovery.Drop(partnerID, nfInstanceID)
}

func (h *HomeSBI) Do(ctx context.Context, partnerID string, req HomeRequest) (HomeAnswer, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	target := req.URL

	for redirects := 0; ; redirects++ {
		answer, err := h.send(ctx, partnerID, target, req)

		h.record(partnerID, target, answer.Status, err)

		if err != nil {
			return HomeAnswer{}, err
		}

		if (answer.Status == http.StatusTemporaryRedirect || answer.Status == http.StatusPermanentRedirect) && answer.Location != "" && redirects == 0 {
			next, err := resolveReference(target, answer.Location)
			if err != nil {
				return HomeAnswer{}, fmt.Errorf("%w: redirect to %q: %w", ErrHomeAnswer, answer.Location, err)
			}

			target = next

			continue
		}

		if !slices.Contains(req.Accept, answer.Status) {
			return answer, wire.ParseProblem(answer.Status, answer.Body)
		}

		answer.Location = resolveOrKeep(target, answer.Location)

		return answer, nil
	}
}

func (h *HomeSBI) send(ctx context.Context, partnerID, target string, req HomeRequest) (HomeAnswer, error) {
	var body io.Reader

	if req.Body != nil {
		data, err := wire.Encode(req.Body)
		if err != nil {
			return HomeAnswer{}, fmt.Errorf("encode the %s %s request: %w", req.Method, target, err)
		}

		body = bytes.NewReader(data)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, target, body)
	if err != nil {
		return HomeAnswer{}, fmt.Errorf("build the %s request: %w", req.Method, err)
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

	resp, err := h.n32.Client(partnerID).Do(httpReq)
	if err != nil {
		return HomeAnswer{}, homeTransportError(err)
	}

	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxHomeAnswer))
	if err != nil {
		return HomeAnswer{}, homeTransportError(fmt.Errorf("read the answer: %w", err))
	}

	return HomeAnswer{Status: resp.StatusCode, Location: resp.Header.Get("Location"), Body: data}, nil
}

func homeTransportError(err error) error {
	if errors.Is(err, ErrHomeUnreachable) {
		return err
	}

	return fmt.Errorf("%w: %w", ErrHomeUnreachable, err)
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
}](a HomeAnswer) (T, error) {
	v, err := wire.Decode[T, P](a.Body)
	if err != nil {
		return v, fmt.Errorf("%w: %w", ErrHomeAnswer, err)
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
