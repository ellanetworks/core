// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"go.uber.org/zap"
)

func newSUPI(t *testing.T, imsi string) etsi.SUPI {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		t.Fatalf("invalid IMSI %q: %v", imsi, err)
	}

	return supi
}

func TestCommitUEIdentity_EmptySupi(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	ue := amf.NewUeContext()

	err := amfInstance.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit())
	if err == nil {
		t.Fatal("expected error for empty SUPI, got nil")
	}
}

func TestCommitUEIdentity_Success(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000002")

	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)

	if err := amfInstance.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	found, ok := amfInstance.LookupUeBySupi(supi)
	if !ok {
		t.Fatal("UE not found after adding")
	}

	if found != ue {
		t.Fatal("found UE does not match added UE")
	}
}

func TestFindAMFUEBySupi_NotFound(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000003")

	_, ok := amfInstance.LookupUeBySupi(supi)
	if ok {
		t.Fatal("expected not found for missing UE")
	}
}

func TestDeregisterAndRemoveAMFUE(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000005")

	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)

	if err := amfInstance.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	amfInstance.DeregisterAndRemoveUeContext(context.Background(), ue)

	_, ok := amfInstance.LookupUeBySupi(supi)
	if ok {
		t.Fatal("UE should have been removed after deregistration")
	}
}

func TestCountRegisteredSubscribers(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	if count := amfInstance.CountRegisteredSubscribers(); count != 0 {
		t.Fatalf("expected 0 registered, got %d", count)
	}

	addTestUE(t, amfInstance, "001010000000007", func(ue *amf.UeContext) {
		ue.ForceStateForTest(amf.Registered)
	})

	addTestUE(t, amfInstance, "001010000000008", func(ue *amf.UeContext) {
		// default state is Deregistered
	})

	if count := amfInstance.CountRegisteredSubscribers(); count != 1 {
		t.Fatalf("expected 1 registered, got %d", count)
	}
}

func TestLookupUeByGuti(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	tmsi, err := etsi.NewTMSI(42)
	if err != nil {
		t.Fatalf("NewTMSI: %v", err)
	}

	guti, err := etsi.NewGUTI5G("001", "01", "cafe00", tmsi)
	if err != nil {
		t.Fatalf("NewGUTI: %v", err)
	}

	ue := addTestUE(t, amfInstance, "001010000000009", func(ue *amf.UeContext) {})
	amfInstance.AssignGutiForTest(ue, guti)

	found, ok := amfInstance.LookupUeByGuti(guti)
	if !ok {
		t.Fatal("expected to find UE by GUTI")
	}

	if found.TmsiForTest() != guti.Tmsi {
		t.Fatalf("5G-TMSI mismatch: got %v, want %v", found.TmsiForTest(), guti.Tmsi)
	}
}

// TestGutiIndexLifecycle verifies the GUTI resolution index is maintained through
// the production reallocation window (old GUTI resolves until freed) and removal.
func TestGutiIndexLifecycle(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	ue := addTestUE(t, amfInstance, "001010000000011", func(ue *amf.UeContext) {})

	if err := amfInstance.ReallocateGUTI(context.Background(), ue); err != nil {
		t.Fatalf("ReallocateGUTI: %v", err)
	}

	// The AMF stores only the 5G-TMSI and resolves an inbound GUTI by its TMSI part, so
	// a lookup GUTI need only carry the UE's current TMSI.
	guti1, _ := etsi.NewGUTI5G("001", "01", "cafe00", ue.TmsiForTest())
	if found, ok := amfInstance.LookupUeByGuti(guti1); !ok || found != ue {
		t.Fatal("UE not resolvable by its GUTI after allocation")
	}

	// Reallocation: both the new and the in-flight old 5G-TMSI resolve to the UE.
	if err := amfInstance.ReallocateGUTI(context.Background(), ue); err != nil {
		t.Fatalf("ReallocateGUTI (realloc): %v", err)
	}

	guti2, _ := etsi.NewGUTI5G("001", "01", "cafe00", ue.TmsiForTest())
	if guti2 == guti1 {
		t.Fatal("reallocation should produce a new GUTI")
	}

	if found, ok := amfInstance.LookupUeByGuti(guti2); !ok || found != ue {
		t.Fatal("UE not resolvable by its new GUTI")
	}

	if found, ok := amfInstance.LookupUeByGuti(guti1); !ok || found != ue {
		t.Fatal("UE not resolvable by its old GUTI during the reallocation window")
	}

	// CommitGUTIRealloc: the old GUTI stops resolving; the current one still resolves.
	amfInstance.CommitGUTIRealloc(ue)

	if _, ok := amfInstance.LookupUeByGuti(guti1); ok {
		t.Fatal("old GUTI must not resolve after CommitGUTIRealloc")
	}

	if _, ok := amfInstance.LookupUeByGuti(guti2); !ok {
		t.Fatal("current GUTI must still resolve after CommitGUTIRealloc")
	}

	// Removal: no GUTI resolves to the removed UE.
	amfInstance.DeregisterAndRemoveUeContext(context.Background(), ue)

	if _, ok := amfInstance.LookupUeByGuti(guti2); ok {
		t.Fatal("removed UE must not resolve by GUTI")
	}
}

