// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/ngap"
)

func testUserLocation(kind ngap.UserLocationInformationKind, cell uint64) ngap.UserLocationInformation {
	plmn := ngap.PLMNIdentity{0x00, 0xf1, 0x10}

	return ngap.UserLocationInformation{
		Kind:         kind,
		PLMNIdentity: plmn,
		CellIdentity: cell,
		TAI:          ngap.TAI{PLMNIdentity: plmn, TAC: 7},
	}
}

func TestUpdateLocationConversion(t *testing.T) {
	c := &UeConn{}

	c.UpdateLocation(context.Background(), testUserLocation(ngap.UserLocationNR, 0x123456789))

	nr := c.Location.NrLocation
	if nr == nil || nr.Tai == nil || nr.Ncgi == nil {
		t.Fatalf("NrLocation not populated: %+v", nr)
	}

	if nr.Ncgi.NrCellID != "123456789" {
		t.Fatalf("cell id = %q, want 123456789", nr.Ncgi.NrCellID)
	}

	if nr.Tai.Tac != "000007" {
		t.Fatalf("tac = %q, want 000007", nr.Tai.Tac)
	}

	if nr.Ncgi.PlmnID == nil || nr.Ncgi.PlmnID.Mcc != "001" || nr.Ncgi.PlmnID.Mnc != "01" {
		t.Fatalf("ncgi plmn = %+v, want 001/01", nr.Ncgi.PlmnID)
	}

	if nr.UeLocationTimestamp == nil {
		t.Fatal("timestamp not set")
	}
}

func TestUpdateLocationEUTRA(t *testing.T) {
	c := &UeConn{}

	c.UpdateLocation(context.Background(), testUserLocation(ngap.UserLocationEUTRA, 0x0abcde1))

	el := c.Location.EutraLocation
	if el == nil || el.Ecgi == nil {
		t.Fatalf("EutraLocation not populated: %+v", el)
	}

	if el.Ecgi.EutraCellID != "0abcde1" {
		t.Fatalf("cell id = %q, want 0abcde1", el.Ecgi.EutraCellID)
	}

	if c.Location.NrLocation != nil {
		t.Errorf("NrLocation set for an E-UTRA location: %+v", c.Location.NrLocation)
	}
}

func TestUpdateLocationReplacesPreviousAccess(t *testing.T) {
	ctx := context.Background()

	c := &UeConn{}
	c.UpdateLocation(ctx, testUserLocation(ngap.UserLocationNR, 0x123456789))
	c.UpdateLocation(ctx, testUserLocation(ngap.UserLocationEUTRA, 0x0abcde1))

	if c.Location.NrLocation != nil {
		t.Errorf("NR location survived a move to E-UTRA: %+v", c.Location.NrLocation)
	}

	c.UpdateLocation(ctx, testUserLocation(ngap.UserLocationNR, 0x123456789))

	if c.Location.EutraLocation != nil {
		t.Errorf("E-UTRA location survived a move to NR: %+v", c.Location.EutraLocation)
	}
}

func TestUpdateLocationTimeStamp(t *testing.T) {
	c := &UeConn{}

	uli := testUserLocation(ngap.UserLocationNR, 1)
	c.UpdateLocation(context.Background(), uli)

	if got := c.Location.NrLocation.AgeOfLocationInformation; got != 0 {
		t.Errorf("age = %d with no TimeStamp, want 0", got)
	}

	uli.TimeStamp = ntpTimeStamp(time.Now().UTC().Add(-90 * time.Minute))
	c.UpdateLocation(context.Background(), uli)

	if got := c.Location.NrLocation.AgeOfLocationInformation; got != 90 {
		t.Errorf("age = %d for a stamp 90 minutes old, want 90", got)
	}
}

func ntpTimeStamp(t time.Time) *ngap.TimeStamp {
	var ts ngap.TimeStamp

	binary.BigEndian.PutUint32(ts[:], uint32(t.Unix()+ntpEpochOffset))

	return &ts
}

