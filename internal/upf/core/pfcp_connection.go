// Copyright 2024 Ella Networks
package core

import (
	"context"
	"fmt"
	"maps"
	"net"
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
)

var connection *PfcpConnection

type PfcpConnection struct {
	mu sync.Mutex

	sessions             map[uint64]*Session
	nodeID               string
	nodeAddrV4           net.IP
	n3Address            net.IP
	advertisedN3Address  net.IP
	BpfObjects           *ebpf.BpfObjects
	FteIDResourceManager *FteIDResourceManager
	SdfIndexAllocator    *SdfIndexAllocator
	filterMu             sync.Mutex
	filtersByKey         map[string]uint32
}

func (pc *PfcpConnection) ListSessions() map[uint64]*Session {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	sessCopy := make(map[uint64]*Session, len(pc.sessions))
	maps.Copy(sessCopy, pc.sessions)

	return sessCopy
}

func (pc *PfcpConnection) GetSession(seid uint64) *Session {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	session, ok := pc.sessions[seid]
	if !ok {
		return nil
	}

	return session
}

func (pc *PfcpConnection) DeleteSession(seid uint64) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	delete(pc.sessions, seid)
}

func (pc *PfcpConnection) AddSession(seid uint64, session *Session) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.sessions[seid] = session
}

func (pc *PfcpConnection) SetBPFObjects(bpfObjects *ebpf.BpfObjects, dbInstance *db.Database) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.BpfObjects = bpfObjects

	if dbInstance != nil {
		if err := pc.InitializeFiltersFromDB(dbInstance); err != nil {
			logger.WithTrace(context.Background(), logger.DBLog).Warn(
				"failed to initialize filters from DB",
				zap.Error(err),
			)
		}
	}
}

func (pc *PfcpConnection) InitializeFiltersFromDB(dbInstance *db.Database) error {
	ctx := context.Background()

	policies, _, err := dbInstance.ListPoliciesPage(ctx, 1, 1000)
	if err != nil {
		logger.WithTrace(ctx, logger.DBLog).Error("failed to list policies", zap.Error(err))
		return nil
	}

	for _, policy := range policies {
		rules, err := dbInstance.ListRulesForPolicy(ctx, int64(policy.ID))
		if err != nil {
			logger.WithTrace(ctx, logger.DBLog).Error(
				"failed to list rules for policy",
				zap.Int("policyID", policy.ID),
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
			if err := UpdateFilters(pc, int64(policy.ID), "uplink", uplinkRules); err != nil {
				logger.WithTrace(ctx, logger.DBLog).Error(
					"failed to update uplink filters",
					zap.Int("policyID", policy.ID),
					zap.Error(err),
				)
			}
		}

		if len(downlinkRules) > 0 {
			if err := UpdateFilters(pc, int64(policy.ID), "downlink", downlinkRules); err != nil {
				logger.WithTrace(ctx, logger.DBLog).Error(
					"failed to update downlink filters",
					zap.Int("policyID", policy.ID),
					zap.Error(err),
				)
			}
		}
	}

	return nil
}

func (pc *PfcpConnection) GetAdvertisedN3Address() net.IP {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	return pc.advertisedN3Address
}

func (pc *PfcpConnection) SetAdvertisedN3Address(newN3Addr net.IP) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.advertisedN3Address = newN3Addr
}

func CreatePfcpConnection(addr string, nodeID string, n3Ip string, advertisedN3Ip string, bpfObjects *ebpf.BpfObjects, resourceManager *FteIDResourceManager) (*PfcpConnection, error) {
	addrV4 := net.ParseIP(addr)
	if addrV4 == nil {
		return nil, fmt.Errorf("failed to parse IP address ID: %s", addr)
	}

	n3Addr := net.ParseIP(n3Ip)
	if n3Addr == nil {
		return nil, fmt.Errorf("failed to parse N3 IP address ID: %s", n3Ip)
	}

	advertisedN3Addr := net.ParseIP(advertisedN3Ip)
	if advertisedN3Addr == nil {
		return nil, fmt.Errorf("failed to parse advertised N3 IP address ID: %s", advertisedN3Ip)
	}

	connection = &PfcpConnection{
		sessions:             make(map[uint64]*Session),
		nodeID:               nodeID,
		nodeAddrV4:           addrV4,
		n3Address:            n3Addr,
		advertisedN3Address:  advertisedN3Addr,
		BpfObjects:           bpfObjects,
		FteIDResourceManager: resourceManager,
		SdfIndexAllocator:    NewSdfIndexAllocator(ebpf.MaxSdfFilters),
		filtersByKey:         make(map[string]uint32),
	}

	return connection, nil
}

func GetConnection() *PfcpConnection {
	return connection
}

func (connection *PfcpConnection) ReleaseResources(seID uint64) {
	if connection.FteIDResourceManager == nil {
		return
	}

	if connection.FteIDResourceManager != nil {
		connection.FteIDResourceManager.ReleaseTEID(seID)
	}
}
