// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/s1ap"
)

func TestListOfOnlyUnknownItemsIsRenderedAsAnUnknownIE(t *testing.T) {
	b, err := (&s1ap.ERABSetupResponse{
		MMEUES1APID: s1ap.Ptr(s1ap.MMEUES1APID(1)),
		ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(2)),
		ERABSetup: s1ap.ERABSetupListBearerSURes{{
			ERABID:                1,
			TransportLayerAddress: ap.TransportLayerAddressFromOctets([]byte{10, 0, 0, 1}),
			GTPTEID:               1,
		}},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	raw, err := s1ap.ParseRawPDU(b)
	if err != nil {
		t.Fatal(err)
	}

	i, err := raw.Index(s1ap.IDERABSetupListBearerSURes)
	if err != nil {
		t.Fatal(err)
	}

	if err := raw.SetValue(i, []byte{0x00, 0x27, 0x0f, 0x40, 0x01, 0xab}); err != nil {
		t.Fatal(err)
	}

	b, err = raw.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := json.Marshal(DecodeS1APMessage(b))
	if err != nil {
		t.Fatal(err)
	}

	got := string(out)

	if strings.Contains(got, "erab_id") {
		t.Fatalf("a zero E-RAB is rendered for a list that holds none: %s", got)
	}

	if !strings.Contains(got, `"hex":"00270f4001ab"`) {
		t.Fatalf("the list IE is not rendered as an unknown {id, criticality, hex} IE: %s", got)
	}
}
