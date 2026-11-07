// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/diameter"
)

func testLocation() *EPSLocationInformation {
	age := uint32(3)

	return &EPSLocationInformation{MME: &MMELocationInformation{
		ECGI: []byte{0x00, 0xf1, 0x10, 0x01, 0x23, 0x45, 0x67},
		TAI:  []byte{0x00, 0xf1, 0x10, 0x00, 0x01},
		Age:  &age,
	}}
}

func TestPurgeUEWithLocationRoundTrip(t *testing.T) {
	r := PurgeUERequest{IMSI: testIMSI, Location: testLocation()}

	req := must[*diameter.Message](t)(NewPurgeUERequest(mmeEnvelope, r))

	got, err := ParsePurgeUERequest(roundTrip(t, req))
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
}

func TestInsertSubscriberDataAnswerUserStateAndLocation(t *testing.T) {
	req := request(CommandInsertSubscriberData, hssEnvelope)
	state := UserStateConnectedReachableForPaging
	a := InsertSubscriberData{UserState: &state, Location: testLocation()}

	ans := must[*diameter.Message](t)(NewInsertSubscriberDataAnswer(req, mmeIdentity, a))

	back, err := ParseInsertSubscriberDataAnswer(roundTrip(t, ans))
	if err != nil || !reflect.DeepEqual(back, a) {
		t.Fatalf("IDA = %+v, %v, want %+v", back, err, a)
	}

	empty := InsertSubscriberData{Location: &EPSLocationInformation{}}

	ans = must[*diameter.Message](t)(NewInsertSubscriberDataAnswer(req, mmeIdentity, empty))
	if back, err := ParseInsertSubscriberDataAnswer(roundTrip(t, ans)); err != nil || !reflect.DeepEqual(back, empty) {
		t.Fatalf("IDA without a known location = %+v, %v", back, err)
	}
}

func TestLocationValidation(t *testing.T) {
	for name, loc := range map[string]*EPSLocationInformation{
		"short ECGI": {MME: &MMELocationInformation{ECGI: []byte{1, 2, 3}}},
		"long TAI":   {MME: &MMELocationInformation{TAI: []byte{1, 2, 3, 4, 5, 6}}},
	} {
		if _, err := NewPurgeUERequest(mmeEnvelope, PurgeUERequest{IMSI: testIMSI, Location: loc}); !errors.Is(err, ErrInvalidMessage) {
			t.Fatalf("%s: %v, want %v", name, err, ErrInvalidMessage)
		}
	}

	bad := UserState(9)
	if _, err := NewInsertSubscriberDataAnswer(request(CommandInsertSubscriberData, hssEnvelope), mmeIdentity, InsertSubscriberData{UserState: &bad}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("unknown User-State: %v", err)
	}
}
