// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func validGlobalRANNodeID() *ngapType.GlobalRANNodeID {
	return &ngapType.GlobalRANNodeID{
		Present: ngapType.GlobalRANNodeIDPresentGlobalGNBID,
		GlobalGNBID: &ngapType.GlobalGNBID{
			PLMNIdentity: ngapType.PLMNIdentity{Value: aper.OctetString{0x00, 0xF1, 0x10}},
			GNBID: ngapType.GNBID{
				Present: ngapType.GNBIDPresentGNBID,
				GNBID:   &aper.BitString{Bytes: []byte{0xAB, 0xCD, 0xE1}, BitLength: 24},
			},
		},
	}
}

func validSupportedTAList() *ngapType.SupportedTAList {
	return &ngapType.SupportedTAList{
		List: []ngapType.SupportedTAItem{
			{
				TAC: ngapType.TAC{Value: aper.OctetString{0x00, 0x00, 0x64}},
				BroadcastPLMNList: ngapType.BroadcastPLMNList{
					List: []ngapType.BroadcastPLMNItem{
						{
							PLMNIdentity: ngapType.PLMNIdentity{Value: aper.OctetString{0x00, 0xF1, 0x10}},
							TAISliceSupportList: ngapType.SliceSupportList{
								List: []ngapType.SliceSupportItem{
									{
										SNSSAI: ngapType.SNSSAI{
											SST: ngapType.SST{Value: aper.OctetString{0x01}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func validNGSetupRequest() *ngapType.NGSetupRequest {
	msg := &ngapType.NGSetupRequest{}

	msg.ProtocolIEs.List = []ngapType.NGSetupRequestIEs{
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDGlobalRANNodeID},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.NGSetupRequestIEsValue{
				Present:         ngapType.NGSetupRequestIEsPresentGlobalRANNodeID,
				GlobalRANNodeID: validGlobalRANNodeID(),
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDRANNodeName},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.NGSetupRequestIEsValue{
				Present:     ngapType.NGSetupRequestIEsPresentRANNodeName,
				RANNodeName: &ngapType.RANNodeName{Value: "TestRAN"},
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDSupportedTAList},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
			Value: ngapType.NGSetupRequestIEsValue{
				Present:         ngapType.NGSetupRequestIEsPresentSupportedTAList,
				SupportedTAList: validSupportedTAList(),
			},
		},
		{
			Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDDefaultPagingDRX},
			Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentIgnore},
			Value: ngapType.NGSetupRequestIEsValue{
				Present:          ngapType.NGSetupRequestIEsPresentDefaultPagingDRX,
				DefaultPagingDRX: &ngapType.PagingDRX{Value: ngapType.PagingDRXPresentV128},
			},
		},
	}

	return msg
}

func TestDecodeNGSetupRequest_Happy(t *testing.T) {
	msg := validNGSetupRequest()

	out, report := decode.DecodeNGSetupRequest(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.GlobalRANNodeID.Kind() != decode.GlobalRANNodeKindGNB {
		t.Errorf("kind = %d, want GNB", out.GlobalRANNodeID.Kind())
	}

	if out.GlobalRANNodeID.Raw() == nil {
		t.Error("Raw() should be non-nil after happy-path decode")
	}

	if out.RANNodeName != "TestRAN" {
		t.Errorf("RANNodeName = %q, want TestRAN", out.RANNodeName)
	}

	if len(out.SupportedTAItems) != 1 {
		t.Fatalf("SupportedTAItems len = %d, want 1", len(out.SupportedTAItems))
	}
}

func TestDecodeNGSetupRequest_HappyNgENB(t *testing.T) {
	msg := validNGSetupRequest()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDGlobalRANNodeID {
			msg.ProtocolIEs.List[i].Value.GlobalRANNodeID = &ngapType.GlobalRANNodeID{
				Present: ngapType.GlobalRANNodeIDPresentGlobalNgENBID,
				GlobalNgENBID: &ngapType.GlobalNgENBID{
					PLMNIdentity: ngapType.PLMNIdentity{Value: aper.OctetString{0x00, 0xF1, 0x10}},
					NgENBID: ngapType.NgENBID{
						Present:      ngapType.NgENBIDPresentMacroNgENBID,
						MacroNgENBID: &aper.BitString{Bytes: []byte{0xAB, 0xCD, 0xE0}, BitLength: 20},
					},
				},
			}
		}
	}

	out, report := decode.DecodeNGSetupRequest(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.GlobalRANNodeID.Kind() != decode.GlobalRANNodeKindNgENB {
		t.Errorf("kind = %d, want NgENB", out.GlobalRANNodeID.Kind())
	}

	raw := out.GlobalRANNodeID.Raw()
	if raw == nil || raw.GlobalNgENBID == nil || raw.GlobalNgENBID.NgENBID.MacroNgENBID == nil {
		t.Fatal("decoded NgENB structure incomplete")
	}
}

func TestDecodeNGSetupRequest_HappyN3IWF(t *testing.T) {
	msg := validNGSetupRequest()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDGlobalRANNodeID {
			msg.ProtocolIEs.List[i].Value.GlobalRANNodeID = &ngapType.GlobalRANNodeID{
				Present: ngapType.GlobalRANNodeIDPresentGlobalN3IWFID,
				GlobalN3IWFID: &ngapType.GlobalN3IWFID{
					PLMNIdentity: ngapType.PLMNIdentity{Value: aper.OctetString{0x00, 0xF1, 0x10}},
					N3IWFID: ngapType.N3IWFID{
						Present: ngapType.N3IWFIDPresentN3IWFID,
						N3IWFID: &aper.BitString{Bytes: []byte{0x12, 0x34}, BitLength: 16},
					},
				},
			}
		}
	}

	out, report := decode.DecodeNGSetupRequest(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if out.GlobalRANNodeID.Kind() != decode.GlobalRANNodeKindN3IWF {
		t.Errorf("kind = %d, want N3IWF", out.GlobalRANNodeID.Kind())
	}

	raw := out.GlobalRANNodeID.Raw()
	if raw == nil || raw.GlobalN3IWFID == nil || raw.GlobalN3IWFID.N3IWFID.N3IWFID == nil {
		t.Fatal("decoded N3IWF structure incomplete")
	}
}

func TestDecodeNGSetupRequest_NilBody(t *testing.T) {
	out, report := decode.DecodeNGSetupRequest(nil)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if !report.Fatal() {
		t.Error("expected fatal report for nil body")
	}

	if !report.ProcedureRejected {
		t.Error("expected ProcedureRejected to be set")
	}

	if len(report.Items) != 0 {
		t.Errorf("expected no per-IE items for nil body, got %+v", report.Items)
	}

	if out.RANNodeName != "" {
		t.Errorf("expected zero value, got %q", out.RANNodeName)
	}
}

func TestDecodeNGSetupRequest_EmptyIEs(t *testing.T) {
	_, report := decode.DecodeNGSetupRequest(&ngapType.NGSetupRequest{})
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}

	wantIEs := map[int64]bool{
		ngapType.ProtocolIEIDGlobalRANNodeID:  true,
		ngapType.ProtocolIEIDSupportedTAList:  true,
		ngapType.ProtocolIEIDDefaultPagingDRX: true,
	}

	gotIEs := make(map[int64]bool)
	for _, item := range report.Items {
		gotIEs[item.IEID] = true
	}

	for id := range wantIEs {
		if !gotIEs[id] {
			t.Errorf("missing report item for IE %d", id)
		}
	}
}

func TestDecodeNGSetupRequest_RANNodeNameOptional(t *testing.T) {
	msg := validNGSetupRequest()

	filtered := msg.ProtocolIEs.List[:0]
	for _, ie := range msg.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDRANNodeName {
			continue
		}

		filtered = append(filtered, ie)
	}

	msg.ProtocolIEs.List = filtered

	out, report := decode.DecodeNGSetupRequest(msg)
	if report != nil {
		t.Fatalf("RANNodeName is optional, expected nil report; got %+v", report)
	}

	if out.RANNodeName != "" {
		t.Errorf("expected empty RANNodeName, got %q", out.RANNodeName)
	}
}

func TestDecodeNGSetupRequest_MalformedGlobalRANNodeID(t *testing.T) {
	cases := []struct {
		name string
		id   *ngapType.GlobalRANNodeID
	}{
		{
			name: "nil pointer",
			id:   nil,
		},
		{
			name: "GNB present but variant nil",
			id: &ngapType.GlobalRANNodeID{
				Present: ngapType.GlobalRANNodeIDPresentGlobalGNBID,
			},
		},
		{
			name: "GNB inner CHOICE wrong discriminator",
			id: &ngapType.GlobalRANNodeID{
				Present: ngapType.GlobalRANNodeIDPresentGlobalGNBID,
				GlobalGNBID: &ngapType.GlobalGNBID{
					GNBID: ngapType.GNBID{Present: ngapType.GNBIDPresentNothing},
				},
			},
		},
		{
			name: "GNB inner BitString nil",
			id: &ngapType.GlobalRANNodeID{
				Present: ngapType.GlobalRANNodeIDPresentGlobalGNBID,
				GlobalGNBID: &ngapType.GlobalGNBID{
					GNBID: ngapType.GNBID{Present: ngapType.GNBIDPresentGNBID},
				},
			},
		},
		{
			name: "NgENB present but variant nil",
			id: &ngapType.GlobalRANNodeID{
				Present: ngapType.GlobalRANNodeIDPresentGlobalNgENBID,
			},
		},
		{
			name: "NgENB Macro discriminator with nil pointer",
			id: &ngapType.GlobalRANNodeID{
				Present: ngapType.GlobalRANNodeIDPresentGlobalNgENBID,
				GlobalNgENBID: &ngapType.GlobalNgENBID{
					NgENBID: ngapType.NgENBID{Present: ngapType.NgENBIDPresentMacroNgENBID},
				},
			},
		},
		{
			name: "N3IWF present but variant nil",
			id: &ngapType.GlobalRANNodeID{
				Present: ngapType.GlobalRANNodeIDPresentGlobalN3IWFID,
			},
		},
		{
			name: "N3IWF inner pointer nil",
			id: &ngapType.GlobalRANNodeID{
				Present: ngapType.GlobalRANNodeIDPresentGlobalN3IWFID,
				GlobalN3IWFID: &ngapType.GlobalN3IWFID{
					N3IWFID: ngapType.N3IWFID{Present: ngapType.N3IWFIDPresentN3IWFID},
				},
			},
		},
		{
			name: "unknown discriminator",
			id:   &ngapType.GlobalRANNodeID{Present: 99},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := validNGSetupRequest()
			for i := range msg.ProtocolIEs.List {
				if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDGlobalRANNodeID {
					msg.ProtocolIEs.List[i].Value.GlobalRANNodeID = tc.id
				}
			}

			_, report := decode.DecodeNGSetupRequest(msg)
			if report == nil || !report.Fatal() {
				t.Fatalf("expected fatal report; got %+v", report)
			}
		})
	}
}

func TestDecodeNGSetupRequest_NilSupportedTAListValue(t *testing.T) {
	msg := validNGSetupRequest()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDSupportedTAList {
			msg.ProtocolIEs.List[i].Value.SupportedTAList = nil
		}
	}

	_, report := decode.DecodeNGSetupRequest(msg)
	if report == nil || !report.Fatal() {
		t.Fatalf("expected fatal report; got %+v", report)
	}
}

func TestDecodeNGSetupRequest_NilDefaultPagingDRXValue(t *testing.T) {
	msg := validNGSetupRequest()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDDefaultPagingDRX {
			msg.ProtocolIEs.List[i].Value.DefaultPagingDRX = nil
		}
	}

	_, report := decode.DecodeNGSetupRequest(msg)
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	// DefaultPagingDRX is mandatory-ignore, so a malformed value is
	// non-fatal: the handler must still be invoked.
	if report.Fatal() {
		t.Errorf("expected non-fatal report for malformed DefaultPagingDRX")
	}
}

func TestDecodeNGSetupRequest_DuplicateIELastWins(t *testing.T) {
	msg := validNGSetupRequest()

	second := validGlobalRANNodeID()
	second.GlobalGNBID.GNBID.GNBID = &aper.BitString{
		Bytes:     []byte{0x12, 0x34, 0x56},
		BitLength: 24,
	}

	msg.ProtocolIEs.List = append(msg.ProtocolIEs.List, ngapType.NGSetupRequestIEs{
		Id:          ngapType.ProtocolIEID{Value: ngapType.ProtocolIEIDGlobalRANNodeID},
		Criticality: ngapType.Criticality{Value: ngapType.CriticalityPresentReject},
		Value: ngapType.NGSetupRequestIEsValue{
			Present:         ngapType.NGSetupRequestIEsPresentGlobalRANNodeID,
			GlobalRANNodeID: second,
		},
	})

	out, report := decode.DecodeNGSetupRequest(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	got := out.GlobalRANNodeID.Raw()
	if got == nil || got.GlobalGNBID == nil || got.GlobalGNBID.GNBID.GNBID == nil {
		t.Fatal("decoded GlobalRANNodeID structure incomplete")
	}

	if got.GlobalGNBID.GNBID.GNBID.Bytes[0] != 0x12 {
		t.Errorf("expected last-wins to keep second value, got %x", got.GlobalGNBID.GNBID.GNBID.Bytes)
	}
}

func TestDecodeNGSetupRequest_EmptySupportedTAList(t *testing.T) {
	// Empty list (size 0) violates TS 38.413 sizeLB:1 but the
	// existing handler treats it as a business condition (NGSetupFailure
	// with CauseMiscPresentUnspecified). Decode succeeds and the
	// handler decides.
	msg := validNGSetupRequest()
	for i := range msg.ProtocolIEs.List {
		if msg.ProtocolIEs.List[i].Id.Value == ngapType.ProtocolIEIDSupportedTAList {
			msg.ProtocolIEs.List[i].Value.SupportedTAList = &ngapType.SupportedTAList{}
		}
	}

	out, report := decode.DecodeNGSetupRequest(msg)
	if report != nil {
		t.Fatalf("expected nil report, got %+v", report)
	}

	if len(out.SupportedTAItems) != 0 {
		t.Errorf("expected empty SupportedTAItems, got %d", len(out.SupportedTAItems))
	}
}
