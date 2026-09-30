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
	AllowedEach(ctx context.Context, imsis []string) (map[string]bool, error)
	Uplink(ctx context.Context, imsi string, payload []byte)
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

const (
	smsDetachIdle uint32 = iota
	smsDetachPending
	smsDetachSent
)

const smsIMSIDetachProcedure = "Detach Request (IMSI detach)"

const maxSMSDetachAttempts = 2

const maxSMSDecisionAttempts = 3

func (m *MME) DecideSMS(ctx context.Context, ue *UeContext, requested bool) SMSDecision {
	for range maxSMSDecisionAttempts {
		generation := ue.beginSMSDecision()
		decision := m.evaluateSMS(ctx, ue, requested)

		if m.commitSMSDecision(ctx, ue, generation, decision) {
			return decision
		}
	}

	logger.From(ctx, logger.MmeLog).Warn("SMS decision kept racing configuration changes; answering it as temporarily unavailable", logger.SUPIFromIMSI(ue.imsiOrEmpty()))

	m.commitSMSDecision(ctx, ue, ue.beginSMSDecision(), SMSUnavailable)

	return SMSUnavailable
}

func (m *MME) evaluateSMS(ctx context.Context, ue *UeContext, requested bool) SMSDecision {
	imsi := ue.imsiOrEmpty()
	if !requested || m.SMS == nil || imsi == "" {
		return SMSDenied
	}

	allowed, err := m.SMS.Allowed(ctx, imsi)

	switch {
	case err != nil:
		logger.From(ctx, logger.MmeLog).Warn("could not decide whether the UE may use SMS", logger.SUPIFromIMSI(imsi), zap.Error(err))

		return SMSUnavailable
	case allowed:
		return SMSGranted
	default:
		return SMSDenied
	}
}

func (m *MME) commitSMSDecision(ctx context.Context, ue *UeContext, generation uint64, decision SMSDecision) bool {
	ue.smsMu.Lock()
	defer ue.smsMu.Unlock()

	if ue.smsGeneration != generation {
		return false
	}

	ue.smsDetach.CompareAndSwap(smsDetachPending, smsDetachIdle)
	storeSMSGrant(ctx, ue, decision == SMSGranted)

	return true
}

func (ue *UeContext) beginSMSDecision() uint64 {
	ue.smsMu.Lock()
	defer ue.smsMu.Unlock()

	ue.smsGeneration++

	return ue.smsGeneration
}

func storeSMSGrant(ctx context.Context, ue *UeContext, granted bool) bool {
	if previous := ue.smsOnly.Swap(granted); previous != granted {
		logger.From(ctx, logger.MmeLog).Info("SMS grant changed", logger.SUPIFromIMSI(ue.imsiOrEmpty()), zap.Bool("sms_only", granted))
		return true
	}

	return false
}

