// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

func idleMobilityGuami() *models.Guami {
	return &models.Guami{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, AmfID: "010040"}
}

func idleMobilityAMF() *AMF {
	return New(operatorOnlyDB{operator: &db.Operator{
		Mcc: "001", Mnc: "01", AmfRegionID: 1, AmfSetID: 1,
		Ciphering: `["AES"]`, Integrity: `["AES"]`,
	}}, nil, nil)
}

func idleMobilityGUTI(t *testing.T) etsi.GUTI5G {
	t.Helper()

	tmsi, err := etsi.NewTMSI(0x0000dead)
	if err != nil {
		t.Fatal(err)
	}

	guti, err := etsi.NewGUTI5G("001", "01", idleMobilityGuami().AmfID, tmsi)
	if err != nil {
		t.Fatal(err)
	}

	return guti
}

func leavingUE(t *testing.T, a *AMF, guti etsi.GUTI5G) *UeContext {
	t.Helper()

	ue := newSecuredUE(t)

	kamf := make([]byte, 32)
	for i := range kamf {
		kamf[i] = byte(i + 1)
	}

	ue.kamf = kamf

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	ue.SetSupiForTest(supi)
	ue.SetAllow4G(true)
	ue.Ambr = &models.Ambr{
		Uplink:   models.MustParseBitRate("50 Mbps"),
		Downlink: models.MustParseBitRate("100 Mbps"),
	}
	ue.SetUESecurityCapability(&fgs.UESecurityCapability{EA: 0xe0, IA: 0xe0}, MintAuthProofForSecurityMode())
	ue.SetUECapabilities(&fgs.GMMCapability{S1Mode: true}, mustEncodeNetworkCapability(t))

	if _, ok := ue.SelectEPSNASAlgorithms([]nas.IntegrityAlgorithm{nas.IntegrityAES}, []nas.CipheringAlgorithm{nas.CipheringAES}); !ok {
		t.Fatal("could not select the EPS NAS algorithms")
	}

	ue.MarkEPSNASAlgorithmsDelivered()

	if err := a.CommitUEIdentity(context.Background(), ue, MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	a.AssignGutiForTest(ue, guti)

	if err := ue.CreateSmContext(3, "ref-3", &models.Snssai{Sst: 1, Sd: "010203"}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	ue.SetEPSBearerIdentity(3, 6)

	return ue
}

func mustEncodeNetworkCapability(t *testing.T) []byte {
	t.Helper()

	raw, err := (eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	return raw
}

func kdfFor(t *testing.T, key []byte, fc byte, p0 []byte) []byte {
	t.Helper()

	s := append([]byte{fc}, p0...)
	s = append(s, byte(len(p0)>>8), byte(len(p0)))

	mac := hmac.New(sha256.New, key)
	if _, err := mac.Write(s); err != nil {
		t.Fatal(err)
	}

	return mac.Sum(nil)
}

func mappedRequest(t *testing.T, ue *UeContext, guti etsi.GUTI5G, count nas.Count) interworking.EPSContextRequest {
	t.Helper()

	id, err := guti.MobileIdentity()
	if err != nil {
		t.Fatal(err)
	}

	return interworking.EPSContextRequest{
		Mapped5GGUTI: *id.GUTI,
		EPSNAS:       epsFramedTAU(t, ue, count, nas.Bearer3GPP),
	}
}

// TS 33.501 §8.5.2 steps 4-5
func TestEPSContextReturnsTheMappedContext(t *testing.T) {
	a := idleMobilityAMF()
	guti := idleMobilityGUTI(t)
	ue := leavingUE(t, a, guti)

	count, err := ue.ulCount.Estimate(0)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := a.EPSContext(context.Background(), mappedRequest(t, ue, guti, count))
	if err != nil {
		t.Fatalf("EPSContext: %v", err)
	}

	if resp.SUPI != ue.Supi() {
		t.Errorf("SUPI = %s, want %s", resp.SUPI, ue.Supi())
	}

	want := kdfFor(t, ue.kamf, 0x73, []byte{0, 0, 0, uint8(count)})
	if !bytes.Equal(resp.Security.KASME[:], want) {
		t.Errorf("K'ASME = %x, want the FC 0x73 derivation over the TAU's count %x", resp.Security.KASME, want)
	}

	if !resp.Security.EKSI.Mapped {
		t.Errorf("eKSI = %+v, want a mapped one", resp.Security.EKSI)
	}

	if resp.Security.Algorithms != (interworking.EPSNASAlgorithms{Ciphering: nas.CipheringAES, Integrity: nas.IntegrityAES}) {
		t.Errorf("EPS algorithms = %+v, want the pair signalled to the UE", resp.Security.Algorithms)
	}

	if len(resp.PDNConnections) != 1 || resp.PDNConnections[0].EPSBearerIdentity != 6 {
		t.Errorf("PDN connections = %+v, want the session holding EBI 6", resp.PDNConnections)
	}

	if resp.AMBRUplink != ue.Ambr.Uplink || resp.AMBRDownlink != ue.Ambr.Downlink {
		t.Errorf("AMBR = %v/%v, want the subscribed one", resp.AMBRUplink, resp.AMBRDownlink)
	}
}

// TS 33.501 §8.5.2 step 1
func TestEPSContextCommitsTheCountItVerifiedAt(t *testing.T) {
	a := idleMobilityAMF()
	guti := idleMobilityGUTI(t)
	ue := leavingUE(t, a, guti)

	count, err := ue.ulCount.Estimate(0)
	if err != nil {
		t.Fatal(err)
	}

	req := mappedRequest(t, ue, guti, count)

	if _, err := a.EPSContext(context.Background(), req); err != nil {
		t.Fatalf("EPSContext: %v", err)
	}

	if _, err := a.EPSContext(context.Background(), req); !errors.Is(err, interworking.ErrIntegrityCheckFailed) {
		t.Fatalf("replayed request returned %v, want an integrity failure", err)
	}
}

func TestEPSContextRefusals(t *testing.T) {
	t.Run("a 5G-GUTI this AMF did not issue", func(t *testing.T) {
		a := idleMobilityAMF()
		guti := idleMobilityGUTI(t)
		ue := leavingUE(t, a, guti)

		count, err := ue.ulCount.Estimate(0)
		if err != nil {
			t.Fatal(err)
		}

		req := mappedRequest(t, ue, guti, count)
		req.Mapped5GGUTI.AMFPointer++

		if _, err := a.EPSContext(context.Background(), req); !errors.Is(err, interworking.ErrUnknownUEContext) {
			t.Fatalf("error = %v, want an unknown context", err)
		}
	})

	t.Run("a TAU MAC'd over the EPS bearer", func(t *testing.T) {
		a := idleMobilityAMF()
		guti := idleMobilityGUTI(t)
		ue := leavingUE(t, a, guti)

		count, err := ue.ulCount.Estimate(0)
		if err != nil {
			t.Fatal(err)
		}

		req := mappedRequest(t, ue, guti, count)
		req.EPSNAS = epsFramedTAU(t, ue, count, nas.BearerEPS)

		if _, err := a.EPSContext(context.Background(), req); !errors.Is(err, interworking.ErrIntegrityCheckFailed) {
			t.Fatalf("error = %v, want an integrity failure", err)
		}
	})
}

// TS 23.502 §4.11.1.3.2 step 8
// TS 23.401 §5.3.3.1 step 7, which TS 23.502 §4.11.1.3.2 steps 7-14 performs: an
// inter-system change the peer abandons leaves this AMF serving the UE exactly as
// before, so the UE that stays on NR keeps its sessions. Nothing is released until
// the acknowledgement says what moved.
func TestEPSContextHandedOverButNeverAcknowledgedKeepsServingTheUE(t *testing.T) {
	a := idleMobilityAMF()
	guti := idleMobilityGUTI(t)
	ue := leavingUE(t, a, guti)
	ue.ForceStateForTest(Registered)

	count, err := ue.ulCount.Estimate(0)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := a.EPSContext(context.Background(), mappedRequest(t, ue, guti, count)); err != nil {
		t.Fatalf("EPSContext: %v", err)
	}

	if _, ok := ue.SmContextFindByPDUSessionID(3); !ok {
		t.Error("the PDU session was released before EPS said it took the context")
	}

	if ue.State() == Deregistered {
		t.Error("the UE was deregistered before EPS said it took the context")
	}

	if found, ok := a.LookupUeByGuti(idleMobilityGuami(), guti); !ok || found != ue {
		t.Error("the UE context is no longer resolvable by its 5G-GUTI, so a retry would re-authenticate")
	}
}

func TestEPSContextAckReleasesWhatDidNotTransferAndKeepsTheContext(t *testing.T) {
	a := idleMobilityAMF()
	guti := idleMobilityGUTI(t)
	ue := leavingUE(t, a, guti)

	kamf := bytes.Clone(ue.kamf)

	if err := a.EPSContextAck(context.Background(), ue.Supi(), nil); err != nil {
		t.Fatalf("EPSContextAck: %v", err)
	}

	if _, ok := ue.SmContextFindByPDUSessionID(3); ok {
		t.Error("a PDU session EPS did not adopt is still held on 5GS")
	}

	if ue.State() != Deregistered {
		t.Errorf("state = %v, want deregistered", ue.State())
	}

	if !bytes.Equal(ue.kamf, kamf) {
		t.Error("the native 5G security context was discarded, so a return from EPS could not resume on native keys")
	}

	if found, ok := a.LookupUeByGuti(idleMobilityGuami(), guti); !ok || found != ue {
		t.Error("the UE context is no longer resolvable by its 5G-GUTI, so the Additional GUTI of a later registration names nothing")
	}
}

func TestEPSContextAckForAnUnknownSubscriber(t *testing.T) {
	a := idleMobilityAMF()

	supi, err := etsi.NewSUPIFromIMSI("001010000000999")
	if err != nil {
		t.Fatal(err)
	}

	if err := a.EPSContextAck(context.Background(), supi, nil); !errors.Is(err, interworking.ErrUnknownUEContext) {
		t.Fatalf("error = %v, want an unknown context", err)
	}
}
