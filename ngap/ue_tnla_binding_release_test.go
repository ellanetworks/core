// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"testing"
)

const goldenUETNLABindingReleaseRequest = "002d4016000002000a000680010203040500550005c001020304"

func goldUETNLABindingReleaseRequest() *UETNLABindingReleaseRequest {
	return &UETNLABindingReleaseRequest{AMFUENGAPID: 0x0102030405, RANUENGAPID: 0x01020304}
}

func uETNLABindingReleaseRequestGoldenCases() []goldenRoundTrip {
	return []goldenRoundTrip{{
		"UETNLABindingReleaseRequest",
		goldUETNLABindingReleaseRequest(),
		func(v []byte) (any, error) { return ParseUETNLABindingReleaseRequest(v) },
		goldenUETNLABindingReleaseRequest,
	}}
}

func TestUETNLABindingReleaseRequestGolden(t *testing.T) {
	for _, c := range uETNLABindingReleaseRequestGoldenCases() {
		t.Run(c.name, func(t *testing.T) { checkGoldenRoundTrip(t, c) })
	}
}

func TestUETNLABindingReleaseRequestEnvelope(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenUETNLABindingReleaseRequest))
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcUETNLABindingRelease || im.Criticality != CriticalityIgnore {
		t.Fatalf("got %T %+v", pdu, pdu)
	}
}

func TestUETNLABindingReleaseRequestMissingRejectIE(t *testing.T) {
	value := container(t, ieField{id: IDAMFUENGAPID, crit: CriticalityReject, val: Ptr(AMFUENGAPID(1))})

	_, err := ParseUETNLABindingReleaseRequest(value)

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) || len(ase.IEs) != 1 || ase.IEs[0].IEID != IDRANUENGAPID {
		t.Fatalf("err = %v, want the missing RAN UE NGAP ID reported", err)
	}
}
