// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/etsi"
	amfContext "github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/producer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/ipam"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/upf"
	upfengine "github.com/ellanetworks/core/internal/upf/engine"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ella-core/runtime")

// ---------------------------------------------------------------------------
// leaseStoreAdapter bridges db.Database (which uses *db.IPLease) to
// ipam.LeaseStore (which uses *ipam.Lease), avoiding an import cycle.
// ---------------------------------------------------------------------------

type leaseStoreAdapter struct {
	db *db.Database
}

// mapDBError translates db sentinel errors to ipam sentinel errors so the
// allocator can use errors.Is() without importing the db package.
func mapDBError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, db.ErrNotFound):
		return ipam.ErrNotFound
	case errors.Is(err, db.ErrAlreadyExists):
		return ipam.ErrAlreadyExists
	default:
		return err
	}
}

// ---------------------------------------------------------------------------
// pcfDBAdapter adapts *db.Database to the smf.PCF interface.
// In 3GPP terms this is the Npcf_SMPolicyControl service backed by the
// local subscriber / policy database.
// ---------------------------------------------------------------------------

type pcfDBAdapter struct {
	db *db.Database
}

// NewPCFDBAdapter creates a new PCF database adapter.
func NewPCFDBAdapter(database *db.Database) smf.PCF {
	return &pcfDBAdapter{db: database}
}

func dbLeaseToIPAM(l *db.IPLease) *ipam.Lease {
	return &ipam.Lease{
		ID:        l.ID,
		PoolID:    l.PoolID,
		Address:   l.Address().String(),
		IMSI:      l.IMSI,
		SessionID: l.SessionID,
		Type:      l.Type,
		CreatedAt: l.CreatedAt,
		NodeID:    l.NodeID,
	}
}

func ipamLeaseToDB(l *ipam.Lease) *db.IPLease {
	return &db.IPLease{
		ID:        l.ID,
		PoolID:    l.PoolID,
		IMSI:      l.IMSI,
		SessionID: l.SessionID,
		Type:      l.Type,
		CreatedAt: l.CreatedAt,
		NodeID:    l.NodeID,
	}
}

func (a *leaseStoreAdapter) GetDynamicLease(ctx context.Context, poolID int, imsi string) (*ipam.Lease, error) {
	l, err := a.db.GetDynamicLease(ctx, poolID, imsi)
	if err != nil {
		return nil, mapDBError(err)
	}

	return dbLeaseToIPAM(l), nil
}

func (a *leaseStoreAdapter) GetLeaseBySession(ctx context.Context, poolID int, sessionID int, imsi string) (*ipam.Lease, error) {
	l, err := a.db.GetLeaseBySession(ctx, poolID, sessionID, imsi)
	if err != nil {
		return nil, mapDBError(err)
	}

	return dbLeaseToIPAM(l), nil
}

func (a *leaseStoreAdapter) ListLeaseAddressesByPool(ctx context.Context, poolID int) ([]string, error) {
	return a.db.ListLeaseAddressesByPool(ctx, poolID)
}

func (a *leaseStoreAdapter) CreateLease(ctx context.Context, lease *ipam.Lease) error {
	addr, err := netip.ParseAddr(lease.Address)
	if err != nil {
		return fmt.Errorf("invalid IP address %q: %w", lease.Address, err)
	}

	return mapDBError(a.db.CreateLease(ctx, ipamLeaseToDB(lease), addr))
}

func (a *leaseStoreAdapter) UpdateLeaseSession(ctx context.Context, leaseID int, sessionID int) error {
	return mapDBError(a.db.UpdateLeaseSession(ctx, leaseID, sessionID))
}

func (a *leaseStoreAdapter) UpdateLeaseNode(ctx context.Context, leaseID int, nodeID int, sessionID int) error {
	return mapDBError(a.db.UpdateLeaseNode(ctx, leaseID, nodeID, sessionID))
}

func (a *leaseStoreAdapter) DeleteDynamicLease(ctx context.Context, leaseID int) error {
	return mapDBError(a.db.DeleteDynamicLease(ctx, leaseID))
}

