// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

func idleMobilityGuami() *models.Guami {
	return &models.Guami{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, AmfID: "810040"}
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
	ue.ForceStateForTest(Registered)
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

// TS 33.501 §8.5.2
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

// TS 23.501 §5.17.2
func TestEPSContextRefusesASubscriberBarredFromEPS(t *testing.T) {
	a := idleMobilityAMF()
	guti := idleMobilityGUTI(t)
	ue := leavingUE(t, a, guti)
	ue.SetAllow4G(false)

	count, err := ue.ulCount.Estimate(0)
	if err != nil {
		t.Fatal(err)
	}

	_, err = a.EPSContext(context.Background(), mappedRequest(t, ue, guti, count))
	if !errors.Is(err, interworking.ErrUnknownUEContext) {
		t.Fatalf("error = %v, want the AMF to hand out no context for a subscriber restricted from EPC", err)
	}

	if ue.State() != Registered {
		t.Error("the refusal disturbed the UE's 5GS registration")
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

// TS 24.301 §5.5.3.2.7
func TestEPSContextRemapsARepeatedTrackingAreaUpdate(t *testing.T) {
	a := idleMobilityAMF()
	guti := idleMobilityGUTI(t)
	ue := leavingUE(t, a, guti)

	count, err := ue.ulCount.Estimate(0)
	if err != nil {
		t.Fatal(err)
	}

	first, err := a.EPSContext(context.Background(), mappedRequest(t, ue, guti, count))
	if err != nil {
		t.Fatalf("EPSContext: %v", err)
	}

	if err := a.EPSContextAck(context.Background(), ue.Supi(), []uint8{3}); err != nil {
		t.Fatalf("EPSContextAck: %v", err)
	}

	next, err := ue.ulCount.Estimate(uint8(count) + 1)
	if err != nil {
		t.Fatal(err)
	}

	repeat, err := a.EPSContext(context.Background(), mappedRequest(t, ue, guti, next))
	if err != nil {
		t.Fatalf("the repeated update was refused, so the MME can only reject it: %v", err)
	}

	want := kdfFor(t, ue.kamf, 0x73, []byte{0, 0, 0, uint8(next)})
	if !bytes.Equal(repeat.Security.KASME[:], want) {
		t.Errorf("K'ASME = %x, want the FC 0x73 derivation over the repeated TAU's count %x", repeat.Security.KASME, want)
	}

	if bytes.Equal(repeat.Security.KASME[:], first.Security.KASME[:]) {
		t.Error("the repeated update returned the first mapped context, so the MME would protect the accept under a key the UE has replaced")
	}

	if repeat.Security.ULNASCount != next {
		t.Errorf("mapped uplink NAS COUNT = %d, want the repeated TAU's %d", repeat.Security.ULNASCount, next)
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

	t.Run("a context that is not registered", func(t *testing.T) {
		a := idleMobilityAMF()
		guti := idleMobilityGUTI(t)
		ue := leavingUE(t, a, guti)

		count, err := ue.ulCount.Estimate(0)
		if err != nil {
			t.Fatal(err)
		}

		req := mappedRequest(t, ue, guti, count)
		ue.ForceStateForTest(Deregistered)

		if _, err := a.EPSContext(context.Background(), req); !errors.Is(err, interworking.ErrUnknownUEContext) {
			t.Fatalf("error = %v, want an unknown context", err)
		}
	})

	t.Run("a context with no security context installed", func(t *testing.T) {
		a := idleMobilityAMF()
		guti := idleMobilityGUTI(t)
		ue := leavingUE(t, a, guti)

		count, err := ue.ulCount.Estimate(0)
		if err != nil {
			t.Fatal(err)
		}

		req := mappedRequest(t, ue, guti, count)
		ue.SetSecuredForTest(false)

		if _, err := a.EPSContext(context.Background(), req); !errors.Is(err, interworking.ErrUnknownUEContext) {
			t.Fatalf("error = %v, want an unknown context", err)
		}
	})

	// TS 23.502 §4.11.1.3.2 step 6
	t.Run("a context whose EPS retention has run out", func(t *testing.T) {
		a := idleMobilityAMF()
		guti := idleMobilityGUTI(t)
		ue := leavingUE(t, a, guti)

		count, err := ue.ulCount.Estimate(0)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := a.EPSContext(context.Background(), mappedRequest(t, ue, guti, count)); err != nil {
			t.Fatalf("EPSContext: %v", err)
		}

		if err := a.EPSContextAck(context.Background(), ue.Supi(), []uint8{3}); err != nil {
			t.Fatalf("EPSContextAck: %v", err)
		}

		ue.exportableToEPSUntil = time.Now().Add(-time.Second)

		next, err := ue.ulCount.Estimate(uint8(count) + 1)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := a.EPSContext(context.Background(), mappedRequest(t, ue, guti, next)); !errors.Is(err, interworking.ErrUnknownUEContext) {
			t.Fatalf("error = %v, want an unknown context", err)
		}
	})

	t.Run("a container holding no TAU REQUEST", func(t *testing.T) {
		a := idleMobilityAMF()
		guti := idleMobilityGUTI(t)
		ue := leavingUE(t, a, guti)

		count, err := ue.ulCount.Estimate(0)
		if err != nil {
			t.Fatal(err)
		}

		req := mappedRequest(t, ue, guti, count)
		req.EPSNAS = epsFramedEMM(t, ue, count, nas.Bearer3GPP, eps.MsgDetachRequest, 0x01)

		if _, err := a.EPSContext(context.Background(), req); !errors.Is(err, interworking.ErrIntegrityCheckFailed) {
			t.Fatalf("error = %v, want an integrity failure", err)
		}

		if _, err := a.EPSContext(context.Background(), mappedRequest(t, ue, guti, count)); err != nil {
			t.Fatalf("EPSContext after a refused container: %v: the refusal spent the uplink NAS COUNT the real update needs", err)
		}
	})

	// TS 33.501 §8.5.2 step 4
	t.Run("a TAU citing another 5G NAS security context", func(t *testing.T) {
		a := idleMobilityAMF()
		guti := idleMobilityGUTI(t)
		ue := leavingUE(t, a, guti)

		count, err := ue.ulCount.Estimate(0)
		if err != nil {
			t.Fatal(err)
		}

		req := mappedRequest(t, ue, guti, count)
		req.EPSNAS = epsFramedTAUCitingKSI(t, ue, count, nas.Bearer3GPP, uint8(ue.NgKsi().Ksi)+1)

		if _, err := a.EPSContext(context.Background(), req); !errors.Is(err, interworking.ErrIntegrityCheckFailed) {
			t.Fatalf("error = %v, want an integrity failure", err)
		}

		if _, err := a.EPSContext(context.Background(), mappedRequest(t, ue, guti, count)); err != nil {
			t.Fatalf("EPSContext after a refused eKSI: %v: the refusal spent the uplink NAS COUNT the real update needs", err)
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

// TS 23.401 §5.3.3.1
func TestEPSContextHandedOverButNeverAcknowledgedKeepsServingTheUE(t *testing.T) {
	a := idleMobilityAMF()
	guti := idleMobilityGUTI(t)
	ue := leavingUE(t, a, guti)

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

// TS 23.502 §4.11.1.3.2 step 8
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

	if !ue.MobileReachableActiveForTest() {
		t.Error("idle supervision was not restarted at the transfer, so the escalation left running from before it can remove the retained context at any moment")
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

func TestEPSContextReadsTheAmbrUnderTheLock(t *testing.T) {
	const rounds = 64

	a := idleMobilityAMF()
	guti := idleMobilityGUTI(t)
	ue := leavingUE(t, a, guti)

	reqs := make([]interworking.EPSContextRequest, 0, rounds)

	for i := range rounds {
		count, err := ue.ulCount.Estimate(uint8(i))
		if err != nil {
			t.Fatal(err)
		}

		reqs = append(reqs, mappedRequest(t, ue, guti, count))
	}

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		for range rounds {
			ue.SetAmbr(&models.Ambr{
				Uplink:   models.MustParseBitRate("50 Mbps"),
				Downlink: models.MustParseBitRate("100 Mbps"),
			})
		}
	}()

	for _, req := range reqs {
		if _, err := a.EPSContext(context.Background(), req); err != nil {
			t.Fatalf("EPSContext: %v", err)
		}
	}

	wg.Wait()
}

// TS 29.274 §7.3.6
func TestEPSContextRefusesARegisteredUEWithNoTransferableSession(t *testing.T) {
	for _, tc := range []struct {
		name  string
		strip func(*UeContext)
	}{
		{
			name:  "the UE holds no PDU session",
			strip: func(ue *UeContext) { ue.DeleteSmContext(3) },
		},
		{
			name:  "the UE's only PDU session has no EPS bearer identity",
			strip: func(ue *UeContext) { ue.SetEPSBearerIdentity(3, 0) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := idleMobilityAMF()
			guti := idleMobilityGUTI(t)
			ue := leavingUE(t, a, guti)

			tc.strip(ue)

			count, err := ue.ulCount.Estimate(0)
			if err != nil {
				t.Fatal(err)
			}

			_, err = a.EPSContext(context.Background(), mappedRequest(t, ue, guti, count))
			if !errors.Is(err, interworking.ErrNoTransferableSessions) {
				t.Fatalf("EPSContext returned %v, want a refusal the MME can answer with EMM cause #40", err)
			}

			if ue.State() != Registered {
				t.Errorf("the refused UE is %s, want it still Registered in 5GS", ue.State())
			}
		})
	}
}
