// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

// buildDeregistrationRequest assembles a network-initiated (UE-terminated)
// DEREGISTRATION REQUEST (TS 24.501) over 3GPP access, integrity
// protected and ciphered with the UE's security context. Re-registration is not
// requested: the subscriber was removed, so the UE stays deregistered.
func buildDeregistrationRequest(ue *UeContext) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeDeregistrationRequestUETerminatedDeregistration)

	m.SecurityHeader = nas.SecurityHeader{
		ProtocolDiscriminator: nasMessage.Epd5GSMobilityManagementMessage,
		SecurityHeaderType:    nas.SecurityHeaderTypeIntegrityProtectedAndCiphered,
	}

	req := nasMessage.NewDeregistrationRequestUETerminatedDeregistration(0)
	req.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	req.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	req.SetSpareHalfOctet(0)
	req.SetMessageType(nas.MsgTypeDeregistrationRequestUETerminatedDeregistration)
	req.SetAccessType(nasMessage.AccessType3GPP)
	req.SetReRegistrationRequired(0)
	req.SetSwitchOff(0)

	m.DeregistrationRequestUETerminatedDeregistration = req

	return ue.EncodeNASMessage(m)
}

// sendNetworkInitiatedDeregistration sends a UE-terminated DEREGISTRATION
// REQUEST and arms T3522 (TS 24.501): an unanswered request is
// retransmitted, and on exhaustion the UE context is removed regardless.
func (amf *AMF) sendNetworkInitiatedDeregistration(ctx context.Context, ue *UeContext) error {
	ueConn := ue.Conn()
	if ueConn == nil {
		return fmt.Errorf("ueConn is nil")
	}

	nasMsg, err := buildDeregistrationRequest(ue)
	if err != nil {
		return fmt.Errorf("build deregistration request: %w", err)
	}

	if err := ueConn.SendDownlinkNASTransport(ctx, nasMsg); err != nil {
		return fmt.Errorf("send downlink nas transport: %w", err)
	}

	ue.TransitionTo(DeregistrationInitiated)

	logger.From(ctx, logger.AmfLog).Info("sent network-initiated Deregistration Request")

	conn := ue.Conn()
	if !amf.NASGuardCfg.Enable || conn == nil {
		return nil
	}

	cfg := amf.NASGuardCfg
	conn.armNASGuardWith(cfg, "T3522 (Deregistration Request)", func(expireTimes int32) {
		retryUeConn := ue.Conn()
		if retryUeConn == nil {
			logger.From(ctx, logger.AmfLog).Warn("UE context released, abort retransmission of Deregistration Request")

			return
		}

		logger.From(ctx, logger.AmfLog).Warn("T3522 expired, retransmit Deregistration Request", zap.Int32("retry", expireTimes))

		if err := retryUeConn.SendDownlinkNASTransport(context.Background(), nasMsg); err != nil {
			logger.From(ctx, logger.AmfLog).Error("could not retransmit Deregistration Request", zap.Error(err))
		}
	}, func() {
		logger.From(ctx, logger.AmfLog).Warn("T3522 expired, abort network-initiated deregistration and remove UE context")

		amf.DeregisterAndRemoveUeContext(context.Background(), ue)
	})

	return nil
}
