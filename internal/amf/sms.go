// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

var (
	ErrSMSUENotRegistered = errors.New("amf: UE is not registered")
	ErrSMSUEUnreachable   = errors.New("amf: UE did not become reachable")
	ErrSMSNotAllowed      = errors.New("amf: UE is not registered for SMS over NAS")
)

func (ue *UeContext) SMSOverNAS() bool {
	return ue.smsOverNAS.Load()
}

const maxSMSDecisionAttempts = 3

func (amf *AMF) GrantSMSOverNAS(ctx context.Context, ue *UeContext, requested bool) bool {
	ue.smsRequested.Store(requested)

	for range maxSMSDecisionAttempts {
		generation := ue.beginSMSDecision()

		granted, err := amf.evaluateSMS(ctx, ue)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("could not decide whether the UE may use SMS over NAS", logger.SUPI(ue.Supi().String()), zap.Error(err))

			granted = false
		}

		if amf.commitSMSGrant(ctx, ue, generation, granted) {
			return granted
		}
	}

	logger.From(ctx, logger.AmfLog).Warn("SMS over NAS decision kept racing configuration changes; not allowing SMS", logger.SUPI(ue.Supi().String()))

	amf.commitSMSGrant(ctx, ue, ue.beginSMSDecision(), false)

	return false
}

func (amf *AMF) commitSMSGrant(ctx context.Context, ue *UeContext, generation uint64, granted bool) bool {
	ue.smsMu.Lock()
	defer ue.smsMu.Unlock()

	if ue.smsGeneration != generation {
		return false
	}

	ue.smsIndicationPending.Store(nil)
	ue.smsIndicated = granted
	amf.storeSMSGrant(ctx, ue, granted)

	return true
}

func (ue *UeContext) beginSMSDecision() uint64 {
	ue.smsMu.Lock()
	defer ue.smsMu.Unlock()

	ue.smsGeneration++

	return ue.smsGeneration
}

func (amf *AMF) evaluateSMS(ctx context.Context, ue *UeContext) (bool, error) {
	supi := ue.Supi()
	if !ue.smsRequested.Load() || amf.SMS == nil || !supi.IsIMSI() {
		return false, nil
	}

	return amf.SMS.Allowed(ctx, supi.IMSI())
}

func (amf *AMF) storeSMSGrant(ctx context.Context, ue *UeContext, granted bool) {
	if ue.smsOverNAS.Swap(granted) != granted {
		logger.From(ctx, logger.AmfLog).Info("SMS over NAS grant changed", logger.SUPI(ue.Supi().String()), zap.Bool("allowed", granted))
	}
}

func (amf *AMF) ReevaluateSMS(ctx context.Context) {
	if amf.SMS == nil {
		return
	}

	type decision struct {
		ue         *UeContext
		imsi       string
		generation uint64
	}

	var pending []decision

	for _, ue := range amf.smsRequestingUEs() {
		generation := ue.beginSMSDecision()

		if ue.State() == Registered {
			pending = append(pending, decision{ue: ue, imsi: ue.Supi().IMSI(), generation: generation})
		}
	}

	if len(pending) == 0 {
		return
	}

	imsis := make([]string, len(pending))
	for i, d := range pending {
		imsis[i] = d.imsi
	}

	allowed, err := amf.SMS.AllowedEach(ctx, imsis)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not re-evaluate SMS over NAS", zap.Error(err))
		return
	}

	for _, d := range pending {
		if amf.applySMSDecision(ctx, d.ue, d.generation, allowed[d.imsi]) {
			amf.deliverSMSIndication(ctx, d.ue)
		}
	}
}

func (amf *AMF) smsRequestingUEs() []*UeContext {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	var ues []*UeContext

	for _, ue := range amf.UEs {
		if ue.smsRequested.Load() && ue.Supi().IsIMSI() {
			ues = append(ues, ue)
		}
	}

	return ues
}

func (amf *AMF) applySMSDecision(ctx context.Context, ue *UeContext, generation uint64, allowed bool) bool {
	ue.smsMu.Lock()
	defer ue.smsMu.Unlock()

	if ue.smsGeneration != generation {
		return false
	}

	if !allowed {
		amf.storeSMSGrant(ctx, ue, false)
	}

	if ue.smsIndicated == allowed {
		return false
	}

	ue.smsIndicated = allowed
	ue.smsIndicationPending.Store(&allowed)

	return true
}

