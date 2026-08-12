// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"encoding/binary"
	"errors"
	"sync"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func testSecurityContext(t *testing.T) *nas.SecurityContext {
	t.Helper()

	var (
		ik nas.IntegrityKey
		ck nas.CipherKey
	)

	for i := range ik {
		ik[i] = byte(i + 1)
		ck[i] = byte(i + 0x21)
	}

	sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
		Integrity:    nas.IntegrityAES,
		Ciphering:    nas.CipheringAES,
		IntegrityKey: ik,
		CipherKey:    ck,
	})
	if err != nil {
		t.Fatalf("NewSecurityContext: %v", err)
	}

	return sc
}

// countStamping is a ProtectFunc that records the COUNT it was given in the
// message body, so a test can read back which COUNT protected which write.
func countStamping(plain []byte, _ uint8, count nas.Count, _ *nas.SecurityContext) ([]byte, error) {
	wire := make([]byte, 4, 4+len(plain))
	binary.BigEndian.PutUint32(wire, count.Value())

	return append(wire, plain...), nil
}

func stampedCount(wire []byte) uint32 { return binary.BigEndian.Uint32(wire[:4]) }

func newTestSender(t *testing.T) *nas.DownlinkSender {
	t.Helper()

	s := nas.NewDownlinkSender(countStamping)
	s.Install(testSecurityContext(t), nas.DownlinkCounter{})

	return s
}

// TS 24.301 §4.4.3.1 and TS 24.501 §4.4.3.1: the receiver estimates the sender's
// NAS COUNT from the sequence number, so downlink messages must reach the RAN in
// the order their COUNTs were taken.
func TestDownlinkSenderWritesInCountOrder(t *testing.T) {
	const (
		senders = 8
		each    = 50
	)

	s := newTestSender(t)

	var (
		mu      sync.Mutex
		written []uint32
		wg      sync.WaitGroup
	)

	for range senders {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for range each {
				err := s.Send([]byte{0xaa}, 0, func(wire []byte) error {
					mu.Lock()
					defer mu.Unlock()

					written = append(written, stampedCount(wire))

					return nil
				})
				if err != nil {
					t.Errorf("Send: %v", err)

					return
				}
			}
		}()
	}

	wg.Wait()

	if len(written) != senders*each {
		t.Fatalf("writes = %d, want %d", len(written), senders*each)
	}

	for i, count := range written {
		if count != uint32(i) {
			t.Fatalf("write %d carries NAS COUNT %d: a UE estimates a lower count as an overflow wrap and discards the message", i, count)
		}
	}
}

// A failure burns the NAS COUNT rather than returning it to the pool: a gap is
// free, while a second message under a spent COUNT is key stream reuse
// (TS 33.501 clause 6.4.3, TS 33.401 clause 6.5).
func TestDownlinkSenderBurnsTheCountOnAProtectionFailure(t *testing.T) {
	boom := errors.New("boom")
	s := nas.NewDownlinkSender(func(_ []byte, _ uint8, _ nas.Count, _ *nas.SecurityContext) ([]byte, error) {
		return nil, boom
	})
	s.Install(testSecurityContext(t), nas.DownlinkCounter{})

	if err := s.Send([]byte{0x01}, 0, func([]byte) error { return nil }); !errors.Is(err, boom) {
		t.Fatalf("error = %v, want the protection failure", err)
	}

	if got := s.Next(); got != 1 {
		t.Errorf("next NAS COUNT = %d, want 1: a spent COUNT must never protect a second message", got)
	}
}

func TestDownlinkSenderBurnsTheCountOnAWriteFailure(t *testing.T) {
	boom := errors.New("boom")
	s := newTestSender(t)

	if err := s.Send([]byte{0x01}, 0, func([]byte) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("error = %v, want the write failure", err)
	}

	if got := s.Next(); got != 1 {
		t.Errorf("next NAS COUNT = %d, want 1: the message may have reached the RAN", got)
	}
}

func TestDownlinkSenderRefusesWithoutASecurityContext(t *testing.T) {
	s := nas.NewDownlinkSender(countStamping)

	if err := s.Send([]byte{0x01}, 0, func([]byte) error { return nil }); !errors.Is(err, nas.ErrNoSecurityContext) {
		t.Fatalf("error = %v, want ErrNoSecurityContext", err)
	}
}

// A retransmission is a new outbound protected message and takes the next COUNT
// (TS 24.301 §4.4.3.1, TS 24.501 §4.4.3.1: "new or retransmitted").
func TestDownlinkSenderGivesARetransmissionAFreshCount(t *testing.T) {
	s := newTestSender(t)

	plain := []byte{0x7e, 0x00}

	var counts []uint32

	for range 3 {
		if err := s.Send(plain, 0, func(wire []byte) error {
			counts = append(counts, stampedCount(wire))

			return nil
		}); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}

	if len(counts) != 3 || counts[0] != 0 || counts[1] != 1 || counts[2] != 2 {
		t.Errorf("NAS COUNTs = %v, want 0,1,2", counts)
	}
}

func TestDownlinkSenderInstallTakesOverACount(t *testing.T) {
	s := nas.NewDownlinkSender(countStamping)
	s.Install(testSecurityContext(t), nas.NewDownlinkCounter(7))

	if got := s.Next(); got != 7 {
		t.Fatalf("next NAS COUNT = %d, want the 7 it took over", got)
	}

	if _, err := s.UseForMappedContext(); err != nil {
		t.Fatalf("UseForMappedContext: %v", err)
	}

	if got := s.Next(); got != 8 {
		t.Errorf("next NAS COUNT = %d, want 8: the mapped context spent one", got)
	}
}
