// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/sctp"
)

func connectENB(t *testing.T, m *MME, name string, id uint32) *sctp.SCTPConn {
	t.Helper()

	conn := new(sctp.SCTPConn)
	m.trackRadio(conn, RadioInfo{Name: name})
	m.ClaimENBID(m.RadioForConn(conn), testENBID(id))

	return conn
}

func enbByName(radios []RadioInfo, name string) (RadioInfo, int) {
	var found RadioInfo

	count := 0

	for _, r := range radios {
		if r.Name == name {
			found = r
			count++
		}
	}

	return found, count
}

func TestMMEDisconnectRadioRetainsItAsOffline(t *testing.T) {
	m := newTestMME(t)

	conn := connectENB(t, m, "enb-a", 1)

	listed, count := enbByName(m.ListRadios(), "enb-a")
	if count != 1 || !listed.Connected {
		t.Fatalf("connected eNB listed %d times with connected=%t, want 1 connected", count, listed.Connected)
	}

	m.DisconnectRadio(conn)

	listed, count = enbByName(m.ListRadios(), "enb-a")
	if count != 1 {
		t.Fatalf("disconnected eNB listed %d times, want exactly 1", count)
	}

	if listed.Connected {
		t.Error("a disconnected eNB is still reported as connected")
	}

	if listed.DisconnectedAt.IsZero() {
		t.Error("DisconnectedAt is zero on an offline eNB")
	}

	if m.CountRadios() != 0 {
		t.Errorf("CountRadios() = %d, want 0: an offline eNB is not connected", m.CountRadios())
	}

	if !m.HasRadio("enb-a") {
		t.Error("HasRadio did not report a retained offline eNB as known")
	}
}

func TestMMEReconnectFlipsOfflineToOnline(t *testing.T) {
	m := newTestMME(t)

	conn := connectENB(t, m, "enb-a", 1)
	m.DisconnectRadio(conn)

	connectENB(t, m, "enb-a", 1)

	listed, count := enbByName(m.ListRadios(), "enb-a")
	if count != 1 {
		t.Fatalf("reconnected eNB listed %d times, want exactly 1", count)
	}

	if !listed.Connected {
		t.Error("a reconnected eNB is not reported as connected")
	}

	if got := m.OfflineRadioCountForTest(); got != 0 {
		t.Errorf("%d offline entries retained after the reconnect, want 0", got)
	}
}

func TestMMEDisconnectRadioWithoutENBIDIsNotRetained(t *testing.T) {
	m := newTestMME(t)

	conn := new(sctp.SCTPConn)
	m.trackRadio(conn, RadioInfo{Name: "enb-unclaimed"})

	m.DisconnectRadio(conn)

	if got := len(m.ListRadios()); got != 0 {
		t.Errorf("ListRadios() returned %d eNBs, want 0", got)
	}
}

func TestMMEDisconnectSupersededRadioIsNotRetained(t *testing.T) {
	m := newTestMME(t)

	stale := connectENB(t, m, "enb-a", 1)
	connectENB(t, m, "enb-a", 1)

	m.DisconnectRadio(stale)

	listed, count := enbByName(m.ListRadios(), "enb-a")
	if count != 1 {
		t.Fatalf("eNB listed %d times, want exactly 1", count)
	}

	if !listed.Connected {
		t.Error("the live eNB was marked offline when the stale association closed")
	}
}

// TS 36.413 §8.4.2
func TestMMEFindConnectedRadioByGlobalENBIDSkipsOffline(t *testing.T) {
	m := newTestMME(t)

	conn := connectENB(t, m, "enb-a", 1)

	if _, ok := m.FindConnectedRadioByGlobalENBID(testENBID(1)); !ok {
		t.Fatal("a connected eNB did not resolve by its Global eNB ID")
	}

	m.DisconnectRadio(conn)

	if _, ok := m.FindConnectedRadioByGlobalENBID(testENBID(1)); ok {
		t.Error("an offline eNB resolved as a handover target")
	}

	connectENB(t, m, "enb-a", 1)

	if _, ok := m.FindConnectedRadioByGlobalENBID(testENBID(1)); !ok {
		t.Error("a reconnected eNB did not resolve by its Global eNB ID")
	}
}

func TestMMEOfflineRadioEvictedOnTTL(t *testing.T) {
	m := newTestMME(t)
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)

	m.SetRadioRetentionForTest(time.Hour, DefaultMaxOfflineRadios, func() time.Time { return now })

	conn := connectENB(t, m, "enb-a", 1)
	m.DisconnectRadio(conn)

	now = now.Add(2 * time.Hour)

	if got := len(m.ListRadios()); got != 0 {
		t.Errorf("ListRadios() returned %d eNBs past the TTL, want 0", got)
	}
}

func TestMMEOfflineRadiosEvictedOnLRUCap(t *testing.T) {
	m := newTestMME(t)
	m.SetRadioRetentionForTest(DefaultRadioOfflineTTL, 1, time.Now)

	first := connectENB(t, m, "enb-a", 1)
	m.DisconnectRadio(first)

	second := connectENB(t, m, "enb-b", 2)
	m.DisconnectRadio(second)

	radios := m.ListRadios()
	if _, count := enbByName(radios, "enb-b"); len(radios) != 1 || count != 1 {
		t.Errorf("ListRadios() = %+v, want only the most recent disconnection", radios)
	}
}

func TestMMEForgetRadio(t *testing.T) {
	m := newTestMME(t)

	conn := connectENB(t, m, "enb-a", 1)
	m.DisconnectRadio(conn)

	if err := m.ForgetRadio(ENBID(testENBID(1))); err != nil {
		t.Fatalf("ForgetRadio() = %v, want nil", err)
	}

	if got := len(m.ListRadios()); got != 0 {
		t.Errorf("ListRadios() returned %d eNBs after forgetting, want 0", got)
	}
}

func TestMMEForgetRadioUnknown(t *testing.T) {
	m := newTestMME(t)

	if err := m.ForgetRadio(ENBID(testENBID(99))); err != ErrRadioNotFound {
		t.Errorf("ForgetRadio() = %v, want ErrRadioNotFound", err)
	}
}

func TestMMEForgetRadioOnline(t *testing.T) {
	m := newTestMME(t)

	connectENB(t, m, "enb-a", 1)

	if err := m.ForgetRadio(ENBID(testENBID(1))); err != ErrRadioOnline {
		t.Errorf("ForgetRadio() = %v, want ErrRadioOnline", err)
	}

	if _, count := enbByName(m.ListRadios(), "enb-a"); count != 1 {
		t.Errorf("connected eNB listed %d times after a refused forget, want 1", count)
	}
}

func TestMMEForgottenRadioReappearsOnReconnect(t *testing.T) {
	m := newTestMME(t)

	conn := connectENB(t, m, "enb-a", 1)
	m.DisconnectRadio(conn)

	if err := m.ForgetRadio(ENBID(testENBID(1))); err != nil {
		t.Fatalf("ForgetRadio() = %v, want nil", err)
	}

	connectENB(t, m, "enb-a", 1)

	listed, count := enbByName(m.ListRadios(), "enb-a")
	if count != 1 || !listed.Connected {
		t.Errorf("forgotten eNB listed %d times with connected=%t after reconnecting, want 1 connected", count, listed.Connected)
	}
}