const smsIndicationProcedure = "T3555 (SMS indication)"

func (amf *AMF) deliverSMSIndication(ctx context.Context, ue *UeContext) {
	conn := ue.Conn()
	if conn == nil || ue.State() != Registered {
		return
	}

	available := ue.smsIndicationPending.Load()
	if available == nil {
		return
	}

	plain, err := (&fgs.ConfigurationUpdateCommand{
		ConfigurationUpdateIndication: &fgs.ConfigurationUpdateIndication{ACK: true},
		SMSAvailable:                  available,
	}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("could not build the SMS indication", zap.Error(err))
		return
	}

	send := func(ctx context.Context) error {
		return ue.SendDownlinkNAS(plain, uint8(fgs.SHTIntegrityProtectedCiphered), func(wire []byte) error {
			return conn.SendDownlinkNASTransport(ctx, wire)
		})
	}

	if !amf.NASGuardCfg.Enable {
		if err := send(ctx); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("could not send the SMS indication", logger.SUPI(ue.Supi().String()), zap.Error(err))
			return
		}

		ue.smsIndicationPending.CompareAndSwap(available, nil)

		return
	}

	retransmit := func(ctx context.Context, _ int32) {
		if err := send(ctx); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("could not retransmit the SMS indication", logger.SUPI(ue.Supi().String()), zap.Error(err))
		}
	}

	abort := func(ctx context.Context) {
		ue.smsIndicationPending.CompareAndSwap(available, nil)
		logger.From(ctx, logger.AmfLog).Warn("T3555 expired; abandoned the SMS indication", logger.SUPI(ue.Supi().String()))
	}

	if !conn.claimNASGuard(ctx, amf.NASGuardCfg, smsIndicationProcedure, retransmit, abort) {
		return
	}

	ue.smsIndicationSent.Store(available)

	if err := send(ctx); err != nil {
		conn.stopNASGuardNamed(ctx, smsIndicationProcedure)
		logger.From(ctx, logger.AmfLog).Warn("could not send the SMS indication", logger.SUPI(ue.Supi().String()), zap.Error(err))

		return
	}

	logger.From(ctx, logger.AmfLog).Info("sent SMS indication", logger.SUPI(ue.Supi().String()), zap.Bool("sms_available", *available))
}

func (amf *AMF) SMSIndicationAcknowledged(ctx context.Context, ue *UeContext, conn *UeConn) {
	if conn == nil || conn.nasGuardProcName() != smsIndicationProcedure {
		return
	}

	conn.StopNASGuard(ctx)

	if sent := ue.smsIndicationSent.Swap(nil); sent != nil {
		ue.smsIndicationPending.CompareAndSwap(sent, nil)
	}

	amf.deliverSMSIndication(ctx, ue)
}

func (amf *AMF) SMSReachable(ctx context.Context, ue *UeContext) {
	amf.deliverSMSIndication(ctx, ue)

	supi := ue.Supi()
	if amf.SMS == nil || !ue.SMSOverNAS() || !supi.IsIMSI() {
		return
	}

	amf.SMS.UEReachable(ctx, supi.IMSI())
}

func (ue *UeContext) deactivateSMS() {
	ue.smsMu.Lock()
	defer ue.smsMu.Unlock()

	ue.smsGeneration++
	ue.smsIndicated = false
	ue.smsOverNAS.Store(false)
	ue.smsRequested.Store(false)
	ue.smsIndicationPending.Store(nil)
}

func (ue *UeContext) smsTransactionPending() bool {
	if ue == nil {
		return false
	}

	supi := ue.Supi()

	return ue.sms != nil && supi.IsIMSI() && ue.sms.TransactionPending(supi.IMSI())
}

func (ue *UeContext) SMSDeliveryFailed() {
	if supi := ue.Supi(); ue.sms != nil && supi.IsIMSI() {
		ue.sms.DeliveryFailed(supi.IMSI())
	}
}

func (amf *AMF) SMSSignallingSettled(ctx context.Context, imsi string) {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return
	}

	if ue, ok := amf.LookupUeBySupi(supi); ok {
		ue.Conn().ResumeDeferredReleaseIfSettled(ctx)
	}
}

