// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6c

import (
	"encoding/hex"
	"errors"
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

	return tgpp.FeatureList(m.AVPs, tgpp.VendorID, FeatureListID)&FeatureSMSFSupport != 0
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