func (m *MME) RevokeSMS(ctx context.Context, ue *UeContext) {
	ue.smsMu.Lock()
	defer ue.smsMu.Unlock()

	ue.smsGeneration++
	storeSMSGrant(ctx, ue, false)
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

func (m *MME) smsUE(imsi string, registering bool) (*UeContext, error) {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return nil, ErrSMSUENotRegistered
	}

	switch state := ue.EMMState(); {
	case state == EMMRegistered:
	case registering && state == EMMRegistrationInitiated && ue.Conn() != nil:
	case ue.Conn() == nil:
		return nil, ErrSMSUENotRegistered
	default:
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
			return ErrSMSUENotRegistered
		case state != EMMRegistered:
			return ErrSMSUEUnreachable
		case !ue.SMSOnly():
			return ErrSMSNotAllowed
		case ue.Conn() != nil:
			released := ue.Conn().releaseInProgress()
			if released == nil {
				return nil
			}

			select {
			case <-released:
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		case paged:
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

	var (
		ues         []*UeContext
		generations []uint64
		imsis       []string
	)

	for _, ue := range m.uesWithIMSI() {
		generation := ue.beginSMSDecision()

		if ue.SMSOnly() {
			ues = append(ues, ue)
			generations = append(generations, generation)
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
		if !allowed[imsis[i]] && m.revokeSMSGrant(ctx, ue, generations[i]) {
			m.deliverSMSDetach(ctx, ue)
		}
	}
}

func (m *MME) revokeSMSGrant(ctx context.Context, ue *UeContext, generation uint64) bool {
	ue.smsMu.Lock()
	defer ue.smsMu.Unlock()

	if ue.smsGeneration != generation || !storeSMSGrant(ctx, ue, false) {
		return false
	}

	ue.smsDetach.CompareAndSwap(smsDetachIdle, smsDetachPending)
	ue.smsDetachAborts.Store(0)

	return true
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

func (m *MME) deliverSMSDetach(ctx context.Context, ue *UeContext) {
	conn := ue.Conn()
	if ue.smsDetach.Load() != smsDetachPending || conn == nil || !conn.SecureExchangeEstablished() || ue.EMMState() != EMMRegistered {
		return
	}

	plain, err := (&eps.DetachRequestNetwork{TypeOfDetach: eps.DetachTypeNetworkIMSI}).MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("could not build the IMSI detach", zap.Error(err))
		return
	}

	onAbort := func(context.Context) {
		next := smsDetachPending
		if ue.smsDetachAborts.Add(1) >= maxSMSDetachAttempts {
			next = smsDetachIdle
		}

		ue.smsDetach.CompareAndSwap(smsDetachSent, next)
	}

	if !conn.claimNASGuard(ctx, smsIMSIDetachProcedure, plain, eps.SHTIntegrityProtectedCiphered, onAbort) {
		return
	}

	if !ue.smsDetach.CompareAndSwap(smsDetachPending, smsDetachSent) {
		conn.stopSMSIMSIDetachGuard(ctx)
		return
	}

	if err := conn.SendProtectedNASTransport(ctx, plain, eps.SHTIntegrityProtectedCiphered); err != nil {
		conn.stopSMSIMSIDetachGuard(ctx)
		ue.smsDetach.CompareAndSwap(smsDetachSent, smsDetachPending)
		ReportProtectFailure(ctx, conn, smsIMSIDetachProcedure, err)

		return
	}

	logger.From(ctx, logger.MmeLog).Info("detaching the UE from SMS", logger.SUPIFromIMSI(ue.imsiOrEmpty()))
}

func (c *UeConn) TakeSMSIMSIDetach(ctx context.Context) bool {
	ue := c.UeContext()
	if ue == nil || !ue.smsDetach.CompareAndSwap(smsDetachSent, smsDetachIdle) {
		return false
	}

	c.stopSMSIMSIDetachGuard(ctx)

	return true
}

func (c *UeConn) AbortSMSIMSIDetach(ctx context.Context) {
	ue := c.UeContext()
	if ue == nil || !ue.smsDetach.CompareAndSwap(smsDetachSent, smsDetachPending) {
		return
	}

	c.stopSMSIMSIDetachGuard(ctx)
}

func (c *UeConn) CancelSMSIMSIDetach(ctx context.Context) {
	ue := c.UeContext()
	if ue == nil || ue.smsDetach.Swap(smsDetachIdle) != smsDetachSent {
		return
	}

	c.stopSMSIMSIDetachGuard(ctx)
}

func (m *MME) SMSRoute(imsi string) (granted, connected bool) {
	ue, err := m.smsUE(imsi, true)
	if err != nil {
		return false, false
	}

	return true, ue.Conn() != nil
}

func (c *UeConn) releaseInProgress() <-chan struct{} {
	c.m.mu.RLock()
	defer c.m.mu.RUnlock()

	if !c.releasing || c.released == nil {
		return nil
	}

	return c.released
}
