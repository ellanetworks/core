// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

func relocatableUE(t *testing.T) *amf.UeContext {
	t.Helper()

	ue := mappableUE(t)
	ue.SetAllow4G(true)
	ue.Ambr = &models.Ambr{
		Uplink:   models.MustParseBitRate("50 Mbps"),
		Downlink: models.MustParseBitRate("100 Mbps"),
	}

	if err := ue.CreateSmContext(1, "ref-1", &models.Snssai{Sst: 1}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if err := assignEBI(t, ue, 1); err != nil {
		t.Fatalf("assign an EPS bearer identity: %v", err)
	}

	return ue
}

var testTarget = interworking.ENBIdentity{
	PlmnID:         models.PlmnID{Mcc: "001", Mnc: "01"},
	ID:             0x1234,
	Bits:           20,
	SelectedEPSTAI: interworking.EPSTAI{PlmnID: models.PlmnID{Mcc: "001", Mnc: "01"}, TAC: 7},
}

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

// TS 23.502 §4.11.1.4.1: the EPS bearer identity is what makes a PDU session an
// EPS interworking session, so a session that holds one stays transferable. The
// AMF must not re-derive interworking support at mobility time from state that
// can legitimately be absent — a UE arriving from EPS on a resumed native security
// context has sent no 5GMM capability (TS 24.501 §4.4.6) — or it hands EPS zero
// sessions and strands the very PDN connection it just adopted.
func TestTransferableEPSSessionsKeepsASessionThatHoldsABearerIdentity(t *testing.T) {
	for _, tc := range []struct {
		name string
		blur func(ue *amf.UeContext)
	}{
		{"the AMF holds no 5GMM capability", func(ue *amf.UeContext) { ue.ForgetGMMCapabilityForTest() }},
		{"the UE reports no S1 mode support", func(ue *amf.UeContext) {
			ue.SetUECapabilities(&fgs.GMMCapability{S1Mode: false}, nil)
		}},
		{"the subscriber is barred from 4G", func(ue *amf.UeContext) { ue.SetAllow4G(false) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ue := relocatableUE(t)
			tc.blur(ue)

			got := ue.TransferableEPSSessions([]uint8{1})
			if len(got) != 1 || got[0].EPSBearerIdentity != 5 {
				t.Fatalf("transferable sessions = %+v, want the session holding EBI 5", got)
			}

			if all := ue.AllTransferableEPSSessions(); len(all) != 1 {
				t.Fatalf("got %d transferable sessions, want the same one", len(all))
			}
		})
	}
}

// TS 23.502 §4.11.5.3 step 3: the decision belongs at EBI assignment instead.
func TestEPSInterworkingAllowedGatesBearerIdentityAssignment(t *testing.T) {
	for _, tc := range []struct {
		name string
		blur func(ue *amf.UeContext)
	}{
		{"the AMF holds no 5GMM capability", func(ue *amf.UeContext) { ue.ForgetGMMCapabilityForTest() }},
		{"the UE reports no S1 mode support", func(ue *amf.UeContext) {
			ue.SetUECapabilities(&fgs.GMMCapability{S1Mode: false}, nil)
		}},
		{"the subscriber is barred from 4G", func(ue *amf.UeContext) { ue.SetAllow4G(false) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ue := relocatableUE(t)

			if !ue.EPSInterworkingAllowed() {
				t.Fatal("a UE that reports S1 mode and is allowed 4G may not open an interworking session")
			}

			tc.blur(ue)

			if ue.EPSInterworkingAllowed() {
				t.Fatal("a new PDU session would be given a mapped EPS bearer context anyway")
			}
		})
	}
}

// TS 24.501 §4.4.6, TS 23.502 §4.11.5.3: an arrival from EPS is evidence of S1
// mode support in its own right, for the window before the UE re-sends the
// non-cleartext 5GMM capability that carries the bit.
func TestArrivalFromEPSAttestsS1Mode(t *testing.T) {
	ue := relocatableUE(t)
	ue.ForgetGMMCapabilityForTest()

	if ue.SupportsS1Mode() {
		t.Fatal("S1 mode support is claimed with nothing to claim it from")
	}

	ue.AttestS1Mode()

	if !ue.SupportsS1Mode() {
		t.Fatal("a UE whose EPS context the MME just handed over does not support S1 mode")
	}

	// The UE's own word still wins where it has given one.
	ue.SetUECapabilities(&fgs.GMMCapability{S1Mode: false}, nil)

	if ue.SupportsS1Mode() {
		t.Fatal("the attestation outranks the 5GMM capability the UE actually sent")
	}
}

func TestTransferableEPSSessionsSkipsASessionWithNoBearerIdentity(t *testing.T) {
	ue := relocatableUE(t)

	if err := ue.CreateSmContext(2, "ref-2", &models.Snssai{Sst: 2}, "ims"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	got := ue.TransferableEPSSessions([]uint8{2})
	if len(got) != 0 {
		t.Fatalf("got %d transferable sessions, want none for a session with no EBI", len(got))
	}
}

// TS 23.502 §4.11.1.3.2 step 5a
func TestAllTransferableEPSSessions(t *testing.T) {
	ue := relocatableUE(t)

	if err := ue.CreateSmContext(2, "ref-2", &models.Snssai{Sst: 2}, "ims"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if err := assignEBI(t, ue, 2); err != nil {
		t.Fatalf("assign an EPS bearer identity: %v", err)
	}

	got := ue.AllTransferableEPSSessions()
	if len(got) != 2 {
		t.Fatalf("got %d transferable sessions, want both", len(got))
	}

	if asked := ue.TransferableEPSSessions(nil); len(asked) != 0 {
		t.Fatalf("got %d sessions for an empty allow-list, want none", len(asked))
	}
}

func TestAllTransferableEPSSessionsAppliesTheSameFilters(t *testing.T) {
	ue := relocatableUE(t)

	if err := ue.CreateSmContext(2, "ref-2", &models.Snssai{Sst: 2}, "ims"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if got := ue.AllTransferableEPSSessions(); len(got) != 1 || got[0].PDUSessionID != 1 {
		t.Fatalf("transferable sessions = %+v, want PDU session 1 alone", got)
	}
}

func TestBuildForwardRelocationRequest(t *testing.T) {
	ue := relocatableUE(t)

	consumed := ue.NextDownlinkCountForTest()

	req, mapped, err := ue.BuildForwardRelocationRequest(testTarget, []byte{0xaa, 0xbb}, []uint8{1}, nil)
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

	if got, want := mapped.Container.SequenceNumber, consumed.SQN(); got != want {
		t.Fatalf("container sequence number = %d, want the consumed count's %d", got, want)
	}

	if got, want := req.SecurityContext.DLNASCount, consumed.Next(); got != want {
		t.Fatalf("mapped downlink NAS COUNT = %d, want the one after the consumed %d", got, want)
	}
}

func TestBuildForwardRelocationRequestWithoutSessions(t *testing.T) {
	ue := mappableUE(t)

	before := ue.NextDownlinkCountForTest()

	if _, _, err := ue.BuildForwardRelocationRequest(testTarget, nil, []uint8{1}, nil); !errors.Is(err, amf.ErrNoTransferableSessions) {
		t.Fatalf("error = %v, want ErrNoTransferableSessions", err)
	}

	after := ue.NextDownlinkCountForTest()

	if after != before {
		t.Fatal("a refused handover must not consume a downlink NAS COUNT")
	}
}

func TestENBIdentityFromNGAP(t *testing.T) {
	var plmn ngap.PLMNIdentity

	plmn[0], plmn[1], plmn[2] = 0x00, 0xf1, 0x10

	var sharedPLMN ngap.PLMNIdentity

	sharedPLMN[0], sharedPLMN[1], sharedPLMN[2] = 0x00, 0xf2, 0x10

	got, err := amf.ENBIdentityFromNGAP(ngap.TargeteNBID{
		GlobalENBID: ngap.GlobalNgENBID{
			PLMNIdentity: plmn,
			NgENBID:      ngap.NgENBID{Kind: ngap.NgENBIDMacro, Value: 0x1234},
		},
		SelectedEPSTAI: ngap.EPSTAI{PLMNIdentity: sharedPLMN, TAC: 7},
	})
	if err != nil {
		t.Fatalf("ENBIdentityFromNGAP: %v", err)
	}

	if got.ID != 0x1234 || got.Bits != 20 || got.SelectedEPSTAI.TAC != 7 {
		t.Fatalf("identity = %+v, want macro 0x1234 in TAC 7", got)
	}

	if got.PlmnID.Mcc != "001" || got.PlmnID.Mnc != "01" {
		t.Fatalf("PLMN = %+v, want 001/01", got.PlmnID)
	}

	if got.SelectedEPSTAI.PlmnID.Mcc != "002" || got.SelectedEPSTAI.PlmnID.Mnc != "01" {
		t.Fatalf("selected TAI PLMN = %+v, want the 002/01 the source chose", got.SelectedEPSTAI.PlmnID)
	}
}

func TestENBIdentityFromNGAPRejectsAnUnknownKind(t *testing.T) {
	if _, err := amf.ENBIdentityFromNGAP(ngap.TargeteNBID{
		GlobalENBID: ngap.GlobalNgENBID{NgENBID: ngap.NgENBID{Kind: 99}},
	}); err == nil {
		t.Fatal("an unknown ng-eNB identity kind must be refused")
	}
}
