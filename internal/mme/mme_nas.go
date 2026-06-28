// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"net/netip"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
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
func (m *MME) handleInitialUEMessage(ctx context.Context, conn nasWriter, value []byte) {
	msg, err := s1ap.ParseInitialUEMessage(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Initial UE Message", zap.Error(err))
		return
	}

	nas := []byte(msg.NASPDU)
	if len(nas) > 0 && nas[0]>>4 == uint8(eps.SHTServiceRequest) {
		m.handleServiceRequest(ctx, conn, msg)
		return
	}

	// A security-protected NAS message from a UE that presents its S-TMSI is a
	// resume in an existing security context (e.g. a TAU from idle). The message
	// is authenticated against the resolved context before the context is bound to
	// the requesting association (TS 24.301 §4.4.4.3), so an unverified message
	// cannot move the UE. A UE without a resolvable context (e.g. after an MME
	// restart) falls through to a fresh context below.
	if len(nas) > 0 && nas[0]>>4 != uint8(eps.SHTPlain) && msg.STMSI != nil {
		if ue, ok := m.lookupUeByMTMSI(msg.STMSI.MTMSI); ok && ue.emmState.load() == EMMRegistered && ue.Secured() {
			plain, count, err := ue.tryUnprotectUplink(nas)
			if err != nil {
				logger.MmeLog.Warn("Initial UE Message (resume) failed integrity check",
					zap.Uint32("m-tmsi", msg.STMSI.MTMSI))

				return
			}

			ue.touchLastSeen()
			m.establishS1Connection(ue, conn, msg.ENBUES1APID)
			ue.commitUplinkCount(count)

			logger.MmeLog.Info("Initial UE Message (resume)",
				zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)),
				zap.Uint32("mme-ue-id", uint32(ue.s1.MMEUES1APID)),
			)

			m.dispatchEMM(ctx, ue, plain, true)

			return
		}
	}

	// A fresh connection is tracked by a bare UE-associated S1-connection. A
	// persistent UE context is bound to it only when its first NAS message is an
	// ATTACH REQUEST; any other (or malformed) initial message releases the bare
	// connection without binding one, so an unauthenticated peer cannot exhaust UE
	// contexts.
	c := m.newConn(conn, msg.ENBUES1APID)

	if !isInitialAttach(nas) {
		// A protected TRACKING AREA UPDATE the MME cannot resolve (e.g. a periodic
		// update after an MME restart) is rejected with EMM cause #9 over the bare
		// connection, so the UE re-attaches at once instead of waiting out T3430
		// (TS 24.301 §5.5.3.2.5).
		if isProtectedTrackingAreaUpdate(nas) {
			metrics.RegistrationAttempt(metrics.RAT4G, "Tracking Area Update", metrics.ResultReject)
			logger.MmeLog.Info("Tracking Area Update rejected; UE will re-attach",
				zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)))
			m.sendOverConn(ctx, c, &eps.TrackingAreaUpdateReject{Cause: emmCauseUEIdentityUnderivable})
		} else {
			logger.MmeLog.Debug("dropping non-Attach Initial UE Message",
				zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)))
		}

		m.releaseBareConn(c)

		return
	}

	m.dropStaleUe(conn, msg.ENBUES1APID)
	ue := m.bindConn(c)

	logger.MmeLog.Info("Initial UE Message",
		zap.Uint32("enb-ue-id", uint32(msg.ENBUES1APID)),
		zap.Uint32("mme-ue-id", uint32(ue.s1.MMEUES1APID)),
	)

	m.handleNAS(ctx, ue, nas)
}

// isInitialAttach reports whether a fresh connection's first NAS message is an
// ATTACH REQUEST — the only message warranting a new UE context (TS 24.301):
// plain for an IMSI or foreign-GUTI attach, or integrity-only for a native-GUTI
// re-attach whose body is readable without a security context. A ciphered or
// non-EMM message cannot be an initial attach the network can act on.
func isInitialAttach(nas []byte) bool {
	pd, err := eps.ProtocolDiscriminator(nas)
	if err != nil || pd != eps.PDEMM {
		return false
	}

	body := nas

	switch nas[0] >> 4 {
	case uint8(eps.SHTPlain):
	case uint8(eps.SHTIntegrityProtected), uint8(eps.SHTIntegrityProtectedNewContext):
		if len(nas) < 6 {
			return false
		}

		body = nas[6:]
	default:
		return false
	}

	mt, err := eps.PeekMessageType(body)

	return err == nil && mt == eps.MsgAttachRequest
}

// isProtectedTrackingAreaUpdate reports whether nas is an integrity-protected
// (peekable) TRACKING AREA UPDATE REQUEST. An idle UE sends this from a security
// context the MME may have lost (e.g. after a restart); when the context is
// unresolvable the MME rejects it so the UE re-attaches (TS 24.301 §5.5.3.2.5). A
// ciphered body cannot be peeked, so it is not matched.
func isProtectedTrackingAreaUpdate(nas []byte) bool {
	if len(nas) < 6 {
		return false
	}

	pd, err := eps.ProtocolDiscriminator(nas)
	if err != nil || pd != eps.PDEMM {
		return false
	}

	switch nas[0] >> 4 {
	case uint8(eps.SHTIntegrityProtected), uint8(eps.SHTIntegrityProtectedNewContext):
	default:
		return false
	}

	mt, err := eps.PeekMessageType(nas[6:])

	return err == nil && mt == eps.MsgTrackingAreaUpdateRequest
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
// the eNB advertises both families the IPv6 endpoint is used. It reports false
// when no address is present.
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

	if ue.s1 != nil {
		ue.s1.bearersUp = true
	}

	if err := m.session.ModifyEPSSession(ctx, ue.IMSI(), p.ebi, p.enbFTEID); err != nil {
		logger.MmeLog.Error("failed to set the eNB F-TEID on the EPS session",
			zap.String("imsi", ue.IMSI()), zap.Error(err))

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

// sendOverConn wraps a plain NAS message in a Downlink NAS Transport and sends it
// over a connection that carries no bound UE context — a reject to an
// unidentified peer (an Initial UE Message the MME cannot act on).
func (m *MME) sendOverConn(ctx context.Context, c *s1Conn, msg nasMessage) {
	b, err := msg.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal NAS message", zap.Error(err))
		return
	}

	pdu, err := downlinkNASTransportBytes(c.MMEUES1APID, c.ENBUES1APID, b)
	if err != nil {
		logger.MmeLog.Error("failed to build Downlink NAS Transport", zap.Error(err))
		return
	}

	if _, err := c.conn.WriteMsg(pdu, &sctp.SndRcvInfo{PPID: s1apWirePPID, Stream: s1apStreamUE}); err != nil {
		logger.MmeLog.Error("failed to send Downlink NAS Transport", zap.Error(err))
		return
	}

	m.logOutboundS1AP(ctx, c.conn, S1APProcedureDownlinkNASTransport, pdu)
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
