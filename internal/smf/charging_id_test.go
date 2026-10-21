// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestEachSessionGetsItsOwnChargingID(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	seen := make(map[uint32]bool)

	for id := uint8(1); id <= 3; id++ {
		ref, _, err := s.CreateSmContext(context.Background(), models.SmContextCreateRequest{SUPI: testSUPI(), PDUSessionID: id, DNN: testDNN, Snssai: testSnssai, RequestType: fgs.RequestTypeInitialRequest, N1SmMsg: buildPDUSessionEstRequest()})
		if err != nil {
			t.Fatalf("CreateSmContext %d: %v", id, err)
		}

		sc := s.GetSession(ref)

		sc.Mutex.Lock()
		cid := sc.ChargingID
		sc.Mutex.Unlock()

		if cid == 0 || seen[cid] {
			t.Fatalf("session %d Charging ID = %d, want a fresh non-zero one (seen %v)", id, cid, seen)
		}

		seen[cid] = true
	}
}

func TestADedicatedBearerGetsItsOwnChargingID(t *testing.T) {
	s, _, upf, _, ref := epsReconcileFixture(t, 0)

	pushRules(t, s, ref, voiceRule())

	if m := lastModify(t, upf); len(bearerUplinkPDRs(m)) != 1 {
		t.Fatalf("UPF modification %+v adds no uplink endpoint for the bearer", m.UpdatePDRs)
	}

	sc := s.GetSession(ref)

	ids := sc.DedicatedChargingIDsForTest()
	if len(ids) != 1 || ids[0] == 0 || ids[0] == sc.ChargingID {
		t.Fatalf("dedicated bearer Charging IDs %v, want one distinct from the PDN connection's %d", ids, sc.ChargingID)
	}

	updated := voiceRule()
	updated.MBR.Uplink = models.BitRateFromBps(64000)
	pushRules(t, s, ref, updated)

	if got := sc.DedicatedChargingIDsForTest(); len(got) != 1 || got[0] != ids[0] {
		t.Fatalf("dedicated bearer Charging IDs after a modification %v, want %v kept", got, ids)
	}
}
