// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "sync"

// ProtectFunc protects a plain NAS message under a security context and returns
// the bytes to put on the wire, with sht the security header type of the
// resulting message and count the NAS COUNT protecting it.
type ProtectFunc func(plain []byte, sht uint8, count Count, sc *SecurityContext) ([]byte, error)

// WriteFunc hands a protected NAS message to the transport carrying it to the
// RAN.
type WriteFunc func(wire []byte) error

// DownlinkSender protects and sends the downlink NAS messages of one UE.
//
// It holds a NAS security context and its [DownlinkCounter] together under one
// lock, so that a COUNT is taken, used to protect a message and written within
// the same critical section. Concurrent senders therefore reach the RAN in the
// order their COUNTs were taken, which is what lets the UE estimate the NAS
// COUNT of each message from its sequence number (TS 24.301 §4.4.3.1,
// TS 24.501 §4.4.3.1).
type DownlinkSender struct {
	mu      sync.Mutex
	count   DownlinkCounter
	sc      *SecurityContext
	protect ProtectFunc
}

// NewDownlinkSender returns a sender that protects messages with protect. It
// carries no security context until one is installed, so [DownlinkSender.Send]
// reports [ErrNoSecurityContext] until then.
func NewDownlinkSender(protect ProtectFunc) *DownlinkSender {
	return &DownlinkSender{protect: protect}
}

// Install makes sc the security context protecting downlink messages, with
// count as its downlink NAS COUNT. A context established here takes the zero
// [DownlinkCounter], whose first message carries NAS COUNT zero; one restored
// or taken over from another node takes the count it left off at.
func (s *DownlinkSender) Install(sc *SecurityContext, count DownlinkCounter) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sc = sc
	s.count = count
}

// Clear drops the security context, leaving the counter as it is.
// [DownlinkSender.Send] reports [ErrNoSecurityContext] until another context is
// installed.
func (s *DownlinkSender) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sc = nil
}

// Reset returns the downlink NAS COUNT to the state of a new security context,
// whose first message carries NAS COUNT zero (TS 24.301 §4.4.3.1,
// TS 24.501 §4.4.3.1). The installed security context is left in place.
func (s *DownlinkSender) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.count.Reset()
}

// Send protects plain under the installed security context, as a message with
// security header type sht, and hands the result to write.
//
// The COUNT it takes is spent whether or not protection and the write succeed:
// a COUNT is never reused under the same key (TS 33.501 §6.4.3.1,
// TS 33.401 §6.5), so a retransmission is protected with a fresh one.
func (s *DownlinkSender) Send(plain []byte, sht uint8, write WriteFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sc == nil {
		return ErrNoSecurityContext
	}

	count, err := s.count.Use()
	if err != nil {
		return err
	}

	wire, err := s.protect(plain, sht, count, s.sc)
	if err != nil {
		return err
	}

	return write(wire)
}

// Next returns the NAS COUNT the next protected message will carry, without
// spending it. It is the K_eNB/K_gNB derivation input on the downlink
// (TS 33.401 §A.3, TS 33.501 §A.9).
func (s *DownlinkSender) Next() Count {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.count.Next()
}

// UseForMappedContext spends a downlink NAS COUNT without sending a message,
// for the inter-system NAS transparent container that derives a mapped security
// context from it (TS 33.501 §8.6.1). It returns [ErrCountExhausted] rather
// than reusing a COUNT.
func (s *DownlinkSender) UseForMappedContext() (Count, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.count.Use()
}

// Exhausted reports whether every downlink NAS COUNT has been used, after which
// no further message can be protected under this security context.
func (s *DownlinkSender) Exhausted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.count.Exhausted()
}
