// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

const SMSOnlyLAC uint16 = 0x0001

type SMSHandler interface {
	Allowed(ctx context.Context, imsi string) (bool, error)
	Uplink(ctx context.Context, imsi string, payload []byte)
	Activate(imsi string, owner any)
	Deactivate(imsi string, owner any)
	UEReachable(ctx context.Context, imsi string)
	TransactionPending(imsi string) bool
	DeliveryFailed(imsi string)
}

var (
	ErrSMSUENotRegistered = errors.New("mme: UE is not registered")
	ErrSMSUEUnreachable   = errors.New("mme: UE did not become reachable")
	ErrSMSNotAllowed      = errors.New("mme: UE is not attached for SMS")
)

func (ue *UeContext) SMSOnly() bool {
	return ue.smsOnly.Load()
}

type SMSDecision uint8

const (
	SMSDenied SMSDecision = iota
	SMSGranted
	SMSUnavailable
)

func (m *MME) DecideSMS(ctx context.Context, ue *UeContext, requested bool) SMSDecision {
	imsi := ue.imsiOrEmpty()
	decision := SMSDenied

	if requested && m.SMS != nil && imsi != "" {
		allowed, err := m.SMS.Allowed(ctx, imsi)

		switch {
		case err != nil:
			logger.From(ctx, logger.MmeLog).Warn("could not decide whether the UE may use SMS", logger.SUPIFromIMSI(imsi), zap.Error(err))

			decision = SMSUnavailable
		case allowed:
			decision = SMSGranted
		}
	}

	ue.smsDetachPending.Store(false)

	if previous := ue.smsOnly.Swap(decision == SMSGranted); previous != (decision == SMSGranted) {
		logger.From(ctx, logger.MmeLog).Info("SMS grant changed", logger.SUPIFromIMSI(imsi), zap.Bool("sms_only", decision == SMSGranted))
	}

	return decision
}

func (m *MME) GrantSMSOnly(ctx context.Context, ue *UeContext, requested bool) bool {
	return m.DecideSMS(ctx, ue, requested) == SMSGranted
}

func (m *MME) RevokeSMS(ctx context.Context, ue *UeContext) {
	if ue.smsOnly.Swap(false) {
		logger.From(ctx, logger.MmeLog).Info("UE detached from SMS", logger.SUPIFromIMSI(ue.imsiOrEmpty()))
	}

	if imsi := ue.imsiOrEmpty(); m.SMS != nil && imsi != "" {
		m.SMS.Deactivate(imsi, ue)
	}
}

func SMSOnlyLAI(plmn models.PlmnID) *nas.LAI {
	return &nas.LAI{PLMN: nas.PLMN{MCC: plmn.Mcc, MNC: plmn.Mnc}, LAC: SMSOnlyLAC}
}

func (m *MME) SMSReachable(ctx context.Context, ue *UeContext) {
	m.deliverSMSDetach(ctx, ue)

	imsi := ue.imsiOrEmpty()
	if m.SMS == nil || !ue.SMSOnly() || imsi == "" || ue.EMMState() != EMMRegistered {
		return
	}

	m.SMS.UEReachable(ctx, imsi)
}

func (m *MME) SyncSMSRegistration(ctx context.Context, ue *UeContext) {
	m.deliverSMSDetach(ctx, ue)

	imsi := ue.imsiOrEmpty()
	if m.SMS == nil || imsi == "" || ue.EMMState() != EMMRegistered {
		return
	}

	if !ue.SMSOnly() {
		m.SMS.Deactivate(imsi, ue)
		return
	}

	m.SMS.Activate(imsi, ue)
	m.SMS.UEReachable(ctx, imsi)
}

func (ue *UeContext) smsTransactionPending() bool {
	if ue == nil {
		return false
	}

	imsi := ue.imsiOrEmpty()

	return ue.sms != nil && imsi != "" && ue.sms.TransactionPending(imsi)
}

func (ue *UeContext) SMSDeliveryFailed() {
	if imsi := ue.imsiOrEmpty(); ue.sms != nil && imsi != "" {
		ue.sms.DeliveryFailed(imsi)
	}
}

func (m *MME) SMSSignallingSettled(ctx context.Context, imsi string) {
	if ue, ok := m.LookupUeByIMSI(imsi); ok {
		ue.Conn().ResumeDeferredReleaseIfSettled(ctx)
	}
}

func (m *MME) ForwardSMS(ctx context.Context, ue *UeContext, payload []byte) {
	imsi := ue.imsiOrEmpty()
	if m.SMS == nil || !ue.SMSOnly() || imsi == "" {
		logger.From(ctx, logger.MmeLog).Warn("discarding an SMS from a UE not attached for SMS", logger.SUPIFromIMSI(imsi))
		return
	}

	m.SMS.Uplink(ctx, imsi, payload)
}

func (m *MME) EnableUEReachabilityForSMS(ctx context.Context, imsi string) error {
	ctx, span := Tracer.Start(ctx, "mme/enable_ue_reachability_for_sms")
	defer span.End()

	ue, err := m.smsUE(imsi)
	if err != nil {
		return err
	}

	span.SetAttributes(attrs.SUPI(ue.Supi().String()))

	return m.reachForSMS(ctx, ue)
}

