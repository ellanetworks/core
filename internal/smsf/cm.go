// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
	"go.uber.org/zap"
)

const maxTransactionValue = 6

type moTransaction struct {
	ack        chan struct{}
	aborted    chan struct{}
	retry      chan struct{}
	reportSent bool
}

func newMOTransaction() *moTransaction {
	return &moTransaction{ack: make(chan struct{}, 1), aborted: make(chan struct{}, 1), retry: make(chan struct{}, 1)}
}

type mtTransaction struct {
	ti         sms.TransactionIdentifier
	reference  uint8
	ack        chan struct{}
	report     chan sms.RPMessage
	aborted    chan struct{}
	retry      chan struct{}
	abortCause sms.CPCause
}

type ueState struct {
	mo      map[uint8]*moTransaction
	mt      *mtTransaction
	nextTI  uint8
	nextRef uint8
}

func (s *SMSF) ue(imsi string) *ueState {
	u, ok := s.ues[imsi]
	if !ok {
		u = &ueState{mo: make(map[uint8]*moTransaction)}
		s.ues[imsi] = u
	}

	return u
}

func (s *SMSF) DeliveryFailed(imsi string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.ues[imsi]
	if !ok {
		return
	}

	if u.mt != nil {
		signal(u.mt.retry)
	}

	for _, t := range u.mo {
		if t.reportSent {
			signal(t.retry)
		}
	}
}

func (s *SMSF) TransactionPending(imsi string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.ues[imsi]

	return ok && u.holdsConnection()
}

func (u *ueState) holdsConnection() bool {
	if u.mt != nil {
		return true
	}

	for _, t := range u.mo {
		if !t.reportSent {
			return true
		}
	}

	return false
}

func (s *SMSF) transactionEnded(ctx context.Context, imsi string) {
	s.mu.Lock()
	u, ok := s.ues[imsi]
	idle := !ok || !u.holdsConnection()
	r, registered := s.registered[imsi]
	s.mu.Unlock()

	transport := s.currentTransport()
	if !idle || !registered || transport == nil {
		return
	}

	transport.SignallingSettled(ctx, imsi, r.access)
}

func (s *SMSF) Uplink(ctx context.Context, imsi string, payload []byte) {
	msg, err := sms.ParseCP(payload)
	if err != nil {
		s.rejectCP(ctx, imsi, payload, err)
		return
	}

	switch m := msg.(type) {
	case *sms.CPAck:
		s.cpAck(imsi, m.TransactionIdentifier)
	case *sms.CPError:
		s.cpError(imsi, m)
	case *sms.CPData:
		s.cpData(ctx, imsi, m)
	}
}

func (s *SMSF) rejectCP(ctx context.Context, imsi string, payload []byte, err error) {
	header, headerErr := sms.ParseCPHeader(payload)
	if headerErr != nil {
		s.logger.Warn("Dropped an undecodable SMS CP message", zap.String("imsi", imsi), zap.Error(err))
		return
	}

	cause := sms.CPCauseProtocolErrorUnspecified

	switch {
	case errors.Is(err, sms.ErrUnknownMessageType):
		cause = sms.CPCauseMessageTypeNonExistent
	case errors.Is(err, sms.ErrInvalidMandatoryIE), errors.Is(err, nas.ErrTruncated):
		cause = sms.CPCauseInvalidMandatoryInformation
	}

	s.logger.Warn("Rejected an invalid SMS CP message", zap.String("imsi", imsi), zap.Error(err))

	if header.MessageType == sms.CPMessageTypeError {
		return
	}

	_ = s.send(ctx, imsi, &sms.CPError{TransactionIdentifier: header.TransactionIdentifier.Peer(), Cause: cause})
}

func (s *SMSF) cpAck(imsi string, ti sms.TransactionIdentifier) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.ues[imsi]
	if !ok {
		return
	}

	if ti.Flag {
		if u.mt != nil && u.mt.ti.Value == ti.Value {
			signal(u.mt.ack)
		}

		return
	}

	if t, ok := u.mo[ti.Value]; ok {
		signal(t.ack)
	}
}

func (s *SMSF) cpError(imsi string, m *sms.CPError) {
	ti := m.TransactionIdentifier

	s.logger.Info("UE aborted an SMS transaction",
		zap.String("imsi", imsi), zap.Stringer("ti", ti), zap.Stringer("cause", m.Cause))

	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.ues[imsi]
	if !ok {
		return
	}

	if ti.Flag {
		if u.mt != nil && u.mt.ti.Value == ti.Value {
			u.mt.abortCause = m.Cause
			signal(u.mt.aborted)
		}

		return
	}

	if t, ok := u.mo[ti.Value]; ok {
		signal(t.aborted)
	}
}

