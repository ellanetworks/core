// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6c

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func fuzzSeeds(f *testing.F, msgs ...*diameter.Message) {
	f.Helper()

	for _, m := range msgs {
		b, err := m.Marshal()
		if err != nil {
			f.Fatal(err)
		}

		f.Add(b)
	}

	f.Add([]byte{})
}

func seeder(f *testing.F) func(*diameter.Message, error) *diameter.Message {
	return func(m *diameter.Message, err error) *diameter.Message {
		f.Helper()

		if err != nil {
			f.Fatal(err)
		}

		return m
	}
}

func requireAVPError(t *testing.T, err error) {
	t.Helper()

	var avpErr *diameter.AVPError
	if err != nil && !errors.As(err, &avpErr) {
		t.Fatalf("request parser returned %v, want an AVP error", err)
	}
}

func FuzzParseRequests(f *testing.F) {
	mustMessage := seeder(f)

	srr := mustMessage(NewSendRoutingInfoForSMRequest(testEnvelope, RoutingRequest{
		MSISDN: "15551230002", ServiceCentreAddress: testServiceCentreAddress, GPRSIndicator: true, SMSFSupport: true,
	}))
	rdr := mustMessage(NewReportSMDeliveryStatusRequest(testEnvelope, DeliveryReport{
		MSISDN: "15551230002", ServiceCentreAddress: testServiceCentreAddress, SMSFSupport: true,
		MME:    &DeliveryOutcome{Cause: DeliveryCauseAbsentUser, AbsentDiagnostic: absent(1)},
		Failed: ServingNodes{Serving: mmeNode(), SMSF3GPP: smsfNode()},
	}))
	alr := mustMessage(NewMMEAlertServiceCentreRequest(hssEnvelope, Alert{
		ServiceCentreAddress: testServiceCentreAddress, User: tgpp.UserIdentifier{IMSI: "001010000000001"},
		Event: AlertEventUEUnderNewNode, ServingNode: mmeNode(),
	}))

	fuzzSeeds(f, srr, rdr, alr)

	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := diameter.Unmarshal(b)
		if err != nil {
			return
		}

		r, err := ParseSendRoutingInfoForSMRequest(m)
		requireAVPError(t, err)

		if err == nil {
			if _, err := NewSendRoutingInfoForSMRequest(testEnvelope, r); err != nil {
				t.Fatalf("parsed SRR %+v does not rebuild: %v", r, err)
			}
		}

		_, err = ParseReportSMDeliveryStatusRequest(m)
		requireAVPError(t, err)

		_, err = ParseAlertServiceCentreRequest(m)
		requireAVPError(t, err)
	})
}

func FuzzParseAnswers(f *testing.F) {
	mustMessage := seeder(f)
	req := request(CommandSendRoutingInfoForSM)

	sra := mustMessage(NewSendRoutingInfoForSMAnswer(req, hssIdentity, Routing{
		ServingNodes: ServingNodes{Serving: mmeNode(), SMSF3GPP: smsfNode()},
		IMSI:         "001010000000001", MWDStatus: MWDStatusMNRF, AlertMSISDN: "15559999000",
	}, true))
	sraError := mustMessage(NewSendRoutingInfoForSMErrorAnswer(req, hssIdentity, ResultError{
		Result: tgpp.Experimental(tgpp.ResultErrorAbsentUser), MWDStatus: MWDStatusMNRF, Absent: AbsentUserDiagnostics{MME: absent(1)},
	}, true))
	rda := mustMessage(NewReportSMDeliveryStatusAnswer(req, hssIdentity, ReportResult{
		ServingNodes: ServingNodes{Serving: mmeNode()}, AlertMSISDN: "15559999000",
	}, true))

	fuzzSeeds(f, sra, sraError, rda)

	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := diameter.Unmarshal(b)
		if err != nil {
			return
		}

		checkAnswerError := func(err error) {
			var re *ResultError
			if err != nil && !errors.As(err, &re) && !errors.Is(err, ErrMalformedAnswer) {
				t.Fatalf("answer parser returned %v", err)
			}
		}

		_, err = ParseSendRoutingInfoForSMAnswer(m)
		checkAnswerError(err)

		_, err = ParseReportSMDeliveryStatusAnswer(m)
		checkAnswerError(err)

		checkAnswerError(ParseAlertServiceCentreAnswer(m))
	})
}