// ---------------------------------------------------------------------------
// smfDBAdapter adapts *db.Database to the smf.SessionStore interface.
// ---------------------------------------------------------------------------

type smfDBAdapter struct {
	db        *db.Database
	allocator ipam.Allocator
}

// resolvePoolByDNN looks up the IP pool for a data network by name.
func (a *smfDBAdapter) resolvePoolByDNN(ctx context.Context, dnn string) (ipam.Pool, error) {
	dn, err := a.db.GetDataNetwork(ctx, dnn)
	if err != nil {
		return ipam.Pool{}, fmt.Errorf("get data network: %w", err)
	}

	return ipam.NewPool(dn.ID, dn.IPPool)
}

func (a *smfDBAdapter) AllocateIP(ctx context.Context, imsi string, dnn string, pduSessionID uint8) (netip.Addr, error) {
	ctx, span := tracer.Start(ctx, "smf/allocate_ip",
		trace.WithAttributes(
			attribute.String("imsi", imsi),
			attribute.String("dnn", dnn),
			attribute.Int("pdu_session_id", int(pduSessionID)),
		),
	)
	defer span.End()

	pool, err := a.resolvePoolByDNN(ctx, dnn)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "resolve pool failed")

		return netip.Addr{}, fmt.Errorf("resolve pool: %w", err)
	}

	addr, err := a.allocator.Allocate(ctx, pool, imsi, int(pduSessionID), a.db.NodeID())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "allocate failed")

		return netip.Addr{}, err
	}

	span.SetAttributes(attribute.String("ip", addr.String()))

	return addr, nil
}

func (a *smfDBAdapter) ReleaseIP(ctx context.Context, imsi string, dnn string, pduSessionID uint8) (netip.Addr, error) {
	ctx, span := tracer.Start(ctx, "smf/release_ip",
		trace.WithAttributes(
			attribute.String("imsi", imsi),
			attribute.String("dnn", dnn),
			attribute.Int("pdu_session_id", int(pduSessionID)),
		),
	)
	defer span.End()

	pool, err := a.resolvePoolByDNN(ctx, dnn)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "resolve pool failed")

		return netip.Addr{}, fmt.Errorf("resolve pool: %w", err)
	}

	addr, err := a.allocator.Release(ctx, pool, int(pduSessionID), imsi)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "release failed")

		return netip.Addr{}, err
	}

	return addr, nil
}

func (a *pcfDBAdapter) GetSessionPolicy(ctx context.Context, imsi string, snssai *models.Snssai, dnn string) (*smf.Policy, error) {
	pol, dbRules, dn, err := a.db.GetSessionPolicy(ctx, imsi, snssai.Sst, snssai.Sd, dnn)
	if err != nil {
		if errors.Is(err, db.ErrDataNetworkNotFound) {
			return nil, fmt.Errorf("%w: %v", smf.ErrDNNNotFound, err)
		}

		return nil, fmt.Errorf("get session policy: %w", err)
	}

	dns := net.ParseIP(dn.DNS)

	policy := &smf.Policy{
		PolicyID: int64(pol.ID),
		Ambr: models.Ambr{
			Uplink:   pol.SessionAmbrUplink,
			Downlink: pol.SessionAmbrDownlink,
		},
		QosData: models.QosData{
			QFI:    1,
			Var5qi: pol.Var5qi,
			Arp: &models.Arp{
				PriorityLevel: pol.Arp,
			},
		},
		DNS: dns,
		MTU: uint16(dn.MTU),
	}

	resolvedRules := make([]*smf.ResolvedNetworkRule, len(dbRules))
	for i, dbRule := range dbRules {
		dir, err := models.ParseDirection(dbRule.Direction)
		if err != nil {
			return nil, fmt.Errorf("invalid direction for rule %d: %w", dbRule.ID, err)
		}

		resolvedRules[i] = &smf.ResolvedNetworkRule{
			Description:  dbRule.Description,
			PolicyID:     dbRule.PolicyID,
			Direction:    dir,
			RemotePrefix: dbRule.RemotePrefix,
			Protocol:     dbRule.Protocol,
			PortLow:      dbRule.PortLow,
			PortHigh:     dbRule.PortHigh,
			Action:       dbRule.Action,
			Precedence:   dbRule.Precedence,
		}
	}

	policy.NetworkRules = resolvedRules

	return policy, nil
}