func (m *MME) SendSMS(ctx context.Context, imsi string, payload []byte) error {
	ue, err := m.smsUE(imsi)
	if err != nil {
		return err
	}

	conn := ue.Conn()
	if conn == nil {
		return ErrSMSUEUnreachable
	}

	plain, err := (&eps.DownlinkNASTransport{NASMessageContainer: payload}).MarshalBinary()
	if err != nil {
		return fmt.Errorf("build Downlink NAS Transport: %w", err)
	}

	if err := conn.SendProtectedNASTransport(ctx, plain, eps.SHTIntegrityProtectedCiphered); err != nil {
		return fmt.Errorf("send Downlink NAS Transport: %w", err)
	}

	logger.From(ctx, logger.MmeLog).Debug("sent SMS to UE", logger.SUPIFromIMSI(imsi))

	return nil
}

func (m *MME) smsUE(imsi string) (*UeContext, error) {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return nil, ErrSMSUENotRegistered
	}

	if ue.EMMState() != EMMRegistered {
		if ue.Conn() == nil {
			return nil, ErrSMSUENotRegistered
		}

		return nil, ErrSMSUEUnreachable
	}

	if !ue.SMSOnly() {
		return nil, ErrSMSNotAllowed
	}

	return ue, nil
}

func (m *MME) reachForSMS(ctx context.Context, ue *UeContext) error {
	paged := false
	req := &MTRequest{Signalling: true}

	for {
		if settled := ue.pagingSettled(); settled != nil {
			paged = true

			select {
			case <-settled:
				continue
			case <-ctx.Done():
				ue.withdrawPage(req)
				return ctx.Err()
			}
		}

		if ue.Conn() != nil {
			if ue.EMMState() != EMMRegistered {
				return ErrSMSUEUnreachable
			}

			return nil
		}

		if paged || ue.EMMState() != EMMRegistered {
			return ErrSMSUEUnreachable
		}

		err := m.sendPage(ctx, ue, func() error { return ue.beginPaging(req) }, false)
		if err != nil && !errors.Is(err, errPagingSkipped) {
			return fmt.Errorf("%w: %w", ErrSMSUEUnreachable, err)
		}

		paged = true
	}
}

func (m *MME) ReevaluateSMS(ctx context.Context) {
	if m.SMS == nil {
		return
	}

	for _, ue := range m.smsGrantedUEs() {
		imsi := ue.imsiOrEmpty()

		allowed, err := m.SMS.Allowed(ctx, imsi)
		if err != nil {
			logger.From(ctx, logger.MmeLog).Warn("could not re-evaluate SMS", logger.SUPIFromIMSI(imsi), zap.Error(err))
			continue
		}

		if allowed || !ue.smsOnly.CompareAndSwap(true, false) {
			continue
		}

		logger.From(ctx, logger.MmeLog).Info("SMS grant changed", logger.SUPIFromIMSI(imsi), zap.Bool("sms_only", false))

		m.SMS.Deactivate(imsi, ue)
		ue.smsDetachPending.Store(true)
		m.deliverSMSDetach(ctx, ue)
	}
}

func (m *MME) smsGrantedUEs() []*UeContext {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ues := make([]*UeContext, 0, len(m.UEs))

	for _, ue := range m.UEs {
		if ue.SMSOnly() {
			ues = append(ues, ue)
		}
	}

	return ues
}

func (m *MME) deliverSMSDetach(ctx context.Context, ue *UeContext) {
	conn := ue.Conn()
	if !ue.smsDetachPending.Load() || conn == nil || !conn.SecureExchangeEstablished() || conn.nasGuard.Active() || ue.EMMState() != EMMRegistered {
		return
	}

	if !ue.smsDetachPending.CompareAndSwap(true, false) {
		return
	}

	plain, err := (&eps.DetachRequestNetwork{TypeOfDetach: eps.DetachTypeNetworkIMSI}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("could not build the IMSI detach", zap.Error(err))
		return
	}

	ue.smsIMSIDetaching.Store(true)

	if err := conn.SendProtectedNASTransport(ctx, plain, eps.SHTIntegrityProtectedCiphered); err != nil {
		ue.smsIMSIDetaching.Store(false)
		ue.smsDetachPending.Store(true)
		ReportProtectFailure(ctx, conn, "Detach Request (IMSI detach)", err)

		return
	}

	conn.ArmNASGuardAbortOnly(ctx, "Detach Request (IMSI detach)", plain, eps.SHTIntegrityProtectedCiphered, func(context.Context) {
		ue.smsIMSIDetaching.Store(false)
	})

	logger.From(ctx, logger.MmeLog).Info("detaching the UE from SMS", logger.SUPIFromIMSI(ue.imsiOrEmpty()))
}

func (ue *UeContext) TakeSMSIMSIDetach() bool {
	return ue.smsIMSIDetaching.CompareAndSwap(true, false)
}

func (ue *UeContext) AbortSMSIMSIDetach() {
	if ue.smsIMSIDetaching.CompareAndSwap(true, false) {
		ue.smsDetachPending.Store(true)
	}
}
