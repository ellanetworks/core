// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
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
		Cause:      Ptr(CauseProtocolSemanticError.Cause()),
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

	if out.Cause == nil || out.Cause.Protocol == nil {
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

func goldAMFConfigurationUpdateRelay() *AMFConfigurationUpdate {
	m := goldAMFConfigurationUpdatePLMNSupport()
	m.PLMNSupportList[0].NPNSupport = &NPNSupport{SNPN: Ptr(NID(0x123456789ab))}
	m.PLMNSupportList[0].ExtendedSliceSupportList = ExtendedSliceSupportList{{SNSSAI: SNSSAI{SST: 3, SD: &SD{0x00, 0x00, 0x02}}}}
	m.PLMNSupportList[1].OnboardingSupport = Ptr(OnboardingSupportTrue)
	m.AMFTNLAssociationToAddList = AMFTNLAssociationToAddList{
		{
			AMFTNLAssociationAddress: CPTransportLayerInformation{EndpointIPAddress: Ptr(tlaOf(10, 0, 0, 2))},
			TNLAssociationUsage:      Ptr(TNLAssociationUsageBoth),
			TNLAddressWeightFactor:   255,
		},
		{
			AMFTNLAssociationAddress: CPTransportLayerInformation{EndpointIPAddressAndPort: &EndpointIPAddressAndPort{
				EndpointIPAddress: tlaOf(10, 0, 0, 3), PortNumber: 38412,
			}},
			TNLAddressWeightFactor: 1,
		},
	}
	m.AMFTNLAssociationToRemoveList = AMFTNLAssociationToRemoveList{{
		AMFTNLAssociationAddress:                 CPTransportLayerInformation{EndpointIPAddress: Ptr(tlaOf(10, 0, 0, 4))},
		TNLAssociationTransportLayerAddressNGRAN: &CPTransportLayerInformation{EndpointIPAddress: Ptr(tlaOf(10, 0, 1, 4))},
	}}
	m.AMFTNLAssociationToUpdateList = AMFTNLAssociationToUpdateList{{
		AMFTNLAssociationAddress: CPTransportLayerInformation{EndpointIPAddress: Ptr(tlaOf(10, 0, 0, 2))},
		TNLAssociationUsage:      Ptr(TNLAssociationUsageNonUE),
		TNLAddressWeightFactor:   Ptr(TNLAddressWeightFactor(7)),
	}}
	m.ExtendedAMFName = &ExtendedAMFName{
		AMFNameVisibleString: Ptr(AMFNameVisibleString("ella-amf-extended")),
		AMFNameUTF8String:    Ptr(AMFNameUTF8String("ella-amf-ü")),
	}

	return m
}

func amfConfigurationUpdateWireCases(t *testing.T) []wireCase {
	t.Helper()

	return []wireCase{{
		name:  "AMFConfigurationUpdateTNLListsExtendedAMFNamePLMNExtensions",
		pdu:   marshalT(t, goldAMFConfigurationUpdateRelay()),
		parse: func(v []byte) (any, error) { return ParseAMFConfigurationUpdate(v) },
		check: func(t *testing.T, a any) {
			t.Helper()

			if !equalByMethod(a, goldAMFConfigurationUpdateRelay()) {
				t.Fatalf("decoded\n got  %+v\n want %+v", a, goldAMFConfigurationUpdateRelay())
			}
		},
	}}
}

func TestPLMNSupportItemUnknownExtensionKeepsItsPosition(t *testing.T) {
	item := goldAMFConfigurationUpdateRelay().PLMNSupportList[0]
	restore := swapTable(&extIEsPLMNSupportItem,
		tableRow(extIEsPLMNSupportItem, IDNPNSupport, CriticalityReject),
		rawRow[PLMNSupportItem](9999, CriticalityIgnore, []byte{0x5a}),
		tableRow(extIEsPLMNSupportItem, IDExtendedSliceSupportList, CriticalityReject),
	)
	m := &AMFConfigurationUpdate{PLMNSupportList: PLMNSupportList{item}}
	pdu := marshalT(t, m)

	restore()

	p, err := Unmarshal(pdu)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseAMFConfigurationUpdate(p.value())
	if err != nil {
		t.Fatal(err)
	}

	got := out.PLMNSupportList[0]
	if got.NPNSupport == nil || len(got.ExtendedSliceSupportList) != 1 || len(got.iEExtensions.UnknownIEs()) != 1 {
		t.Fatalf("decoded %+v", got)
	}

	if re := marshalT(t, out); !bytes.Equal(re, pdu) {
		t.Fatalf("re-encoded\n got  %x\n want %x", re, pdu)
	}
}

func TestPLMNSupportItemUnknownRejectExtensionRejectsProcedure(t *testing.T) {
	item := goldAMFConfigurationUpdateRelay().PLMNSupportList[1]
	item.iEExtensions.Append(rawIE{ID: 9999, Criticality: CriticalityReject, Value: []byte{0x5a}})

	p, err := Unmarshal(marshalT(t, &AMFConfigurationUpdate{PLMNSupportList: PLMNSupportList{item}}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = ParseAMFConfigurationUpdate(p.value())

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) || len(ase.IEs) != 1 || ase.IEs[0].IEID != 9999 {
		t.Fatalf("err = %v, want the unknown reject extension reported", err)
	}
}

func TestAMFTNLAssociationToRemoveItemNGRANAddressIsComprehended(t *testing.T) {
	m := &AMFConfigurationUpdate{AMFTNLAssociationToRemoveList: goldAMFConfigurationUpdateRelay().AMFTNLAssociationToRemoveList}
	pdu := marshalT(t, m)

	p, err := Unmarshal(pdu)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseAMFConfigurationUpdate(p.value())
	if err != nil {
		t.Fatal(err)
	}

	if !equalByMethod(out, m) {
		t.Fatalf("decoded %+v, want %+v", out, m)
	}
}

func goldAMFConfigurationUpdateAcknowledgeTNL() *AMFConfigurationUpdateAcknowledge {
	return &AMFConfigurationUpdateAcknowledge{
		AMFTNLAssociationSetupList: AMFTNLAssociationSetupList{
			{AMFTNLAssociationAddress: CPTransportLayerInformation{EndpointIPAddress: Ptr(tlaOf(10, 0, 0, 2))}},
			{AMFTNLAssociationAddress: CPTransportLayerInformation{EndpointIPAddressAndPort: &EndpointIPAddressAndPort{
				EndpointIPAddress: tlaOf(10, 0, 0, 3), PortNumber: 38412,
			}}},
		},
		AMFTNLAssociationFailedToSetupList: TNLAssociationList{{
			TNLAssociationAddress: CPTransportLayerInformation{EndpointIPAddress: Ptr(tlaOf(10, 0, 0, 5))},
			Cause:                 CauseTransportResourceUnavailable.Cause(),
		}},
		CriticalityDiagnostics: &CriticalityDiagnostics{
			ProcedureCode:     Ptr(ProcAMFConfigurationUpdate),
			TriggeringMessage: Ptr(TriggeringInitiatingMessage),
		},
	}
}

func goldAMFConfigurationUpdateServedGUAMIExtensions() *AMFConfigurationUpdate {
	m := &AMFConfigurationUpdate{ServedGUAMIList: goldResponse().ServedGUAMIList}
	m.ServedGUAMIList[0].BackupAMFName = Ptr(Name("ella-amf-b"))
	m.ServedGUAMIList[0].GUAMIType = Ptr(GUAMITypeMapped)
	m.ServedGUAMIList[0].ExtendedBackupAMFName = &ExtendedAMFName{
		AMFNameVisibleString: Ptr(AMFNameVisibleString("ella-amf-backup-extended")),
	}

	return m
}

func amfConfigurationUpdateAcknowledgeGoldenCases() []goldenRoundTrip {
	return []goldenRoundTrip{
		{
			"AMFConfigurationUpdateAcknowledgeTNLLists",
			goldAMFConfigurationUpdateAcknowledgeTNL(),
			func(v []byte) (any, error) { return ParseAMFConfigurationUpdateAcknowledge(v) },
			"2000002e00000300054014080f800a0000022000a9000807c00a000003960c00044008000f800a0000052000134003600000",
		},
		{
			"AMFConfigurationUpdateServedGUAMIExtensions",
			goldAMFConfigurationUpdateServedGUAMIExtensions(),
			func(v []byte) (any, error) { return ParseAMFConfigurationUpdate(v) },
			"0000004000000100600039006002f8390200430480656c6c612d616d662d62000100b040014001ba401a40b8656c6c612d616d662d6261636b75702d657874656e646564",
		},
	}
}

func TestAMFConfigurationUpdateAcknowledgeTNLListsGolden(t *testing.T) {
	for _, c := range amfConfigurationUpdateAcknowledgeGoldenCases() {
		t.Run(c.name, func(t *testing.T) { checkGoldenRoundTrip(t, c) })
	}
}

func tnlAssociationItemWithRawExtension(t *testing.T) []byte {
	t.Helper()

	w := per.NewWriter()
	mustNil(t, per.EncodeLength(w, per.Aligned, 1, maxnoofTNLAssociations, true, 1, func(int64) error {
		w.WriteBit(false)
		w.WriteBit(true)

		addr := CPTransportLayerInformation{EndpointIPAddress: Ptr(tlaOf(10, 0, 0, 5))}
		if err := addr.MarshalPER(w, per.Aligned); err != nil {
			return err
		}

		cause := CauseTransportUnspecified.Cause()
		if err := cause.MarshalPER(w, per.Aligned); err != nil {
			return err
		}

		return encodeExtensionContainer(w, per.Aligned, []ieField{{ID: 9999, Criticality: CriticalityIgnore, Value: apimpl.RawValue([]byte{0x5a})}})
	}))
	w.AlignToByte()

	return w.Bytes()
}

func TestTNLAssociationItemRawExtensionRoundTrips(t *testing.T) {
	value := container(t,
		ieField{ID: IDAMFTNLAssociationFailedToSetupList, Criticality: CriticalityIgnore, Value: apimpl.RawValue(tnlAssociationItemWithRawExtension(t))},
	)

	out, err := ParseAMFConfigurationUpdateAcknowledge(value)
	if err != nil {
		t.Fatal(err)
	}

	if len(out.AMFTNLAssociationFailedToSetupList) != 1 || out.AMFTNLAssociationFailedToSetupList[0].iEExtensions == (extensionMeta{}) {
		t.Fatalf("decoded %+v", out)
	}

	if got := out.AMFTNLAssociationFailedToSetupList[0].Cause; !got.Equal(CauseTransportUnspecified.Cause()) {
		t.Fatalf("cause = %+v", got)
	}

	w := per.NewWriter()
	mustNil(t, out.encodeBody(w, per.Aligned))
	w.AlignToByte()

	if !bytes.Equal(w.Bytes(), value) {
		t.Fatalf("re-encoded\n got  %x\n want %x", w.Bytes(), value)
	}
}

func TestServedGUAMIItemUnknownExtensionKeepsItsPosition(t *testing.T) {
	item := goldAMFConfigurationUpdateServedGUAMIExtensions().ServedGUAMIList[0]
	restore := swapTable(&extIEsServedGUAMIItem,
		tableRow(extIEsServedGUAMIItem, IDGUAMIType, CriticalityIgnore),
		rawRow[ServedGUAMIItem](9999, CriticalityIgnore, []byte{0x5a}),
		tableRow(extIEsServedGUAMIItem, IDExtendedBackupAMFName, CriticalityIgnore),
	)
	pdu := marshalT(t, &AMFConfigurationUpdate{ServedGUAMIList: ServedGUAMIList{item}})

	restore()

	p, err := Unmarshal(pdu)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseAMFConfigurationUpdate(p.value())
	if err != nil {
		t.Fatal(err)
	}

	got := out.ServedGUAMIList[0]
	if got.GUAMIType == nil || got.ExtendedBackupAMFName == nil || len(got.iEExtensions.UnknownIEs()) != 1 {
		t.Fatalf("decoded %+v", got)
	}

	if re := marshalT(t, out); !bytes.Equal(re, pdu) {
		t.Fatalf("re-encoded\n got  %x\n want %x", re, pdu)
	}
}

func TestServedGUAMIItemUnknownRejectExtensionRejectsProcedure(t *testing.T) {
	item := goldResponse().ServedGUAMIList[0]
	item.iEExtensions.Append(rawIE{ID: 9999, Criticality: CriticalityReject, Value: []byte{0x5a}})

	p, err := Unmarshal(marshalT(t, &AMFConfigurationUpdate{ServedGUAMIList: ServedGUAMIList{item}}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = ParseAMFConfigurationUpdate(p.value())

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) || len(ase.IEs) != 1 || ase.IEs[0].IEID != 9999 {
		t.Fatalf("err = %v, want the unknown reject extension reported", err)
	}
}
