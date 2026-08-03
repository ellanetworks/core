// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
	ngaplib "github.com/ellanetworks/core/ngap"
	libngap "github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
)

// The response is encoded by the in-house library and read back with the
// reference decoder, so the test also shows a peer can parse what we emit.
func TestBuildNGSetupResponse_MultipleSlices(t *testing.T) {
	guami := &models.Guami{
		PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"},
		AmfID:  "cafe00",
	}

	snssaiList := []models.Snssai{
		{Sst: 1, Sd: "010203"},
		{Sst: 2, Sd: "aabbcc"},
		{Sst: 3, Sd: ""},
	}

	resp, err := buildNGSetupResponse(guami, snssaiList, "TestAMF", 255)
	if err != nil {
		t.Fatalf("buildNGSetupResponse failed: %v", err)
	}

	encoded, err := resp.Marshal()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	pdu, err := libngap.Decoder(encoded)
	if err != nil {
		t.Fatalf("NGAP decode failed: %v", err)
	}

	if pdu.Present != ngapType.NGAPPDUPresentSuccessfulOutcome {
		t.Fatalf("expected SuccessfulOutcome, got %d", pdu.Present)
	}

	out := pdu.SuccessfulOutcome.Value.NGSetupResponse
	if out == nil {
		t.Fatal("expected NGSetupResponse, got nil")
	}

	var plmnSupportList *ngapType.PLMNSupportList

	for _, ie := range out.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDPLMNSupportList {
			plmnSupportList = ie.Value.PLMNSupportList

			break
		}
	}

	if plmnSupportList == nil {
		t.Fatal("PLMNSupportList IE not found")
	}

	if len(plmnSupportList.List) != 1 {
		t.Fatalf("expected 1 PLMN support item, got %d", len(plmnSupportList.List))
	}

	sliceList := plmnSupportList.List[0].SliceSupportList.List
	if len(sliceList) != 3 {
		t.Fatalf("expected 3 slice support items, got %d", len(sliceList))
	}

	// The third slice has no SD, which must stay absent rather than encode as
	// three zero octets — a valid, different slice differentiator.
	if sliceList[2].SNSSAI.SD != nil {
		t.Errorf("slice 3 SD = % x, want absent", sliceList[2].SNSSAI.SD.Value)
	}
}

// operatorFor builds the operator view NG Setup validates against.
func operatorFor(mcc, mnc, tac string) *amf.OperatorInfo {
	plmn := models.PlmnID{Mcc: mcc, Mnc: mnc}

	return &amf.OperatorInfo{
		Tais:  []models.Tai{{PlmnID: &plmn, Tac: tac}},
		Guami: &models.Guami{PlmnID: &plmn, AmfID: "cafe00"},
	}
}

func outcomeRequest(t *testing.T, mcc, mnc string, tac uint32) *ngaplib.NGSetupRequest {
	t.Helper()

	plmn, err := util.PLMNToNGAP(models.PlmnID{Mcc: mcc, Mnc: mnc})
	if err != nil {
		t.Fatal(err)
	}

	return &ngaplib.NGSetupRequest{
		GlobalRANNodeID: ngaplib.GlobalRANNodeID{
			Kind: ngaplib.RANNodeIDGNB, PLMNIdentity: plmn, Value: 0x000102, Bits: 24,
		},
		RANNodeName: ngaplib.Ptr("ella-gnb"),
		SupportedTAList: ngaplib.SupportedTAList{{
			TAC: ngaplib.TAC(tac),
			BroadcastPLMNList: ngaplib.BroadcastPLMNList{{
				PLMNIdentity:        plmn,
				TAISliceSupportList: ngaplib.SliceSupportList{{SNSSAI: ngaplib.SNSSAI{SST: 1}}},
			}},
		}},
		DefaultPagingDRX: ngaplib.Ptr(ngaplib.PagingDRXv128),
	}
}

func TestNGSetupOutcomeAccepts(t *testing.T) {
	req := outcomeRequest(t, "001", "01", 0x000001)

	tais, out, accepted, reason, err := ngSetupOutcomeFor(req,
		operatorFor("001", "01", "000001"), []models.Snssai{{Sst: 1}}, "ella-amf", 255)
	if err != nil {
		t.Fatalf("outcome: %v", err)
	}

	if !accepted || reason != "" {
		t.Fatalf("accepted = %v, reason = %q, want accepted with no reason", accepted, reason)
	}

	if len(tais) != 1 || tais[0].Tai.Tac != "000001" {
		t.Fatalf("tais = %+v", tais)
	}

	pdu, err := ngaplib.Unmarshal(out)
	if err != nil {
		t.Fatalf("unmarshal outcome: %v", err)
	}

	if _, ok := pdu.(*ngaplib.SuccessfulOutcome); !ok {
		t.Fatalf("outcome is %T, want an NG Setup Response", pdu)
	}
}

func TestNGSetupOutcomeRejects(t *testing.T) {
	tests := []struct {
		name      string
		req       func(*testing.T) *ngaplib.NGSetupRequest
		wantCause ngaplib.Cause
	}{
		{
			"unknown PLMN",
			func(t *testing.T) *ngaplib.NGSetupRequest { return outcomeRequest(t, "999", "99", 0x000001) },
			causeUnknownPLMN,
		},
		{
			"served PLMN but unserved TAC",
			func(t *testing.T) *ngaplib.NGSetupRequest { return outcomeRequest(t, "001", "01", 0x000064) },
			causeNoServedTAC,
		},
		{
			"no supported TA at all",
			func(t *testing.T) *ngaplib.NGSetupRequest {
				req := outcomeRequest(t, "001", "01", 0x000001)
				req.SupportedTAList = nil

				return req
			},
			causeNoServedTAC,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, out, accepted, reason, err := ngSetupOutcomeFor(tt.req(t),
				operatorFor("001", "01", "000001"), []models.Snssai{{Sst: 1}}, "ella-amf", 255)
			if err != nil {
				t.Fatalf("outcome: %v", err)
			}

			if accepted {
				t.Fatal("accepted = true, want a rejection")
			}

			if reason == "" {
				t.Error("rejection carries no reason")
			}

			pdu, err := ngaplib.Unmarshal(out)
			if err != nil {
				t.Fatalf("unmarshal outcome: %v", err)
			}

			uo, ok := pdu.(*ngaplib.UnsuccessfulOutcome)
			if !ok {
				t.Fatalf("outcome is %T, want an NG Setup Failure", pdu)
			}

			fail, err := ngaplib.ParseNGSetupFailure(uo.Value)
			if err != nil {
				t.Fatalf("parse failure: %v", err)
			}

			if fail.Cause == nil || *fail.Cause != tt.wantCause {
				t.Errorf("cause = %v, want %v", fail.Cause, tt.wantCause)
			}
		})
	}
}