func (s *SMSF) cpData(ctx context.Context, imsi string, m *sms.CPData) {
	ti := m.TransactionIdentifier

	if ti.Flag {
		s.mtReport(ctx, imsi, m)
		return
	}

	_ = s.send(ctx, imsi, &sms.CPAck{TransactionIdentifier: ti.Peer()})

	s.mu.Lock()
	u := s.ue(imsi)
	_, duplicate := u.mo[ti.Value]

	if duplicate {
		s.mu.Unlock()
		return
	}

	t := newMOTransaction()
	u.mo[ti.Value] = t

	for value, other := range u.mo {
		if value != ti.Value && other.reportSent {
			signal(other.ack)
		}
	}
	s.mu.Unlock()

	go s.mobileOriginated(context.WithoutCancel(ctx), imsi, ti, t, m.UserData)
}

func (s *SMSF) mtReport(ctx context.Context, imsi string, m *sms.CPData) {
	ti := m.TransactionIdentifier

	rp, err := sms.ParseRP(m.UserData, nas.DirectionUplink)

	s.mu.Lock()
	u, ok := s.ues[imsi]

	var t *mtTransaction
	if ok && u.mt != nil && u.mt.ti.Value == ti.Value {
		t = u.mt
	}
	s.mu.Unlock()

	if t == nil {
		_ = s.send(ctx, imsi, &sms.CPError{TransactionIdentifier: ti.Peer(), Cause: sms.CPCauseInvalidTransactionIdentifier})
		return
	}

	_ = s.send(ctx, imsi, &sms.CPAck{TransactionIdentifier: ti.Peer()})

	signal(t.ack)

	if err != nil {
		s.logger.Warn("UE answered a mobile-terminated SMS with an invalid RP message", zap.String("imsi", imsi), zap.Error(err))

		rp = &sms.RPError{Direction: nas.DirectionUplink, Reference: t.reference, Cause: sms.RPCauseInvalidMandatoryInformation}
	}

	select {
	case t.report <- rp:
	default:
	}
}

func (s *SMSF) send(ctx context.Context, imsi string, m sms.CPMessage) error {
	payload, err := m.MarshalBinary()
	if err != nil {
		return fmt.Errorf("encode %s: %w", m.MessageType(), err)
	}

	transport, access, err := s.route(imsi)
	if err != nil {
		return err
	}

	if err := transport.SendSMS(ctx, imsi, access, payload); err != nil {
		s.logger.Debug("Could not send an SMS CP message", zap.String("imsi", imsi), zap.Stringer("type", m.MessageType()), zap.Error(err))
		return err
	}

	return nil
}

func (s *SMSF) reachAndSend(ctx context.Context, imsi string, data *sms.CPData) error {
	transport, access, err := s.route(imsi)
	if err != nil {
		return err
	}

	if err := transport.EnableUEReachability(ctx, imsi, access); err != nil {
		s.logger.Debug("Could not reach the UE for an SMS", zap.String("imsi", imsi), zap.Error(err))
		return err
	}

	return s.send(ctx, imsi, data)
}

func (s *SMSF) sendReliably(ctx context.Context, imsi string, data *sms.CPData, ack, aborted, retry <-chan struct{}) error {
	for attempt := 0; ; attempt++ {
		if err := s.reachAndSend(ctx, imsi, data); err != nil {
			if attempt > 0 {
				return fmt.Errorf("%w: retransmission failed: %w", errNoCPAck, err)
			}

			return err
		}

		timer := time.NewTimer(s.timers.TC1)

		select {
		case <-ack:
			timer.Stop()
			return nil
		case <-aborted:
			timer.Stop()
			return errAborted
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("%w: %w", errNoCPAck, ctx.Err())
		case <-retry:
			timer.Stop()
		case <-timer.C:
		}

		if attempt >= s.timers.MaxRetransmissions {
			return errNoCPAck
		}
	}
}

var (
	errNoCPAck = errors.New("smsf: no CP-ACK after the last CP-DATA retransmission")
	errAborted = errors.New("smsf: UE aborted the transaction with CP-ERROR")
)

func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
