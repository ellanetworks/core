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
	discoverySweepAt     = 4096
)

var ErrNoCandidate = errors.New("roaming: the home NRF returned no usable NF instance")

type N32 interface {
	Client(partnerID string) *http.Client
	HPLMNNRF(partnerID string, home wire.PlmnID) (string, error)
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

func (r DiscoveryRequest) subject() wire.Subject {
	return wire.Subject{Supi: r.Supi, RoutingIndicator: r.RoutingIndicator, GroupID: r.GroupID}
}

func (r DiscoveryRequest) selector() string {
	switch {
	case r.GroupID != "":
		return "group:" + r.GroupID
	case r.Supi != "":
		return "supi:" + r.Supi
	default:
		return "ri:" + r.RoutingIndicator
	}
}

type discoveryKey struct {
	partnerID string
	home      wire.PlmnID
	target    wire.NFType
	requester wire.NFType
	service   wire.ServiceName
	selector  string
}

func (r DiscoveryRequest) key(selector string) discoveryKey {
	return discoveryKey{partnerID: r.PartnerID, home: r.HomePLMN, target: r.Target, requester: r.Requester, service: r.Service, selector: selector}
}

type cachedCandidate struct {
	c       wire.Candidate
	expires time.Time
}

type Discovery struct {
	n32 N32
	now func() time.Time

	mu    sync.Mutex
	cache map[discoveryKey][]cachedCandidate
}

func NewDiscovery(n32 N32) *Discovery {
	return &Discovery{n32: n32, now: time.Now, cache: map[discoveryKey][]cachedCandidate{}}
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

	sub := req.subject()
	serving := slices.DeleteFunc(slices.Clone(candidates), func(c wire.Candidate) bool { return !c.Scope.Serves(sub) })

	if validity > 0 {
		d.store(req, serving, candidates, d.now().Add(validity))
	}

	if len(serving) == 0 {
		return wire.Candidate{}, fmt.Errorf("%w: no %s instance offers %s", ErrNoCandidate, req.Target, req.Service)
	}

	candidates = serving

	return pick(candidates, req.Pinned), nil
}

func (d *Discovery) cached(req DiscoveryRequest) (wire.Candidate, bool) {
	sub := req.subject()
	now := d.now()

	d.mu.Lock()
	defer d.mu.Unlock()

	exact := d.live(req.key(req.selector()), now, func(c wire.Candidate) bool { return c.Scope.Serves(sub) })
	shared := d.live(req.key(""), now, func(c wire.Candidate) bool { return c.Scope.Proves(sub) })

	for _, set := range [][]wire.Candidate{exact, shared} {
		if len(set) == 0 {
			continue
		}

		if req.Pinned != "" && !slices.ContainsFunc(set, func(c wire.Candidate) bool { return c.NfInstanceID == req.Pinned }) {
			continue
		}

		return pick(set, req.Pinned), true
	}

	return wire.Candidate{}, false
}

func (d *Discovery) live(key discoveryKey, now time.Time, keep func(wire.Candidate) bool) []wire.Candidate {
	entries := d.cache[key]

	var out []wire.Candidate

	fresh := entries[:0]

	for _, e := range entries {
		if !now.Before(e.expires) {
			continue
		}

		fresh = append(fresh, e)

		if keep(e.c) {
			out = append(out, e.c)
		}
	}

	if len(fresh) == 0 {
		delete(d.cache, key)
	} else {
		d.cache[key] = fresh
	}

	return out
}

func (d *Discovery) store(req DiscoveryRequest, candidates, all []wire.Candidate, expires time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(d.cache) >= discoverySweepAt {
		d.sweep()
	}

	entries := make([]cachedCandidate, 0, len(candidates))
	for _, c := range candidates {
		entries = append(entries, cachedCandidate{c: c, expires: expires})
	}

	if len(entries) > 0 {
		d.cache[req.key(req.selector())] = entries

		if group := commonGroup(candidates); group != "" && req.GroupID == "" {
			d.cache[req.key("group:"+group)] = slices.Clone(entries)
		}
	}

	shared := req.key("")

	for _, c := range all {
		if !c.Scope.SelfDescribing() {
			continue
		}

		e := cachedCandidate{c: c, expires: expires}

		merged := slices.DeleteFunc(d.cache[shared], func(o cachedCandidate) bool { return o.c.NfInstanceID == e.c.NfInstanceID })
		d.cache[shared] = append(merged, e)
	}

	if s := d.cache[shared]; len(s) > 0 {
		slices.SortStableFunc(s, func(a, b cachedCandidate) int { return cmp.Compare(priorityOf(a.c), priorityOf(b.c)) })
	}
}

func (d *Discovery) sweep() {
	now := d.now()

	for k, entries := range d.cache {
		entries = slices.DeleteFunc(entries, func(e cachedCandidate) bool { return !now.Before(e.expires) })
		if len(entries) == 0 {
			delete(d.cache, k)
		} else {
			d.cache[k] = entries
		}
	}
}

func commonGroup(candidates []wire.Candidate) string {
	group := ""

	for _, c := range candidates {
		switch {
		case c.Scope.GroupID == "":
			return ""
		case group == "":
			group = c.Scope.GroupID
		case group != c.Scope.GroupID:
			return ""
		}
	}

	return group
}

func pick(candidates []wire.Candidate, pinned string) wire.Candidate {
	if pinned != "" {
		if i := slices.IndexFunc(candidates, func(c wire.Candidate) bool { return c.NfInstanceID == pinned }); i >= 0 {
			return candidates[i]
		}
	}

	return candidates[0]
}

func (d *Discovery) Drop(partnerID, nfInstanceID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for k, entries := range d.cache {
		if k.partnerID != partnerID {
			continue
		}

		entries = slices.DeleteFunc(entries, func(e cachedCandidate) bool { return e.c.NfInstanceID == nfInstanceID })
		if len(entries) == 0 {
			delete(d.cache, k)
		} else {
			d.cache[k] = entries
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
