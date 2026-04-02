// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/etsi"
	amfContext "github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/producer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/ipam"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	"github.com/ellanetworks/core/internal/smf"
	upf_pfcp "github.com/ellanetworks/core/internal/upf/core"
	"github.com/wmnsk/go-pfcp/ie"
)

// ---------------------------------------------------------------------------
// smfDBAdapter adapts *db.Database to the smf.SessionStore interface.
// ---------------------------------------------------------------------------

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

// NewSMFDBAdapter creates a new SMF database adapter.
func NewSMFDBAdapter(database *db.Database) smf.SessionStore {
	return &smfDBAdapter{db: database}
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
	pool, err := a.resolvePoolByDNN(ctx, dnn)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("resolve pool: %w", err)
	}

	return a.allocator.Allocate(ctx, pool, imsi, int(pduSessionID))
}

func (a *smfDBAdapter) ReleaseIP(ctx context.Context, imsi string, dnn string, pduSessionID uint8) (netip.Addr, error) {
	pool, err := a.resolvePoolByDNN(ctx, dnn)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("resolve pool: %w", err)
	}

	return a.allocator.Release(ctx, pool, int(pduSessionID), imsi)
}

func (a *smfDBAdapter) GetSessionPolicy(ctx context.Context, imsi string, snssai *models.Snssai, dnn string) (*smf.Policy, error) {
	pol, dbRules, err := a.db.GetSessionPolicy(ctx, imsi, snssai.Sst, snssai.Sd, dnn)
	if err != nil {
		return nil, fmt.Errorf("get session policy: %w", err)
	}

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
	}

	resolvedRules := make([]*smf.ResolvedNetworkRule, len(dbRules))
	for i, dbRule := range dbRules {
		dir, err := smf.ParseDirection(dbRule.Direction)
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

func (a *smfDBAdapter) GetDataNetwork(ctx context.Context, _ *models.Snssai, dnn string) (*smf.DataNetworkInfo, error) {
	dn, err := a.db.GetDataNetwork(ctx, dnn)
	if err != nil {
		return nil, err
	}

	dns := net.ParseIP(dn.DNS)

	return &smf.DataNetworkInfo{
		DNS: dns,
		MTU: uint16(dn.MTU),
	}, nil
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

func (a *smfDBAdapter) InsertFlowReport(ctx context.Context, report *smf.FlowReport) error {
	return a.db.InsertFlowReport(ctx, &dbwriter.FlowReport{
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
	})
}

// ---------------------------------------------------------------------------
// smfUPFAdapter adapts the in-process PFCP dispatcher to smf.UPFClient.
// ---------------------------------------------------------------------------

var sessionSeq uint32

type smfUPFAdapter struct {
	dispatcher *pfcp_dispatcher.PfcpDispatcher
	nodeID     net.IP
}

func (a *smfUPFAdapter) EstablishSession(ctx context.Context, req *smf.PFCPEstablishmentRequest) (*smf.PFCPEstablishmentResponse, error) {
	seq := atomic.AddUint32(&sessionSeq, 1)

	pfcpMsg, err := smf.BuildPfcpSessionEstablishmentRequest(
		seq,
		req.NodeID.String(),
		req.NodeID,
		req.LocalSEID,
		req.PDRs,
		req.FARs,
		req.QERs,
		req.URRs,
		req.SUPI,
	)
	if err != nil {
		return nil, fmt.Errorf("build PFCP establishment request: %w", err)
	}

	rsp, err := upf_pfcp.HandlePfcpSessionEstablishmentRequest(ctx, pfcpMsg, req.FilterIndexByPDRID)
	if err != nil {
		return nil, fmt.Errorf("PFCP establishment: %w", err)
	}

	if rsp.UPFSEID == nil {
		return nil, fmt.Errorf("PFCP establishment response missing UPF SEID")
	}

	fseid, err := rsp.UPFSEID.FSEID()
	if err != nil {
		return nil, fmt.Errorf("parse FSEID: %w", err)
	}

	fteid, err := findFTEID(rsp.CreatedPDR)
	if err != nil {
		return nil, fmt.Errorf("parse FTEID: %w", err)
	}

	if rsp.Cause == nil {
		return nil, fmt.Errorf("PFCP establishment response missing Cause")
	}

	causeValue, err := rsp.Cause.Cause()
	if err != nil {
		return nil, fmt.Errorf("parse Cause: %w", err)
	}

	if causeValue != ie.CauseRequestAccepted {
		return nil, fmt.Errorf("PFCP establishment rejected: cause %d", causeValue)
	}

	return &smf.PFCPEstablishmentResponse{
		RemoteSEID: fseid.SEID,
		TEID:       fteid.TEID,
		N3IP:       fteid.IPv4Address,
	}, nil
}

func (a *smfUPFAdapter) ModifySession(ctx context.Context, req *smf.PFCPModificationRequest) error {
	seq := atomic.AddUint32(&sessionSeq, 1)

	pfcpMsg, err := smf.BuildPfcpSessionModificationRequest(
		seq,
		req.LocalSEID,
		req.RemoteSEID,
		a.nodeID,
		req.PDRs,
		req.FARs,
		req.QERs,
	)
	if err != nil {
		return fmt.Errorf("build PFCP modification request: %w", err)
	}

	rsp, err := upf_pfcp.HandlePfcpSessionModificationRequest(ctx, pfcpMsg, req.FilterIndexByPDRID)
	if err != nil {
		return fmt.Errorf("PFCP modification: %w", err)
	}

	if rsp.Cause == nil {
		return fmt.Errorf("PFCP modification response missing Cause")
	}

	causeValue, err := rsp.Cause.Cause()
	if err != nil {
		return fmt.Errorf("parse Cause: %w", err)
	}

	if causeValue != ie.CauseRequestAccepted {
		return fmt.Errorf("PFCP modification rejected: cause %d", causeValue)
	}

	return nil
}

func (a *smfUPFAdapter) DeleteSession(ctx context.Context, localSEID, remoteSEID uint64) error {
	seq := atomic.AddUint32(&sessionSeq, 1)
	pfcpMsg := smf.BuildPfcpSessionDeletionRequest(seq, localSEID, remoteSEID, a.nodeID)

	rsp, err := a.dispatcher.UPF.HandlePfcpSessionDeletionRequest(ctx, pfcpMsg)
	if err != nil {
		return fmt.Errorf("PFCP deletion: %w", err)
	}

	if rsp.Cause == nil {
		return fmt.Errorf("PFCP deletion response missing Cause")
	}

	causeValue, err := rsp.Cause.Cause()
	if err != nil {
		return fmt.Errorf("parse Cause: %w", err)
	}

	if causeValue != ie.CauseRequestAccepted {
		return fmt.Errorf("PFCP deletion rejected: cause %d", causeValue)
	}

	return nil
}

func (a *smfUPFAdapter) UpdateFilters(ctx context.Context, req *smf.FilterUpdateRequest) (*smf.FilterUpdateResponse, error) {
	rules := make([]upf_pfcp.UpdateFilterRule, 0, len(req.Rules))
	for _, r := range req.Rules {
		remote := ""
		if r.RemotePrefix != nil {
			remote = *r.RemotePrefix
		}

		rules = append(rules, upf_pfcp.UpdateFilterRule{
			RemotePrefix: remote,
			Protocol:     r.Protocol,
			PortLow:      r.PortLow,
			PortHigh:     r.PortHigh,
			Action:       upf_pfcp.StringToAction(r.Action),
		})
	}

	conn := upf_pfcp.GetConnection()

	resp, err := upf_pfcp.UpdateFilters(conn, upf_pfcp.UpdateFiltersRequest{
		PolicyID:  req.PolicyID,
		Direction: req.Direction,
		Rules:     rules,
	})
	if err != nil {
		return nil, err
	}

	return &smf.FilterUpdateResponse{FilterMapIndex: resp.FilterMapIndex}, nil
}

func (a *smfUPFAdapter) ReleaseFilter(ctx context.Context, index uint32) error {
	conn := upf_pfcp.GetConnection()
	return upf_pfcp.ReleaseFilter(conn, index)
}

func findFTEID(createdPDRIEs []*ie.IE) (*ie.FTEIDFields, error) {
	for _, createdPDRIE := range createdPDRIEs {
		teid, err := createdPDRIE.FTEID()
		if err == nil {
			return teid, nil
		}
	}

	return nil, fmt.Errorf("FTEID not found in CreatedPDR")
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
