// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	faper "github.com/free5gc/aper"
)

// TestEncodeRequestCapabilitiesIsUnaligned pins the exact octets the LMF opens
// an A-GNSS session with. Under the aligned variant the same message encodes
// with padding a handset rejects, returning errorCause lppMessageBodyError
// (see TestDecodeErrorFromUE).
func TestEncodeRequestCapabilitiesIsUnaligned(t *testing.T) {
	got, err := EncodeRequestCapabilities(0, 0)
	if err != nil {
		t.Fatalf("EncodeRequestCapabilities: %v", err)
	}

	const want = "d000000021c0"

	if hex.EncodeToString(got) != want {
		t.Errorf("RequestCapabilities: got %s, want %s", hex.EncodeToString(got), want)
	}
}

// TestRequestsCarrySequenceNumber guards TS 37.355 §4.3.2: a sender shall
// include a sequence number in all LPP messages sent for a location session,
// and consecutive messages must differ or the peer discards the second as a
// duplicate.
func TestRequestsCarrySequenceNumber(t *testing.T) {
	for seq := byte(0); seq < 3; seq++ {
		b, err := EncodeRequestCapabilities(0, seq)
		if err != nil {
			t.Fatalf("EncodeRequestCapabilities: %v", err)
		}

		msg, err := DecodeMessage(b)
		if err != nil {
			t.Fatalf("DecodeMessage: %v", err)
		}

		if msg.SequenceNumber == nil {
			t.Fatalf("seq %d: sequenceNumber absent, want present", seq)
		}

		if *msg.SequenceNumber != int64(seq) {
			t.Errorf("sequenceNumber: got %d, want %d", *msg.SequenceNumber, seq)
		}
	}
}

// TestRequestsDoNotEndTransaction guards TS 37.355 §4.2: the last message
// carrying a body in a transaction sets endTransaction, and a request is never
// last. A request that sets it tells the target the exchange is over, and a
// conformant target answers nothing.
func TestRequestsDoNotEndTransaction(t *testing.T) {
	for _, tc := range []struct {
		name   string
		encode func(byte, byte) ([]byte, error)
	}{
		{"RequestCapabilities", EncodeRequestCapabilities},
		{"RequestLocationInformation", EncodeRequestLocationInformation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := tc.encode(0, 0)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}

			msg, err := DecodeMessage(b)
			if err != nil {
				t.Fatalf("DecodeMessage: %v", err)
			}

			if msg.EndTransaction {
				t.Error("endTransaction: got true, want false")
			}
		})
	}
}

// TestEncodeMessageRoundTripAcknowledgement round-trips a body-less LPP message
// carrying only an acknowledgement. TS 37.355 §6.2 describes exactly this shape
// ("an LPP message sent only to acknowledge a previously received message"),
// and it is what the LMF owes a peer that sets ackRequested.
func TestEncodeMessageRoundTripAcknowledgement(t *testing.T) {
	seq := int64(7)
	ind := int64(3)

	msg := &lpptype.LPPMessage{
		EndTransaction:  false,
		SequenceNumber:  &seq,
		Acknowledgement: &lpptype.Acknowledgement{AckRequested: false, AckIndicator: &ind},
	}

	b, err := EncodeMessage(msg)
	if err != nil {
		t.Fatalf("EncodeMessage: %v", err)
	}

	got, err := DecodeMessage(b)
	if err != nil {
		t.Fatalf("DecodeMessage: %v", err)
	}

	if got.TransactionID != nil {
		t.Errorf("transactionID: got %+v, want nil", got.TransactionID)
	}

	if got.EndTransaction {
		t.Error("endTransaction: got true, want false")
	}

	if got.SequenceNumber == nil || *got.SequenceNumber != seq {
		t.Errorf("sequenceNumber: got %v, want %d", got.SequenceNumber, seq)
	}

	if got.Acknowledgement == nil || got.Acknowledgement.AckRequested ||
		got.Acknowledgement.AckIndicator == nil || *got.Acknowledgement.AckIndicator != ind {
		t.Errorf("acknowledgement: got %+v, want {false, %d}", got.Acknowledgement, ind)
	}

	if got.LppMessageBody != nil {
		t.Errorf("lpp-MessageBody: got %+v, want nil", got.LppMessageBody)
	}
}

