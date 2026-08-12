// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// SendProtected takes the next downlink NAS COUNT, protects plain under it, and
// hands the wire bytes to write — one critical section, so the messages reach the
// eNB in the order their COUNTs were taken (TS 24.301 §4.4.3.1). The protected
// bytes never escape write, so no caller can hold a COUNT it has not written.
//
// write frames the protected PDU for its S1AP procedure and sends it. It runs
// under the sender's lock, so it must not block, take UeContext.mu, call the
// anchor, or send a second protected message; MME.mu it reaches only through the
// S1AP send chokepoint.
func (c *UeConn) SendProtected(plain []byte, sht eps.SecurityHeaderType, write nas.WriteFunc) error {
	if c == nil || c.ue == nil {
		return nil
	}

	return c.ue.downlink().Send(plain, uint8(sht), write)
}

// SendProtectedNASTransport protects plain and sends it in a Downlink NAS
// Transport (TS 36.413 §8.6.2).
func (c *UeConn) SendProtectedNASTransport(ctx context.Context, plain []byte, sht eps.SecurityHeaderType) error {
	return c.SendProtected(plain, sht, func(wire []byte) error {
		c.SendDownlinkNASTransport(ctx, wire)

		return nil
	})
}

func (c *UeConn) SendDownlinkMessage(ctx context.Context, msg nasMessage) {
	if c == nil {
		return
	}

	b, err := msg.MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal NAS message", zap.Error(err))
		return
	}

	c.SendDownlinkNASTransport(ctx, b)
}

// SendDownlinkProtected encodes a plain NAS message, integrity-protects and
// ciphers it with the UE's security context, and sends it downlink.
func (c *UeConn) SendDownlinkProtected(ctx context.Context, msg nasMessage) {
	if c == nil || c.ue == nil {
		return
	}

	plain, err := msg.MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal NAS message", zap.Error(err))
		return
	}

	if err := c.SendProtectedNASTransport(ctx, plain, eps.SHTIntegrityProtectedCiphered); err != nil {
		c.reportProtectFailure(ctx, err)
	}
}

// ResendAttachAccept resends the last ATTACH ACCEPT and restarts T3450 without
// re-authenticating, for a duplicate ATTACH REQUEST whose IEs match the one being
// served (TS 24.301 §5.5.1.2.7 case d). The retransmission is protected afresh
// under the next downlink NAS COUNT (TS 24.301 §4.4.3.1). Re-arming resets the
// guard, so this retransmission is not charged against the T3450 retransmission
// count.
func (c *UeConn) ResendAttachAccept(ctx context.Context) {
	if c == nil || len(c.AttachAcceptPlain) == 0 {
		return
	}

	_ = c.SendGuardedProtected(ctx, "Attach Accept", c.AttachAcceptPlain, eps.SHTIntegrityProtectedCiphered)
}

// ResendTauAccept resends the stored TAU ACCEPT and restarts its T3450 guard
// (TS 24.301 §5.5.3.2.7 case d).
func (c *UeConn) ResendTauAccept(ctx context.Context) {
	if c == nil || len(c.TauAcceptPlain) == 0 {
		return
	}

	_ = c.SendGuardedProtected(ctx, "Tracking Area Update Accept", c.TauAcceptPlain, eps.SHTIntegrityProtectedCiphered)
}

// SendDownlinkNASTransport wraps NAS bytes (plain or security-protected) in a Downlink NAS
// Transport and sends them to the UE's eNB through the single send chokepoint.
func (c *UeConn) SendDownlinkNASTransport(ctx context.Context, nas []byte) {
	if c == nil {
		return
	}

	b, err := downlinkNASTransportBytes(c.MMEUES1APID, c.ENBUES1APID, nas)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build Downlink NAS Transport", zap.Error(err))
		return
	}

	c.SendS1AP(ctx, S1APProcedureDownlinkNASTransport, b)
}