func (amf *AMF) ForwardSMS(ctx context.Context, ue *UeContext, payload []byte) {
	supi := ue.Supi()
	if amf.SMS == nil || !ue.SMSOverNAS() || !supi.IsIMSI() {
		logger.From(ctx, logger.AmfLog).Warn("discarding an SMS from a UE not registered for SMS over NAS", logger.SUPI(supi.String()))
		return
	}

	amf.SMS.Uplink(ctx, supi.IMSI(), payload)
}

func (amf *AMF) EnableUEReachabilityForSMS(ctx context.Context, imsi string) error {
	ctx, span := tracer.Start(ctx, "amf/enable_ue_reachability_for_sms")
	defer span.End()

	ue, err := amf.smsUE(imsi)
	if err != nil {
		return err
	}

	span.SetAttributes(attrs.SUPI(ue.Supi().String()))

	return amf.reachForSMS(ctx, ue)
}

func (amf *AMF) SendSMS(ctx context.Context, imsi string, payload []byte) error {
	ue, err := amf.smsUE(imsi)
	if err != nil {
		return err
	}

	conn := ue.Conn()
	if conn == nil {
		return ErrSMSUEUnreachable
	}

	plain, err := BuildDLNASTransport(fgs.PayloadContainerTypeSMS, payload, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("build DL NAS Transport: %w", err)
	}

	return ue.SendDownlinkNAS(plain, uint8(fgs.SHTIntegrityProtectedCiphered), func(wire []byte) error {
		if err := conn.SendDownlinkNASTransport(ctx, wire); err != nil {
			return fmt.Errorf("send DL NAS Transport: %w", err)
		}

		logger.From(ctx, logger.AmfLog).Debug("sent SMS to UE", logger.SUPI(ue.Supi().String()))

		return nil
	})
}

func (amf *AMF) smsUE(imsi string) (*UeContext, error) {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return nil, fmt.Errorf("invalid IMSI: %w", err)
	}

	ue, ok := amf.LookupUeBySupi(supi)
	if !ok || ue.State() == Deregistered {
		return nil, ErrSMSUENotRegistered
	}

	if !ue.SMSOverNAS() {
		return nil, ErrSMSNotAllowed
	}

	return ue, nil
}

const smsTemporaryRejectBackoff = 200 * time.Millisecond

func temporaryReject(cause models.N1N2ApplicationError) bool {
	return cause == models.N1N2ErrTemporaryRejectRegistrationOngoing || cause == models.N1N2ErrTemporaryRejectHandoverOngoing
}

func (amf *AMF) reachForSMS(ctx context.Context, ue *UeContext) error {
	paged := false

	for {
		if settled := ue.pagingSettled(); settled != nil {
			paged = true

			select {
			case <-settled:
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		state, changed := ue.watchState()

		switch {
		case state == Deregistered:
			return ErrSMSUENotRegistered
		case state != Registered:
			select {
			case <-changed:
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		case ue.Conn() != nil && !ue.SMSOverNAS():
			return ErrSMSNotAllowed
		case ue.Conn() != nil:
			return nil
		case paged:
			return ErrSMSUEUnreachable
		}

		if err := guardIdlePaging(ue); err != nil {
			var rejected *models.N1N2MessageTransferError

			switch {
			case errors.Is(err, errUEConnected):
				continue
			case errors.As(err, &rejected) && temporaryReject(rejected.Cause):
				select {
				case <-time.After(smsTemporaryRejectBackoff):
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			default:
				return ErrSMSUEUnreachable
			}
		}

		req := &MTRequest{Req: models.N1N2MessageTransferRequest{N1Class: models.N1ClassSMS}, Signalling: true}

		_, err := amf.pageIdleUE(ctx, ue, req)

		var rejected *models.N1N2MessageTransferError

		switch {
		case err == nil, errors.As(err, &rejected):
			paged = true
		case errors.Is(err, errUEConnected):
		default:
			return fmt.Errorf("%w: %w", ErrSMSUEUnreachable, err)
		}
	}
}

func (amf *AMF) SMSRoute(imsi string) (granted, connected bool) {
	ue, err := amf.smsUE(imsi)
	if err != nil {
		return false, false
	}

	return true, ue.Conn() != nil
}
