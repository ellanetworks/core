// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"slices"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"go.uber.org/zap"
)

func registeredUE(t *testing.T) (*AMF, *UeContext, etsi.SUPI, *deregisterTestSmf) {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatalf("NewSUPIFromIMSI: %v", err)
	}

	a := New(nil, nil, nil)

	ue := NewUeContext()
	ue.SetSupi(supi)

	fakeSmf := &deregisterTestSmf{}
	ue.smf = fakeSmf

	ue.TransitionTo(RegistrationInitiated)
	ue.TransitionTo(Registered)

	a.mu.Lock()
	a.UEs[supi] = ue
	a.mu.Unlock()

	return a, ue, supi, fakeSmf
}

// TS 23.501 §5.17.2.2.1
func TestCancelRegistrationDropsTheFiveGSRegistration(t *testing.T) {
	a, ue, supi, _ := registeredUE(t)

	a.CancelRegistration(context.Background(), supi)

	if state := ue.State(); state != Deregistered {
		t.Errorf("5GMM state = %s after the UE attached in EPS, want Deregistered: the subscriber holds two MM states on one 3GPP access", state)
	}

	if _, _, found := a.LookupSubscriber(supi); found {
		t.Error("the subscriber still reports a 5GS registration, so the API reports it on both 4G and 5G")
	}
}

// TS 23.501 §5.17.2.1
func TestCancelRegistrationKeepsTheFiveGSecurityContextForAReturn(t *testing.T) {
	a, ue, supi, _ := registeredUE(t)

	a.CancelRegistration(context.Background(), supi)

	if !ue.ExportableToEPS() {
		t.Error("the 5G security context was discarded with the registration: a return from EPS would need a fresh primary authentication")
	}
}

// TS 23.502 §4.11.1.3.2 step 15
func TestCancelRegistrationReleasesSessionsEPSDidNotAdopt(t *testing.T) {
	a, ue, supi, fakeSmf := registeredUE(t)

	ue.SmContextList[3] = &SmContext{Ref: "stranded-ref"}

	a.CancelRegistration(context.Background(), supi)

	if len(fakeSmf.releaseCalls) != 1 || fakeSmf.releaseCalls[0] != "stranded-ref" {
		t.Errorf("released %v, want [stranded-ref]: a PDU session EPS never adopted is stranded in the SMF", fakeSmf.releaseCalls)
	}
}

func TestCancelRegistrationDefersToARelocationArrivingFromEPS(t *testing.T) {
	a, ue, supi, _ := registeredUE(t)

	if !a.beginRelocationFromEPS(supi, 7, ue) {
		t.Fatal("beginRelocationFromEPS refused a fresh relocation")
	}

	a.CancelRegistration(context.Background(), supi)

	if state := ue.State(); state != Registered {
		t.Errorf("5GMM state = %s, want Registered: the cancel tore down a context an in-flight relocation from EPS was still building", state)
	}
}

func TestCancelRegistrationDefersToAHandoverToEPS(t *testing.T) {
	a, ue, supi, _ := registeredUE(t)

	if _, ok := a.stageRelocationToEPS(ue, nil, nil); !ok {
		t.Fatal("stageRelocationToEPS refused a fresh handover")
	}

	a.CancelRegistration(context.Background(), supi)

	if state := ue.State(); state != Registered {
		t.Errorf("5GMM state = %s, want Registered: the cancel pre-empted an in-flight handover to EPS", state)
	}
}

func TestCancelRegistrationIsANoOpForASubscriberFiveGSNeverHeld(t *testing.T) {
	a, ue, _, fakeSmf := registeredUE(t)

	other, err := etsi.NewSUPIFromIMSI("001010000000002")
	if err != nil {
		t.Fatalf("NewSUPIFromIMSI: %v", err)
	}

	a.CancelRegistration(context.Background(), other)

	if _, ok := a.LookupUeBySupi(other); ok {
		t.Error("cancelling an unknown SUPI materialised a UE context for it")
	}

	if state := ue.State(); state != Registered {
		t.Errorf("5GMM state = %s, want Registered: the cancel hit the wrong subscriber", state)
	}

	if len(fakeSmf.releaseCalls) != 0 {
		t.Errorf("released %v for a subscriber the cancel does not name", fakeSmf.releaseCalls)
	}
}

func TestCancelRegistrationIsIdempotent(t *testing.T) {
	a, ue, supi, fakeSmf := registeredUE(t)

	for range 3 {
		a.CancelRegistration(context.Background(), supi)
	}

	if !ue.ExportableToEPS() {
		t.Error("a repeated cancel discarded the retained 5G security context")
	}

	if len(fakeSmf.releaseCalls) != 0 {
		t.Errorf("released %v on a UE with no sessions", fakeSmf.releaseCalls)
	}
}

