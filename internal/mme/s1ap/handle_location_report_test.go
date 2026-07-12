// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func TestHandleLocationReport(t *testing.T) {
	m := newTestMME(t)
	conn := &captureConn{}
	ue := m.NewUe(conn, 7)
	m.RegisterUEForTest(ue, "001010000000001")

	plmn := s1ap.PLMNIdentity{0x00, 0xf1, 0x10}

	wire, err := (&s1ap.LocationReport{
		MMEUES1APID: ue.Conn().MMEUES1APID,
		ENBUES1APID: 7,
		EUTRANCGI:   s1ap.EUTRANCGI{PLMNIdentity: plmn, CellID: 0x0abcde1},
		TAI:         s1ap.TAI{PLMNIdentity: plmn, TAC: 9},
		RequestType: s1ap.RequestType{EventType: s1ap.EventTypeChangeOfServeCell, ReportArea: s1ap.ReportAreaECGI},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleLocationReport(m, context.Background(), mme.NewRadioForTest(conn), initiatingValue(t, wire))

	loc := ue.GetUserLocation()
	if loc.EutraLocation == nil || loc.EutraLocation.Ecgi.EutraCellID != "0abcde1" {
		t.Fatalf("serving cell not updated from Location Report: %+v", loc.EutraLocation)
	}
}

func TestHandleLocationReportMalformed(t *testing.T) {
	m := newTestMME(t)

	handleLocationReport(m, context.Background(), mme.NewRadioForTest(&captureConn{}), []byte{0xff, 0xff, 0xff})
}
