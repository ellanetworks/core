// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/decoder/rrc"
	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_UERadioCapabilityInfoIndication(t *testing.T) {
	raw, err := decodeB64(ueRadioCapabilityInfoIndicationCapture)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcUERadioCapabilityInfoIndication) {
		t.Errorf("expected ProcedureCode=UERadioCapabilityInfoIndication, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != int64(lib.ProcUERadioCapabilityInfoIndication) {
		t.Errorf("procedure code = %d, want %d", ngapMsg.ProcedureCode.Value, lib.ProcUERadioCapabilityInfoIndication)
	}

	if ngapMsg.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", ngapMsg.Criticality)
	}

	if len(ngapMsg.Value.IEs) != 3 {
		t.Errorf("expected 3 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Value != int64(lib.IDAMFUENGAPID) {
		t.Errorf("IE id = %d, want %d", item0.ID.Value, lib.IDAMFUENGAPID)
	}

	if item0.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item0.Criticality)
	}

	amfUENGAPID, ok := item0.Value.(int64)
	if !ok {
		t.Errorf("expected AMF-UE-NGAP-ID type=int64, got %T", item0.Value)
	}

	if amfUENGAPID != 6 {
		t.Errorf("expected AMF-UE-NGAP-ID=6, got %d", amfUENGAPID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Value != int64(lib.IDRANUENGAPID) {
		t.Errorf("IE id = %d, want %d", item1.ID.Value, lib.IDRANUENGAPID)
	}

	if item1.Criticality.Value != int64(lib.CriticalityReject) {
		t.Errorf("Criticality = %v, want reject", item1.Criticality)
	}

	ranUENGAPID, ok := item1.Value.(int64)
	if !ok {
		t.Errorf("expected RAN-UE-NGAP-ID type=int64, got %T", item1.Value)
	}

	if ranUENGAPID != 5 {
		t.Errorf("expected RAN-UE-NGAP-ID=5, got %d", ranUENGAPID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Value != int64(lib.IDUERadioCapability) {
		t.Errorf("IE id = %d, want %d", item2.ID.Value, lib.IDUERadioCapability)
	}

	if item2.Criticality.Value != int64(lib.CriticalityIgnore) {
		t.Errorf("Criticality = %v, want ignore", item2.Criticality)
	}

	ueRadioCapability, ok := item2.Value.(rrc.PDU)
	if !ok {
		t.Fatalf("expected UERadioCapability to be of type rrc.PDU, got %T", item2.Value)
	}

	if ueRadioCapability.Decoded.Error != "" {
		t.Fatalf("UERadioCapability decode error: %s", ueRadioCapability.Decoded.Error)
	}

	if ueRadioCapability.Decoded.Summary.NR == nil {
		t.Fatal("UERadioCapability carries no NR capability")
	}

	if ueRadioCapability.Decoded.Summary.NR.AccessStratumRelease != "rel15" {
		t.Errorf("accessStratumRelease = %q, want rel15", ueRadioCapability.Decoded.Summary.NR.AccessStratumRelease)
	}

	wantBands := []int64{28, 78, 20, 7, 3, 1, 77, 41, 40, 38, 8, 5}
	if len(ueRadioCapability.Decoded.Summary.NR.Bands) != len(wantBands) {
		t.Fatalf("got %d NR bands, want %d", len(ueRadioCapability.Decoded.Summary.NR.Bands), len(wantBands))
	}

	for i, want := range wantBands {
		if ueRadioCapability.Decoded.Summary.NR.Bands[i].Band != want {
			t.Errorf("band[%d] = %d, want %d", i, ueRadioCapability.Decoded.Summary.NR.Bands[i].Band, want)
		}
	}

	if ueRadioCapability.Decoded.Summary.EUTRA == nil {
		t.Fatal("UERadioCapability carries no E-UTRA capability")
	}

	if ueRadioCapability.Decoded.Summary.EUTRA.UECategory != 4 {
		t.Errorf("ue-Category = %d, want 4", ueRadioCapability.Decoded.Summary.EUTRA.UECategory)
	}

	expectedUERadioCapability := "BE1JCDIumgUABXT1oDFkADAkAsEmLAAzh6BgmyDDnzDHlCwOCYBAYjgWUHwb1gjCGggQeIBElsmCoJHHkOfGOfMMeULA4HDwJ/QAAAH9AAAAqDYm6wRhDQQIOkAiy2TBUNDnxHHoGfYE+Mc+YY8oWBwOHguAAvgA4AC+AASQDgA2UAAFAAABQfBPWCMIaCBB4gESWyYKgkceQ58Y58wx5QsDgcPAn8AAAAfwAAAAoPgNrBGENBAg8QCJLZMFQSOPIc+Mc+YY8oWBwOHgT+AAAAP4AAAAUHwC1gjCGggQeIBElsmCoJHHkOfGOfMMeULA4HDwJ/AAAAH8AAAAKD4AawRhDQQIPEAiS2TBUEjjyHPjHPmGPKFgcDh4E/sAAAD+wAAAVBsTNYIwhoIEHSARZbJgqGhz4jj0DPsCfGOfMMeULA4HDwXAAWwAcABbAAJABwOg2FGBrBGENBAg6QCLLZMFQ0OfEcegZ9gT4xz5hjyhYHA4eC4AC+ADgAL4ABJAOADZQAAUAAAFBsJ9YIwhoIEHSARZbJgqGhz4jj0DPsCfGOfMMeULA4HDwnAAXwAcABfAAoNhLrBGENBAg6QCLLZMFQ0OfEcegZ9gT4xz5hjyhYHA4eE4ACwADgALAAFB8B9YIwhoIEHiARJbJgqCRx5DnxjnzDHlCwOBw8CfwAAAB/AAAACg+AmsEYQ0ECDxAIktkwVBI48hz4xz5hjyhYHA4eBP4AAAA/gAAAACEHEwAAAAAICYCoNAMCFsc+GBQEZ6B4A+ACgcDAOBgCoDgAMBIAIDAfA0AQAAAACAAAEAQAABACAAAEAQAABgCAAAQAQAACgCAAAYAQAADgCAAAgAQAAEgCAAAoAQAAFgCAAAwDGCl5U1gZWBtYMVlTWUtZM1grWDNYCVgLWUNAgEAAAllJAQAAGWUkBAAApZSQEAADllJAQAASWUkBAABZZSQEAAGllJAQAAeWUkBAACJZSQCNKoqgsVQOKoNFUqiqUxVJoqgMVSiKBhKAawCAwCAwGAQCAQGAwGAwCAAAAAIBwCiVCAAAgQIDA4mNj5CSkxOUFJUhoQBQ0Du3AKIAhgh8mCAB0ptCmTqFKp+Qof//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/3/P/r6IKAIyFOGU6VJcfwFhOAAAAAvQAACBnfAAAAAuggBEAMAKAEQAwAwAwAoIQwAwQBggDBAGCAMEIYIQwQhghCggDBAGCAMEAYIAwQBghDBCGCEMAMEAYIAwAwAwAwAwgBDCAEMIAQwgBDCAEMIAQwgBDCAEMIAQwgBDCAEMIAQwghDCCEMIIQwghDCCEEIIAwgBDCCEEIIAwgBDCCEEIIAwgBDCAEMIAQwgBDCAEMIAQwgBDADADADADADACWn1UaFIAKAGCAAgAYwACCBiAAIAC+EADTAAQAMIABBAwAAEEDHAAQAOUABBA2wAEADhAAQAOYABAA5wAEEDoAAQQOkABBA6gAEED/AAQAcYABBRgggYwACSDtgAICMEEFbCBjAAIIOmAAgIwQQVMIGMAAgg4wACChBBBRgg4QACCDjAAIKAEEFGCDgAAIIK+EDTAAQAdMABAQgggqYQMIABBB0wAEBACCCphAwAAEEHbAAQEIIIK2EDCAAQQcIABBQggg4QACCgBBBQgg4AACCBhAAJIO2AAgIAQQVsIGAAAggYAACSBygAJIHOAAkgdAACSFtgAICMEFCCCEthAxgAEFCCCEthARgg4QACCFtgAICMEFACCEthAxgAEFACCEthARgg4AACCFpgAICMEFCCCEphAxgAEFCCCEphARgg4QACCFpgAICMEFACCEphAxgAEFACCEphARgg4AACCFjAAIKMEFCCCEjBBRgg4QACCFjAAIKEEFCCCEjBBwgAEFCCCFjAAIKEEFACAFjAAIKEEBACCFjAAICEEFACCEjBBwgAEFACAEjBBwgAEBACCEjBAwgAEFACCEjBBQgg4AACAEjBBQggYAACCEjBAQgg4AACCFpgAICEEFACCEphAwgAEFACCEphAQgg4AACCFtgAICEEFACCEthAwgAEFACCEthAQgg4AACCBjAIJIGEAgkgYACCSBygIJIHOAgkgdACCTQof//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/fOwAAI6AAAI/i8PDxeLw8PDw8PDw5Dw8Xh4dDw8Oh4/EXi8Xh4eHi8Xi8PDw8PDw8PDw8PDw8OBF0Oh4/9AAAAAAAAAAAAAAAAAAAAAAAQMEAAEAQBAEAQBAEAQBAEAQBAEAQBAEAQBAABAEAAAAAAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAAAAAAAACYIA8ZeaBAFKH9////5pS7xwDoAFggAZJKEAEAAlhQgCQBIAggCQBIAkASAJAEEASAJAEgCQBIAkBQIAkBQICCQFAgIJAUCAgkBQICCQEEgKBAQSAoEBBICgQEEgKBAUCAgkASAoEBBIAkASAJAEgSAECBAgQAkCQAgQIECAEgSAECBAgQAkCQAgQIECAEgSAECAEgSAECBAgSAECQAgSAECBAgQIECBAgQAkCAEgQAkCQAgQIECAEgSAECBAgQAkAIHACBwAgcAIHACBwAgYglTwLAAAgEAYBwTBsJQnCgTBNEACIBBQwYYFgAAQCAMA4Jg2EoThQJgmiAAA=="

	expectedUERadioCapabilityRaw, err := decodeB64(expectedUERadioCapability)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if ueRadioCapability.RawHex != hex.EncodeToString(expectedUERadioCapabilityRaw) {
		t.Errorf("UERadioCapability hex = %s, want %s", ueRadioCapability.RawHex, hex.EncodeToString(expectedUERadioCapabilityRaw))
	}
}

// A UERadioCapabilityInfoIndication captured on the 001/01 test PLMN.
const ueRadioCapabilityInfoIndicationCapture = "ACxAicIAAAMACgACAAYAVQACAAUAdUCJromsBE1JCDIumgUABXT1oDFkADAkAsEmLAAzh6BgmyDDnzDHlCwOCYBAYjgWUHwb1gjCGggQeIBElsmCoJHHkOfGOfMMeULA4HDwJ/QAAAH9AAAAqDYm6wRhDQQIOkAiy2TBUNDnxHHoGfYE+Mc+YY8oWBwOHguAAvgA4AC+AASQDgA2UAAFAAABQfBPWCMIaCBB4gESWyYKgkceQ58Y58wx5QsDgcPAn8AAAAfwAAAAoPgNrBGENBAg8QCJLZMFQSOPIc+Mc+YY8oWBwOHgT+AAAAP4AAAAUHwC1gjCGggQeIBElsmCoJHHkOfGOfMMeULA4HDwJ/AAAAH8AAAAKD4AawRhDQQIPEAiS2TBUEjjyHPjHPmGPKFgcDh4E/sAAAD+wAAAVBsTNYIwhoIEHSARZbJgqGhz4jj0DPsCfGOfMMeULA4HDwXAAWwAcABbAAJABwOg2FGBrBGENBAg6QCLLZMFQ0OfEcegZ9gT4xz5hjyhYHA4eC4AC+ADgAL4ABJAOADZQAAUAAAFBsJ9YIwhoIEHSARZbJgqGhz4jj0DPsCfGOfMMeULA4HDwnAAXwAcABfAAoNhLrBGENBAg6QCLLZMFQ0OfEcegZ9gT4xz5hjyhYHA4eE4ACwADgALAAFB8B9YIwhoIEHiARJbJgqCRx5DnxjnzDHlCwOBw8CfwAAAB/AAAACg+AmsEYQ0ECDxAIktkwVBI48hz4xz5hjyhYHA4eBP4AAAA/gAAAACEHEwAAAAAICYCoNAMCFsc+GBQEZ6B4A+ACgcDAOBgCoDgAMBIAIDAfA0AQAAAACAAAEAQAABACAAAEAQAABgCAAAQAQAACgCAAAYAQAADgCAAAgAQAAEgCAAAoAQAAFgCAAAwDGCl5U1gZWBtYMVlTWUtZM1grWDNYCVgLWUNAgEAAAllJAQAAGWUkBAAApZSQEAADllJAQAASWUkBAABZZSQEAAGllJAQAAeWUkBAACJZSQCNKoqgsVQOKoNFUqiqUxVJoqgMVSiKBhKAawCAwCAwGAQCAQGAwGAwCAAAAAIBwCiVCAAAgQIDA4mNj5CSkxOUFJUhoQBQ0Du3AKIAhgh8mCAB0ptCmTqFKp+Qof//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/3/P/r6IKAIyFOGU6VJcfwFhOAAAAAvQAACBnfAAAAAuggBEAMAKAEQAwAwAwAoIQwAwQBggDBAGCAMEIYIQwQhghCggDBAGCAMEAYIAwQBghDBCGCEMAMEAYIAwAwAwAwAwgBDCAEMIAQwgBDCAEMIAQwgBDCAEMIAQwgBDCAEMIAQwghDCCEMIIQwghDCCEEIIAwgBDCCEEIIAwgBDCCEEIIAwgBDCAEMIAQwgBDCAEMIAQwgBDADADADADADACWn1UaFIAKAGCAAgAYwACCBiAAIAC+EADTAAQAMIABBAwAAEEDHAAQAOUABBA2wAEADhAAQAOYABAA5wAEEDoAAQQOkABBA6gAEED/AAQAcYABBRgggYwACSDtgAICMEEFbCBjAAIIOmAAgIwQQVMIGMAAgg4wACChBBBRgg4QACCDjAAIKAEEFGCDgAAIIK+EDTAAQAdMABAQgggqYQMIABBB0wAEBACCCphAwAAEEHbAAQEIIIK2EDCAAQQcIABBQggg4QACCgBBBQgg4AACCBhAAJIO2AAgIAQQVsIGAAAggYAACSBygAJIHOAAkgdAACSFtgAICMEFCCCEthAxgAEFCCCEthARgg4QACCFtgAICMEFACCEthAxgAEFACCEthARgg4AACCFpgAICMEFCCCEphAxgAEFCCCEphARgg4QACCFpgAICMEFACCEphAxgAEFACCEphARgg4AACCFjAAIKMEFCCCEjBBRgg4QACCFjAAIKEEFCCCEjBBwgAEFCCCFjAAIKEEFACAFjAAIKEEBACCFjAAICEEFACCEjBBwgAEFACAEjBBwgAEBACCEjBAwgAEFACCEjBBQgg4AACAEjBBQggYAACCEjBAQgg4AACCFpgAICEEFACCEphAwgAEFACCEphAQgg4AACCFtgAICEEFACCEthAwgAEFACCEthAQgg4AACCBjAIJIGEAgkgYACCSBygIJIHOAgkgdACCTQof//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/fOwAAI6AAAI/i8PDxeLw8PDw8PDw5Dw8Xh4dDw8Oh4/EXi8Xh4eHi8Xi8PDw8PDw8PDw8PDw8OBF0Oh4/9AAAAAAAAAAAAAAAAAAAAAAAQMEAAEAQBAEAQBAEAQBAEAQBAEAQBAEAQBAABAEAAAAAAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAAAAAAAACYIA8ZeaBAFKH9////5pS7xwDoAFggAZJKEAEAAlhQgCQBIAggCQBIAkASAJAEEASAJAEgCQBIAkBQIAkBQICCQFAgIJAUCAgkBQICCQEEgKBAQSAoEBBICgQEEgKBAUCAgkASAoEBBIAkASAJAEgSAECBAgQAkCQAgQIECAEgSAECBAgQAkCQAgQIECAEgSAECAEgSAECBAgSAECQAgSAECBAgQIECBAgQAkCAEgQAkCQAgQIECAEgSAECBAgQAkAIHACBwAgcAIHACBwAgYglTwLAAAgEAYBwTBsJQnCgTBNEACIBBQwYYFgAAQCAMA4Jg2EoThQJgmiAAA=="
