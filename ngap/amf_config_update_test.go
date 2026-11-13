// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"strings"
	"testing"
)

func TestAMFConfigurationUpdateRoundTrips(t *testing.T) {
	in := &AMFConfigurationUpdate{
		AMFName:             Ptr("ella"),
		RelativeAMFCapacity: Ptr(uint8(0)),
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcAMFConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseAMFConfigurationUpdate(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if deref(out.AMFName) != "ella" {
		t.Fatalf("AMFName = %q", deref(out.AMFName))
	}

	if out.RelativeAMFCapacity == nil || *out.RelativeAMFCapacity != 0 {
		t.Fatalf("RelativeAMFCapacity = %v, want 0", out.RelativeAMFCapacity)
	}
}

func TestAMFConfigurationUpdateCapacityOnly(t *testing.T) {
	b, err := (&AMFConfigurationUpdate{RelativeAMFCapacity: Ptr(uint8(0))}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseAMFConfigurationUpdate(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.AMFName != nil || len(out.ServedGUAMIList) != 0 {
		t.Fatalf("unexpected IEs: %+v", out)
	}

	if out.RelativeAMFCapacity == nil || *out.RelativeAMFCapacity != 0 {
		t.Fatalf("RelativeAMFCapacity = %v, want 0", out.RelativeAMFCapacity)
	}
}

func TestAMFConfigurationUpdateAcknowledgeRoundTrips(t *testing.T) {
	b, err := (&AMFConfigurationUpdateAcknowledge{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcAMFConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	if _, err := ParseAMFConfigurationUpdateAcknowledge(so.Value); err != nil {
		t.Fatal(err)
	}
}

func TestAMFConfigurationUpdateFailureRoundTrips(t *testing.T) {
	ttw := TimeToWaitV2s

	in := &AMFConfigurationUpdateFailure{
		Cause:      Ptr(Cause{Group: CauseGroupProtocol, Value: CauseProtocolSemanticError}),
		TimeToWait: &ttw,
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	uo, ok := pdu.(*UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != ProcAMFConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseAMFConfigurationUpdateFailure(uo.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.Cause == nil || out.Cause.Group != CauseGroupProtocol {
		t.Fatalf("cause mismatch: %+v", out.Cause)
	}

	if out.TimeToWait == nil || *out.TimeToWait != ttw {
		t.Fatalf("TimeToWait mismatch: %+v", out.TimeToWait)
	}
}

func goldAMFConfigurationUpdatePLMNSupport() *AMFConfigurationUpdate {
	return &AMFConfigurationUpdate{
		AMFName:             Ptr("ella-amf"),
		ServedGUAMIList:     goldResponse().ServedGUAMIList,
		RelativeAMFCapacity: Ptr(uint8(255)),
		PLMNSupportList: PLMNSupportList{
			{
				PLMNIdentity:     goldPLMN(),
				SliceSupportList: SliceSupportList{{SNSSAI: SNSSAI{SST: 1, SD: &SD{0x01, 0x02, 0x03}}}},
			},
			{
				PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10},
				SliceSupportList: SliceSupportList{
					{SNSSAI: SNSSAI{SST: 1}},
					{SNSSAI: SNSSAI{SST: 2, SD: &SD{0xff, 0xff, 0xfe}}},
				},
			},
		},
	}
}

func amfConfigurationUpdateGoldenCases() []goldenRoundTrip {
	return []goldenRoundTrip{
		{
			"AMFConfigurationUpdatePLMNSupportList",
			goldAMFConfigurationUpdatePLMNSupport(),
			func(v []byte) (any, error) { return ParseAMFConfigurationUpdate(v) },
			"0000003e0000040001000a0380656c6c612d616d6600600008000002f83902004300564001ff005000181002f839000010080102030000f110000100088080fffffe",
		},
		{
			"AMFConfigurationUpdatePLMNSupportListOnly",
			&AMFConfigurationUpdate{PLMNSupportList: goldAMFConfigurationUpdatePLMNSupport().PLMNSupportList[:1]},
			func(v []byte) (any, error) { return ParseAMFConfigurationUpdate(v) },
			"000000120000010050000b0002f83900001008010203",
		},
	}
}

func TestAMFConfigurationUpdatePLMNSupportListGolden(t *testing.T) {
	for _, c := range amfConfigurationUpdateGoldenCases() {
		t.Run(c.name, func(t *testing.T) { checkGoldenRoundTrip(t, c) })
	}
}

func TestAMFConfigurationUpdateUnsetPLMNSupportListLeavesEncodingUnchanged(t *testing.T) {
	m := goldAMFConfigurationUpdatePLMNSupport()
	m.PLMNSupportList = nil

	got := mustMarshalHex(t, m)
	if strings.Contains(got, "0050000b") {
		t.Fatalf("PLMN Support List encoded while unset: %s", got)
	}
}