// TestDecodeErrorFromUE decodes an LPP Error captured from a commercial
// handset. It guards the whole unaligned envelope against a real peer: the
// aligned codec fails these octets with "sequence truncated".
func TestDecodeErrorFromUE(t *testing.T) {
	raw, err := hex.DecodeString("f001004e48")
	if err != nil {
		t.Fatalf("hex: %v", err)
	}

	msg, err := DecodeMessage(raw)
	if err != nil {
		t.Fatalf("DecodeMessage: %v", err)
	}

	// TS 37.355 §4.2: every message in a transaction carries the transaction's
	// ID, so this has to be the 0 the LMF opened with. Reading endTransaction
	// ahead of transactionID also decodes, but yields 1 here.
	if msg.TransactionID == nil || msg.TransactionID.TransactionNumber != 0 ||
		msg.TransactionID.Initiator.Value != lpptype.InitiatorLocationServer {
		t.Errorf("transactionID: got %+v, want {locationServer, 0}", msg.TransactionID)
	}

	if !msg.EndTransaction {
		t.Error("endTransaction: got false, want true")
	}

	if msg.SequenceNumber == nil || *msg.SequenceNumber != 0 {
		t.Errorf("sequenceNumber: got %v, want 0", msg.SequenceNumber)
	}

	if msg.Acknowledgement == nil || !msg.Acknowledgement.AckRequested {
		t.Fatalf("acknowledgement: got %+v, want ackRequested=true", msg.Acknowledgement)
	}

	if msg.LppMessageBody == nil || msg.LppMessageBody.C1 == nil ||
		msg.LppMessageBody.C1.Present != lpptype.LPPMessageBodyC1PresentError {
		t.Fatalf("body: got %+v, want an Error", msg.LppMessageBody)
	}

	cause := msg.LppMessageBody.C1.Error.ErrorR9.CommonIEsError.ErrorCause.Value
	if cause != lpptype.CommonIEsErrorErrorCausePresentLPPMessageBodyError {
		t.Errorf("errorCause: got %d, want %d (lppMessageBodyError)",
			cause, lpptype.CommonIEsErrorErrorCausePresentLPPMessageBodyError)
	}
}

// TestDecodeAbortFromUE decodes the Abort a commercial handset returned to the
// capabilities request pinned by TestEncodeRequestCapabilitiesIsUnaligned. The
// abortCause is the only account the target gives of why it stopped, so the
// decode has to reach it.
func TestDecodeAbortFromUE(t *testing.T) {
	raw, err := hex.DecodeString("d001003040")
	if err != nil {
		t.Fatalf("hex: %v", err)
	}

	msg, err := DecodeMessage(raw)
	if err != nil {
		t.Fatalf("DecodeMessage: %v", err)
	}

	if msg.TransactionID == nil || msg.TransactionID.TransactionNumber != 0 ||
		msg.TransactionID.Initiator.Value != lpptype.InitiatorLocationServer {
		t.Errorf("transactionID: got %+v, want {locationServer, 0}", msg.TransactionID)
	}

	if !msg.EndTransaction {
		t.Error("endTransaction: got false, want true")
	}

	if msg.SequenceNumber == nil || *msg.SequenceNumber != 0 {
		t.Errorf("sequenceNumber: got %v, want 0", msg.SequenceNumber)
	}

	if msg.LppMessageBody == nil || msg.LppMessageBody.C1 == nil ||
		msg.LppMessageBody.C1.Present != lpptype.LPPMessageBodyC1PresentAbort {
		t.Fatalf("body: got %+v, want an Abort", msg.LppMessageBody)
	}

	cause := msg.LppMessageBody.C1.Abort.CriticalExtensions.C1.AbortR9.CommonIEsAbort.AbortCause.Value
	if cause != lpptype.CommonIEsAbortCausePresentUndefined {
		t.Errorf("abortCause: got %s, want undefined", lpptype.AbortCauseString(cause))
	}
}

func gnssCapabilities(ids ...int) *lpptype.LPPMessage {
	els := make([]lpptype.GNSSSupportElement, 0, len(ids))
	for _, id := range ids {
		els = append(els, lpptype.GNSSSupportElement{
			GnssID:      lpptype.GNSSID{Value: faper.Enumerated(id)},
			AGNSSModes:  lpptype.PositioningModes{PosModes: faper.BitString{Bytes: []byte{0xA0}, BitLength: 3}},
			GnssSignals: lpptype.GNSSSignalIDs{GnssSignalIDs: faper.BitString{Bytes: []byte{0x80}, BitLength: 8}},
		})
	}

	return &lpptype.LPPMessage{
		TransactionID:  &lpptype.LPPTransactionID{Initiator: lpptype.Initiator{Value: lpptype.InitiatorLocationServer}},
		EndTransaction: false,
		LppMessageBody: &lpptype.LPPMessageBody{Present: 1, C1: &lpptype.LPPMessageBodyC1{
			Present: lpptype.LPPMessageBodyC1PresentProvideCapabilities,
			ProvideCapabilities: &lpptype.ProvideCapabilities{
				CriticalExtensions: lpptype.ProvideCapabilitiesCriticalExtensions{
					Present: 1,
					C1: &lpptype.ProvideCapabilitiesCriticalExtensionsC1{
						Present: 1,
						ProvideCapabilitiesR9: &lpptype.ProvideCapabilitiesR9IEs{
							AGNSSProvideCapabilities: &lpptype.AGNSSProvideCapabilities{
								GnssSupportList: &lpptype.GNSSSupportList{List: els},
							},
						},
					},
				},
			},
		}},
	}
}

