// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsue

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/sms"
)

const maxTransactionValue = 6

var ErrMemoryFull = errors.New("smsue: the UE reported its SMS memory full")

type Message struct {
	From string
	Text string
}

type Stack struct {
	send func(cp []byte) error

	mu         sync.Mutex
	nextTI     uint8
	nextRef    uint8
	memoryFull bool
	mo         map[uint8]chan sms.CPMessage
	answered   map[uint8][]byte
	inbox      chan Message
	rejected   chan struct{}
}

func New(send func(cp []byte) error) *Stack {
	return &Stack{
		send:     send,
		mo:       make(map[uint8]chan sms.CPMessage),
		answered: make(map[uint8][]byte),
		inbox:    make(chan Message, 16),
		rejected: make(chan struct{}, 16),
	}
}

func (s *Stack) SetMemoryFull(full bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.memoryFull = full
}

func (s *Stack) Submit(ctx context.Context, serviceCentre, to, text string) (sms.RPMessage, error) {
	s.mu.Lock()
	ref := s.nextRef
	s.nextRef++
	s.mu.Unlock()

	tpdu, err := EncodeSubmit(ref, to, text)
	if err != nil {
		return nil, err
	}

	return s.originate(ctx, &sms.RPData{Direction: nas.DirectionUplink, Reference: ref, Destination: sms.E164Address(strings.TrimPrefix(serviceCentre, "+")), UserData: tpdu})
}

func (s *Stack) MemoryAvailable(ctx context.Context) (sms.RPMessage, error) {
	s.mu.Lock()
	ref := s.nextRef
	s.nextRef++
	s.memoryFull = false
	s.mu.Unlock()

	return s.originate(ctx, &sms.RPSMMA{Reference: ref})
}

func (s *Stack) Receive(ctx context.Context) (Message, error) {
	select {
	case m := <-s.inbox:
		return m, nil
	case <-ctx.Done():
		return Message{}, fmt.Errorf("smsue: no SMS received: %w", ctx.Err())
	}
}

func (s *Stack) AwaitRejection(ctx context.Context) error {
	select {
	case <-s.rejected:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("smsue: no SMS was refused for lack of memory: %w", ctx.Err())
	}
}

func (s *Stack) originate(ctx context.Context, rp sms.RPMessage) (sms.RPMessage, error) {
	rpdu, err := rp.AppendBinary(nil)
	if err != nil {
		return nil, fmt.Errorf("encode RP message: %w", err)
	}

	s.mu.Lock()
	ti := sms.TransactionIdentifier{Value: s.nextTI}
	s.nextTI = (s.nextTI + 1) % (maxTransactionValue + 1)
	replies := make(chan sms.CPMessage, 8)
	s.mo[ti.Value] = replies
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.mo, ti.Value)
		s.mu.Unlock()
	}()

	if err := s.sendCP(&sms.CPData{TransactionIdentifier: ti, UserData: rpdu}); err != nil {
		return nil, err
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("smsue: no report for the %T: %w", rp, ctx.Err())
		case m := <-replies:
			switch m := m.(type) {
			case *sms.CPAck:
				continue
			case *sms.CPError:
				return nil, fmt.Errorf("smsue: the network aborted the transaction with %s", m.Cause)
			case *sms.CPData:
				report, err := sms.ParseRP(m.UserData, nas.DirectionDownlink)
				if err != nil {
					return nil, fmt.Errorf("smsue: invalid report: %w", err)
				}

				if err := s.sendCP(&sms.CPAck{TransactionIdentifier: ti}); err != nil {
					return nil, err
				}

				return report, nil
			}
		}
	}
}

func (s *Stack) Deliver(cp []byte) error {
	m, err := sms.ParseCP(cp)
	if err != nil {
		return fmt.Errorf("smsue: invalid CP message: %w", err)
	}

	ti := m.TI()

	if ti.Flag {
		s.mu.Lock()
		replies, ok := s.mo[ti.Value]
		s.mu.Unlock()

		if ok {
			replies <- m
		}

		return nil
	}

	data, ok := m.(*sms.CPData)
	if !ok {
		return nil
	}

	return s.terminate(data)
}

func (s *Stack) terminate(data *sms.CPData) error {
	peer := data.TransactionIdentifier.Peer()

	if err := s.sendCP(&sms.CPAck{TransactionIdentifier: peer}); err != nil {
		return err
	}

	rp, err := sms.ParseRP(data.UserData, nas.DirectionDownlink)
	if err != nil {
		return fmt.Errorf("smsue: invalid RP message: %w", err)
	}

	rpData, ok := rp.(*sms.RPData)
	if !ok {
		return nil
	}

	s.mu.Lock()
	previous, duplicate := s.answered[data.TransactionIdentifier.Value]
	full := s.memoryFull
	s.mu.Unlock()

	if duplicate && previous != nil && previous[1] == rpData.Reference {
		return s.sendCP(&sms.CPData{TransactionIdentifier: peer, UserData: previous})
	}

	var report sms.RPMessage = &sms.RPAck{Direction: nas.DirectionUplink, Reference: rpData.Reference}
	if full {
		report = &sms.RPError{Direction: nas.DirectionUplink, Reference: rpData.Reference, Cause: sms.RPCauseMemoryCapacityExceeded}
	}

	payload, err := report.AppendBinary(nil)
	if err != nil {
		return fmt.Errorf("encode RP report: %w", err)
	}

	s.mu.Lock()
	s.answered[data.TransactionIdentifier.Value] = payload
	s.mu.Unlock()

	if err := s.sendCP(&sms.CPData{TransactionIdentifier: peer, UserData: payload}); err != nil {
		return err
	}

	if full {
		s.rejected <- struct{}{}
		return nil
	}

	from, text, err := DecodeDeliver(rpData.UserData)
	if err != nil {
		return err
	}

	s.inbox <- Message{From: from, Text: text}

	return nil
}

func (s *Stack) sendCP(m sms.CPMessage) error {
	b, err := m.AppendBinary(nil)
	if err != nil {
		return fmt.Errorf("encode %s: %w", m.MessageType(), err)
	}

	if err := s.send(b); err != nil {
		return fmt.Errorf("send %s: %w", m.MessageType(), err)
	}

	return nil
}