func (a *smfDBAdapter) IncrementDailyUsage(ctx context.Context, imsi string, uplinkBytes, downlinkBytes uint64) error {
	epochDay := time.Now().UTC().Unix() / 86400

	return a.db.IncrementDailyUsage(ctx, db.DailyUsage{
		EpochDay:      epochDay,
		IMSI:          imsi,
		BytesUplink:   int64(uplinkBytes),
		BytesDownlink: int64(downlinkBytes),
	})
}

func (a *smfDBAdapter) InsertFlowReports(ctx context.Context, reports []*models.FlowReportRequest) error {
	batch := make([]*dbwriter.FlowReport, len(reports))

	for i, report := range reports {
		batch[i] = &dbwriter.FlowReport{
			SubscriberID:    report.IMSI,
			SourceIP:        report.SourceIP,
			DestinationIP:   report.DestinationIP,
			SourcePort:      report.SourcePort,
			DestinationPort: report.DestinationPort,
			Protocol:        report.Protocol,
			Packets:         report.Packets,
			Bytes:           report.Bytes,
			StartTime:       report.StartTime,
			EndTime:         report.EndTime,
			Direction:       report.Direction.String(),
			Action:          int(report.Action),
		}
	}

	return a.db.InsertFlowReports(ctx, batch)
}

// ---------------------------------------------------------------------------
// smfUPFAdapter adapts the in-process UPF calls to smf.UPFClient.
// ---------------------------------------------------------------------------

type smfUPFAdapter struct {
	engine *upfengine.SessionEngine
	upf    *upf.UPF
}

func (a *smfUPFAdapter) EstablishSession(ctx context.Context, req *models.EstablishRequest) (*models.EstablishResponse, error) {
	return a.engine.EstablishSession(ctx, req)
}

func (a *smfUPFAdapter) ModifySession(ctx context.Context, req *models.ModifyRequest) error {
	return a.engine.ModifySession(ctx, req)
}

func (a *smfUPFAdapter) FlushUsage(ctx context.Context, remoteSEID uint64) {
	if a.upf != nil {
		a.upf.FlushUsage(ctx, remoteSEID)
	}
}

func (a *smfUPFAdapter) DeleteSession(ctx context.Context, remoteSEID uint64) error {
	return a.engine.DeleteSession(ctx, &models.DeleteRequest{SEID: remoteSEID})
}

func (a *smfUPFAdapter) UpdateFilters(ctx context.Context, policyID int64, direction models.Direction, rules []models.FilterRule) error {
	return a.engine.UpdateFilters(ctx, policyID, direction, rules)
}

// ---------------------------------------------------------------------------
// smfAMFAdapter adapts AMF methods to smf.AMFCallback.
// ---------------------------------------------------------------------------

type smfAMFAdapter struct {
	amf *amfContext.AMF
}

func (a *smfAMFAdapter) TransferN1(ctx context.Context, supi etsi.SUPI, n1Msg []byte, pduSessionID uint8) error {
	return producer.TransferN1Msg(ctx, a.amf, supi, n1Msg, pduSessionID)
}

func (a *smfAMFAdapter) TransferN1N2(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n1Msg, n2Msg []byte) error {
	return producer.TransferN1N2Message(ctx, a.amf, supi, models.N1N2MessageTransferRequest{
		PduSessionID:            pduSessionID,
		SNssai:                  snssai,
		BinaryDataN1Message:     n1Msg,
		BinaryDataN2Information: n2Msg,
	})
}

func (a *smfAMFAdapter) N2TransferOrPage(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n2Msg []byte) error {
	return producer.N2MessageTransferOrPage(ctx, a.amf, supi, models.N1N2MessageTransferRequest{
		PduSessionID:            pduSessionID,
		SNssai:                  snssai,
		BinaryDataN2Information: n2Msg,
	})
}
