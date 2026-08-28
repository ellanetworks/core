// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package radioreg_test

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/radioreg"
)

type radio struct {
	name           string
	id             string
	disconnectedAt time.Time
}

func (r *radio) IDKey() (string, bool)         { return r.id, r.id != "" }
func (r *radio) DisconnectedAt() time.Time     { return r.disconnectedAt }
func (r *radio) SetDisconnectedAt(t time.Time) { r.disconnectedAt = t }

type conn struct{ port int }

func named(name string) func(*radio) bool {
	return func(r *radio) bool { return r.name == name }
}

type registry = radioreg.Registry[*conn, string, *radio]

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

const testTTL = time.Hour

func newRegistry(maxOffline int) (*registry, *clock) {
	c := &clock{t: time.Unix(1, 0)}

	return radioreg.New[*conn, string, *radio](testTTL, maxOffline, c.now), c
}

func connect(reg *registry, name, id string) (*conn, *radio) {
	c := &conn{port: len(reg.ByConn) + 1}
	r := &radio{name: name, id: id}

	reg.Track(c, r)
	reg.Claim(id, r)

	return c, r
}

func TestFindConnectedSkipsOfflineRadio(t *testing.T) {
	reg, _ := newRegistry(10)

	c, r := connect(reg, "radio-a", "id-a")

	if got, ok := reg.FindConnected("id-a"); !ok || got != r {
		t.Fatal("a connected radio did not resolve by its identifier")
	}

	reg.Disconnect(c, r)

	if _, ok := reg.FindConnected("id-a"); ok {
		t.Error("an offline radio resolved as a handover target")
	}
}

func TestDisconnectRetainsRadioAsOffline(t *testing.T) {
	reg, _ := newRegistry(10)

	c, r := connect(reg, "radio-a", "id-a")
	reg.Disconnect(c, r)

	if got := reg.CountConnected(); got != 0 {
		t.Errorf("CountConnected() = %d, want 0", got)
	}

	if got := reg.CountOffline(); got != 1 {
		t.Errorf("CountOffline() = %d, want 1", got)
	}

	if got := len(reg.All()); got != 1 {
		t.Errorf("All() returned %d radios, want the offline one", got)
	}
}

func TestDisconnectDropsRadioWithoutIdentifier(t *testing.T) {
	reg, _ := newRegistry(10)

	c, r := &conn{port: 1}, &radio{name: "radio-a"}
	reg.Track(c, r)

	reg.Disconnect(c, r)

	if got := len(reg.All()); got != 0 {
		t.Errorf("All() returned %d radios, want 0", got)
	}
}

func TestDisconnectDropsSupersededRadio(t *testing.T) {
	reg, _ := newRegistry(10)

	stale, staleRadio := connect(reg, "radio-a", "id-a")
	_, live := connect(reg, "radio-a", "id-a")

	reg.Disconnect(stale, staleRadio)

	if got, ok := reg.FindConnected("id-a"); !ok || got != live {
		t.Error("the live radio stopped resolving when the stale association closed")
	}

	if got := reg.CountOffline(); got != 0 {
		t.Errorf("CountOffline() = %d, want 0", got)
	}
}

func TestOfflineRadioEvictedOnTTL(t *testing.T) {
	reg, clk := newRegistry(10)

	c, r := connect(reg, "radio-a", "id-a")
	reg.Disconnect(c, r)

	clk.advance(testTTL - time.Second)

	if got := reg.CountOffline(); got != 1 {
		t.Fatalf("CountOffline() = %d before the TTL elapsed, want 1", got)
	}

	clk.advance(2 * time.Second)

	if got := reg.CountOffline(); got != 0 {
		t.Errorf("CountOffline() = %d past the TTL, want 0", got)
	}
}

func TestOfflineRadiosEvictedOnCap(t *testing.T) {
	reg, clk := newRegistry(2)

	for _, id := range []string{"id-a", "id-b", "id-c"} {
		c, r := connect(reg, "radio-"+id, id)

		reg.Disconnect(c, r)
		clk.advance(time.Minute)
	}

	connect(reg, "radio-live", "id-live")

	if got := reg.CountOffline(); got != 2 {
		t.Fatalf("CountOffline() = %d, want the cap of 2", got)
	}

	if _, ok := reg.ClaimedBy("id-a"); ok {
		t.Error("the least recently disconnected radio survived the cap")
	}

	if _, ok := reg.FindConnected("id-live"); !ok {
		t.Error("a connected radio was evicted by the offline cap")
	}
}

func TestForgetDropsOfflineRadio(t *testing.T) {
	reg, _ := newRegistry(10)

	c, r := connect(reg, "radio-a", "id-a")
	reg.Disconnect(c, r)

	online, forgotten := reg.Forget(named("radio-a"))
	if online || forgotten != 1 {
		t.Fatalf("Forget() = (online %t, forgotten %d), want (false, 1)", online, forgotten)
	}

	if got := len(reg.All()); got != 0 {
		t.Errorf("All() returned %d radios after Forget, want 0", got)
	}
}

func TestForgetReportsConnectedRadio(t *testing.T) {
	reg, _ := newRegistry(10)

	connect(reg, "radio-a", "id-a")

	online, forgotten := reg.Forget(named("radio-a"))
	if !online || forgotten != 0 {
		t.Errorf("Forget() = (online %t, forgotten %d), want (true, 0)", online, forgotten)
	}
}

func TestForgetUnknownRadio(t *testing.T) {
	reg, _ := newRegistry(10)

	online, forgotten := reg.Forget(named("radio-missing"))
	if online || forgotten != 0 {
		t.Errorf("Forget() = (online %t, forgotten %d), want (false, 0)", online, forgotten)
	}
}

func TestClaimReturnsIncumbent(t *testing.T) {
	reg, _ := newRegistry(10)

	_, first := connect(reg, "radio-a", "id-a")

	second := &radio{name: "radio-a", id: "id-a"}
	if got := reg.Claim("id-a", second); got != first {
		t.Errorf("Claim() returned %v, want the incumbent it superseded", got)
	}

	if got := reg.Claim("id-a", second); got != nil {
		t.Errorf("Claim() by the current holder returned %v, want nil", got)
	}
}

func TestUnclaimReleasesIdentifier(t *testing.T) {
	reg, _ := newRegistry(10)

	_, r := connect(reg, "radio-a", "id-a")

	reg.Unclaim("id-a")

	r.id = "id-b"

	reg.Claim("id-b", r)

	if _, ok := reg.FindConnected("id-a"); ok {
		t.Error("the released identifier still resolves")
	}

	if got, ok := reg.FindConnected("id-b"); !ok || got != r {
		t.Error("the radio did not resolve under its new identifier")
	}
}
