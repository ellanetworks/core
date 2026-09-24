// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func TestDownlinkNASCountConcurrent(t *testing.T) {
	m := newTestMME(t)

	conn := new(sctp.SCTPConn)
	m.trackRadio(context.Background(), conn, RadioInfo{Name: "enb-a", ID: "00f110-1"})

	ue := m.NewUe(t.Context(), conn, 7)
	ue.supi, _ = etsi.NewSUPIFromIMSI("001010000000001")
	ue.ForceStateForTest(EMMRegistered)
	ue.cipheringAlg, ue.integrityAlg = 0, 0

	for i := range ue.knasInt {
		ue.knasInt[i], ue.knasEnc[i] = byte(i+1), byte(i+1)
	}

	sc, err := ue.installSecurityContextLocked()
	if err != nil {
		t.Fatal(err)
	}

	ue.downlink().Install(sc, nas.DownlinkCounter{})

	ue.Imei, _ = etsi.NewIMEIFromPEI("353456789012347")

	const (
		writers    = 8
		perWriter  = 256
		totalCount = writers * perWriter
	)

	var wg sync.WaitGroup

	for w := 0; w < writers; w++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			plain, err := (&eps.IdentityRequest{IdentityType: 1}).MarshalBinary()
			if err != nil {
				t.Errorf("marshal Identity Request: %v", err)
				return
			}

			for i := 0; i < perWriter; i++ {
				err := ue.Conn().SendProtected(plain, eps.SHTIntegrityProtectedCiphered, func([]byte) error { return nil })
				if err != nil {
					t.Errorf("SendProtected: %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()

	if got := ue.downlink().Next(); got != totalCount {
		t.Fatalf("downlink NAS COUNT = %d after %d protected messages, want %d (%d counts reused)",
			got, totalCount, totalCount, totalCount-uint32(got))
	}
}

func TestAcceptUplinkAcceptsAMessageOnceUnderConcurrentReplays(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	pdu := uplinkOn(t, ue, encodePlainEPSEMMStatus(t), eps.SHTIntegrityProtectedCiphered)

	var (
		wg       sync.WaitGroup
		accepted atomic.Int32
	)

	for range 16 {
		wg.Go(func() {
			if _, err := ue.AcceptUplink(pdu, nil, eps.SHTIntegrityProtected, eps.SHTIntegrityProtectedCiphered); err == nil {
				accepted.Add(1)
			}
		})
	}

	wg.Wait()

	if n := accepted.Load(); n != 1 {
		t.Fatalf("a protected uplink message was accepted %d times, want once (TS 24.301 §4.4.3.3)", n)
	}
}
