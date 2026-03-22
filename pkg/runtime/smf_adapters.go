// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/etsi"
	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/producer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/wmnsk/go-pfcp/ie"
)

// ---------------------------------------------------------------------------
// smfDBAdapter adapts *db.Database to the smf.SessionStore interface.
// ---------------------------------------------------------------------------

type smfDBAdapter struct {
	db *db.Database
}

func (a *smfDBAdapter) AllocateIP(ctx context.Context, supi string) (net.IP, error) {
	return a.db.AllocateIP(ctx, supi)
}

func (a *smfDBAdapter) ReleaseIP(ctx context.Context, supi string) error {
	return a.db.ReleaseIP(ctx, supi)
}

func (a *smfDBAdapter) GetSubscriberPolicy(ctx context.Context, imsi string) (*smf.Policy, error) {
	sub, err := a.db.GetSubscriber(ctx, imsi)
	if err != nil {
		return nil, fmt.Errorf("get subscriber: %w", err)
	}

	pol, err := a.db.GetPolicyByID(ctx, sub.PolicyID)
	if err != nil {
		return nil, fmt.Errorf("get policy: %w", err)
	}

	return &smf.Policy{
		Ambr: models.Ambr{
			Uplink:   pol.BitrateUplink,
			Downlink: pol.BitrateDownlink,
		},
		QosData: models.QosData{
			QFI:    1,
			Var5qi: pol.Var5qi,
			Arp: &models.Arp{
				PriorityLevel: pol.Arp,
			},
		},
	}, nil
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
		Direction:       report.Direction,
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

	rsp, err := a.dispatcher.UPF.HandlePfcpSessionEstablishmentRequest(ctx, pfcpMsg)
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

	rsp, err := a.dispatcher.UPF.HandlePfcpSessionModificationRequest(ctx, pfcpMsg)
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