// TestReallocateGUTIReuseAndFree verifies that a retransmitted reallocation
// trigger reuses the staged 5G-TMSI (TS 24.501 §5.4.4) and that tearing down
// mid-reallocation returns both the current and staged 5G-TMSI to the allocator.
func TestReallocateGUTIReuseAndFree(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	ue := addTestUE(t, amfInstance, "001010000000012", func(ue *amf.UeContext) {})

	// Initial GUTI assignment: a fresh UE has no TMSI, so nothing is staged as old.
	if err := amfInstance.ReallocateGUTI(context.Background(), ue); err != nil {
		t.Fatalf("ReallocateGUTI (initial): %v", err)
	}

	// Staging reallocation: the current 5G-TMSI moves to old, a new one becomes current.
	if err := amfInstance.ReallocateGUTI(context.Background(), ue); err != nil {
		t.Fatalf("ReallocateGUTI (realloc): %v", err)
	}

	current := ue.TmsiForTest()
	old := ue.OldTmsi()

	if old == etsi.InvalidTMSI {
		t.Fatal("reallocation must stage an old 5G-TMSI")
	}

	if !amfInstance.TmsiInUseForTest(current) || !amfInstance.TmsiInUseForTest(old) {
		t.Fatal("both current and staged 5G-TMSI must be allocated during the reallocation window")
	}

	// A retransmitted trigger while the reallocation is in flight reuses the staged
	// 5G-TMSI rather than allocating another.
	if err := amfInstance.ReallocateGUTI(context.Background(), ue); err != nil {
		t.Fatalf("ReallocateGUTI (retransmit): %v", err)
	}

	if ue.TmsiForTest() != current || ue.OldTmsi() != old {
		t.Fatalf("retransmit must reuse the staged 5G-TMSI: got current=%v old=%v, want current=%v old=%v",
			ue.TmsiForTest(), ue.OldTmsi(), current, old)
	}

	// Teardown mid-reallocation returns both 5G-TMSIs to the allocator.
	amfInstance.DeregisterAndRemoveUeContext(context.Background(), ue)

	if amfInstance.TmsiInUseForTest(current) || amfInstance.TmsiInUseForTest(old) {
		t.Fatal("teardown must free both the current and staged 5G-TMSI")
	}
}

func TestFindUeContextByGuti_InvalidGUTI(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	addTestUE(t, amfInstance, "001010000000010", func(ue *amf.UeContext) {})

	_, ok := amfInstance.LookupUeByGuti(etsi.InvalidGUTI5G)
	if ok {
		t.Fatal("should not find UE with InvalidGUTI")
	}
}

func TestGetUESnapshot(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000011")

	_, ok := amfInstance.UESnapshot(supi)
	if ok {
		t.Fatal("expected no snapshot for missing UE")
	}

	now := time.Now()

	addTestUE(t, amfInstance, "001010000000011", func(ue *amf.UeContext) {
		ue.ForceStateForTest(amf.Registered)
		ue.SetLastSeenForTest(now, "")
	})

	snap, ok := amfInstance.UESnapshot(supi)
	if !ok {
		t.Fatal("expected snapshot for existing UE")
	}

	if !snap.LastSeenAt.Equal(now) {
		t.Fatalf("LastSeenAt mismatch: got %v, want %v", snap.LastSeenAt, now)
	}
}

func TestDeregisterSubscriber(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	supi := newSUPI(t, "001010000000012")

	addTestUE(t, amfInstance, "001010000000012", func(ue *amf.UeContext) {})

	amfInstance.DeregisterSubscriber(context.Background(), supi)

	_, ok := amfInstance.LookupUeBySupi(supi)
	if ok {
		t.Fatal("UE should have been removed after DeregisterSubscriber")
	}
}

// TestDeregisterSubscriberConnectedUnsecuredRemovesLocally checks that deleting a
// subscriber whose UE is connected but has no security context removes the context
// locally without signalling — a protected DEREGISTRATION REQUEST cannot be built,
// so the UE is removed rather than left connected (TS 24.501 §5.5.2.3.1 local
// de-registration). Mirrors the MME's DetachSubscriber.
func TestDeregisterSubscriberConnectedUnsecuredRemovesLocally(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	sender := &fakeNGAPSender{}
	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	ueConn := amf.NewUeConnForTest(radio, 1, 1, zap.NewNop())

	supi := newSUPI(t, "001010000000013")

	ue := addTestUE(t, amfInstance, "001010000000013", func(ue *amf.UeContext) {
		ue.SetSecuredForTest(false)
	})
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	amfInstance.DeregisterSubscriber(context.Background(), supi)

	if _, ok := amfInstance.LookupUeBySupi(supi); ok {
		t.Fatal("connected-but-unsecured UE not removed on subscriber deletion")
	}

	if sender.downlinkNasTransportCalls != 0 {
		t.Fatalf("no deregistration should be signalled for an unsecured UE, got %d DL NAS transports", sender.downlinkNasTransportCalls)
	}
}
