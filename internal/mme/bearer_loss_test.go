// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

func addSecondPDN(ue *UeContext) *PdnConnection {
	p := ue.EnsurePDN(7)
	p.Apn = "ims"
	p.SessionRef = "ref-ims"
	p.PdnType = eps.PDNTypeIPv4

	return p
}

func lastERABRelease(t *testing.T, cc *captureConn) *s1ap.ERABReleaseCommand {
	t.Helper()

	cmd, err := s1ap.ParseERABReleaseCommand(initiating(t, lastSent(cc), s1ap.ProcERABRelease))
	if err != nil {
		t.Fatalf("parse E-RAB Release Command: %v", err)
	}

	return cmd
}

func releasedEBIs(cmd *s1ap.ERABReleaseCommand) []uint8 {
	var out []uint8
	for _, it := range cmd.ERABToBeReleased {
		out = append(out, uint8(it.ERABID))
	}

	slices.Sort(out)

	return out
}

func TestLostDefaultBearerDisconnectsItsPDNWithItsDedicatedBearers(t *testing.T) {
	m, ue, cc, _ := activeVoiceBearer(t)
	addSecondPDN(ue)

	m.DisconnectLostPDN(context.Background(), ue, m.LookupPDN(ue, 5))

	cmd := lastERABRelease(t, cc)
	if got := releasedEBIs(cmd); !slices.Equal(got, []uint8{5, 6}) {
		t.Fatalf("released E-RABs %v, want the default bearer and its voice bearer", got)
	}

	if deact, err := eps.ParseDeactivateEPSBearerContextRequest(downlinkPlain(t, ue, []byte(cmd.NASPDU))); err != nil || deact.EPSBearerIdentity != 5 {
		t.Fatalf("NAS %+v (%v), want the PDN's default bearer deactivated (TS 23.401 §5.10.3)", deact, err)
	}

	if m.LookupPDN(ue, 7) == nil {
		t.Fatal("the other PDN connection was released too")
	}
}

func TestLostLastDefaultBearerDetachesTheUE(t *testing.T) {
	m, ue, cc, _ := activeVoiceBearer(t)

	m.DisconnectLostPDN(context.Background(), ue, m.LookupPDN(ue, 5))

	detach, err := eps.ParseDetachRequestNetwork(downlinkPlain(t, ue, decodeDownlinkNAS(t, lastSent(cc))))
	if err != nil || detach.TypeOfDetach != eps.DetachTypeReattachRequired {
		t.Fatalf("detach %+v (%v), want a network detach with re-attach required (TS 36.413 §8.2.3.2.2)", detach, err)
	}
}

func TestSignallingOnlyReleaseKeepsAPreservedVoiceBearer(t *testing.T) {
	m, ue, _, fake := activeVoiceBearer(t)
	ctx := context.Background()

	ue.Conn().releaseCause.Store(&s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUserInactivity})
	m.FreeUeConn(ctx, ue)

	c := m.NewUeConn(&captureConn{}, 9)
	m.AttachUeConn(ctx, ue, c)
	c.releaseCause.Store(&s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: s1ap.CauseNASNormalRelease})
	m.FreeUeConn(ctx, ue)

	if _, b := m.LookupDedicated(ue, 6); b == nil {
		t.Fatal("a release of a connection that never carried the voice E-RAB dropped the bearer (TS 23.401 §5.3.5)")
	}

	if _, dropped := fake.dedicatedOutcomes(); len(dropped) != 0 {
		t.Fatalf("SMF told %+v released, want none", dropped)
	}
}

func TestDeactivationDuringAHandoverWaitsForIt(t *testing.T) {
	m, ue, cc, _ := activeVoiceBearer(t)
	ctx := context.Background()

	m.mu.Lock()
	ue.handover = &handoverContext{state: hoPreparing}
	m.mu.Unlock()

	sent := cc.count()

	if err := m.DeactivateDedicatedBearer(ctx, ue.imsiOrEmpty(), 6, voiceSGWTEID); err != nil {
		t.Fatalf("DeactivateDedicatedBearer: %v", err)
	}

	if cc.count() != sent {
		t.Fatal("a deactivation was signalled during the handover preparation (TS 36.413 §8.4.1.2)")
	}

	if _, b := m.LookupDedicated(ue, 6); b == nil {
		t.Fatal("the bearer was deleted locally during the handover while the UE is connected (TS 24.301 §6.4.4.2)")
	}

	m.mu.Lock()
	ue.handover = nil
	m.mu.Unlock()

	m.ResumeBearerReconfigurationAfterHandover(ctx, ue)

	if got := releasedEBIs(lastERABRelease(t, cc)); !slices.Equal(got, []uint8{6}) {
		t.Fatalf("released E-RABs %v after the handover, want the voice bearer", got)
	}
}

