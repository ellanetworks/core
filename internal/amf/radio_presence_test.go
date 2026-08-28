// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/sctp"
)

func connectRadio(t *testing.T, a *amf.AMF, name, gnbID string) *amf.Radio {
	t.Helper()

	conn := &sctp.SCTPConn{}
	radio := newRadioForTest(a, conn, name)
	a.SetRadioForTest(conn, radio)
	a.ClaimRanID(radio, gnbGlobalRANNodeID(t, gnbID))

	return radio
}

func radioByName(radios []amf.RadioInfo, name string) (amf.RadioInfo, int) {
	var found amf.RadioInfo

	count := 0

	for _, r := range radios {
		if r.Name == name {
			found = r
			count++
		}
	}

	return found, count
}

func TestDisconnectRadioRetainsItAsOffline(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	radio := connectRadio(t, amfInstance, "gNB-A", "ABCDE1")

	listed, count := radioByName(amfInstance.ListRadios(), "gNB-A")
	if count != 1 || !listed.Connected {
		t.Fatalf("connected radio listed %d times with connected=%t, want 1 connected", count, listed.Connected)
	}

	amfInstance.DisconnectRadio(context.Background(), radio)

	listed, count = radioByName(amfInstance.ListRadios(), "gNB-A")
	if count != 1 {
		t.Fatalf("disconnected radio listed %d times, want exactly 1", count)
	}

	if listed.Connected {
		t.Error("a disconnected radio is still reported as connected")
	}

	if listed.DisconnectedAt.IsZero() {
		t.Error("DisconnectedAt is zero on an offline radio")
	}

	if amfInstance.CountRadios() != 0 {
		t.Errorf("CountRadios() = %d, want 0: an offline radio is not connected", amfInstance.CountRadios())
	}

	if !amfInstance.HasRadio("gNB-A") {
		t.Error("HasRadio did not report a retained offline radio as known")
	}
}

func TestReconnectFlipsOfflineToOnline(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	radio := connectRadio(t, amfInstance, "gNB-A", "ABCDE1")
	amfInstance.DisconnectRadio(context.Background(), radio)

	connectRadio(t, amfInstance, "gNB-A", "ABCDE1")

	listed, count := radioByName(amfInstance.ListRadios(), "gNB-A")
	if count != 1 {
		t.Fatalf("reconnected radio listed %d times, want exactly 1", count)
	}

	if !listed.Connected {
		t.Error("a reconnected radio is not reported as connected")
	}

	if !listed.DisconnectedAt.IsZero() {
		t.Errorf("DisconnectedAt = %v on a connected radio, want zero", listed.DisconnectedAt)
	}

	if got := amfInstance.OfflineRadioCountForTest(); got != 0 {
		t.Errorf("%d offline entries retained after the reconnect, want 0", got)
	}
}

func TestDisconnectRadioWithoutRanIDIsNotRetained(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	conn := &sctp.SCTPConn{}
	radio := newRadioForTest(amfInstance, conn, "gNB-unclaimed")
	amfInstance.SetRadioForTest(conn, radio)

	amfInstance.DisconnectRadio(context.Background(), radio)

	if got := len(amfInstance.ListRadios()); got != 0 {
		t.Errorf("ListRadios() returned %d radios, want 0", got)
	}
}

func TestDisconnectSupersededRadioIsNotRetained(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	stale := connectRadio(t, amfInstance, "gNB-A", "ABCDE1")
	connectRadio(t, amfInstance, "gNB-A", "ABCDE1")

	amfInstance.DisconnectRadio(context.Background(), stale)

	listed, count := radioByName(amfInstance.ListRadios(), "gNB-A")
	if count != 1 {
		t.Fatalf("radio listed %d times, want exactly 1", count)
	}

	if !listed.Connected {
		t.Error("the live radio was marked offline when the stale association closed")
	}
}

func TestOfflineRadioEvictedOnTTL(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)

	amfInstance.SetRadioRetentionForTest(time.Hour, amf.DefaultMaxOfflineRadios, func() time.Time { return now })

	radio := connectRadio(t, amfInstance, "gNB-A", "ABCDE1")
	amfInstance.DisconnectRadio(context.Background(), radio)

	now = now.Add(2 * time.Hour)

	if got := len(amfInstance.ListRadios()); got != 0 {
		t.Errorf("ListRadios() returned %d radios past the TTL, want 0", got)
	}
}

