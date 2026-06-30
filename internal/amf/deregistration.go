// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

// buildDeregistrationRequest assembles a network-initiated (UE-terminated)
// DEREGISTRATION REQUEST (TS 24.501 §8.2.14) over 3GPP access, integrity
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
// REQUEST and arms T3522 (TS 24.501 §5.5.2.3): an unanswered request is
// retransmitted, and on exhaustion the UE context is removed regardless.
func (amf *AMF) sendNetworkInitiatedDeregistration(ctx context.Context, ue *UeContext) error {
	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ranUe is nil")
	}

	nasMsg, err := buildDeregistrationRequest(ue)
	if err != nil {
		return fmt.Errorf("build deregistration request: %w", err)
	}

	if err := ranUe.SendDownlinkNasTransport(ctx, nasMsg, nil); err != nil {
		return fmt.Errorf("send downlink nas transport: %w", err)
	}

	ue.Log.Info("sent network-initiated Deregistration Request")

	conn := ue.NasConn()
	if !amf.T3522Cfg.Enable || conn == nil {
		return nil
	}

	cfg := amf.T3522Cfg
	conn.T3522.Arm(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
		retryRanUe := ue.RanUe()
		if retryRanUe == nil {
			ue.Log.Warn("UE context released, abort retransmission of Deregistration Request")

			return
		}

		ue.Log.Warn("T3522 expired, retransmit Deregistration Request", zap.Int32("retry", expireTimes))

		if err := retryRanUe.SendDownlinkNasTransport(context.Background(), nasMsg, nil); err != nil {
			ue.Log.Error("could not retransmit Deregistration Request", zap.Error(err))
		}
	}, func() {
		ue.Log.Warn("T3522 expired, abort network-initiated deregistration and remove UE context")

		amf.DeregisterAndRemoveUeContext(context.Background(), ue)
	})

	return nil
}
