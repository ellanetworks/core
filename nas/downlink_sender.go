// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import "sync"

type ProtectFunc func(plain []byte, sht uint8, count Count, sc *SecurityContext) ([]byte, error)

type WriteFunc func(wire []byte) error

type DownlinkSender struct {
	mu      sync.Mutex
	count   DownlinkCounter
	sc      *SecurityContext
	protect ProtectFunc
}

func NewDownlinkSender(protect ProtectFunc) *DownlinkSender {
	return &DownlinkSender{protect: protect}
}

func (s *DownlinkSender) Install(sc *SecurityContext, count DownlinkCounter) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sc = sc
	s.count = count
}

func (s *DownlinkSender) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sc = nil
}

func (s *DownlinkSender) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.count.Reset()
}

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

func (s *DownlinkSender) Next() Count {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.count.Next()
}

func (s *DownlinkSender) UseForMappedContext() (Count, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.count.Use()
}

func (s *DownlinkSender) Exhausted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.count.Exhausted()
}
