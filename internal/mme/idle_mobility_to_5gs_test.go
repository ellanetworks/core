// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func idleMobilityUE(t *testing.T, m *MME) (*UeContext, eps.GUTI) {
	t.Helper()

	ue := idleRegisteredUE(t, m)

	p := testPDN(ue)
	p.PDUSessionID = 3
	p.Apn = "internet"
	p.Snssai = &models.Snssai{Sst: 1, Sd: "010203"}

	ue.Ambr = &models.Ambr{
		Uplink:   models.MustParseBitRate("50 Mbps"),
		Downlink: models.MustParseBitRate("100 Mbps"),
	}

	group, code, err := m.MmeIdentity(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	return ue, eps.GUTI{
		PLMN:       nas.PLMN{MCC: "001", MNC: "01"},
		MMEGroupID: group,
		MMECode:    code,
		TMSI:       tmsiOctets(ue.Tmsi()),
	}
}

func enclosedTAU(t *testing.T, ue *UeContext, count nas.Count) []byte {
	t.Helper()

	plain, err := (&eps.TrackingAreaUpdateRequest{
		EPSUpdateType: 0,
		OldGUTI:       testGUTIIdentity(t, ue),
	}).MarshalBinary()
	if err != nil {
		t.Fatalf("encode TAU: %v", err)
	}

	ue.mu.Lock()
	sc := ue.sc
	ue.mu.Unlock()

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtected, count, nas.DirectionUplink, sc)
	if err != nil {
		t.Fatalf("protect TAU: %v", err)
	}

	return wire
}

func testGUTIIdentity(t *testing.T, ue *UeContext) eps.EPSMobileIdentity {
	t.Helper()

	return eps.GUTIIdentity(eps.GUTI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: tmsiOctets(ue.Tmsi()),
	})
}

// TS 23.502 §4.11.1.3.3 step 5a, TS 33.501 §8.2
func TestMMContextReturnsTheContextForAVerifiedTAU(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleMobilityUE(t, m)

	resp, err := m.MMContext(t.Context(), interworking.MMContextRequest{
		MappedEPSGUTI: guti,
		EPSNAS:        enclosedTAU(t, ue, nas.MakeCount(0, 0)),
	})
	if err != nil {
		t.Fatalf("MMContext: %v", err)
	}

	if resp.SUPI != ue.Supi() {
		t.Errorf("SUPI = %s, want %s", resp.SUPI, ue.Supi())
	}

	if len(resp.PDNConnections) != 1 || resp.PDNConnections[0].PDUSessionID != 3 {
		t.Fatalf("PDN connections = %+v, want the UE's one on PDU session 3", resp.PDNConnections)
	}

	if resp.AMBRUplink != ue.Ambr.Uplink || resp.AMBRDownlink != ue.Ambr.Downlink {
		t.Errorf("AMBR = %v/%v, want the subscribed %v/%v",
			resp.AMBRUplink, resp.AMBRDownlink, ue.Ambr.Uplink, ue.Ambr.Downlink)
	}

	if resp.Security.ULNASCount != nas.MakeCount(0, 0) {
		t.Errorf("uplink NAS COUNT = %d, want the TAU's 0", resp.Security.ULNASCount)
	}
}

// TS 33.501 §8.2
func TestMMContextCommitsTheCountItVerifiedAt(t *testing.T) {
	m := newTestMME(t)
	ue, guti := idleMobilityUE(t, m)

	req := interworking.MMContextRequest{
		MappedEPSGUTI: guti,
		EPSNAS:        enclosedTAU(t, ue, nas.MakeCount(0, 0)),
	}

	if _, err := m.MMContext(t.Context(), req); err != nil {
		t.Fatalf("MMContext: %v", err)
	}

	if _, err := m.MMContext(t.Context(), req); !errors.Is(err, interworking.ErrIntegrityCheckFailed) {
		t.Fatalf("replayed request returned %v, want an integrity failure", err)
	}
}

