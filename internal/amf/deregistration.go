// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

// buildDeregistrationRequest assembles a network-initiated (UE-terminated)
// DEREGISTRATION REQUEST (TS 24.501) over 3GPP access, integrity
// protected and ciphered with the UE's security context. Re-registration is not
// requested: the subscriber was removed, so the UE stays deregistered.
func buildDeregistrationRequest() ([]byte, error) {
	return (&fgs.DeregistrationRequestUETerminated{AccessType: fgs.AccessType3GPP}).MarshalBinary()
}

// sendNetworkInitiatedDeregistration sends a UE-terminated DEREGISTRATION
// REQUEST and arms T3522 (TS 24.501): an unanswered request is
// retransmitted, and on exhaustion the UE context is removed regardless.
func (amf *AMF) sendNetworkInitiatedDeregistration(ctx context.Context, ue *UeContext) error {
	ueConn := ue.Conn()
	if ueConn == nil {
		return fmt.Errorf("ueConn is nil")
	}

	plain, err := buildDeregistrationRequest()
	if err != nil {
		return fmt.Errorf("build deregistration request: %w", err)
	}

	sht := uint8(fgs.SHTIntegrityProtectedCiphered)

	if err := ue.SendDownlinkNAS(plain, sht, func(wire []byte) error {
		return ueConn.SendDownlinkNASTransport(ctx, wire)
	}); err != nil {
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

		// A retransmission is a new outbound protected message and takes the next
		// downlink NAS COUNT (TS 24.501 §4.4.3.1).
		if err := ue.SendDownlinkNAS(plain, sht, func(wire []byte) error {
			return retryUeConn.SendDownlinkNASTransport(context.Background(), wire)
		}); err != nil {
			logger.From(ctx, logger.AmfLog).Error("could not retransmit Deregistration Request", zap.Error(err))
		}
	}, func() {
		logger.From(ctx, logger.AmfLog).Warn("T3522 expired, abort network-initiated deregistration and remove UE context")

		amf.DeregisterAndRemoveUeContext(context.Background(), ue)
	})

	return nil
}
