// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

func locationReportULI() UserLocationInformation {
	return UserLocationInformation{
		Kind:         UserLocationNR,
		PLMNIdentity: PLMNIdentity{0x02, 0xf8, 0x39},
		CellIdentity: 1,
		TAI:          TAI{PLMNIdentity: PLMNIdentity{0x02, 0xf8, 0x39}, TAC: 1},
	}
}

const goldenLocationReport = "0012402f000005000a000200010055000200020079400f4002f839000000001002" +
	"f83900000100744003000400002140020000"

func TestLocationReportGolden(t *testing.T) {
	uli := locationReportULI()
	requestType := LocationReportingRequestType{EventType: EventTypeDirect, ReportArea: ReportAreaCell}

	in := LocationReport{
		AMFUENGAPID: 1, RANUENGAPID: 2,
		UserLocationInformation:        &uli,
		UEPresenceInAreaOfInterestList: UEPresenceInAreaOfInterestList{{LocationReportingReferenceID: 3, UEPresence: UEPresenceIn}},
		LocationReportingRequestType:   &requestType,
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenLocationReport {
		t.Fatalf("encoded %s, want %s", got, goldenLocationReport)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcLocationReport {
		t.Fatalf("decoded %T, want an initiating LocationReport", pdu)
	}

	out, err := ParseLocationReport(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.UserLocationInformation == nil || out.UserLocationInformation.TAI.TAC != 1 ||
		out.LocationReportingRequestType == nil || out.LocationReportingRequestType.EventType != EventTypeDirect ||
		len(out.UEPresenceInAreaOfInterestList) != 1 ||
		out.UEPresenceInAreaOfInterestList[0].LocationReportingReferenceID != 3 ||
		out.UEPresenceInAreaOfInterestList[0].UEPresence != UEPresenceIn {
		t.Fatalf("round trip %+v", out)
	}
}

const goldenLocationReportingControl = "00104015000003000a00020001005500020002002140020300"

func TestLocationReportingControlGolden(t *testing.T) {
	in := LocationReportingControl{
		AMFUENGAPID: 1, RANUENGAPID: 2,
		LocationReportingRequestType: &LocationReportingRequestType{
			EventType: EventTypeStopChangeOfServeCell, ReportArea: ReportAreaCell,
		},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenLocationReportingControl {
		t.Fatalf("encoded %s, want %s", got, goldenLocationReportingControl)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseLocationReportingControl(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.LocationReportingRequestType == nil ||
		out.LocationReportingRequestType.EventType != EventTypeStopChangeOfServeCell {
		t.Fatalf("round trip %+v", out)
	}
}

// TS 38.413 §9.3.1.65 gives EventType six root members; TS 36.413 §9.2.1.34
// gives its own three, and the two agree only on the first two. A test that
// pins the count is what stops a value being copied across.
func TestEventTypeRootCountMatchesSpec(t *testing.T) {
	if eventTypeRootCount != 6 {
		t.Errorf("root count is %d, TS 38.413 §9.3.1.65 defines six members before the extension marker", eventTypeRootCount)
	}

	if EventTypeStopChangeOfServeCell != 3 {
		t.Errorf("stop-change-of-serve-cell is %d; it is 3 in NGAP and 2 in S1AP, so the value cannot be shared",
			EventTypeStopChangeOfServeCell)
	}
}

// This AMF never asks for area-of-interest reporting, so a peer naming one back
// is refused rather than misread. The preamble bit must still be consumed, or
// every field after it would be read at the wrong offset.
func TestLocationReportingRequestTypeRefusesAreaOfInterest(t *testing.T) {
	w := per.NewWriter()
	w.WriteBit(false) // no extension
	w.WriteBit(true)  // areaOfInterestList present
	w.WriteBit(false) // no locationReportingReferenceIDToBeCancelled
	w.WriteBit(false) // no iE-Extensions

	if err := encodeRootEnumerated(w, per.Aligned, eventTypeRootCount, int64(EventTypeUEPresenceInAreaOfInterest), "EventType"); err != nil {
		t.Fatal(err)
	}

	if err := encodeRootEnumerated(w, per.Aligned, reportAreaRootCount, int64(ReportAreaCell), "ReportArea"); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	var v LocationReportingRequestType

	err := v.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned)
	if err == nil {
		t.Fatal("an areaOfInterestList decoded")
	}

	if !errors.Is(err, errNotComprehended) {
		t.Errorf("err = %v, want errNotComprehended so criticality decides", err)
	}
}

// The reference-ID-to-be-cancelled is modelled, so it must survive a round trip
// with the area-of-interest bit clear.
func TestLocationReportingRequestTypeReferenceIDRoundTrip(t *testing.T) {
	id := LocationReportingReferenceID(maxnoofAoI)

	in := LocationReportingRequestType{
		EventType:  EventTypeStopUEPresenceInAreaOfInterest,
		ReportArea: ReportAreaCell,
		LocationReportingReferenceIDToBeCancelled: &id,
	}

	w := per.NewWriter()
	if err := in.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	var out LocationReportingRequestType
	if err := out.UnmarshalPER(per.NewReader(w.Bytes()), per.Aligned); err != nil {
		t.Fatal(err)
	}

	if out.EventType != in.EventType || out.LocationReportingReferenceIDToBeCancelled == nil ||
		*out.LocationReportingReferenceIDToBeCancelled != id {
		t.Fatalf("round trip %+v", out)
	}
}

// The ASN.1 makes locationReportingReferenceIDToBeCancelled present exactly when
// Event Type is "stop UE presence in the area of interest"
// (ifEventTypeisStopUEPresinAoI, TS 38.413 §9.3.1.65). Both halves of the
// condition are enforced: a conforming gNB answers a violation with LOCATION
// REPORTING FAILURE INDICATION (§8.12.1.3).
func TestLocationReportingRequestTypeConditionalReferenceID(t *testing.T) {
	id := LocationReportingReferenceID(7)

	tests := []struct {
		name    string
		in      LocationReportingRequestType
		wantErr bool
	}{
		{"direct without a reference id", LocationReportingRequestType{EventType: EventTypeDirect}, false},
		{"direct with a reference id", LocationReportingRequestType{EventType: EventTypeDirect, LocationReportingReferenceIDToBeCancelled: &id}, true},
		{"stop without a reference id", LocationReportingRequestType{EventType: EventTypeStopUEPresenceInAreaOfInterest}, true},
		{"stop with a reference id", LocationReportingRequestType{EventType: EventTypeStopUEPresenceInAreaOfInterest, LocationReportingReferenceIDToBeCancelled: &id}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.MarshalPER(per.NewWriter(), per.Aligned)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
