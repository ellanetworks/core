// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func (amf *AMF) steerAfterRegistration(ctx context.Context, ue *UeContext, info *wire.SorInfo) {
	log := logger.From(ctx, logger.AmfLog).With(logger.SUPI(ue.Supi().String()))

	sor, err := roaming.SoRFromHPLMN(info)
	if err != nil || sor == nil {
		log.Warn("ignoring the steering of roaming information of the home network", zap.Error(err))
		return
	}

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.SoR = sor })

	if amf.deliverSoR(ctx, ue) {
		return
	}

	if amf.pageForSoR(ctx, ue) {
		log.Info("paging the visitor to deliver the steering of roaming information")
		return
	}

	log.Info("the visitor is not connected; the steering of roaming information waits for its next NAS signalling connection")
}

func (amf *AMF) pageForSoR(ctx context.Context, ue *UeContext) bool {
	err := guardIdlePaging(ue)
	if errors.Is(err, errUEConnected) {
		return amf.deliverSoR(ctx, ue)
	}

	if err != nil {
		return false
	}

	req := &MTRequest{Req: models.N1N2MessageTransferRequest{N1Class: models.N1Class5GMM}, Signalling: true}

	_, err = amf.pageIdleUE(ctx, ue, req)

	var rejected *models.N1N2MessageTransferError

	switch {
	case err == nil, errors.As(err, &rejected):
		return true
	case errors.Is(err, errUEConnected):
		return amf.deliverSoR(ctx, ue)
	default:
		logger.From(ctx, logger.AmfLog).Warn("could not page the visitor for the steering of roaming information",
			logger.SUPI(ue.Supi().String()), zap.Error(err))

		return false
	}
}

func (amf *AMF) deliverSoR(ctx context.Context, ue *UeContext) bool {
	conn := ue.Conn()
	if conn == nil || ue.State() != Registered {
		return false
	}

	v, ok := ue.Visitor()
	if !ok || v.SoR == nil || v.SoR.Delivered {
		return false
	}

	sor := *v.SoR

	plain, err := BuildDLNASTransport(fgs.PayloadContainerTypeSOR, sor.Container, nil, nil, nil)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to build the steering of roaming DL NAS Transport", zap.Error(err))
		return false
	}

	if err := ue.SendDownlinkNAS(plain, uint8(fgs.SHTIntegrityProtectedCiphered), func(nasPdu []byte) error {
		return conn.SendDownlinkNASTransport(ctx, nasPdu)
	}); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to send the steering of roaming information", zap.Error(err))
		return false
	}

	ue.updateVisitor(func(cur *roaming.Visitor5G) {
		if cur.SoR == nil || cur.SoR.ProvisioningTime != sor.ProvisioningTime {
			return
		}

		if sor.AckRequested {
			cur.SoR.Delivered = true
		} else {
			cur.SoR = nil
		}
	})

	logger.From(ctx, logger.AmfLog).Info("sent the steering of roaming information of the home network in a DL NAS Transport",
		logger.SUPI(ue.Supi().String()), zap.Bool("ack_requested", sor.AckRequested))

	return true
}

func (amf *AMF) AcknowledgeSoRTransport(ctx context.Context, ue *UeContext, container []byte) {
	v, ok := ue.Visitor()
	if !ok || v.SoR == nil || !v.SoR.Delivered {
		logger.From(ctx, logger.AmfLog).Info("ignoring a steering of roaming acknowledgement that no DL NAS Transport asked for")
		return
	}

	sor := *v.SoR

	ue.updateVisitor(func(cur *roaming.Visitor5G) {
		if cur.SoR != nil && cur.SoR.Delivered {
			cur.SoR = nil
		}
	})

	amf.relaySoRAck(ctx, ue, v, sor, container)
}

func (amf *AMF) requestReregistration(ctx context.Context, ue *UeContext) {
	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.Reregister = true })

	if !amf.sendReregistration(ctx, ue) {
		logger.From(ctx, logger.AmfLog).Info("the visitor is not connected; the registration request waits for its next NAS signalling connection",
			logger.SUPI(ue.Supi().String()))
	}
}

func (amf *AMF) sendReregistration(ctx context.Context, ue *UeContext) bool {
	conn := ue.Conn()
	if conn == nil || ue.State() != Registered {
		return false
	}

	if v, ok := ue.Visitor(); !ok || !v.Reregister {
		return false
	}

	plain, err := (&fgs.ConfigurationUpdateCommand{
		ConfigurationUpdateIndication: &fgs.ConfigurationUpdateIndication{ACK: true, RED: true},
	}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to build the Configuration Update Command", zap.Error(err))
		return false
	}

	send := func(ctx context.Context, c *UeConn) error {
		return ue.SendDownlinkNAS(plain, uint8(fgs.SHTIntegrityProtectedCiphered), func(nasPdu []byte) error {
			return c.SendDownlinkNASTransport(ctx, nasPdu)
		})
	}

	if err := send(ctx, conn); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to send the Configuration Update Command", zap.Error(err))
		return false
	}

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.ReregisterSent = true })

	if cfg := amf.NASGuardCfg; cfg.Enable {
		conn.armNASGuardWith(ctx, cfg, "T3555 (Configuration Update)", func(ctx context.Context, expireTimes int32) {
			retry := ue.Conn()
			if retry == nil {
				return
			}

			retry.Log(ctx).Warn("timer T3555 expired, retransmit the Configuration Update Command", zap.Int32("retry", expireTimes))

			if err := send(ctx, retry); err != nil {
				retry.Log(ctx).Warn("failed to resend the Configuration Update Command", zap.Error(err))
			}
		}, func(ctx context.Context) {
			conn.Log(ctx).Warn("timer T3555 expired too many times; the visitor registers again at its next NAS signalling connection")
			ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.ReregisterSent = false })
		})
	}

	logger.From(ctx, logger.AmfLog).Info("sent a Configuration Update Command requesting registration", logger.SUPI(ue.Supi().String()))

	return true
}

func (amf *AMF) ResumeHPLMNChanges(ctx context.Context, ue *UeContext) {
	if !ue.IsVisitor() {
		return
	}

	amf.sendReregistration(ctx, ue)
	amf.deliverSoR(ctx, ue)
}

func (amf *AMF) ConfigurationUpdateCompleted(ctx context.Context, ue *UeContext, conn *UeConn) {
	v, ok := ue.Visitor()
	if !ok || !v.ReregisterSent {
		return
	}

	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.Reregister, cur.ReregisterSent = false, false })

	if conn == nil {
		return
	}

	logger.From(ctx, logger.AmfLog).Info("the visitor acknowledged the registration request; releasing its NAS signalling connection",
		logger.SUPI(ue.Supi().String()))

	conn.ReleaseWithAction(ctx, UeContextN2NormalRelease, ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASNormalRelease})
}

func (amf *AMF) reregistrationFulfilled(ue *UeContext) {
	ue.updateVisitor(func(cur *roaming.Visitor5G) { cur.Reregister, cur.ReregisterSent = false, false })
}
