// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
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
	HPLMNNRF(partnerID string, home wire.PlmnID) (string, error)
	SEPPID() int
	WaitEstablished(ctx context.Context, partnerID string, timeout time.Duration) error
}

type DiscoveryRequest struct {
	PartnerID        string
	HomePLMN         wire.PlmnID
	ServingPLMN      wire.PlmnID
	Target           wire.NFType
	Requester        wire.NFType
	Service          wire.ServiceName
	RoutingIndicator string
	Supi             string
	GroupID          string
	Pinned           string
}

type discoveryKey struct {
	partnerID        string
	home             wire.PlmnID
	target           wire.NFType
	requester        wire.NFType
	service          wire.ServiceName
	routingIndicator string
	supi             string
	groupID          string
}

func (r DiscoveryRequest) key() discoveryKey {
	return discoveryKey{
		partnerID:        r.PartnerID,
		home:             r.HomePLMN,
		target:           r.Target,
		requester:        r.Requester,
		service:          r.Service,
		routingIndicator: r.RoutingIndicator,
		supi:             r.Supi,
		groupID:          r.GroupID,
	}
}

type discoveryResult struct {
	candidates []wire.Candidate
	expires    time.Time
}

type Discovery struct {
	n32 N32
	now func() time.Time

	mu    sync.Mutex
	cache map[discoveryKey]discoveryResult
}

func NewDiscovery(n32 N32) *Discovery {
	return &Discovery{n32: n32, now: time.Now, cache: map[discoveryKey]discoveryResult{}}
}

func (d *Discovery) NRF(partnerID string, home wire.PlmnID) (string, error) {
	return d.n32.HPLMNNRF(partnerID, home)
}

func (d *Discovery) Discover(ctx context.Context, req DiscoveryRequest) (wire.Candidate, error) {
	if c, ok := d.cached(req); ok {
		return c, nil
	}

	candidates, validity, err := d.search(ctx, req)
	if err != nil {
		return wire.Candidate{}, err
	}

	if validity > 0 && len(candidates) > 0 {
		d.store(req.key(), discoveryResult{candidates: candidates, expires: d.now().Add(validity)})
	}

	if len(candidates) == 0 {
		return wire.Candidate{}, fmt.Errorf("%w: no %s instance offers %s", ErrNoCandidate, req.Target, req.Service)
	}

	return pick(candidates, req.Pinned), nil
}

func (d *Discovery) cached(req DiscoveryRequest) (wire.Candidate, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	r, ok := d.cache[req.key()]
	if !ok || !d.now().Before(r.expires) {
		return wire.Candidate{}, false
	}

	if req.Pinned != "" && !slices.ContainsFunc(r.candidates, func(c wire.Candidate) bool { return c.NfInstanceID == req.Pinned }) {
		return wire.Candidate{}, false
	}

	return pick(r.candidates, req.Pinned), true
}

func (d *Discovery) store(k discoveryKey, r discoveryResult) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := d.now()

	maps.DeleteFunc(d.cache, func(_ discoveryKey, e discoveryResult) bool { return !now.Before(e.expires) })

	d.cache[k] = r
}

func pick(candidates []wire.Candidate, pinned string) wire.Candidate {
	if pinned != "" {
		if i := slices.IndexFunc(candidates, func(c wire.Candidate) bool { return c.NfInstanceID == pinned }); i >= 0 {
			return candidates[i]
		}
	}

	return candidates[0]
}

func (d *Discovery) Forget(partnerID, nfInstanceID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for k, r := range d.cache {
		if k.partnerID != partnerID {
			continue
		}

		r.candidates = slices.DeleteFunc(slices.Clone(r.candidates), func(c wire.Candidate) bool { return c.NfInstanceID == nfInstanceID })
		if len(r.candidates) == 0 {
			delete(d.cache, k)
		} else {
			d.cache[k] = r
		}
	}
}

func (d *Discovery) search(ctx context.Context, req DiscoveryRequest) ([]wire.Candidate, time.Duration, error) {
	root, err := d.NRF(req.PartnerID, req.HomePLMN)
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
		Supi:              req.Supi,
	}

	if req.GroupID != "" {
		query.GroupIDList = []string{req.GroupID}
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

func priorityOf(c wire.Candidate) int {
	if c.Service.Priority != nil {
		return *c.Service.Priority
	}

	if c.Priority != nil {
		return *c.Priority
	}

	return noPriority
}

func rankCandidates(result wire.SearchResult, sel wire.Selector, target wire.NFType) []wire.Candidate {
	var out []wire.Candidate

	for _, profile := range result.NfInstances {
		if profile.NfType != target {
			continue
		}

		if c, ok := profile.Candidate(sel); ok {
			out = append(out, c)
		}
	}

	slices.SortStableFunc(out, func(a, b wire.Candidate) int { return cmp.Compare(priorityOf(a), priorityOf(b)) })

	return out
}