func TestOfflineRadiosEvictedOnLRUCap(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	amfInstance.SetRadioRetentionForTest(amf.DefaultRadioOfflineTTL, 1, time.Now)

	first := connectRadio(t, amfInstance, "gNB-A", "ABCDE1")
	amfInstance.DisconnectRadio(context.Background(), first)

	second := connectRadio(t, amfInstance, "gNB-B", "ABCDE2")
	amfInstance.DisconnectRadio(context.Background(), second)

	radios := amfInstance.ListRadios()
	if _, count := radioByName(radios, "gNB-B"); len(radios) != 1 || count != 1 {
		t.Errorf("ListRadios() = %+v, want only the most recent disconnection", radios)
	}
}

func TestForgetRadio(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	radio := connectRadio(t, amfInstance, "gNB-A", "ABCDE1")
	amfInstance.DisconnectRadio(context.Background(), radio)

	if err := amfInstance.ForgetRadio("gNB", radio.NodeID()); err != nil {
		t.Fatalf("ForgetRadio() = %v, want nil", err)
	}

	if got := len(amfInstance.ListRadios()); got != 0 {
		t.Errorf("ListRadios() returned %d radios after forgetting, want 0", got)
	}
}

func TestForgetRadioUnknown(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	if err := amfInstance.ForgetRadio("gNB", "FFFFFF"); err != amf.ErrRadioNotFound {
		t.Errorf("ForgetRadio() = %v, want ErrRadioNotFound", err)
	}
}

func TestForgetRadioOnline(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	radio := connectRadio(t, amfInstance, "gNB-A", "ABCDE1")

	if err := amfInstance.ForgetRadio("gNB", radio.NodeID()); err != amf.ErrRadioOnline {
		t.Errorf("ForgetRadio() = %v, want ErrRadioOnline", err)
	}

	if _, count := radioByName(amfInstance.ListRadios(), "gNB-A"); count != 1 {
		t.Errorf("connected radio listed %d times after a refused forget, want 1", count)
	}
}

func TestForgottenRadioReappearsOnReconnect(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	radio := connectRadio(t, amfInstance, "gNB-A", "ABCDE1")
	amfInstance.DisconnectRadio(context.Background(), radio)

	if err := amfInstance.ForgetRadio("gNB", radio.NodeID()); err != nil {
		t.Fatalf("ForgetRadio() = %v, want nil", err)
	}

	connectRadio(t, amfInstance, "gNB-A", "ABCDE1")

	listed, count := radioByName(amfInstance.ListRadios(), "gNB-A")
	if count != 1 || !listed.Connected {
		t.Errorf("forgotten radio listed %d times with connected=%t after reconnecting, want 1 connected", count, listed.Connected)
	}
}

func TestClaimRanIDOverOfflineRadioEvictsNothing(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	radio := connectRadio(t, amfInstance, "gNB-A", "ABCDE1")
	amfInstance.DisconnectRadio(context.Background(), radio)

	conn := &sctp.SCTPConn{}
	reconnected := newRadioForTest(amfInstance, conn, "gNB-A")
	amfInstance.SetRadioForTest(conn, reconnected)

	if evicted := amfInstance.ClaimRanID(reconnected, gnbGlobalRANNodeID(t, "ABCDE1")); evicted != nil {
		t.Errorf("ClaimRanID evicted %q, want nothing: the incumbent was offline", amfInstance.RadioNameForTest(evicted))
	}
}

func TestRebindRanIDOverOfflineRadioSucceeds(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	stale := connectRadio(t, amfInstance, "gNB-A", "ABCDE1")
	amfInstance.DisconnectRadio(context.Background(), stale)

	live := connectRadio(t, amfInstance, "gNB-B", "ABCDE2")

	if !amfInstance.RebindRanID(live, gnbGlobalRANNodeID(t, "ABCDE1")) {
		t.Fatal("RebindRanID refused an identity held only by an offline radio")
	}

	radios := amfInstance.ListRadios()
	if len(radios) != 1 || !radios[0].Connected || radios[0].Name != "gNB-B" {
		t.Errorf("ListRadios() = %+v, want only the rebound radio, connected", radios)
	}
}
