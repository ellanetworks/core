// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
package engine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math"
	"net"
	"net/netip"
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
)

type SessionEngine struct {
	mu sync.RWMutex

	sessions                map[uint64]*Session
	policyToSEIDs           map[string]map[uint64]struct{}
	nodeID                  string
	nodeAddrV4              net.IP
	n3AddressIPv4           netip.Addr // may be zero if not available
	n3AddressIPv6           netip.Addr // may be zero if not available
	n3VRFDevice             string
	advertisedN3AddressIPv4 netip.Addr
	advertisedN3AddressIPv6 netip.Addr
	BpfObjects              *ebpf.BpfObjects
	// Downlink packet buffering; nil disables it.
	dlBuffer             DownlinkBuffer
	FteIDResourceManager *FteIDResourceManager
	SdfIndexAllocator    *SdfIndexAllocator
	// Guards filtersByKey and, more importantly, the lifetime of a filter slot:
	// held for writing across the whole allocate/write/propagate/free sequence,
	// and by readers across the whole resolve-and-apply rather than just the map
	// lookup, or they apply an index freed behind them. The outermost of the
	// engine's locks: taken before Session.opMu and before mu, never while
	// holding either.
	filterMu     sync.RWMutex
	filtersByKey map[string]uint32

	classes *TrafficClasses
	radios  *Radios

	networkInstances map[string]uint8

	dnTagMu   sync.Mutex
	dnTags    map[string]*dnTagEntry
	dnTagUsed map[uint16]struct{}
	lastDNTag uint16

	endMarkers endMarkerSockets

	programs ebpf.ProgramWaiter
}

func (conn *SessionEngine) TrafficClasses() *TrafficClasses {
	if conn == nil {
		return nil
	}

	return conn.classes
}

func (conn *SessionEngine) Radios() *Radios {
	if conn == nil {
		return nil
	}

	return conn.radios
}

func (conn *SessionEngine) radioID(far models.FAR, existing uint16) uint16 {
	if conn.radios == nil || far.ForwardingParameters == nil || far.ForwardingParameters.OuterHeaderCreation == nil {
		return existing
	}

	return conn.radios.ID(far.ForwardingParameters.OuterHeaderCreation.RadioID)
}

func (conn *SessionEngine) trafficClassID(sess *Session, fiveQI uint8) uint16 {
	if conn.classes == nil {
		return 0
	}

	c, ok := sess.trafficClass(fiveQI)
	if !ok {
		return 0
	}

	return conn.classes.ID(c)
}

func (conn *SessionEngine) Close() error {
	return conn.endMarkers.Close()
}

func (pc *SessionEngine) ListSessions() map[uint64]*Session {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	sessCopy := make(map[uint64]*Session, len(pc.sessions))
	maps.Copy(sessCopy, pc.sessions)

	return sessCopy
}

func (pc *SessionEngine) eachSession(visit func(*Session) bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	for _, session := range pc.sessions {
		if !visit(session) {
			return
		}
	}
}

func (pc *SessionEngine) GetSession(seid uint64) *Session {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	session, ok := pc.sessions[seid]
	if !ok {
		return nil
	}

	return session
}

func (pc *SessionEngine) AddSession(seid uint64, session *Session) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.sessions[seid] = session
	pc.registerPolicy(session.PolicyID(), seid)
}

// registerPolicy links a policyID to a session SEID in the reverse index.
// Caller must hold pc.mu.
func (pc *SessionEngine) registerPolicy(policyID string, seid uint64) {
	if policyID == "" {
		return
	}

	seids, ok := pc.policyToSEIDs[policyID]
	if !ok {
		seids = make(map[uint64]struct{})
		pc.policyToSEIDs[policyID] = seids
	}

	seids[seid] = struct{}{}
}

// deregisterPolicy removes a session SEID from the reverse index.
// Caller must hold pc.mu.
func (pc *SessionEngine) deregisterPolicy(policyID string, seid uint64) {
	if policyID == "" {
		return
	}

	seids, ok := pc.policyToSEIDs[policyID]
	if !ok {
		return
	}

	delete(seids, seid)

	if len(seids) == 0 {
		delete(pc.policyToSEIDs, policyID)
	}
}

func (pc *SessionEngine) SetBPFObjects(ctx context.Context, bpfObjects *ebpf.BpfObjects, dbInstance *db.Database) {
	pc.mu.Lock()
	pc.BpfObjects = bpfObjects
	pc.mu.Unlock()

	if dbInstance != nil {
		if err := pc.InitializeFiltersFromDB(ctx, dbInstance); err != nil {
			logger.From(ctx, logger.DBLog).Warn(
				"failed to initialize filters from DB",
				zap.Error(err),
			)
		}
	}
}

