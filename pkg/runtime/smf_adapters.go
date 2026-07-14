// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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

// ---------------------------------------------------------------------------
// smfDBAdapter adapts *db.Database to the smf.SessionStore interface.
// ---------------------------------------------------------------------------

type smfDBAdapter struct {
	db *db.Database
}

// resolvePoolByDNN looks up the IP pool for a data network by name.
func (a *smfDBAdapter) resolvePoolByDNN(ctx context.Context, dnn string) (ipam.Pool, error) {
	dn, err := a.db.GetDataNetwork(ctx, dnn)
	if err != nil {
		return ipam.Pool{}, fmt.Errorf("get data network: %w", err)
	}

	return ipam.NewPool(dn.ID, dn.IPv4Pool)
}

// resolveIPv6PoolByDNN looks up the IPv6 prefix delegation pool for a data
// network by name. It delegates /64 prefixes from the configured IPv6 CIDR.
func (a *smfDBAdapter) resolveIPv6PoolByDNN(ctx context.Context, dnn string) (ipam.Pool, error) {
	dn, err := a.db.GetDataNetwork(ctx, dnn)
	if err != nil {
		return ipam.Pool{}, fmt.Errorf("get data network: %w", err)
	}

	if dn.IPv6Pool == "" {
		return ipam.Pool{}, fmt.Errorf("data network %q has no IPv6 pool configured", dnn)
	}

	return ipam.NewPool6(dn.ID, dn.IPv6Pool, 64)
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

	// AllocateIPLease runs the SELECT-then-INSERT atomically on the
	// leader inside leaderCaptureAndPropose's proposeMu, so concurrent
	// allocations from any node serialise correctly. The legacy
	// pre-pick-on-follower path is gone.
	addr, err := a.db.AllocateIPLease(ctx, pool.ID, pool.IPVersion, imsi, int(pduSessionID), a.db.NodeID())
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

	lease, err := a.db.GetLeaseBySession(ctx, pool.ID, pool.IPVersion, int(pduSessionID), imsi)
	if err != nil {
		// Reservation deleted while bound: nothing left to free.
		if errors.Is(err, db.ErrNotFound) {
			return netip.Addr{}, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "lookup failed")

		return netip.Addr{}, fmt.Errorf("lookup lease: %w", err)
	}

	if err := a.releaseLease(ctx, lease); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "release failed")

		return netip.Addr{}, err
	}

	return lease.Address(), nil
}

// releaseLease clears a static reservation to reserved (row kept) or
// deletes a dynamic lease. The explicit clear is required so
// listActiveLeases / BGP (sessionID IS NOT NULL) drop the address.
func (a *smfDBAdapter) releaseLease(ctx context.Context, lease *db.IPLease) error {
	if lease.Type == "static" {
		if err := a.db.ClearStaticLeaseSession(ctx, lease.ID); err != nil {
			return fmt.Errorf("clear static lease session: %w", err)
		}

		return nil
	}

	if err := a.db.DeleteDynamicLease(ctx, lease.ID); err != nil {
		return fmt.Errorf("delete lease: %w", err)
	}

	return nil
}

func (a *smfDBAdapter) AllocateIPv6(ctx context.Context, imsi string, dnn string, pduSessionID uint8) (netip.Addr, error) {
	ctx, span := tracer.Start(ctx, "smf/allocate_ipv6",
		trace.WithAttributes(
			attribute.String("imsi", imsi),
			attribute.String("dnn", dnn),
			attribute.Int("pdu_session_id", int(pduSessionID)),
		),
	)
	defer span.End()

	pool, err := a.resolveIPv6PoolByDNN(ctx, dnn)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "resolve IPv6 pool failed")

		return netip.Addr{}, fmt.Errorf("resolve IPv6 pool: %w", err)
	}

	addr, err := a.db.AllocateIPv6Lease(ctx, pool.ID, pool.IPVersion, imsi, int(pduSessionID), a.db.NodeID())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "allocate IPv6 failed")

		return netip.Addr{}, err
	}

	span.SetAttributes(attribute.String("ipv6", addr.String()))

	return addr, nil
}