func TestMMContextRefusals(t *testing.T) {
	t.Run("a GUTI this MME did not assign", func(t *testing.T) {
		m := newTestMME(t)
		ue, guti := idleMobilityUE(t, m)

		guti.MMECode++

		_, err := m.MMContext(t.Context(), interworking.MMContextRequest{
			MappedEPSGUTI: guti,
			EPSNAS:        enclosedTAU(t, ue, nas.MakeCount(0, 0)),
		})
		if !errors.Is(err, interworking.ErrUnknownUEContext) {
			t.Fatalf("error = %v, want an unknown context", err)
		}
	})

	t.Run("no context for the M-TMSI", func(t *testing.T) {
		m := newTestMME(t)
		ue, guti := idleMobilityUE(t, m)

		guti.TMSI = [4]byte{0xde, 0xad, 0xbe, 0xef}

		_, err := m.MMContext(t.Context(), interworking.MMContextRequest{
			MappedEPSGUTI: guti,
			EPSNAS:        enclosedTAU(t, ue, nas.MakeCount(0, 0)),
		})
		if !errors.Is(err, interworking.ErrUnknownUEContext) {
			t.Fatalf("error = %v, want an unknown context", err)
		}
	})

	t.Run("a TAU that does not verify", func(t *testing.T) {
		m := newTestMME(t)
		ue, guti := idleMobilityUE(t, m)

		tau := enclosedTAU(t, ue, nas.MakeCount(0, 0))
		tau[2] ^= 0xff // corrupt the MAC

		_, err := m.MMContext(t.Context(), interworking.MMContextRequest{MappedEPSGUTI: guti, EPSNAS: tau})
		if !errors.Is(err, interworking.ErrIntegrityCheckFailed) {
			t.Fatalf("error = %v, want an integrity failure", err)
		}
	})

	t.Run("a container holding no TAU REQUEST", func(t *testing.T) {
		m := newTestMME(t)
		ue, guti := idleMobilityUE(t, m)

		ue.mu.Lock()
		sc := ue.sc
		ue.mu.Unlock()

		plain, err := (&eps.DetachRequestUE{TypeOfDetach: 1, EPSMobileIdentity: testGUTIIdentity(t, ue)}).MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		wire, err := eps.Protect(plain, eps.SHTIntegrityProtected, nas.MakeCount(0, 0), nas.DirectionUplink, sc)
		if err != nil {
			t.Fatal(err)
		}

		_, err = m.MMContext(t.Context(), interworking.MMContextRequest{MappedEPSGUTI: guti, EPSNAS: wire})
		if !errors.Is(err, interworking.ErrIntegrityCheckFailed) {
			t.Fatalf("error = %v, want an integrity failure", err)
		}
	})
}

// TS 23.502 §4.11.1.3.3 step 8
func TestMMContextAckReleasesWhatDidNotTransfer(t *testing.T) {
	m := newTestMME(t)
	ue, _ := idleMobilityUE(t, m)

	if err := m.MMContextAck(t.Context(), ue.Supi(), nil); err != nil {
		t.Fatalf("MMContextAck: %v", err)
	}

	if ue.PDNCount() != 0 {
		t.Errorf("PDN connections = %d, want none: 5GS adopted nothing", ue.PDNCount())
	}

	if ue.EMMState() != EMMDeregistered {
		t.Errorf("EMM state = %v, want deregistered", ue.EMMState())
	}
}

func TestMMContextAckKeepsNothingForAnUnknownSubscriber(t *testing.T) {
	m := newTestMME(t)

	supi, err := etsi.NewSUPIFromIMSI("001010000000999")
	if err != nil {
		t.Fatal(err)
	}

	if err := m.MMContextAck(t.Context(), supi, nil); !errors.Is(err, interworking.ErrUnknownUEContext) {
		t.Fatalf("error = %v, want an unknown context", err)
	}
}