// TestProvideCapabilitiesRoundTrip covers the UE's half of the capabilities
// exchange: gnss-SupportList is the one field the session acts on.
func TestProvideCapabilitiesRoundTrip(t *testing.T) {
	want := []int{0, 3, 4} // gps, galileo, glonass

	b, err := EncodeMessage(gnssCapabilities(want...))
	if err != nil {
		t.Fatalf("EncodeMessage: %v", err)
	}

	got, err := DecodeMessage(b)
	if err != nil {
		t.Fatalf("DecodeMessage: %v", err)
	}

	list := got.LppMessageBody.C1.ProvideCapabilities.CriticalExtensions.C1.
		ProvideCapabilitiesR9.AGNSSProvideCapabilities.GnssSupportList
	if len(list.List) != len(want) {
		t.Fatalf("gnss-SupportList: got %d elements, want %d", len(list.List), len(want))
	}

	for i, id := range want {
		if int(list.List[i].GnssID.Value) != id {
			t.Errorf("element %d: got gnss-id %d, want %d", i, list.List[i].GnssID.Value, id)
		}
	}
}

// TestProvideLocationInformationRoundTrip covers the message that carries the
// A-GNSS fix, over the shape the LMF maps to a LocationResult.
func TestProvideLocationInformationRoundTrip(t *testing.T) {
	want := &lpptype.EllipsoidPointWithAltitudeAndUncertaintyEllipsoid{
		LatitudeSign: 0, DegreesLatitude: 4243456, DegreesLongitude: 5000000,
		AltitudeDirection: 0, Altitude: 42, UncertaintySemiMajor: 10,
		UncertaintySemiMinor: 8, OrientationMajorAxis: 30,
		UncertaintyAltitude: 5, Confidence: 68,
	}

	msg := &lpptype.LPPMessage{
		TransactionID:  &lpptype.LPPTransactionID{Initiator: lpptype.Initiator{Value: lpptype.InitiatorLocationServer}, TransactionNumber: 1},
		EndTransaction: true,
		LppMessageBody: &lpptype.LPPMessageBody{Present: 1, C1: &lpptype.LPPMessageBodyC1{
			Present: lpptype.LPPMessageBodyC1PresentProvideLocationInformation,
			ProvideLocationInformation: &lpptype.ProvideLocationInformation{
				CriticalExtensions: lpptype.ProvideLocationInformationCriticalExtensions{
					Present: 1,
					C1: &lpptype.ProvideLocationInformationCriticalExtensionsC1{
						Present: 1,
						ProvideLocationInformationR9: &lpptype.ProvideLocationInformationR9IEs{
							CommonIEsProvideLocationInformation: &lpptype.CommonIEsProvideLocationInformation{
								LocationEstimate: &lpptype.LocationCoordinates{
									Present: lpptype.LocationCoordinatesPresentEllipsoidPointWithAltitudeAndUncertaintyEllipsoid,
									EllipsoidPointWithAltitudeAndUncertaintyEllipsoid: want,
								},
							},
						},
					},
				},
			},
		}},
	}

	b, err := EncodeMessage(msg)
	if err != nil {
		t.Fatalf("EncodeMessage: %v", err)
	}

	got, err := DecodeMessage(b)
	if err != nil {
		t.Fatalf("DecodeMessage: %v", err)
	}

	p := got.LppMessageBody.C1.ProvideLocationInformation.CriticalExtensions.C1.
		ProvideLocationInformationR9.CommonIEsProvideLocationInformation.LocationEstimate.
		EllipsoidPointWithAltitudeAndUncertaintyEllipsoid
	if *p != *want {
		t.Errorf("locationEstimate: got %+v, want %+v", *p, *want)
	}
}
