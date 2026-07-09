// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// ProtectDownlinkMessage serializes a NAS message and integrity-protects and
// ciphers it with the UE's security context.
func (ue *UeContext) ProtectDownlinkMessage(msg nasMessage) ([]byte, error) {
	plain, err := msg.Marshal()
	if err != nil {
		return nil, err
	}

	return ue.ProtectDownlink(plain, eps.SHTIntegrityProtectedCiphered)
}

func (c *UeConn) SendDownlinkMessage(ctx context.Context, msg nasMessage) {
	if c == nil {
		return
	}

	b, err := msg.Marshal()
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

	plain, err := msg.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal NAS message", zap.Error(err))
		return
	}

	wire, err := c.ue.ProtectDownlink(plain, eps.SHTIntegrityProtectedCiphered)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to protect NAS message", zap.Error(err))
		return
	}

	c.SendDownlinkNASTransport(ctx, wire)
}

// ResendAttachAccept resends the last ATTACH ACCEPT and restarts T3450 without
// re-authenticating, for a duplicate ATTACH REQUEST whose IEs match the one being
// served (TS 24.301 §5.5.1.2.7 case d). Re-arming resets the guard, so this
// retransmission is not charged against the T3450 retransmission count.
func (c *UeConn) ResendAttachAccept(ctx context.Context) {
	if c == nil || len(c.AttachAcceptPdu) == 0 {
		return
	}

	c.SendDownlinkNASTransport(ctx, c.AttachAcceptPdu)
	c.ArmNASGuard("Attach Accept", c.AttachAcceptPdu)
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
	Marshal() ([]byte, error)
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

	ack.MMEUES1APID, ack.ENBUES1APID = c.MMEUES1APID, c.ENBUES1APID

	b, err := ack.Marshal()
	if err != nil {
		return fmt.Errorf("marshal Path Switch Request Acknowledge: %w", err)
	}

	c.SendS1AP(ctx, S1APProcedurePathSwitchRequestAck, b)

	return nil
}
