// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

func modifySummary(req *models.ModifyRequest) string {
	var b strings.Builder

	fmt.Fprintf(&b, "policy=%q", req.PolicyID)

	pdrs := make([]string, 0, len(req.UpdatePDRs))
	for _, p := range req.UpdatePDRs {
		ohr := "none"
		if p.OuterHeaderRemoval != nil {
			ohr = fmt.Sprintf("%d", *p.OuterHeaderRemoval)
		}

		pdrs = append(pdrs, fmt.Sprintf("id=%d far=%d qer=%d ohr=%s", p.PDRID, p.FARID, p.QERID, ohr))
	}

	sort.Strings(pdrs)
	fmt.Fprintf(&b, " pdrs=[%s]", strings.Join(pdrs, "; "))

	fars := make([]string, 0, len(req.UpdateFARs))

	for _, f := range req.UpdateFARs {
		action := "drop"

		switch {
		case f.ApplyAction.Forw:
			action = "forw"
		case f.ApplyAction.Buff && f.ApplyAction.Nocp:
			action = "buff+nocp"
		case f.ApplyAction.Buff:
			action = "buff"
		}

		ohc := "none"

		if f.ForwardingParameters != nil && f.ForwardingParameters.OuterHeaderCreation != nil {
			o := f.ForwardingParameters.OuterHeaderCreation
			ohc = fmt.Sprintf("teid=%#x desc=%d s1u=%t", o.TEID, o.Description, o.S1U)
		}

		fars = append(fars, fmt.Sprintf("id=%d %s ohc=%s", f.FARID, action, ohc))
	}

	sort.Strings(fars)
	fmt.Fprintf(&b, " fars=[%s]", strings.Join(fars, "; "))

	qers := make([]string, 0, len(req.UpdateQERs))
	for _, q := range req.UpdateQERs {
		qers = append(qers, fmt.Sprintf("id=%d qfi=%d", q.QERID, q.QFI))
	}

	sort.Strings(qers)
	fmt.Fprintf(&b, " qers=[%s]", strings.Join(qers, "; "))

	return b.String()
}

func lastModify(t *testing.T, upf *fakeUPF) *models.ModifyRequest {
	t.Helper()

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) == 0 {
		t.Fatal("the UPF was sent no session modification")
	}

	return upf.modifyCalls[len(upf.modifyCalls)-1]
}

func assertSEID(t *testing.T, sc *smf.SMContext, req *models.ModifyRequest) {
	t.Helper()

	sc.Mutex.Lock()
	seid := sc.PFCPContext.SEID
	sc.Mutex.Unlock()

	if req.SEID != seid {
		t.Errorf("modification SEID = %d, want the session's %d", req.SEID, seid)
	}
}

// TS 23.502 §4.11.1.3.3 step 8
func TestIdleTransferModificationRebindsTheDownlinkAlone(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	pcf.policy.PolicyID = "policy-1"

	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	sc := establishEPSOnENB(t, s)

	if _, err := s.TransferIdle(context.Background(), testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, smf.Access5G); err != nil {
		t.Fatalf("TransferIdle: %v", err)
	}

	req := lastModify(t, upf)
	assertSEID(t, sc, req)

	const want = `policy="policy-1" pdrs=[] fars=[id=2 buff+nocp ohc=none] qers=[id=1 qfi=1]`
	if got := modifySummary(req); got != want {
		t.Errorf("idle transfer modification:\n got %s\nwant %s", got, want)
	}
}

// TS 23.401 §5.3.3.1
func TestEPSBindModificationCarriesBothPDRs(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	pcf.policy.PolicyID = "policy-1"

	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	create := epsRequest(3)
	create.APN = testDNN
	create.PDUSessionID = movedPDUSessionID
	create.Snssai = testSnssai
	create.PolicyID = "eps-policy"

	bearer, err := s.CreateEPSSession(ctx, create)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.ModifyEPSSession(ctx, bearer.Ref, epsTestEBI, enb); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	sc := s.GetSession(bearer.Ref)
	if sc == nil {
		t.Fatal("EPS session is not in the pool")
	}

	req := lastModify(t, upf)
	assertSEID(t, sc, req)

	const want = `policy="eps-policy" pdrs=[id=1 far=1 qer=1 ohr=0; id=2 far=2 qer=1 ohr=none] fars=[id=2 forw ohc=teid=0x6001 desc=256 s1u=true] qers=[]`
	if got := modifySummary(req); got != want {
		t.Errorf("EPS downlink bind modification:\n got %s\nwant %s", got, want)
	}
}

// TS 23.502 §4.3.2.2.1
func TestNGRANBindModificationCarriesTheN2Rules(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	pcf.policy.PolicyID = "policy-1"

	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc := establish5GS(t, s)

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x7001, net.ParseIP("10.3.0.9"))
	if err != nil {
		t.Fatalf("build N2 setup response: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, sc.Ref, n2); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	req := lastModify(t, upf)
	assertSEID(t, sc, req)

	const want = `policy="" pdrs=[id=1 far=1 qer=1 ohr=0; id=2 far=2 qer=1 ohr=none] fars=[id=2 forw ohc=teid=0x7001 desc=256 s1u=false] qers=[]`
	if got := modifySummary(req); got != want {
		t.Errorf("NG-RAN downlink bind modification:\n got %s\nwant %s", got, want)
	}
}

// TS 23.502 §4.11.1.2.1
func TestHandoverFromEPSModificationSwitchesTheDownlink(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	pcf.policy.PolicyID = "policy-1"

	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc := establishEPSOnENB(t, s)

	ref, _, err := s.PrepareSmContextFromEPS(ctx, testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai)
	if err != nil {
		t.Fatalf("PrepareSmContextFromEPS: %v", err)
	}

	ack, err := buildHandoverRequestAcknowledgeTransfer(0x8001, net.ParseIP("10.3.0.11"))
	if err != nil {
		t.Fatalf("build handover ack transfer: %v", err)
	}

	if _, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, ack); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	if err := s.UpdateSmContextN2HandoverComplete(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverComplete: %v", err)
	}

	req := lastModify(t, upf)
	assertSEID(t, sc, req)

	const want = `policy="policy-1" pdrs=[id=1 far=1 qer=1 ohr=0] fars=[id=2 forw ohc=teid=0x8001 desc=256 s1u=false] qers=[id=1 qfi=1]`
	if got := modifySummary(req); got != want {
		t.Errorf("handover-from-EPS modification:\n got %s\nwant %s", got, want)
	}
}
