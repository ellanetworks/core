// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
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

// TS 23.501 §5.17.2.2.1, TS 23.502 §4.11.1.5.2 step 2
func TestCancelRegistrationDropsTheFiveGSRegistration(t *testing.T) {
	a, ue, supi, _ := registeredUE(t)

	a.CancelRegistration(context.Background(), supi)

	if state := ue.State(); state != Deregistered {
		t.Errorf("5GMM state = %s after the UE attached in EPS, want Deregistered: the subscriber holds two MM states on one 3GPP access", state)
	}

	if _, _, _, found := a.LookupSubscriber(supi); found {
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
	a := New(nil, nil, nil)

	other, err := etsi.NewSUPIFromIMSI("001010000000002")
	if err != nil {
		t.Fatalf("NewSUPIFromIMSI: %v", err)
	}

	a.CancelRegistration(context.Background(), other)
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

// TS 23.501 §5.17.2.3.1 item 1
func TestCancelRegistrationDefersToAUEThatDeclaredItKeepsFiveGS(t *testing.T) {
	a, ue, _, _ := registeredUE(t)

	ue.SetRetainsEPSRegistration(true)

	a.SupersedeEPSRegistration(context.Background(), ue)

	if state := ue.State(); state != Registered {
		t.Errorf("5GMM state = %s, want Registered", state)
	}
}
