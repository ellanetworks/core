// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
	"go.uber.org/zap"
)

const maxTransactionValue = 6

type cpTxn struct {
	ack     chan struct{}
	aborted chan struct{}
	sent    bool
	sentAt  time.Time
}

func newCPTxn() cpTxn {
	return cpTxn{ack: make(chan struct{}, 1), aborted: make(chan struct{}, 1)}
}

type moTransaction struct {
	cpTxn

	reference uint8
}

type mtTransaction struct {
	cpTxn

	ti            sms.TransactionIdentifier
	reference     uint8
	report        chan sms.RPMessage
	serviceCentre string
	reported      bool
}

type ueState struct {
	mo      map[uint8]*moTransaction
	mt      *mtTransaction
	nextTI  uint8
	nextRef uint8

	deferredAlert bool
	moreMessages  *time.Timer
}

func (s *SMSF) ue(imsi string) *ueState {
	u, ok := s.ues[imsi]
	if !ok {
		u = &ueState{mo: make(map[uint8]*moTransaction)}
		s.ues[imsi] = u
	}

	return u
}

func (s *SMSF) TransactionPending(imsi string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.ues[imsi]

	return ok && u.holdsConnection()
}

func (u *ueState) holdsConnection() bool {
	return u.mt != nil || len(u.mo) > 0 || u.moreMessages != nil
}

func (s *SMSF) holdForMoreMessagesLocked(ctx context.Context, imsi string, u *ueState) {
	var hold *time.Timer

	hold = time.AfterFunc(s.timers.MoreMessages, func() {
		s.mu.Lock()

		expired := u.moreMessages == hold
		if expired {
			u.moreMessages = nil
		}
		s.mu.Unlock()

		if expired {
			s.transactionEnded(context.WithoutCancel(ctx), imsi)
		}
	})

	u.moreMessages = hold
}

func (u *ueState) stopHoldingForMoreMessages() {
	if u.moreMessages != nil {
		u.moreMessages.Stop()
		u.moreMessages = nil
	}
}

func (s *SMSF) transactionEnded(ctx context.Context, imsi string) {
	s.mu.Lock()

	u, ok := s.ues[imsi]
	busy := ok && u.holdsConnection()
	s.mu.Unlock()

	if busy {
		return
	}

	s.transport.SignallingSettled(ctx, imsi)
}

func rpReference(rpdu []byte) (uint8, bool) {
	if len(rpdu) < 2 {
		return 0, false
	}

	return rpdu[1], true
}

func (s *SMSF) Uplink(ctx context.Context, imsi string, payload []byte) {
	msg, err := sms.ParseCP(payload)
	if msg == nil {
		s.rejectCP(ctx, imsi, payload, err)
		return
	}

	if err != nil {
		logger.From(ctx, s.logger).Debug("Ignored non-imperative errors in a CP message", zap.String("imsi", imsi), zap.Error(err))
	}

	switch m := msg.(type) {
	case *sms.CPAck:
		s.cpAck(imsi, m.TransactionIdentifier)
	case *sms.CPError:
		s.cpError(ctx, imsi, m)
	case *sms.CPData:
		s.cpData(ctx, imsi, m)
	}
}

