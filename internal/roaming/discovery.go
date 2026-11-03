// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

const (
	maxDiscoveryBody     = 1 << 20
	maxDiscoveryValidity = 24 * time.Hour
	noPriority           = 65536
)

var ErrNoCandidate = errors.New("roaming: the home NRF returned no usable NF instance")

type N32 interface {
	Client(partnerID string) *http.Client
	NRFAPIRoot(partnerID string) (string, error)
}

type DiscoveryRequest struct {
	PartnerID        string
	HomePLMN         wire.PlmnID
	ServingPLMN      wire.PlmnID
	Target           wire.NFType
	Requester        wire.NFType
	Service          wire.ServiceName
	RoutingIndicator string
}

type discoveryKey struct {
	partnerID        string
	home             wire.PlmnID
	target           wire.NFType
	requester        wire.NFType
	service          wire.ServiceName
	routingIndicator string
}

type discoveryEntry struct {
	expires    time.Time
	candidates []wire.Candidate
}

type Discovery struct {
	n32 N32
	now func() time.Time

	mu    sync.Mutex
	cache map[discoveryKey]*discoveryEntry
}

func NewDiscovery(n32 N32) *Discovery {
	return &Discovery{n32: n32, now: time.Now, cache: map[discoveryKey]*discoveryEntry{}}
}

func (r DiscoveryRequest) key() discoveryKey {
	return discoveryKey{partnerID: r.PartnerID, home: r.HomePLMN, target: r.Target, requester: r.Requester, service: r.Service, routingIndicator: r.RoutingIndicator}
}

func (d *Discovery) Discover(ctx context.Context, req DiscoveryRequest) (wire.Candidate, error) {
	key := req.key()

	d.mu.Lock()
	if e, ok := d.cache[key]; ok {
		if d.now().Before(e.expires) && len(e.candidates) > 0 {
			c := e.candidates[0]
			d.mu.Unlock()

			return c, nil
		}

		delete(d.cache, key)
	}
	d.mu.Unlock()

	candidates, validity, err := d.search(ctx, req)
	if err != nil {
		return wire.Candidate{}, err
	}

	if len(candidates) == 0 {
		return wire.Candidate{}, fmt.Errorf("%w: no %s instance offers %s", ErrNoCandidate, req.Target, req.Service)
	}

	if validity > 0 {
		d.mu.Lock()
		d.cache[key] = &discoveryEntry{expires: d.now().Add(validity), candidates: candidates}
		d.mu.Unlock()
	}

	return candidates[0], nil
}

func (d *Discovery) Drop(partnerID, nfInstanceID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for k, e := range d.cache {
		if k.partnerID != partnerID {
			continue
		}

		e.candidates = slices.DeleteFunc(e.candidates, func(c wire.Candidate) bool { return c.NfInstanceID == nfInstanceID })
		if len(e.candidates) == 0 {
			delete(d.cache, k)
		}
	}
}

func (d *Discovery) Forget(partnerID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for k := range d.cache {
		if k.partnerID == partnerID {
			delete(d.cache, k)
		}
	}
}

func (d *Discovery) search(ctx context.Context, req DiscoveryRequest) ([]wire.Candidate, time.Duration, error) {
	root, err := d.n32.NRFAPIRoot(req.PartnerID)
	if err != nil {
		return nil, 0, err
	}

	query := wire.DiscoveryQuery{
		TargetNFType:      req.Target,
		RequesterNFType:   req.Requester,
		ServiceNames:      []wire.ServiceName{req.Service},
		TargetPlmnList:    []wire.PlmnID{req.HomePLMN},
		RequesterPlmnList: []wire.PlmnID{req.ServingPLMN},
		RoutingIndicator:  req.RoutingIndicator,
	}

	values, err := query.Values()
	if err != nil {
		return nil, 0, err
	}

	target := strings.TrimSuffix(root, "/") + wire.NFInstancesPath() + "?" + values.Encode()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}

	httpReq.Header.Set("Accept", wire.ContentTypeJSON+", "+wire.ContentTypeProblem)
	httpReq.Header.Set("User-Agent", string(req.Requester))

	resp, err := d.n32.Client(req.PartnerID).Do(httpReq)
	if err != nil {
		return nil, 0, err
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDiscoveryBody))
	if err != nil {
		return nil, 0, fmt.Errorf("read the NRF answer: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("NF discovery at the home NRF failed: %w", wire.ParseProblem(resp.StatusCode, body))
	}

	result, err := wire.Decode[wire.SearchResult](body)
	if err != nil {
		return nil, 0, fmt.Errorf("NF discovery at the home NRF: %w", err)
	}

	validity := min(time.Duration(*result.ValidityPeriod)*time.Second, maxDiscoveryValidity)

	return rankCandidates(result, wire.Selector{Service: req.Service, Target: req.HomePLMN, Requester: req.ServingPLMN}, req.Target), validity, nil
}

func rankCandidates(result wire.SearchResult, sel wire.Selector, target wire.NFType) []wire.Candidate {
	type ranked struct {
		c        wire.Candidate
		priority int
	}

	var out []ranked

	for _, profile := range result.NfInstances {
		if profile.NfType != target {
			continue
		}

		c, ok := profile.Candidate(sel)
		if !ok {
			continue
		}

		priority := noPriority

		switch {
		case c.Service.Priority != nil:
			priority = *c.Service.Priority
		case profile.Priority != nil:
			priority = *profile.Priority
		}

		out = append(out, ranked{c: c, priority: priority})
	}

	slices.SortStableFunc(out, func(a, b ranked) int { return cmp.Compare(a.priority, b.priority) })

	candidates := make([]wire.Candidate, 0, len(out))
	for _, r := range out {
		candidates = append(candidates, r.c)
	}

	return candidates
}