func TestDefaultBearerSwitchFailureFailsItsDedicatedBearers(t *testing.T) {
	m, ue, _, fake := activeVoiceBearer(t)
	fake.modifyErr = errors.New("anchor refused")

	enb := models.FTEID{TEID: 0x77, Addr: netip.MustParseAddr("10.3.0.7")}

	result := m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{
		Present:       []RANBearer{{Ebi: 6, EnbFTEID: enb}, {Ebi: 5, EnbFTEID: enb}},
		Authoritative: true,
		ReleaseFailed: true,
		AfterCommit:   true,
	})

	slices.Sort(result.Failed)

	if !slices.Equal(result.Failed, []uint8{5, 6}) || len(result.Applied) != 0 {
		t.Fatalf("result %+v, want the default bearer and its voice bearer failed", result)
	}

	if result.AppliedDefaultBearer(m, ue) {
		t.Fatal("a path switch with no default bearer switched would succeed (TS 23.401 §5.5.1.1.2)")
	}
}

func TestRefusedActivationOnAPathSwitchIsReleased(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	fake := m.Session.(*fakeSessionManager)

	activateVoiceBearer(t, m, ue, cc)
	m.DedicatedBearerAccepted(context.Background(), ue, 6)

	fake.outcomeMu.Lock()
	fake.activateErr = errors.New("UPF refused the rules")
	fake.outcomeMu.Unlock()

	enb := models.FTEID{TEID: 0x77, Addr: netip.MustParseAddr("10.3.0.7")}

	result := m.ReconcileBearersToRAN(context.Background(), ue, RANBearers{
		Present:       []RANBearer{{Ebi: 5, EnbFTEID: enb}, {Ebi: 6, EnbFTEID: enb}},
		ReleaseFailed: true,
		AfterCommit:   true,
	})

	if !slices.Equal(result.Failed, []uint8{6}) {
		t.Fatalf("result %+v, want the refused voice bearer in the E-RAB To Be Released list", result)
	}

	m.DeactivatePendingDedicated(context.Background(), ue)

	if deact, err := eps.ParseDeactivateEPSBearerContextRequest(downlinkPlain(t, ue, decodeDownlinkNAS(t, lastSent(cc)))); err != nil || deact.EPSBearerIdentity != 6 {
		t.Fatalf("deactivation %+v (%v), want EBI 6 in Downlink NAS Transport", deact, err)
	}
}

func TestSCTPLossReleasesTheVoiceBearer(t *testing.T) {
	m, ue, cc, fake := activeVoiceBearer(t)

	m.ReclaimUEsOnConnLossForTest(cc)

	var dropped []dedicatedOutcome

	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline) && len(dropped) == 0; time.Sleep(5 * time.Millisecond) {
		_, dropped = fake.dedicatedOutcomes()
	}

	if len(dropped) != 1 || dropped[0].teid != voiceSGWTEID {
		t.Fatalf("SMF told %+v released, want the voice bearer (TS 23.401 §5.3.5: S1 signalling connection lost)", dropped)
	}

	if _, b := m.LookupDedicated(ue, 6); b != nil {
		t.Fatal("the voice bearer survived the loss of the S1 connection")
	}
}

func TestModificationRejectedWithInvalidEBIReleasesTheBearerLocally(t *testing.T) {
	m, ue, cc, fake := activeVoiceBearer(t)
	ctx := context.Background()

	if err := m.ModifyDedicatedBearer(ctx, ue.imsiOrEmpty(), 6, qosModification()); err != nil {
		t.Fatalf("ModifyDedicatedBearer: %v", err)
	}

	m.DedicatedBearerModifyRejected(ctx, ue, 6, eps.ESMCauseInvalidEPSBearerIdentity)

	if cmd := lastERABRelease(t, cc); len(cmd.NASPDU) != 0 || !slices.Equal(releasedEBIs(cmd), []uint8{6}) {
		t.Fatalf("release %+v, want the E-RAB released without NAS (TS 24.301 §6.4.3.4, #43)", cmd)
	}

	if _, dropped := fake.dedicatedOutcomes(); len(dropped) != 1 {
		t.Fatalf("SMF told %+v released, want the voice bearer", dropped)
	}
}
