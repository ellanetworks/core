// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

func relocatableUE(t *testing.T) *amf.UeContext {
	t.Helper()

	ue := mappableUE(t)
	ue.Ambr = &models.Ambr{
		Uplink:   models.MustParseBitRate("50 Mbps"),
		Downlink: models.MustParseBitRate("100 Mbps"),
	}

	if err := ue.CreateSmContext(1, "ref-1", &models.Snssai{Sst: 1}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if _, err := ue.AllocateEPSBearerIdentity(1); err != nil {
		t.Fatalf("AllocateEPSBearerIdentity: %v", err)
	}

	return ue
}

var testTarget = interworking.ENBIdentity{PlmnID: models.PlmnID{Mcc: "001", Mnc: "01"}, ID: 0x1234, Bits: 20, EPSTAC: 7}

// TS 23.502 §4.11.1.2.1 step 2
func TestTransferableEPSSessions(t *testing.T) {
	ue := relocatableUE(t)

	if err := ue.CreateSmContext(2, "ref-2", &models.Snssai{Sst: 2}, "ims"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	got := ue.TransferableEPSSessions([]uint8{1, 2})
	if len(got) != 1 {
		t.Fatalf("got %d transferable sessions, want 1", len(got))
	}

	if got[0].PDUSessionID != 1 || got[0].EPSBearerIdentity != 5 || got[0].APN != "internet" {
		t.Fatalf("transferable session = %+v, want PDU session 1 as EBI 5 on internet", got[0])
	}
}

func TestBuildForwardRelocationRequest(t *testing.T) {
	ue := relocatableUE(t)

	consumed, err := ue.NextDownlinkCountForTest()
	if err != nil {
		t.Fatalf("downlink counter: %v", err)
	}

	req, mapped, err := ue.BuildForwardRelocationRequest(testTarget, []byte{0xaa, 0xbb}, []uint8{1})
	if err != nil {
		t.Fatalf("BuildForwardRelocationRequest: %v", err)
	}

	if req.SUPI != ue.Supi() {
		t.Fatalf("SUPI = %v, want %v", req.SUPI, ue.Supi())
	}

	if len(req.PDNConnections) != 1 {
		t.Fatalf("got %d PDN connections, want 1", len(req.PDNConnections))
	}

	if req.SecurityContext.NCC != 2 || !req.SecurityContext.EKSI.Mapped {
		t.Fatalf("security context = %+v, want a mapped context with NCC 2", req.SecurityContext)
	}

	if req.Target != testTarget {
		t.Fatalf("target = %+v, want %+v", req.Target, testTarget)
	}

	if string(req.SourceToTarget) != string([]byte{0xaa, 0xbb}) {
		t.Fatal("the source-to-target container must be relayed verbatim")
	}

	if req.UEAMBRUplink != ue.Ambr.Uplink || req.UEAMBRDownlink != ue.Ambr.Downlink {
		t.Fatal("the UE-AMBR must be the UE's")
	}

	if mapped == nil {
		t.Fatal("the NAS transparent container must accompany the request")
	}

	// TS 33.501 §8.3.2 step 2, §8.6.1
	if got, want := mapped.Container.SequenceNumber, consumed.SQN(); got != want {
		t.Fatalf("container sequence number = %d, want the consumed count's %d", got, want)
	}

	if got, want := req.SecurityContext.DLNASCount, consumed.Next(); got != want {
		t.Fatalf("mapped downlink NAS COUNT = %d, want the one after the consumed %d", got, want)
	}
}

func TestBuildForwardRelocationRequestWithoutSessions(t *testing.T) {
	ue := mappableUE(t)

	before, err := ue.NextDownlinkCountForTest()
	if err != nil {
		t.Fatalf("downlink counter: %v", err)
	}

	if _, _, err := ue.BuildForwardRelocationRequest(testTarget, nil, []uint8{1}); !errors.Is(err, amf.ErrNoTransferableSessions) {
		t.Fatalf("error = %v, want ErrNoTransferableSessions", err)
	}

	after, err := ue.NextDownlinkCountForTest()
	if err != nil {
		t.Fatalf("downlink counter: %v", err)
	}

	if after != before {
		t.Fatal("a refused handover must not consume a downlink NAS COUNT")
	}
}

func TestENBIdentityFromNGAP(t *testing.T) {
	var plmn ngap.PLMNIdentity

	plmn[0], plmn[1], plmn[2] = 0x00, 0xf1, 0x10

	got, err := amf.ENBIdentityFromNGAP(ngap.TargeteNBID{
		GlobalENBID: ngap.GlobalNgENBID{
			PLMNIdentity: plmn,
			NgENBID:      ngap.NgENBID{Kind: ngap.NgENBIDMacro, Value: 0x1234},
		},
		SelectedEPSTAI: ngap.EPSTAI{PLMNIdentity: plmn, TAC: 7},
	})
	if err != nil {
		t.Fatalf("ENBIdentityFromNGAP: %v", err)
	}

	if got.ID != 0x1234 || got.Bits != 20 || got.EPSTAC != 7 {
		t.Fatalf("identity = %+v, want macro 0x1234 in TAC 7", got)
	}

	if got.PlmnID.Mcc != "001" || got.PlmnID.Mnc != "01" {
		t.Fatalf("PLMN = %+v, want 001/01", got.PlmnID)
	}
}

func TestENBIdentityFromNGAPRejectsAnUnknownKind(t *testing.T) {
	if _, err := amf.ENBIdentityFromNGAP(ngap.TargeteNBID{
		GlobalENBID: ngap.GlobalNgENBID{NgENBID: ngap.NgENBID{Kind: 99}},
	}); err == nil {
		t.Fatal("an unknown ng-eNB identity kind must be refused")
	}
}
