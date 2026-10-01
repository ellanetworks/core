// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6c

import (
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

var (
	smscIdentity = diameter.Identity{OriginHost: "smsc.example.org", OriginRealm: "example.org"}
	hssIdentity  = diameter.Identity{OriginHost: "hss.example.org", OriginRealm: "example.org"}
	testEnvelope = tgpp.Envelope{
		SessionID:        "smsc.example.org;1;1",
		Origin:           smscIdentity,
		DestinationHost:  "hss.example.org",
		DestinationRealm: "example.org",
	}
)

const testServiceCentreAddress = "15550000000"

func mustHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func unsigned(t *testing.T, a diameter.AVP) uint32 {
	t.Helper()

	v, err := a.Unsigned32()
	if err != nil {
		t.Fatal(err)
	}

	return v
}

func grouped(t *testing.T, a diameter.AVP) []diameter.AVP {
	t.Helper()

	inner, err := a.Grouped()
	if err != nil {
		t.Fatal(err)
	}

	return inner
}

func roundTrip(t *testing.T, m *diameter.Message) *diameter.Message {
	t.Helper()

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := diameter.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	return out
}

func resultCode(t *testing.T, err error) uint32 {
	t.Helper()

	var avpErr *diameter.AVPError
	if !errors.As(err, &avpErr) {
		t.Fatalf("err = %v, want an AVP error", err)
	}

	return avpErr.ResultCode
}

func mmeNode() *ServingNode {
	return &ServingNode{MME: &NodeAddress{Name: "mme.example.org", Realm: "example.org", Number: "15550000010"}}
}

func smsfNode() *NodeAddress {
	return &NodeAddress{Name: "smsf.example.org", Realm: "example.org", Number: "15550000030"}
}

func hasFeatures(t *testing.T, m *diameter.Message) bool {
	t.Helper()

	return tgpp.FeatureList(m.AVPs, tgpp.VendorID, featureListID)&featureSMSFSupport != 0
}

func request(command uint32, avps ...diameter.AVP) *diameter.Message {
	return &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   command,
		ApplicationID: ApplicationID,
		AVPs:          append(testEnvelope.AVPs(), avps...),
	}
}

func TestNewErrorAnswerCarriesFeatures(t *testing.T) {
	req := request(CommandSendRoutingInfoForSM)
	ans := NewErrorAnswer(req, hssIdentity, tgpp.MissingAVP(tgpp.AVPSCAddress, tgpp.VendorID))

	if !hasFeatures(t, ans) {
		t.Fatal("error answer without Supported-Features")
	}

	if _, ok := ans.Find(diameter.AVPFailedAVP, 0); !ok {
		t.Fatal("error answer without Failed-AVP")
	}

	if !hasFeatures(t, NewAnswer(req, hssIdentity, tgpp.Experimental(tgpp.ResultErrorMWDListFull))) {
		t.Fatal("result answer without Supported-Features")
	}
}

func TestEnumStrings(t *testing.T) {
	for _, tc := range []struct {
		value fmt.Stringer
		want  string
	}{
		{SMRPMTIDeliver, "SM_DELIVER"},
		{SMRPMTIStatusReport, "SM_STATUS_REPORT"},
		{MTI(9), "MTI(9)"},
		{SMDeliveryNotIntendedIMSI, "ONLY_IMSI_REQUESTED"},
		{SMDeliveryNotIntendedMCCMNC, "ONLY_MCC_MNC_REQUESTED"},
		{DeliveryNotIntended(2), "DeliveryNotIntended(2)"},
		{DeliveryCauseMemoryCapacityExceeded, "UE_MEMORY_CAPACITY_EXCEEDED"},
		{DeliveryCauseAbsentUser, "ABSENT_USER"},
		{DeliveryCauseSuccessfulTransfer, "SUCCESSFUL_TRANSFER"},
		{DeliveryCause(3), "DeliveryCause(3)"},
		{AlertEvent(0), "0"},
		{AlertEventUEAvailableForMTSMS, "UE_AVAILABLE_FOR_MT_SMS"},
		{AlertEventUEAvailableForMTSMS | AlertEventUEUnderNewNode, "UE_AVAILABLE_FOR_MT_SMS|UE_UNDER_NEW_SERVING_NODE"},
		{AlertEventUEUnderNewNode | 1<<4 | 1<<8, "UE_UNDER_NEW_SERVING_NODE|0x110"},
		{AlertEvent(1 << 31), "0x80000000"},
		{MWDStatus(0), "0"},
		{MWDStatusSCAddressNotIncluded, "SC_ADDRESS_NOT_INCLUDED"},
		{MWDStatusMNRF | MWDStatusMCEF | MWDStatusMNRG, "MNRF_SET|MCEF_SET|MNRG_SET"},
		{MWDStatusMNR5GN3G | MWDStatusMNR5G, "MNR5G_SET|MNR5GN3G_SET"},
		{MWDStatusMNRF | 1<<6, "MNRF_SET|0x40"},
	} {
		if got := tc.value.String(); got != tc.want {
			t.Errorf("%T(%d).String() = %q, want %q", tc.value, tc.value, got, tc.want)
		}
	}
}
