// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/json"
	"strings"
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

func TestUPTransportLayerInformationChoiceExtension(t *testing.T) {
	transfer, err := (&lib.PDUSessionResourceSetupResponseTransfer{
		DLQosFlowPerTNLInformation: lib.QosFlowPerTNLInformation{
			UPTransportLayerInformation: lib.UPTransportLayerInformation{
				ChoiceExtension: &lib.SingleContainer{ID: 999, Criticality: lib.CriticalityIgnore, Value: "\x01\x02"},
			},
			AssociatedQosFlowList: lib.AssociatedQosFlowList{{QosFlowIdentifier: 1}},
		},
	}).Marshal()
	if err != nil {
		t.Fatalf("build transfer: %v", err)
	}

	raw := mustMarshal(t, &lib.PDUSessionResourceSetupResponse{
		AMFUENGAPID:             lib.Ptr(lib.AMFUENGAPID(1)),
		RANUENGAPID:             lib.Ptr(lib.RANUENGAPID(2)),
		PDUSessionResourceSetup: lib.PDUSessionResourceSetupListSURes{{PDUSessionID: 1, Transfer: transfer}},
	})

	b, err := json.Marshal(DecodeNGAPMessage(raw))
	if err != nil {
		t.Fatal(err)
	}

	want := `"gtp_tunnel":{"choice_extensions":{"id":{"type":"enum","value":999,"label":"","unknown":true},` +
		`"criticality":{"type":"enum","value":1,"label":"ignore","unknown":false},"value":{"hex":"0102"}}}`
	if !strings.Contains(string(b), want) {
		t.Errorf("choice extension not rendered as the CHOICE alternative:\n%s", b)
	}

	if strings.Contains(string(b), `"gtp_teid"`) {
		t.Errorf("non-GTP alternative rendered as a GTP tunnel:\n%s", b)
	}
}
