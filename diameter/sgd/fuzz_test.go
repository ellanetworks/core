// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sgd

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func seed(f *testing.F, m *diameter.Message) {
	f.Helper()

	b, err := m.Marshal()
	if err != nil {
		f.Fatal(err)
	}

	f.Add(b)
}

func FuzzParseRequests(f *testing.F) {
	ofr, err := NewMOForwardShortMessageRequest(ofrEnvelope, MOForwardShortMessage{
		ServiceCentreAddress: "15550000000", User: tgpp.UserIdentifier{IMSI: "001010000000001", MSISDN: "15551230002"}, SMRPUI: []byte{1, 2},
	})
	if err != nil {
		f.Fatal(err)
	}

	tfr, err := NewMTForwardShortMessageRequest(tfrEnvelope, MTForwardShortMessage{
		IMSI: "001010000000002", ServiceCentreAddress: "15550000000", SMRPUI: []byte{4}, MMENumberForMTSMS: "15550000010",
		MoreMessagesToSend: true, DeliveryTimer: time.Minute, DeliveryStartTime: time.Unix(1_800_000_000, 0).UTC(),
	})
	if err != nil {
		f.Fatal(err)
	}

	seed(f, ofr)
	seed(f, tfr)
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := diameter.Unmarshal(b)
		if err != nil {
			return
		}

		var avpErr *diameter.AVPError

		mo, err := ParseMOForwardShortMessageRequest(m)
		if err != nil && !errors.As(err, &avpErr) {
			t.Fatalf("MO parser returned %v", err)
		}

		if err == nil {
			rebuilt, err := NewMOForwardShortMessageRequest(ofrEnvelope, mo)
			if err != nil {
				t.Fatalf("parsed OFR %+v does not rebuild: %v", mo, err)
			}

			if again, err := ParseMOForwardShortMessageRequest(rebuilt); err != nil || !reflect.DeepEqual(again, mo) {
				t.Fatalf("OFR round trip = %+v, %v; want %+v", again, err, mo)
			}
		}

		mt, err := ParseMTForwardShortMessageRequest(m)
		if err != nil && !errors.As(err, &avpErr) {
			t.Fatalf("MT parser returned %v", err)
		}

		if err == nil {
			if _, err := NewMTForwardShortMessageRequest(tfrEnvelope, mt); err != nil {
				t.Fatalf("parsed TFR %+v does not rebuild: %v", mt, err)
			}
		}
	})
}

func FuzzParseAnswers(f *testing.F) {
	req := request(CommandMTForwardShortMessage, tfrEnvelope)

	success, err := NewMTForwardShortMessageAnswer(req, mmeIdentity, []byte{1})
	if err != nil {
		f.Fatal(err)
	}

	failure, err := NewDeliveryFailureAnswer(req, mmeIdentity, CauseEquipmentProtocolError, []byte{2})
	if err != nil {
		f.Fatal(err)
	}

	seed(f, success)
	seed(f, failure)
	seed(f, NewAbsentUserAnswer(req, mmeIdentity, u32(1)))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := diameter.Unmarshal(b)
		if err != nil {
			return
		}

		_, err = ParseMTForwardShortMessageAnswer(m)

		var re *ResultError
		if err != nil && !errors.As(err, &re) && !errors.Is(err, ErrMalformedAnswer) {
			t.Fatalf("answer parser returned %v", err)
		}
	})
}