// nasMessage is any EPS NAS message that can serialize itself.
type nasMessage interface {
	MarshalBinary() ([]byte, error)
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

// The per-command Send<Proc> methods below stamp the UE's S1AP identities
// (MME/eNB-UE-S1AP-ID) onto the message in one place and send it on the UE's own eNB
// association — so no handler re-derives the IDs by hand. Each returns a marshal
// error; send errors are logged by
// SendS1AP. Commands targeting a *different* association (in-flight handover) keep
// using SendToRadio.

// SendInitialContextSetup stamps the UE identities and sends the Initial Context
// Setup Request on the UE's eNB association (TS 36.413 §8.3).
func (c *UeConn) SendInitialContextSetup(ctx context.Context, req *s1ap.InitialContextSetupRequest) error {
	if c == nil {
		return nil
	}

	req.MMEUES1APID, req.ENBUES1APID = c.MMEUES1APID, c.ENBUES1APID

	b, err := req.Marshal()
	if err != nil {
		return fmt.Errorf("marshal Initial Context Setup Request: %w", err)
	}

	c.SendS1AP(ctx, S1APProcedureInitialContextSetupRequest, b)

	return nil
}

// SendERABSetup stamps the UE identities and sends the E-RAB Setup Request (TS 36.413 §8.2.1).
func (c *UeConn) SendERABSetup(ctx context.Context, req *s1ap.ERABSetupRequest) error {
	if c == nil {
		return nil
	}

	req.MMEUES1APID, req.ENBUES1APID = c.MMEUES1APID, c.ENBUES1APID

	b, err := req.Marshal()
	if err != nil {
		return fmt.Errorf("marshal E-RAB Setup Request: %w", err)
	}

	c.SendS1AP(ctx, S1APProcedureERABSetupRequest, b)

	return nil
}

// SendERABModify stamps the UE identities and sends the E-RAB Modify Request (TS 36.413 §8.2.2).
func (c *UeConn) SendERABModify(ctx context.Context, req *s1ap.ERABModifyRequest) error {
	if c == nil {
		return nil
	}

	req.MMEUES1APID, req.ENBUES1APID = c.MMEUES1APID, c.ENBUES1APID

	b, err := req.Marshal()
	if err != nil {
		return fmt.Errorf("marshal E-RAB Modify Request: %w", err)
	}

	c.SendS1AP(ctx, S1APProcedureERABModifyRequest, b)

	return nil
}

// SendERABRelease stamps the UE identities and sends the E-RAB Release Command (TS 36.413 §8.2.3).
func (c *UeConn) SendERABRelease(ctx context.Context, cmd *s1ap.ERABReleaseCommand) error {
	if c == nil {
		return nil
	}

	cmd.MMEUES1APID, cmd.ENBUES1APID = c.MMEUES1APID, c.ENBUES1APID

	b, err := cmd.Marshal()
	if err != nil {
		return fmt.Errorf("marshal E-RAB Release Command: %w", err)
	}

	c.SendS1AP(ctx, S1APProcedureERABReleaseCommand, b)

	return nil
}

// SendPathSwitchAcknowledge stamps the UE identities and sends the Path Switch Request
// Acknowledge on the (just-committed) UE association (TS 36.413 §8.6.1). After
// CommitPathSwitch the conn carries the same IDs the ack echoes.
func (c *UeConn) SendPathSwitchAcknowledge(ctx context.Context, ack *s1ap.PathSwitchRequestAcknowledge) error {
	if c == nil {
		return nil
	}

	ack.MMEUES1APID, ack.ENBUES1APID = s1ap.Ptr(c.MMEUES1APID), s1ap.Ptr(c.ENBUES1APID)

	b, err := ack.Marshal()
	if err != nil {
		return fmt.Errorf("marshal Path Switch Request Acknowledge: %w", err)
	}

	c.SendS1AP(ctx, S1APProcedurePathSwitchRequestAck, b)

	return nil
}

// reportProtectFailure logs a downlink protection failure and, when the NAS
// COUNT is exhausted, releases the connection.
func (c *UeConn) reportProtectFailure(ctx context.Context, err error) {
	ReportProtectFailure(ctx, c, "NAS message", err)
}

// ReportProtectFailure logs a failure to protect a downlink NAS message and, when
// the downlink NAS COUNT is exhausted, releases the connection.
//
// Nothing further can be sent under that security context: reusing a COUNT would
// repeat the keystream and make MAC forgery trivial (TS 33.401 §6.5). Releasing
// makes the UE re-attach, which establishes a new context.
func ReportProtectFailure(ctx context.Context, c *UeConn, what string, err error) {
	log := logger.From(ctx, logger.MmeLog)

	if !errors.Is(err, nas.ErrCountExhausted) {
		log.Error("failed to protect "+what, zap.Error(err))
		return
	}

	log.Error("downlink NAS COUNT exhausted, releasing the connection", zap.String("message", what), zap.Error(err))

	if c != nil && c.m != nil {
		SendUEContextRelease(ctx, c.m, c.Conn(), c.MMEUES1APID, c.ENBUES1APID, true, CauseNASNormalRelease)
	}
}