// TS 24.501 §5.5.1.3.2
func TestSupersedeEPSRegistrationIgnoresTheUEStatusIE(t *testing.T) {
	a, ue, supi, _ := registeredUE(t)

	peer := &cancelRecordingEPSPeer{}
	a.EPS = peer

	a.SupersedeEPSRegistration(context.Background(), ue)

	if !slices.Equal(peer.cancelled, []etsi.SUPI{supi}) {
		t.Errorf("cancelled %v, want the subscriber's EPS registration dropped: it holds an MM state in both the AMF and the MME", peer.cancelled)
	}
}

func TestSupersedeEPSRegistrationDefersToARelocationArrivingFromEPS(t *testing.T) {
	a, ue, supi, _ := registeredUE(t)

	peer := &cancelRecordingEPSPeer{}
	a.EPS = peer

	if !a.beginRelocationFromEPS(supi, 7, ue) {
		t.Fatal("beginRelocationFromEPS refused a fresh relocation")
	}

	a.SupersedeEPSRegistration(context.Background(), ue)

	if len(peer.cancelled) != 0 {
		t.Errorf("cancelled %v mid-relocation: the supersede pre-empted the procedure that owns both halves", peer.cancelled)
	}
}

// TS 38.413 §8.3.1
func TestCancelRegistrationReleasesTheNGAPConnection(t *testing.T) {
	a, ue, supi, _ := registeredUE(t)

	radio := &Radio{Conn: new(sctp.SCTPConn), name: "gNB-1", amf: a, Log: zap.NewNop()}

	a.mu.Lock()
	a.reg.Track(radio.Conn, radio)
	a.mu.Unlock()

	ueConn := NewUeConnForTest(radio, models.RanUeNgapID(7), models.AmfUeNgapID(7), zap.NewNop())

	a.mu.Lock()
	a.attachUeConnLocked(ue, ueConn)
	a.mu.Unlock()

	a.CancelRegistration(context.Background(), supi)

	if ueConn.ReleaseAction != UeContextReleaseToEPS {
		t.Errorf("release action = %v, want UeContextReleaseToEPS", ueConn.ReleaseAction)
	}

	a.ReleaseUeConn(context.Background(), ueConn)

	a.mu.Lock()
	_, indexed := a.conns[int64(ueConn.AmfUeNgapID)]
	a.mu.Unlock()

	if indexed {
		t.Error("the NGAP UE context outlived the 5GS registration, so the gNB holds a UE the AMF has given up")
	}

	held, ok := a.LookupUeBySupi(supi)
	if !ok || held != ue {
		t.Fatal("releasing the connection threw away the retained 5G security context")
	}

	if !held.ExportableToEPS() {
		t.Error("the retained context is no longer exportable, so a return from EPS costs a primary authentication")
	}
}

func TestMarkRegisteredLeavesEPSAloneWhenTheTransitionIsRejected(t *testing.T) {
	a, ue, _, _ := registeredUE(t)

	peer := &cancelRecordingEPSPeer{}
	a.EPS = peer

	ue.ForceStateForTest(Deregistered)

	a.MarkRegistered(context.Background(), ue)

	if state := ue.State(); state == Registered {
		t.Fatalf("precondition broken: Deregistered → Registered was accepted (state %s)", state)
	}

	if len(peer.cancelled) != 0 {
		t.Errorf("cancelled %v though the UE never reached 5GMM-REGISTERED", peer.cancelled)
	}
}

type cancelRecordingEPSPeer struct {
	cancelled []etsi.SUPI
}

func (p *cancelRecordingEPSPeer) CancelRegistration(_ context.Context, supi etsi.SUPI) {
	p.cancelled = append(p.cancelled, supi)
}

func (*cancelRecordingEPSPeer) ForwardRelocation(context.Context, interworking.ForwardRelocationRequest) (interworking.ForwardRelocationResponse, error) {
	return interworking.ForwardRelocationResponse{}, nil
}

func (*cancelRecordingEPSPeer) RelocationCancel(context.Context, etsi.SUPI, interworking.RelocationID) error {
	return nil
}

func (*cancelRecordingEPSPeer) RelocationComplete(context.Context, etsi.SUPI, interworking.RelocationID) error {
	return nil
}

func (*cancelRecordingEPSPeer) MMContext(context.Context, interworking.MMContextRequest) (interworking.MMContextResponse, error) {
	return interworking.MMContextResponse{}, nil
}

func (*cancelRecordingEPSPeer) MMContextAck(context.Context, etsi.SUPI, []uint8) error {
	return nil
}
