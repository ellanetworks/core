// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"sync"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// TestDownlinkNASCountConcurrent exercises the per-UE downlink NAS COUNT against
// concurrent protection from the goroutines that touch a connected UE in
// production: the eNB dispatch loop, the data-network reconcile backstop, and the
// network-initiated detach API. Every protected message must consume a unique
// downlink NAS COUNT (TS 24.301); a lost update reuses one and the UE drops the
// message on an integrity failure. Run with -race to also catch the unsynchronised
// access directly.
func TestDownlinkNASCountConcurrent(t *testing.T) {
	m := newTestMME(t)

	conn := new(sctp.SCTPConn)
	m.trackRadio(conn, RadioInfo{Name: "enb-a", ID: "00f110-1"})

	ue := m.NewUe(conn, 7)
	ue.supi, _ = etsi.NewSUPIFromIMSI("001010000000001")
	ue.ForceStateForTest(EMMRegistered)
	// EEA0/EIA0 (null algorithms): the counters, not the cipher, are what this
	// test exercises. The keys are still installed, since a security context
	// without them is not one.
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