func (pc *SessionEngine) InitializeFiltersFromDB(ctx context.Context, dbInstance *db.Database) error {
	policies, _, err := dbInstance.ListPoliciesPage(ctx, 1, 1000)
	if err != nil {
		logger.From(ctx, logger.DBLog).Error("failed to list policies", zap.Error(err))
		return nil
	}

	for _, policy := range policies {
		rules, err := dbInstance.ListRulesForPolicy(ctx, policy.ID)
		if err != nil {
			logger.From(ctx, logger.DBLog).Error(
				"failed to list rules for policy",
				zap.String("policy_id", policy.ID),
				zap.Error(err),
			)

			continue
		}

		uplinkRules := make([]models.FilterRule, 0)
		downlinkRules := make([]models.FilterRule, 0)

		for _, rule := range rules {
			filterRule := models.FilterRule{
				RemotePrefix: "",
				Protocol:     rule.Protocol,
				PortLow:      rule.PortLow,
				PortHigh:     rule.PortHigh,
				Action:       models.ActionFromString(rule.Action),
			}

			if rule.RemotePrefix != nil {
				filterRule.RemotePrefix = *rule.RemotePrefix
			}

			switch rule.Direction {
			case "uplink":
				uplinkRules = append(uplinkRules, filterRule)
			case "downlink":
				downlinkRules = append(downlinkRules, filterRule)
			}
		}

		if len(uplinkRules) > 0 {
			if err := pc.UpdateFilters(ctx, policy.ID, models.DirectionUplink, uplinkRules); err != nil {
				logger.From(ctx, logger.DBLog).Error(
					"failed to update uplink filters",
					zap.String("policy_id", policy.ID),
					zap.Error(err),
				)
			}
		}

		if len(downlinkRules) > 0 {
			if err := pc.UpdateFilters(ctx, policy.ID, models.DirectionDownlink, downlinkRules); err != nil {
				logger.From(ctx, logger.DBLog).Error(
					"failed to update downlink filters",
					zap.String("policy_id", policy.ID),
					zap.Error(err),
				)
			}
		}
	}

	return nil
}

func (pc *SessionEngine) GetAdvertisedN3Addresses() (netip.Addr, netip.Addr) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return pc.advertisedN3AddressIPv4, pc.advertisedN3AddressIPv6
}

func (pc *SessionEngine) SetAdvertisedN3Addresses(newN3AddrIPv4, newN3AddrIPv6 netip.Addr) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.advertisedN3AddressIPv4 = newN3AddrIPv4
	pc.advertisedN3AddressIPv6 = newN3AddrIPv6
}

func (pc *SessionEngine) SetN3VRFDevice(device string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.n3VRFDevice = device
}

func (pc *SessionEngine) N3VRFDevice() string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	return pc.n3VRFDevice
}

func NewSessionEngine(addr string, nodeID string, n3IPv4 netip.Addr, n3IPv6 netip.Addr, advertisedN3IPv4 netip.Addr, advertisedN3IPv6 netip.Addr, bpfObjects *ebpf.BpfObjects, resourceManager *FteIDResourceManager) (*SessionEngine, error) {
	addrV4 := net.ParseIP(addr)
	if addrV4 == nil {
		return nil, fmt.Errorf("failed to parse IP address ID: %s", addr)
	}

	conn := &SessionEngine{
		sessions:                make(map[uint64]*Session),
		policyToSEIDs:           make(map[string]map[uint64]struct{}),
		nodeID:                  nodeID,
		nodeAddrV4:              addrV4,
		n3AddressIPv4:           n3IPv4,
		n3AddressIPv6:           n3IPv6,
		advertisedN3AddressIPv4: advertisedN3IPv4,
		advertisedN3AddressIPv6: advertisedN3IPv6,
		BpfObjects:              bpfObjects,
		FteIDResourceManager:    resourceManager,
		SdfIndexAllocator:       NewSdfIndexAllocator(ebpf.MaxSdfFilters),
		filtersByKey:            make(map[string]uint32),
		classes:                 NewTrafficClasses(),
		radios:                  NewRadios(),
	}

	return conn, nil
}

func (connection *SessionEngine) ReleaseResources(seID uint64) {
	if connection.FteIDResourceManager != nil {
		connection.FteIDResourceManager.ReleaseAllTEIDs(seID)
	}
}

type dnTagEntry struct {
	tag  uint16
	refs int
}

var errDataNetworkTagsExhausted = errors.New("every data network tag is in use")

func (conn *SessionEngine) acquireDataNetworkTag(dataNetworkID string) (uint16, error) {
	if dataNetworkID == "" {
		return 0, nil
	}

	conn.dnTagMu.Lock()
	defer conn.dnTagMu.Unlock()

	if entry, ok := conn.dnTags[dataNetworkID]; ok {
		entry.refs++
		return entry.tag, nil
	}

	if conn.dnTags == nil {
		conn.dnTags = make(map[string]*dnTagEntry)
		conn.dnTagUsed = make(map[uint16]struct{})
	}

	for range math.MaxUint16 {
		conn.lastDNTag++
		if conn.lastDNTag == 0 {
			conn.lastDNTag = 1
		}

		if _, used := conn.dnTagUsed[conn.lastDNTag]; used {
			continue
		}

		conn.dnTags[dataNetworkID] = &dnTagEntry{tag: conn.lastDNTag, refs: 1}
		conn.dnTagUsed[conn.lastDNTag] = struct{}{}

		return conn.lastDNTag, nil
	}

	return 0, errDataNetworkTagsExhausted
}

func (conn *SessionEngine) releaseDataNetworkTag(dataNetworkID string) {
	if dataNetworkID == "" {
		return
	}

	conn.dnTagMu.Lock()
	defer conn.dnTagMu.Unlock()

	entry, ok := conn.dnTags[dataNetworkID]
	if !ok {
		return
	}

	entry.refs--
	if entry.refs > 0 {
		return
	}

	delete(conn.dnTags, dataNetworkID)
	delete(conn.dnTagUsed, entry.tag)
}

func (conn *SessionEngine) SetNetworkInstances(ids map[string]uint8) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	conn.networkInstances = ids
}

func (conn *SessionEngine) networkInstanceID(name string) (uint8, error) {
	if name == "" || name == models.DefaultNetworkInstanceName {
		return 0, nil
	}

	conn.mu.RLock()
	defer conn.mu.RUnlock()

	id, ok := conn.networkInstances[name]
	if !ok {
		return 0, fmt.Errorf("%w: %s", models.ErrNetworkInstanceUnavailable, name)
	}

	return id, nil
}

func (conn *SessionEngine) HasNetworkInstance(name string) bool {
	_, err := conn.networkInstanceID(name)

	return err == nil
}
