package ngap_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func TestDecodeNGAPMessage_UERadioCapabilityInfoIndication(t *testing.T) {
	const message = "ACxAicIAAAMACgACAAYAVQACAAUAdUCJromsBE1JCDIumgUABXT1oDFkADAkAsEmLAAzh6BgmyDDnzDHlCwOCYBAYjgWUHwb1gjCGggQeIBElsmCoJHHkOfGOfMMeULA4HDwJ/QAAAH9AAAAqDYm6wRhDQQIOkAiy2TBUNDnxHHoGfYE+Mc+YY8oWBwOHguAAvgA4AC+AASQDgA2UAAFAAABQfBPWCMIaCBB4gESWyYKgkceQ58Y58wx5QsDgcPAn8AAAAfwAAAAoPgNrBGENBAg8QCJLZMFQSOPIc+Mc+YY8oWBwOHgT+AAAAP4AAAAUHwC1gjCGggQeIBElsmCoJHHkOfGOfMMeULA4HDwJ/AAAAH8AAAAKD4AawRhDQQIPEAiS2TBUEjjyHPjHPmGPKFgcDh4E/sAAAD+wAAAVBsTNYIwhoIEHSARZbJgqGhz4jj0DPsCfGOfMMeULA4HDwXAAWwAcABbAAJABwOg2FGBrBGENBAg6QCLLZMFQ0OfEcegZ9gT4xz5hjyhYHA4eC4AC+ADgAL4ABJAOADZQAAUAAAFBsJ9YIwhoIEHSARZbJgqGhz4jj0DPsCfGOfMMeULA4HDwnAAXwAcABfAAoNhLrBGENBAg6QCLLZMFQ0OfEcegZ9gT4xz5hjyhYHA4eE4ACwADgALAAFB8B9YIwhoIEHiARJbJgqCRx5DnxjnzDHlCwOBw8CfwAAAB/AAAACg+AmsEYQ0ECDxAIktkwVBI48hz4xz5hjyhYHA4eBP4AAAA/gAAAACEHEwAAAAAICYCoNAMCFsc+GBQEZ6B4A+ACgcDAOBgCoDgAMBIAIDAfA0AQAAAACAAAEAQAABACAAAEAQAABgCAAAQAQAACgCAAAYAQAADgCAAAgAQAAEgCAAAoAQAAFgCAAAwDGCl5U1gZWBtYMVlTWUtZM1grWDNYCVgLWUNAgEAAAllJAQAAGWUkBAAApZSQEAADllJAQAASWUkBAABZZSQEAAGllJAQAAeWUkBAACJZSQCNKoqgsVQOKoNFUqiqUxVJoqgMVSiKBhKAawCAwCAwGAQCAQGAwGAwCAAAAAIBwCiVCAAAgQIDA4mNj5CSkxOUFJUhoQBQ0Du3AKIAhgh8mCAB0ptCmTqFKp+Qof//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/3/P/r6IKAIyFOGU6VJcfwFhOAAAAAvQAACBnfAAAAAuggBEAMAKAEQAwAwAwAoIQwAwQBggDBAGCAMEIYIQwQhghCggDBAGCAMEAYIAwQBghDBCGCEMAMEAYIAwAwAwAwAwgBDCAEMIAQwgBDCAEMIAQwgBDCAEMIAQwgBDCAEMIAQwghDCCEMIIQwghDCCEEIIAwgBDCCEEIIAwgBDCCEEIIAwgBDCAEMIAQwgBDCAEMIAQwgBDADADADADADACWn1UaFIAKAGCAAgAYwACCBiAAIAC+EADTAAQAMIABBAwAAEEDHAAQAOUABBA2wAEADhAAQAOYABAA5wAEEDoAAQQOkABBA6gAEED/AAQAcYABBRgggYwACSDtgAICMEEFbCBjAAIIOmAAgIwQQVMIGMAAgg4wACChBBBRgg4QACCDjAAIKAEEFGCDgAAIIK+EDTAAQAdMABAQgggqYQMIABBB0wAEBACCCphAwAAEEHbAAQEIIIK2EDCAAQQcIABBQggg4QACCgBBBQgg4AACCBhAAJIO2AAgIAQQVsIGAAAggYAACSBygAJIHOAAkgdAACSFtgAICMEFCCCEthAxgAEFCCCEthARgg4QACCFtgAICMEFACCEthAxgAEFACCEthARgg4AACCFpgAICMEFCCCEphAxgAEFCCCEphARgg4QACCFpgAICMEFACCEphAxgAEFACCEphARgg4AACCFjAAIKMEFCCCEjBBRgg4QACCFjAAIKEEFCCCEjBBwgAEFCCCFjAAIKEEFACAFjAAIKEEBACCFjAAICEEFACCEjBBwgAEFACAEjBBwgAEBACCEjBAwgAEFACCEjBBQgg4AACAEjBBQggYAACCEjBAQgg4AACCFpgAICEEFACCEphAwgAEFACCEphAQgg4AACCFtgAICEEFACCEthAwgAEFACCEthAQgg4AACCBjAIJIGEAgkgYACCSBygIJIHOAgkgdACCTQof//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/fOwAAI6AAAI/i8PDxeLw8PDw8PDw5Dw8Xh4dDw8Oh4/EXi8Xh4eHi8Xi8PDw8PDw8PDw8PDw8OBF0Oh4/9AAAAAAAAAAAAAAAAAAAAAAAQMEAAEAQBAEAQBAEAQBAEAQBAEAQBAEAQBAABAEAAAAAAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAAAAAAAACYIA8ZeaBAFKH9////5pS7xwDoAFggAZJKEAEAAlhQgCQBIAggCQBIAkASAJAEEASAJAEgCQBIAkBQIAkBQICCQFAgIJAUCAgkBQICCQEEgKBAQSAoEBBICgQEEgKBAUCAgkASAoEBBIAkASAJAEgSAECBAgQAkCQAgQIECAEgSAECBAgQAkCQAgQIECAEgSAECAEgSAECBAgSAECQAgSAECBAgQIECBAgQAkCAEgQAkCQAgQIECAEgSAECBAgQAkAIHACBwAgcAIHACBwAgYglTwLAAAgEAYBwTBsJQnCgTBNEACIBBQwYYFgAAQCAMA4Jg2EoThQJgmiAAA=="

	raw, err := decodeB64(message)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	ngapMsg := ngap.DecodeNGAPMessage(raw)

	if ngapMsg.PDUType != "InitiatingMessage" {
		t.Errorf("expected PDUType=InitiatingMessage, got %v", ngapMsg.PDUType)
	}

	if ngapMsg.MessageType != "UERadioCapabilityInfoIndication" {
		t.Errorf("expected MessageType=UERadioCapabilityInfoIndication, got %v", ngapMsg.MessageType)
	}

	if ngapMsg.ProcedureCode.Label != "UERadioCapabilityInfoIndication" {
		t.Errorf("expected ProcedureCode=UERadioCapabilityInfoIndication, got %v", ngapMsg.ProcedureCode)
	}

	if ngapMsg.ProcedureCode.Value != ngapType.ProcedureCodeUERadioCapabilityInfoIndication {
		t.Errorf("expected ProcedureCode value=44, got %d", ngapMsg.ProcedureCode.Value)
	}

	if ngapMsg.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore (1), got %v", ngapMsg.Criticality)
	}

	if ngapMsg.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", ngapMsg.Criticality.Value)
	}

	if len(ngapMsg.Value.IEs) != 3 {
		t.Errorf("expected 3 ProtocolIEs, got %d", len(ngapMsg.Value.IEs))
	}

	item0 := ngapMsg.Value.IEs[0]

	if item0.ID.Label != "AMFUENGAPID" {
		t.Errorf("expected ID=AMFUENGAPID, got %s", item0.ID.Label)
	}

	if item0.ID.Value != ngapType.ProtocolIEIDAMFUENGAPID {
		t.Errorf("expected ID value=10, got %d", item0.ID.Value)
	}

	if item0.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item0.Criticality)
	}

	if item0.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item0.Criticality.Value)
	}

	amfUENGAPID, ok := item0.Value.(int64)
	if !ok {
		t.Errorf("expected AMFUENGAPID type=int64, got %T", item0.Value)
	}

	if amfUENGAPID != 6 {
		t.Errorf("expected AMFUENGAPID=6, got %d", amfUENGAPID)
	}

	item1 := ngapMsg.Value.IEs[1]

	if item1.ID.Label != "RANUENGAPID" {
		t.Errorf("expected ID=RANUENGAPID, got %s", item1.ID.Label)
	}

	if item1.ID.Value != ngapType.ProtocolIEIDRANUENGAPID {
		t.Errorf("expected ID value=85, got %d", item1.ID.Value)
	}

	if item1.Criticality.Label != "Reject" {
		t.Errorf("expected Criticality=Reject, got %v", item1.Criticality)
	}

	if item1.Criticality.Value != 0 {
		t.Errorf("expected Criticality value=0, got %d", item1.Criticality.Value)
	}

	ranUENGAPID, ok := item1.Value.(int64)
	if !ok {
		t.Errorf("expected RANUENGAPID type=int64, got %T", item1.Value)
	}

	if ranUENGAPID != 5 {
		t.Errorf("expected RANUENGAPID=5, got %d", ranUENGAPID)
	}

	item2 := ngapMsg.Value.IEs[2]

	if item2.ID.Label != "UERadioCapability" {
		t.Errorf("expected ID=UERadioCapability, got %s", item2.ID.Label)
	}

	if item2.ID.Value != ngapType.ProtocolIEIDUERadioCapability {
		t.Errorf("expected ID value=50, got %d", item2.ID.Value)
	}

	if item2.Criticality.Label != "Ignore" {
		t.Errorf("expected Criticality=Ignore, got %v", item2.Criticality)
	}

	if item2.Criticality.Value != 1 {
		t.Errorf("expected Criticality value=1, got %d", item2.Criticality.Value)
	}

	ueRadioCapability, ok := item2.Value.([]byte)
	if !ok {
		t.Fatalf("expected PDUSessionResourceSetupListSURes to be of type []PDUSessionResourceSetupSURes, got %T", item2.Value)
	}

	expectedUERadioCapability := "BE1JCDIumgUABXT1oDFkADAkAsEmLAAzh6BgmyDDnzDHlCwOCYBAYjgWUHwb1gjCGggQeIBElsmCoJHHkOfGOfMMeULA4HDwJ/QAAAH9AAAAqDYm6wRhDQQIOkAiy2TBUNDnxHHoGfYE+Mc+YY8oWBwOHguAAvgA4AC+AASQDgA2UAAFAAABQfBPWCMIaCBB4gESWyYKgkceQ58Y58wx5QsDgcPAn8AAAAfwAAAAoPgNrBGENBAg8QCJLZMFQSOPIc+Mc+YY8oWBwOHgT+AAAAP4AAAAUHwC1gjCGggQeIBElsmCoJHHkOfGOfMMeULA4HDwJ/AAAAH8AAAAKD4AawRhDQQIPEAiS2TBUEjjyHPjHPmGPKFgcDh4E/sAAAD+wAAAVBsTNYIwhoIEHSARZbJgqGhz4jj0DPsCfGOfMMeULA4HDwXAAWwAcABbAAJABwOg2FGBrBGENBAg6QCLLZMFQ0OfEcegZ9gT4xz5hjyhYHA4eC4AC+ADgAL4ABJAOADZQAAUAAAFBsJ9YIwhoIEHSARZbJgqGhz4jj0DPsCfGOfMMeULA4HDwnAAXwAcABfAAoNhLrBGENBAg6QCLLZMFQ0OfEcegZ9gT4xz5hjyhYHA4eE4ACwADgALAAFB8B9YIwhoIEHiARJbJgqCRx5DnxjnzDHlCwOBw8CfwAAAB/AAAACg+AmsEYQ0ECDxAIktkwVBI48hz4xz5hjyhYHA4eBP4AAAA/gAAAACEHEwAAAAAICYCoNAMCFsc+GBQEZ6B4A+ACgcDAOBgCoDgAMBIAIDAfA0AQAAAACAAAEAQAABACAAAEAQAABgCAAAQAQAACgCAAAYAQAADgCAAAgAQAAEgCAAAoAQAAFgCAAAwDGCl5U1gZWBtYMVlTWUtZM1grWDNYCVgLWUNAgEAAAllJAQAAGWUkBAAApZSQEAADllJAQAASWUkBAABZZSQEAAGllJAQAAeWUkBAACJZSQCNKoqgsVQOKoNFUqiqUxVJoqgMVSiKBhKAawCAwCAwGAQCAQGAwGAwCAAAAAIBwCiVCAAAgQIDA4mNj5CSkxOUFJUhoQBQ0Du3AKIAhgh8mCAB0ptCmTqFKp+Qof//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/3/P/r6IKAIyFOGU6VJcfwFhOAAAAAvQAACBnfAAAAAuggBEAMAKAEQAwAwAwAoIQwAwQBggDBAGCAMEIYIQwQhghCggDBAGCAMEAYIAwQBghDBCGCEMAMEAYIAwAwAwAwAwgBDCAEMIAQwgBDCAEMIAQwgBDCAEMIAQwgBDCAEMIAQwghDCCEMIIQwghDCCEEIIAwgBDCCEEIIAwgBDCCEEIIAwgBDCAEMIAQwgBDCAEMIAQwgBDADADADADADACWn1UaFIAKAGCAAgAYwACCBiAAIAC+EADTAAQAMIABBAwAAEEDHAAQAOUABBA2wAEADhAAQAOYABAA5wAEEDoAAQQOkABBA6gAEED/AAQAcYABBRgggYwACSDtgAICMEEFbCBjAAIIOmAAgIwQQVMIGMAAgg4wACChBBBRgg4QACCDjAAIKAEEFGCDgAAIIK+EDTAAQAdMABAQgggqYQMIABBB0wAEBACCCphAwAAEEHbAAQEIIIK2EDCAAQQcIABBQggg4QACCgBBBQgg4AACCBhAAJIO2AAgIAQQVsIGAAAggYAACSBygAJIHOAAkgdAACSFtgAICMEFCCCEthAxgAEFCCCEthARgg4QACCFtgAICMEFACCEthAxgAEFACCEthARgg4AACCFpgAICMEFCCCEphAxgAEFCCCEphARgg4QACCFpgAICMEFACCEphAxgAEFACCEphARgg4AACCFjAAIKMEFCCCEjBBRgg4QACCFjAAIKEEFCCCEjBBwgAEFCCCFjAAIKEEFACAFjAAIKEEBACCFjAAICEEFACCEjBBwgAEFACAEjBBwgAEBACCEjBAwgAEFACCEjBBQgg4AACAEjBBQggYAACCEjBAQgg4AACCFpgAICEEFACCEphAwgAEFACCEphAQgg4AACCFtgAICEEFACCEthAwgAEFACCEthAQgg4AACCBjAIJIGEAgkgYACCSBygIJIHOAgkgdACCTQof//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/of//J/+h//8n/6H//yf/fOwAAI6AAAI/i8PDxeLw8PDw8PDw5Dw8Xh4dDw8Oh4/EXi8Xh4eHi8Xi8PDw8PDw8PDw8PDw8OBF0Oh4/9AAAAAAAAAAAAAAAAAAAAAAAQMEAAEAQBAEAQBAEAQBAEAQBAEAQBAEAQBAABAEAAAAAAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAIAAAAAAAACYIA8ZeaBAFKH9////5pS7xwDoAFggAZJKEAEAAlhQgCQBIAggCQBIAkASAJAEEASAJAEgCQBIAkBQIAkBQICCQFAgIJAUCAgkBQICCQEEgKBAQSAoEBBICgQEEgKBAUCAgkASAoEBBIAkASAJAEgSAECBAgQAkCQAgQIECAEgSAECBAgQAkCQAgQIECAEgSAECAEgSAECBAgSAECQAgSAECBAgQIECBAgQAkCAEgQAkCQAgQIECAEgSAECBAgQAkAIHACBwAgcAIHACBwAgYglTwLAAAgEAYBwTBsJQnCgTBNEACIBBQwYYFgAAQCAMA4Jg2EoThQJgmiAAA=="

	expectedUERadioCapabilityRaw, err := decodeB64(expectedUERadioCapability)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}

	if string(ueRadioCapability) != string(expectedUERadioCapabilityRaw) {
		t.Errorf("expected PDUSessionResourceSetupResponseTransfer=%s, got %s", expectedUERadioCapabilityRaw, ueRadioCapability)
	}
}