func (s *SMSF) rejectCP(ctx context.Context, imsi string, payload []byte, err error) {
	header, headerErr := sms.ParseCPHeader(payload)
	if headerErr != nil {
		logger.From(ctx, s.logger).Warn("Dropped an undecodable SMS CP message", zap.String("imsi", imsi), zap.Error(err))
		return
	}

	cause := sms.CPCauseProtocolErrorUnspecified

	switch {
	case errors.Is(err, sms.ErrUnknownMessageType):
		cause = sms.CPCauseMessageTypeNonExistent
	case errors.Is(err, sms.ErrInvalidMandatoryIE), errors.Is(err, nas.ErrTruncated):
		cause = sms.CPCauseInvalidMandatoryInformation
	}

	logger.From(ctx, s.logger).Warn("Rejected an invalid SMS CP message", zap.String("imsi", imsi), zap.Error(err))

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

func (s *SMSF) cpError(ctx context.Context, imsi string, m *sms.CPError) {
	ti := m.TransactionIdentifier

	logger.From(ctx, s.logger).Info("UE aborted an SMS transaction",
		zap.String("imsi", imsi), zap.Stringer("ti", ti), zap.Stringer("cause", m.Cause))

	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.ues[imsi]
	if !ok {
		return
	}

	if ti.Flag {
		if u.mt != nil && u.mt.ti.Value == ti.Value {
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

	reference, _ := rpReference(m.UserData)

	s.mu.Lock()
	u := s.ue(imsi)

	if old, ok := u.mo[ti.Value]; ok {
		if !old.sent || old.reference == reference {
			s.mu.Unlock()
			return
		}

		signal(old.ack)
	}

	t := &moTransaction{cpTxn: newCPTxn(), reference: reference}
	u.mo[ti.Value] = t

	for value, other := range u.mo {
		if value != ti.Value && other.sent {
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

	if rp == nil {
		logger.From(ctx, s.logger).Warn("UE answered a mobile-terminated SMS with an invalid RP message", zap.String("imsi", imsi), zap.Error(err))

		rp = &sms.RPError{Direction: nas.DirectionUplink, Reference: t.reference, Cause: sms.RPCauseInvalidMandatoryInformation}
	}

	if !s.recordReport(ctx, imsi, t, rp) {
		logger.From(ctx, s.logger).Info("Ignored an RP report for another message reference",
			zap.String("imsi", imsi), zap.Uint8("rp_reference", rp.MessageReference()), zap.Uint8("expected", t.reference))

		return
	}

	select {
	case t.report <- rp:
	default:
	}
}

func (s *SMSF) recordReport(ctx context.Context, imsi string, t *mtTransaction, rp sms.RPMessage) bool {
	s.mu.Lock()
	ok := rp.MessageReference() == t.reference
	s.mu.Unlock()

	if !ok {
		return false
	}

	switch m := rp.(type) {
	case *sms.RPAck:
		s.delivered(ctx, imsi, t.serviceCentre)
	case *sms.RPError:
		if m.Cause == sms.RPCauseMemoryCapacityExceeded {
			s.markMemoryFull(ctx, imsi, t.serviceCentre)
		}
	}

	s.mu.Lock()
	t.reported = true
	s.mu.Unlock()

	return true
}

func (s *SMSF) send(ctx context.Context, imsi string, m sms.CPMessage) error {
	payload, err := m.MarshalBinary()
	if err != nil {
		return fmt.Errorf("encode %s: %w", m.MessageType(), err)
	}

	ctx = logger.Into(ctx, smsMessageFields(m)...)

	if err := s.transport.SendSMS(ctx, imsi, payload); err != nil {
		logger.From(ctx, s.logger).Debug("Could not send an SMS CP message", zap.String("imsi", imsi), zap.Error(err))
		return err
	}

	return nil
}

func smsMessageFields(m sms.CPMessage) []zap.Field {
	fields := []zap.Field{zap.Stringer("cp_message_type", m.MessageType())}

	data, ok := m.(*sms.CPData)
	if !ok {
		return fields
	}

	if h, err := sms.ParseRPHeader(data.UserData, nas.DirectionDownlink); err == nil {
		fields = append(fields, zap.Stringer("rp_message_type", h.MTI), zap.Uint8("rp_reference", h.Reference))
	}

	return fields
}

func (s *SMSF) reachAndSend(ctx context.Context, imsi string, data *sms.CPData) error {
	pagingCtx, cancel := context.WithTimeout(ctx, s.timers.Paging)
	err := s.transport.EnableUEReachability(pagingCtx, imsi)

	cancel()

	if err != nil {
		logger.From(ctx, s.logger).Debug("Could not reach the UE for an SMS", zap.String("imsi", imsi), zap.Error(err))
		return err
	}

	return s.send(ctx, imsi, data)
}

func (s *SMSF) sendReliably(ctx context.Context, imsi string, data *sms.CPData, t *cpTxn) error {
	for attempt := 0; ; attempt++ {
		select {
		case <-t.aborted:
			return errAborted
		default:
		}

		if err := s.reachAndSend(ctx, imsi, data); err != nil {
			if attempt > 0 {
				return fmt.Errorf("%w: retransmission failed: %w", errNoCPAck, err)
			}

			return err
		}

		s.mu.Lock()
		if !t.sent {
			t.sentAt = time.Now()
		}

		t.sent = true
		s.mu.Unlock()

		if err := s.awaitCPAck(ctx, t); err != errTC1Expired {
			return err
		}

		if attempt >= s.timers.MaxRetransmissions {
			return errNoCPAck
		}
	}
}

func (s *SMSF) awaitCPAck(ctx context.Context, t *cpTxn) error {
	timer := time.NewTimer(s.timers.TC1)
	defer timer.Stop()

	select {
	case <-t.ack:
		return nil
	case <-t.aborted:
		return errAborted
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", errNoCPAck, ctx.Err())
	case <-timer.C:
		return errTC1Expired
	}
}

var (
	errNoCPAck = errors.New("smsf: no CP-ACK after the last CP-DATA retransmission")
	errAborted = errors.New("smsf: UE aborted the transaction with CP-ERROR")

	errTC1Expired = errors.New("smsf: TC1* expired")
)

func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
