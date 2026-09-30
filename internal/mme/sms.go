// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smsf"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

const SMSOnlyLAC uint16 = 0x0001

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
	m.smsDecisionMu.RLock()
	defer m.smsDecisionMu.RUnlock()

	decision := m.evaluateSMS(ctx, ue, requested)
	storeSMSGrant(ctx, ue, decision == SMSGranted)

	return decision
}

func (m *MME) evaluateSMS(ctx context.Context, ue *UeContext, requested bool) SMSDecision {
	imsi := ue.imsiOrEmpty()
	if !requested || m.SMS == nil || imsi == "" {
		return SMSDenied
	}

	allowed, err := m.SMS.AllowedEach(ctx, []string{imsi})

	switch {
	case err != nil:
		logger.From(ctx, logger.MmeLog).Warn("could not decide whether the UE may use SMS", logger.SUPIFromIMSI(imsi), zap.Error(err))

		return SMSUnavailable
	case allowed[imsi]:
		return SMSGranted
	default:
		return SMSDenied
	}
}

func storeSMSGrant(ctx context.Context, ue *UeContext, granted bool) {
	if previous := ue.smsOnly.Swap(granted); previous != granted {
		logger.From(ctx, logger.MmeLog).Info("SMS grant changed", logger.SUPIFromIMSI(ue.imsiOrEmpty()), zap.Bool("sms_only", granted))
	}
}

func (m *MME) RevokeSMS(ctx context.Context, ue *UeContext) {
	storeSMSGrant(ctx, ue, false)
}

func SMSOnlyLAI(plmn models.PlmnID) *nas.LAI {
	return &nas.LAI{PLMN: nas.PLMN{MCC: plmn.Mcc, MNC: plmn.Mnc}, LAC: SMSOnlyLAC}
}

func (m *MME) SMSReachable(ctx context.Context, ue *UeContext) {
	imsi := ue.imsiOrEmpty()
	if m.SMS == nil || !ue.SMSOnly() || imsi == "" || ue.EMMState() != EMMRegistered {
		return
	}

	m.SMS.UEReachable(ctx, imsi)
}

func (ue *UeContext) smsTransactionPending() bool {
	if ue == nil {
		return false
	}

	imsi := ue.imsiOrEmpty()

	return ue.sms != nil && imsi != "" && ue.sms.TransactionPending(imsi)
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

	ue, err := m.smsUE(imsi, true)
	if err != nil {
		return err
	}

	span.SetAttributes(attrs.SUPI(ue.Supi().String()))

	return m.reachForSMS(ctx, ue)
}

func (m *MME) SendSMS(ctx context.Context, imsi string, payload []byte) error {
	ue, err := m.smsUE(imsi, false)
	if err != nil {
		return err
	}

	conn := ue.Conn()
	if conn == nil {
		return smsf.ErrUnreachable
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

func (m *MME) smsUE(imsi string, registering bool) (*UeContext, error) {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return nil, smsf.ErrNotRegisteredForSMS
	}

	switch state := ue.EMMState(); {
	case state == EMMRegistered:
	case registering && state == EMMRegistrationInitiated && ue.Conn() != nil:
	case ue.Conn() == nil:
		return nil, smsf.ErrNotRegisteredForSMS
	default:
		return nil, smsf.ErrUnreachable
	}

	if !ue.SMSOnly() {
		return nil, smsf.ErrNotRegisteredForSMS
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

		state, changed := ue.watchEMMState()

		switch {
		case state == EMMRegistrationInitiated && ue.Conn() != nil:
			select {
			case <-changed:
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		case state == EMMDeregistered:
			return smsf.ErrNotRegisteredForSMS
		case state != EMMRegistered:
			return smsf.ErrUnreachable
		case !ue.SMSOnly():
			return smsf.ErrNotRegisteredForSMS
		case ue.Conn() != nil:
			return nil
		case paged:
			return smsf.ErrUnreachable
		}

		err := m.sendPage(ctx, ue, func() error { return ue.beginPaging(req) }, false)
		if err != nil && !errors.Is(err, errPagingSkipped) {
			return fmt.Errorf("%w: %w", smsf.ErrUnreachable, err)
		}

		paged = true
	}
}

func (m *MME) ReevaluateSMS(ctx context.Context) {
	if m.SMS == nil {
		return
	}

	m.smsDecisionMu.Lock()
	defer m.smsDecisionMu.Unlock()

	var (
		ues   []*UeContext
		imsis []string
	)

	for _, ue := range m.uesWithIMSI() {
		if ue.SMSOnly() {
			ues = append(ues, ue)
			imsis = append(imsis, ue.imsiOrEmpty())
		}
	}

	if len(ues) == 0 {
		return
	}

	allowed, err := m.SMS.AllowedEach(ctx, imsis)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("could not re-evaluate SMS", zap.Error(err))
		return
	}

	for i, ue := range ues {
		if !allowed[imsis[i]] {
			storeSMSGrant(ctx, ue, false)
		}
	}
}

func (m *MME) uesWithIMSI() []*UeContext {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ues := make([]*UeContext, 0, len(m.UEs))

	for _, ue := range m.UEs {
		if ue.imsiOrEmpty() != "" {
			ues = append(ues, ue)
		}
	}

	return ues
}

func (m *MME) SMSRoute(imsi string) (granted, connected bool) {
	ue, err := m.smsUE(imsi, true)
	if err != nil {
		return false, false
	}

	return true, ue.Conn() != nil
}
