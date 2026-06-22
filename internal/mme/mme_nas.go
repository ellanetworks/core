// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"net/netip"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// handleInitialUEMessage routes a UE's first NAS message on a new S1 association
// (TS 36.413). A SERVICE REQUEST re-establishes an existing EMM-IDLE
// context (resolved by S-TMSI); anything else (an Attach Request) starts a new
// one.
func (m *MME) handleInitialUEMessage(ctx context.Context, conn *sctp.SCTPConn, value []byte) {
	msg, err := s1ap.ParseInitialUEMessage(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Initial UE Message", zap.Error(err))
		return
	}

	nas := []byte(msg.NASPDU)
	if len(nas) > 0 && nas[0]>>4 == uint8(eps.SHTServiceRequest) {
		m.onServiceRequest(ctx, conn, msg)
		return
	}

	// A security-protected NAS message from a UE that presents its S-TMSI is a
	// resume in an existing security context (e.g. a TAU from idle). Bind the
	// persistent context to a fresh S1 connection (TS 36.413) and dispatch
	// with that context. A UE without a resolvable context (e.g. after an MME
	// restart) falls through to a fresh context below.
	if len(nas) > 0 && nas[0]>>4 != uint8(eps.SHTPlain) && msg.STMSI != nil {
		if ue, ok := m.lookupUeByMTMSI(msg.STMSI.MTMSI); ok && ue.emmState.load() == EMMRegistered && ue.secured {
			m.establishS1Connection(ue, conn, msg.ENBUES1APID)

			logger.MmeLog.Info("Initial UE Message (resume)",
				zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)),
				zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)),
			)

			m.handleNAS(ctx, ue, nas)

			return
		}
	}

	m.dropStaleUe(conn, msg.ENBUES1APID)
	ue := m.newUe(conn, msg.ENBUES1APID)

	logger.MmeLog.Info("Initial UE Message",
		zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)),
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)),
	)

	m.handleNAS(ctx, ue, nas)
}

// handleUplinkNASTransport routes an uplink NAS message to its UE context
// (TS 36.413).
func (m *MME) handleUplinkNASTransport(ctx context.Context, conn nasWriter, value []byte) {
	msg, err := s1ap.ParseUplinkNASTransport(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Uplink NAS Transport", zap.Error(err))
		return
	}

	ue, ok := m.resolveUE(conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	m.handleNAS(ctx, ue, []byte(msg.NASPDU))
}

// enbTransportAddress resolves the eNB S1-U endpoint from an E-RAB Transport
// Layer Address (TS 36.413): IPv4 (4 octets), IPv6 (16), or dual-stack (20). When
// the eNB advertises both families the IPv6 endpoint is used, matching the 5G N3
// handling. It reports false when no address is present.
func enbTransportAddress(tla s1ap.TransportLayerAddress) (netip.Addr, bool) {
	v4, v6, err := models.DecodeTransportLayerAddress([]byte(tla))
	if err != nil {
		return netip.Addr{}, false
	}

	switch {
	case v6.IsValid():
		return v6.Unmap(), true
	case v4.IsValid():
		return v4.Unmap(), true
	default:
		return netip.Addr{}, false
	}
}

// handleInitialContextSetupResponse records the eNB's bearer-setup result
// (TS 36.413): the eNB S1-U F-TEID it returns is handed to the anchor
// as the session's downlink endpoint.
func (m *MME) handleInitialContextSetupResponse(ctx context.Context, conn nasWriter, value []byte) {
	msg, err := s1ap.ParseInitialContextSetupResponse(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Initial Context Setup Response", zap.Error(err))
		return
	}

	ue, ok := m.resolveUE(conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	if len(msg.ERABSetup) == 0 {
		logger.MmeLog.Warn("Initial Context Setup Response without an E-RAB",
			zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)))

		return
	}

	// The eNB returns its S1-U F-TEID (the downlink endpoint); hand it to the
	// anchor so the UPF encapsulates downlink traffic toward the eNB.
	erab := msg.ERABSetup[0]

	enbAddr, ok := enbTransportAddress(erab.TransportLayerAddress)
	if !ok {
		logger.MmeLog.Warn("Initial Context Setup Response with an invalid eNB transport address",
			zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)))

		return
	}

	p := m.lookupPDN(ue, uint8(erab.ERABID))
	if p == nil {
		logger.MmeLog.Warn("Initial Context Setup Response for an unknown E-RAB",
			zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)), zap.Int("erab-id", int(erab.ERABID)))

		return
	}

	p.enbFTEID = models.FTEID{TEID: uint32(erab.GTPTEID), Addr: enbAddr}

	if err := m.session.ModifyEPSSession(ctx, ue.imsi, p.ebi, p.enbFTEID); err != nil {
		logger.MmeLog.Error("failed to set the eNB F-TEID on the EPS session",
			zap.String("imsi", ue.imsi), zap.Error(err))

		return
	}

	logger.MmeLog.Info("Initial Context Setup Response",
		zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)),
		zap.String("enb-s1u", p.enbFTEID.Addr.String()),
	)

	// Deliver any pending data-network change to a UE that just re-established
	// its bearer from ECM-IDLE (Service Request): the radio bearer is now up, so
	// a modify/reactivate is deliverable. During attach this is a no-op — the UE
	// is not yet EMM-REGISTERED — so reconcileUE returns early.
	m.reconcileUE(ctx, ue)
}

// downlinkNASTransportBytes builds a Downlink NAS Transport PDU carrying nas for
// the given S1AP identities (TS 36.413).
func downlinkNASTransportBytes(mmeID s1ap.MMEUES1APID, enbID s1ap.ENBUES1APID, nas []byte) ([]byte, error) {
	msg := &s1ap.DownlinkNASTransport{
		MMEUES1APID: mmeID,
		ENBUES1APID: enbID,
		NASPDU:      s1ap.NASPDU(nas),
	}

	return msg.Marshal()
}

// nasMessage is any EPS NAS message that can serialize itself.
type nasMessage interface {
	Marshal() ([]byte, error)
}

// sendDownlinkMessage serializes a plain NAS message and sends it to the UE.
func (m *MME) sendDownlinkMessage(ctx context.Context, ue *UeContext, msg nasMessage) {
	b, err := msg.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal NAS message", zap.Error(err))
		return
	}

	m.sendDownlink(ctx, ue, b)
}

// sendDownlink wraps NAS bytes (plain or security-protected) in a Downlink NAS
// Transport and sends them to the UE's eNB.
func (m *MME) sendDownlink(ctx context.Context, ue *UeContext, nas []byte) {
	ctx, span := tracer.Start(ctx, "s1ap/send",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("s1ap.message_type", string(S1APProcedureDownlinkNASTransport)),
			attribute.Int("s1ap.message_size", len(nas)),
			attribute.String("network.protocol.name", "s1ap"),
			attribute.String("network.transport", "sctp"),
		),
	)
	defer span.End()

	conn, mmeID, enbID := m.s1Identity(ue)
	if conn == nil {
		return
	}

	b, err := downlinkNASTransportBytes(mmeID, enbID, nas)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to build Downlink NAS Transport")
		logger.MmeLog.Error("failed to build Downlink NAS Transport", zap.Error(err))

		return
	}

	if _, err := conn.WriteMsg(b, &sctp.SndRcvInfo{PPID: s1apWirePPID, Stream: s1apStreamUE}); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send Downlink NAS Transport")
		logger.MmeLog.Error("failed to send Downlink NAS Transport", zap.Error(err))

		return
	}

	m.logOutboundS1AP(ctx, conn, S1APProcedureDownlinkNASTransport, b)
}