func (a *smfDBAdapter) ReleaseIPv6(ctx context.Context, imsi string, dnn string, pduSessionID uint8) (netip.Addr, error) {
	ctx, span := tracer.Start(ctx, "smf/release_ipv6",
		trace.WithAttributes(
			attribute.String("imsi", imsi),
			attribute.String("dnn", dnn),
			attribute.Int("pdu_session_id", int(pduSessionID)),
		),
	)
	defer span.End()

	pool, err := a.resolveIPv6PoolByDNN(ctx, dnn)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "resolve IPv6 pool failed")

		return netip.Addr{}, fmt.Errorf("resolve IPv6 pool: %w", err)
	}

	lease, err := a.db.GetLeaseBySession(ctx, pool.ID, pool.IPVersion, int(pduSessionID), imsi)
	if err != nil {
		// Reservation deleted while bound: nothing left to free.
		if errors.Is(err, db.ErrNotFound) {
			return netip.Addr{}, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "lookup failed")

		return netip.Addr{}, fmt.Errorf("lookup lease: %w", err)
	}

	if err := a.releaseLease(ctx, lease); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "release IPv6 failed")

		return netip.Addr{}, err
	}

	return lease.Address(), nil
}

func (a *pcfDBAdapter) GetSessionPolicy(ctx context.Context, imsi string, snssai *models.Snssai, dnn string) (*smf.Policy, error) {
	pol, dbRules, dn, err := a.db.GetSessionPolicy(ctx, imsi, snssai.Sst, snssai.Sd, dnn)
	if err != nil {
		if errors.Is(err, db.ErrDataNetworkNotFound) {
			return nil, fmt.Errorf("%w: %v", smf.ErrDNNNotFound, err)
		}

		if errors.Is(err, db.ErrDNNNotInSlice) {
			return nil, fmt.Errorf("%w: %v", smf.ErrDNNNotInSlice, err)
		}

		if errors.Is(err, db.ErrNoMatchingPolicy) {
			return nil, fmt.Errorf("%w: %v", smf.ErrNoPolicyMatch, err)
		}

		return nil, fmt.Errorf("get session policy: %w", err)
	}

	dns := net.ParseIP(dn.DNS)

	policy := &smf.Policy{
		PolicyID: pol.ID,
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
		DNS:      dns,
		MTU:      uint16(dn.MTU),
		IPv4Pool: dn.IPv4Pool,
		IPv6Pool: dn.IPv6Pool,
	}

	resolvedRules := make([]*smf.ResolvedNetworkRule, len(dbRules))
	for i, dbRule := range dbRules {
		dir, err := models.ParseDirection(dbRule.Direction)
		if err != nil {
			return nil, fmt.Errorf("invalid direction for rule %s: %w", dbRule.ID, err)
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

// ListFramedRoutes resolves the data network by name, then returns the
// subscriber's framed-route prefixes on it (TS 23.501 §5.6.14). Prefixes are
// stored normalized, so parsing is total.
func (a *smfDBAdapter) ListFramedRoutes(ctx context.Context, imsi string, dnn string) ([]netip.Prefix, error) {
	dn, err := a.db.GetDataNetwork(ctx, dnn)
	if err != nil {
		return nil, fmt.Errorf("get data network: %w", err)
	}

	rows, err := a.db.ListFramedRoutesBySubscriberDataNetwork(ctx, imsi, dn.ID)
	if err != nil {
		return nil, fmt.Errorf("list framed routes: %w", err)
	}

	prefixes := make([]netip.Prefix, 0, len(rows))

	for i := range rows {
		p, err := netip.ParsePrefix(rows[i].Prefix)
		if err != nil {
			return nil, fmt.Errorf("parse framed route %q: %w", rows[i].Prefix, err)
		}

		prefixes = append(prefixes, p)
	}

	return prefixes, nil
}

// GetStaticIP returns the reserved static address for the DNN and family, and
// whether one exists (a missing reservation is not an error).
func (a *smfDBAdapter) GetStaticIP(ctx context.Context, imsi string, dnn string, ipv6 bool) (netip.Addr, bool, error) {
	var (
		pool ipam.Pool
		err  error
	)

	if ipv6 {
		pool, err = a.resolveIPv6PoolByDNN(ctx, dnn)
	} else {
		pool, err = a.resolvePoolByDNN(ctx, dnn)
	}

	if err != nil {
		return netip.Addr{}, false, fmt.Errorf("resolve pool: %w", err)
	}

	lease, err := a.db.GetStaticLease(ctx, pool.ID, pool.IPVersion, imsi)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return netip.Addr{}, false, nil
		}

		return netip.Addr{}, false, fmt.Errorf("get static lease: %w", err)
	}

	return lease.Address(), true, nil
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

func (a *smfUPFAdapter) UpdateFilters(ctx context.Context, policyID string, direction models.Direction, rules []models.FilterRule) error {
	return a.engine.UpdateFilters(ctx, policyID, direction, rules)
}

func (a *smfUPFAdapter) RegisterIPv6Session(_ context.Context, reg *models.IPv6SessionRegistration) error {
	if a.upf == nil {
		return nil
	}

	a.upf.RegisterIPv6Session(reg.UplinkTEID, &upf.IPv6SessionContext{
		DownlinkTEID: reg.DownlinkTEID,
		GnbN3Addr:    reg.GnbN3Addr,
		Prefix:       reg.Prefix,
		MTU:          reg.MTU,
		QFI:          reg.QFI,
		S1U:          reg.S1U,
	})

	return nil
}

func (a *smfUPFAdapter) UnregisterIPv6Session(_ context.Context, ulTEID uint32) error {
	if a.upf == nil {
		return nil
	}

	a.upf.UnregisterIPv6Session(ulTEID)

	return nil
}

// ---------------------------------------------------------------------------
// smfAMFAdapter adapts AMF methods to smf.AMFCallback.
// ---------------------------------------------------------------------------

type smfAMFAdapter struct {
	amf *amfContext.AMF
}

func (a *smfAMFAdapter) TransferN1(ctx context.Context, supi etsi.SUPI, n1Msg []byte, pduSessionID uint8) error {
	return a.amf.TransferN1Msg(ctx, supi, n1Msg, pduSessionID)
}

func (a *smfAMFAdapter) TransferN1N2(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n1Msg, n2Msg []byte) error {
	return a.amf.TransferN1N2Message(ctx, supi, models.N1N2MessageTransferRequest{
		PduSessionID:            pduSessionID,
		SNssai:                  snssai,
		BinaryDataN1Message:     n1Msg,
		BinaryDataN2Information: n2Msg,
	})
}

func (a *smfAMFAdapter) ModifyN1N2(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, n1Msg, n2Msg []byte) error {
	err := a.amf.ModifyN1N2Message(ctx, supi, pduSessionID, n1Msg, n2Msg)
	if errors.Is(err, amfContext.ErrUENotReachable) {
		return smf.ErrUENotReachable
	}

	return err
}

func (a *smfAMFAdapter) ReleaseSession(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, n1Msg, n2Transfer []byte) error {
	err := a.amf.ReleaseSessionMessage(ctx, supi, pduSessionID, n1Msg, n2Transfer)
	if errors.Is(err, amfContext.ErrUENotReachable) {
		return smf.ErrUENotReachable
	}

	return err
}

func (a *smfAMFAdapter) N2TransferOrPage(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n2Msg []byte) error {
	return a.amf.N2MessageTransferOrPage(ctx, supi, models.N1N2MessageTransferRequest{
		PduSessionID:            pduSessionID,
		SNssai:                  snssai,
		BinaryDataN2Information: n2Msg,
	})
}
