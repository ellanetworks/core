// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/nas/eps"
)

func sequenceNumbersOf(t *testing.T, pdus [][]byte) []uint8 {
	t.Helper()

	seqs := make([]uint8, 0, len(pdus))

	for i, pdu := range pdus {
		spm, err := eps.ParseSecurityProtectedMessage(decodeDownlinkNAS(t, pdu))
		if err != nil {
			t.Fatalf("downlink %d: parse security protected message: %v", i, err)
		}

		seqs = append(seqs, spm.SequenceNumber)
	}

	return seqs
}

func TestDownlinkNASWrittenInCountOrder_TS24301_4_4_3_1(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ueConn := ue.Conn()

	const (
		senders  = 8
		messages = 30
	)

	var (
		wg    sync.WaitGroup
		start = make(chan struct{})
	)

	for range senders {
		wg.Add(1)

		go func() {
			defer wg.Done()

			<-start

			for range messages {
				ueConn.SendDownlinkProtected(context.Background(), &eps.EMMStatus{Cause: eps.EMMCauseProtocolErrorUnspecified})
			}
		}()
	}

	close(start)
	wg.Wait()

	if len(cc.sent) != senders*messages {
		t.Fatalf("wrote %d downlink messages, want %d", len(cc.sent), senders*messages)
	}

	for i, seq := range sequenceNumbersOf(t, cc.snapshot()) {
		if int(seq) != i {
			t.Fatalf("downlink %d carries NAS sequence number %d: the MME wrote the downlink NAS COUNTs out of the order it took them in, and a UE estimating the COUNT from the sequence number discards the message",
				i, seq)
		}
	}
}

func TestGuardRetransmissionTakesTheNextCount_TS24301_4_4_3_1(t *testing.T) {
	m := newTestMME(t)
	m.nasGuardCfg.ExpireTime = 5 * time.Millisecond
	m.nasGuardCfg.MaxRetryTimes = 2

	ue, cc := securedUE(t, m)

	plain, err := (&eps.DetachRequestNetwork{TypeOfDetach: eps.DetachTypeReattachNotRequired}).MarshalBinary()
	if err != nil {
		t.Fatalf("marshal Detach Request: %v", err)
	}

	if err := ue.Conn().SendGuardedProtected(context.Background(), "Detach Request", plain, eps.SHTIntegrityProtectedCiphered); err != nil {
		t.Fatalf("SendGuardedProtected: %v", err)
	}

	eventually(t, 2*time.Second, func() bool { return cc.count() >= 3 })

	for i, seq := range sequenceNumbersOf(t, cc.snapshot()[:3]) {
		if int(seq) != i {
			t.Fatalf("transmission %d carries NAS sequence number %d, want %d: a retransmission reusing a spent NAS COUNT repeats the keystream (TS 33.401 §6.5)",
				i, seq, i)
		}
	}
}

func TestResentAttachAcceptTakesTheNextCount_TS24301_5_5_1_2_7d(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	ueConn := ue.Conn()
	ueConn.AttachAcceptPlain = []byte{0x07, 0x42, 0x01}

	ueConn.ResendAttachAccept(context.Background())
	ueConn.ResendAttachAccept(context.Background())
	ueConn.StopNASGuard()

	if cc.count() != 2 {
		t.Fatalf("wrote %d Attach Accepts, want 2", cc.count())
	}

	seqs := sequenceNumbersOf(t, cc.snapshot())
	if seqs[0] != 0 || seqs[1] != 1 {
		t.Fatalf("resent Attach Accepts carry NAS sequence numbers %d and %d, want 0 and 1: a resend under a spent NAS COUNT repeats the keystream (TS 33.401 §6.5)",
			seqs[0], seqs[1])
	}
}
