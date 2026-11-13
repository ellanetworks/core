// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

func sessionView(t *testing.T, s *smf.SMF, ref string) smf.SessionView {
	t.Helper()

	v, ok := s.SessionView(context.Background(), ref)
	if !ok {
		t.Fatalf("no view of session %q", ref)
	}

	return v
}

func assertTraffic(t *testing.T, what string, got *smf.Traffic, uplink, downlink uint64) {
	t.Helper()

	if got == nil {
		t.Fatalf("%s traffic absent, want %d/%d", what, uplink, downlink)
	}

	if got.UplinkVolume != uplink || got.DownlinkVolume != downlink {
		t.Fatalf("%s traffic %d/%d, want %d/%d", what, got.UplinkVolume, got.DownlinkVolume, uplink, downlink)
	}
}

func TestSessionViewTrafficSurvivesUsageCollection(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	sc, _ := setupSessionWithTunnel(t, s)
	upf.count(sc.PFCPContext.SEID, 500, 300)

	v := sessionView(t, s, sc.Ref)
	assertTraffic(t, "session", v.Traffic, 500, 300)
	assertTraffic(t, "default bearer", v.Bearers[0].Traffic, 500, 300)

	if v.ReadAt.IsZero() || v.EstablishedAt.IsZero() {
		t.Fatalf("read at %v, established at %v, want both set", v.ReadAt, v.EstablishedAt)
	}

	s.CollectUsageForTest(ctx)

	assertTraffic(t, "session after collection", sessionView(t, s, sc.Ref).Traffic, 500, 300)

	upf.count(sc.PFCPContext.SEID, 100, 100)

	assertTraffic(t, "session after more traffic", sessionView(t, s, sc.Ref).Traffic, 600, 400)

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.usageLog) != 1 || store.usageLog[0].uplinkBytes != 500 {
		t.Fatalf("usage %+v, want the viewed traffic recorded once by the collection only", store.usageLog)
	}
}

func TestSessionViewOmitsTrafficItCannotReadExactly(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	sc, _ := setupSessionWithTunnel(t, s)
	upf.count(sc.PFCPContext.SEID, 500, 300)
	upf.peekErr = errors.New("map lookup failed")

	v := sessionView(t, s, sc.Ref)
	if v.Traffic != nil || !v.ReadAt.IsZero() {
		t.Fatalf("traffic %+v read at %v, want none when the counters cannot be read", v.Traffic, v.ReadAt)
	}

	if len(v.Bearers) == 0 || v.Bearers[0].Traffic != nil {
		t.Fatalf("bearers %+v, want them listed without traffic", v.Bearers)
	}
}

func TestSessionViewKeepsAReleasedBearersTrafficInTheSessionTotal(t *testing.T) {
	s, _, upf, _, ref := epsReconcileFixture(t, 0)
	ctx := context.Background()

	voice := voiceRule()
	voice.RatingGroup = 20

	pushRules(t, s, ref, voice)

	if err := s.DedicatedBearerActivated(ctx, ref, dedicatedUplinkTEID, 6, models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	bearerURR := bearerUplinkPDRs(lastModify(t, upf))[0].URRID
	seid := s.GetSession(ref).PFCPContext.SEID

	upf.countURR(seid, bearerURR, 10, 20)
	upf.count(seid, 1, 1)

	v := sessionView(t, s, ref)
	if len(v.Bearers) != 2 {
		t.Fatalf("bearers %+v, want the default and the voice bearer", v.Bearers)
	}

	assertTraffic(t, "session", v.Traffic, 11, 21)
	assertTraffic(t, "default bearer", v.Bearers[0].Traffic, 1, 1)
	assertTraffic(t, "voice bearer", v.Bearers[1].Traffic, 10, 20)

	s.DedicatedBearerReleased(ctx, ref, dedicatedUplinkTEID)

	v = sessionView(t, s, ref)
	if len(v.Bearers) != 1 {
		t.Fatalf("bearers %+v, want the default bearer only", v.Bearers)
	}

	assertTraffic(t, "session after the release", v.Traffic, 11, 21)
	assertTraffic(t, "default bearer after the release", v.Bearers[0].Traffic, 1, 1)

	secondCall := secondCallRule()
	secondCall.RatingGroup = 20

	pushRules(t, s, ref, secondCall)

	if err := s.DedicatedBearerActivated(ctx, ref, dedicatedUplinkTEID, 6, models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	v = sessionView(t, s, ref)
	if len(v.Bearers) != 2 {
		t.Fatalf("bearers %+v, want the default and a new voice bearer", v.Bearers)
	}

	assertTraffic(t, "new voice bearer", v.Bearers[1].Traffic, 0, 0)
	assertTraffic(t, "session with the new bearer", v.Traffic, 11, 21)
}