func TestAgeOfLocation(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		at   time.Time
		want int32
	}{
		{"generated now", now, 0},
		{"one minute ago", now.Add(-time.Minute), 1},
		{"partial minute rounds down", now.Add(-119 * time.Second), 1},
		{"a day ago", now.Add(-24 * time.Hour), 1440},
		{"clamped at the upper bound", now.Add(-40000 * time.Minute), maxAgeOfLocationMinutes},
		{"the NTP epoch clamps rather than overflowing", time.Unix(-ntpEpochOffset, 0).UTC(), maxAgeOfLocationMinutes},
		{"a stamp ahead of the clock reads as 0", now.Add(time.Hour), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ageOfLocation(*ntpTimeStamp(tt.at), now); got != tt.want {
				t.Fatalf("ageOfLocation = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestUpdateLocationMirrorsToUeContext(t *testing.T) {
	ue := NewUeContext()
	c := &UeConn{}
	c.ue.Store(ue)

	if !ue.IsUserLocationEmpty() {
		t.Fatal("expected empty location initially")
	}

	c.UpdateLocation(context.Background(), testUserLocation(ngap.UserLocationNR, 0x123456789))

	if ue.IsUserLocationEmpty() {
		t.Fatal("location not mirrored to the persistent UE context")
	}

	loc := ue.GetUserLocation()
	if loc.NrLocation == nil || loc.NrLocation.Ncgi.NrCellID != "123456789" {
		t.Fatalf("mirrored location wrong: %+v", loc.NrLocation)
	}
}

func TestUpdateLocationBareConnectionNotMirrored(t *testing.T) {
	c := &UeConn{}

	c.UpdateLocation(context.Background(), testUserLocation(ngap.UserLocationNR, 1))

	if c.Location.NrLocation == nil {
		t.Fatal("connection location should be set even without a bound UE")
	}
}

func TestUpdateLocationConcurrentReadWrite(t *testing.T) {
	ue := NewUeContext()
	c := &UeConn{}
	c.ue.Store(ue)

	uli := testUserLocation(ngap.UserLocationNR, 0x123456789)

	done := make(chan struct{})

	go func() {
		defer close(done)

		for range 1000 {
			c.UpdateLocation(context.Background(), uli)
		}
	}()

	for range 1000 {
		loc := ue.GetUserLocation()
		if nr := loc.NrLocation; nr != nil && nr.Ncgi != nil {
			_ = nr.Ncgi.NrCellID
			_ = nr.Tai.Tac
		}

		_ = ue.IsUserLocationEmpty()
	}

	<-done
}

type operatorOnlyDB struct {
	DBer
	operator *db.Operator
}

func (d operatorOnlyDB) GetOperator(context.Context) (*db.Operator, error) {
	return d.operator, nil
}

func (d operatorOnlyDB) NodeID() int { return 0 }

func TestUpdateLocationN3IWF(t *testing.T) {
	c := &UeConn{amf: &AMF{DBInstance: operatorOnlyDB{operator: &db.Operator{
		Mcc: "001", Mnc: "01", SupportedTACs: `["000007"]`,
	}}}}

	c.UpdateLocation(context.Background(), ngap.UserLocationInformation{
		Kind:       ngap.UserLocationN3IWF,
		IPAddress:  ngap.TransportLayerAddress{192, 168, 1, 1},
		PortNumber: 500,
	})

	n3 := c.Location.N3gaLocation
	if n3 == nil {
		t.Fatal("N3gaLocation not populated")
	}

	if n3.UeIpv4Addr != "192.168.1.1" || n3.UeIpv6Addr != "" {
		t.Errorf("addresses = %q/%q, want 192.168.1.1 and no IPv6", n3.UeIpv4Addr, n3.UeIpv6Addr)
	}

	if n3.PortNumber != 500 {
		t.Errorf("port = %d, want 500", n3.PortNumber)
	}

	if n3.N3gppTai == nil || n3.N3gppTai.Tac != "000007" {
		t.Fatalf("n3gpp tai = %+v, want the operator's 000007", n3.N3gppTai)
	}

	if c.Tai.Tac != "000007" {
		t.Errorf("serving tac = %q, want 000007", c.Tai.Tac)
	}
}

func TestUpdateLocationN3IWFKeepsLastKnownWithoutOperator(t *testing.T) {
	ctx := context.Background()

	c := &UeConn{}
	c.UpdateLocation(ctx, testUserLocation(ngap.UserLocationNR, 0x123456789))

	c.UpdateLocation(ctx, ngap.UserLocationInformation{
		Kind:      ngap.UserLocationN3IWF,
		IPAddress: ngap.TransportLayerAddress{192, 168, 1, 1},
	})

	if c.Location.NrLocation == nil {
		t.Error("dropped the last known location on an unrecordable update")
	}
}
