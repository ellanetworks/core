// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/per"
)

func fuzzLPPSeeds(f *testing.F) [][]byte {
	f.Helper()

	raw, err := os.ReadFile("lpptype/testdata/vectors.json")
	if err != nil {
		f.Fatalf("read vectors: %v", err)
	}

	var file struct {
		Vectors []struct {
			Hex string `json:"hex"`
		} `json:"vectors"`
	}

	if err := json.Unmarshal(raw, &file); err != nil {
		f.Fatalf("parse vectors: %v", err)
	}

	var seeds [][]byte

	for _, v := range file.Vectors {
		b, err := hex.DecodeString(v.Hex)
		if err != nil {
			f.Fatalf("hex: %v", err)
		}

		seeds = append(seeds, b)
	}

	add := func(b []byte, err error) {
		if err != nil {
			f.Fatalf("encode seed: %v", err)
		}

		seeds = append(seeds, b)
	}

	add(BuildAcknowledgement(3, 2))
	add(BuildRequestCapabilities(1, 0))
	add(EncodeRequestLocationInformation(2, 1, 8))
	add(EncodeProvideCapabilities(6, []lpptype.GNSSIDValue{lpptype.GNSSIDGPS}))
	add(EncodeAssistanceDataNotSupported(4, 5))
	add(EncodeProvideLocationInformation(9, 1000, -2000, 30, 10, 20))
	add(EncodeError(7, &DecodedMessage{TransactionID: 4, TransactionIDPresent: true, Initiator: lpptype.InitiatorTargetDevice}, lpptype.ErrorCauseLPPMessageBodyError))
	add(EncodeError(7, nil, lpptype.ErrorCauseLPPMessageHeaderError))
	add(encodeLPPMessage(4, lpptype.InitiatorTargetDevice, &lpptype.LPPMessageBody{C1: &lpptype.LPPMessageBodyC1{Abort: &lpptype.Abort{CriticalExtensions: lpptype.AbortCriticalExtensions{
		C1: &lpptype.AbortCriticalExtensionsC1{AbortR9: &lpptype.AbortR9IEs{
			CommonIEsAbort: &lpptype.CommonIEsAbort{AbortCause: lpptype.AbortCauseTargetDeviceAbort},
		}},
	}}}}, true, 1))
	add(encodeLPPMessage(5, lpptype.InitiatorLocationServer, &lpptype.LPPMessageBody{C1: &lpptype.LPPMessageBodyC1{Error: &lpptype.Error{ErrorR9: &lpptype.ErrorR9IEs{
		CommonIEsError: &lpptype.CommonIEsError{ErrorCause: lpptype.ErrorCauseLPPSegmentationError},
	}}}}, false, 255))

	for _, body := range []*lpptype.LPPMessageBody{
		{C1: &lpptype.LPPMessageBodyC1{Spare7: &per.Null{}}},
		{C1: &lpptype.LPPMessageBodyC1{Spare0: &per.Null{}}},
		{MessageClassExtension: &per.Null{}},
		nil,
	} {
		add(Encoder(&lpptype.LPPMessage{EndTransaction: true, LPPMessageBody: body}))
	}

	return seeds
}

func FuzzDecodeLPPMessage(f *testing.F) {
	for _, seed := range fuzzLPPSeeds(f) {
		f.Add(seed)

		if len(seed) > 1 {
			f.Add(seed[:len(seed)-1])
		}
	}

	f.Add([]byte{})
	f.Add([]byte{0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		full, fullErr := DecodeLPPMessage(data)
		header, headerErr := DecodeLPPHeader(data)

		if fullErr == nil {
			if _, err := full.Payload(); err == nil && full.BodyKind == lpptype.LPPMessageBodyC1PresentNothing {
				t.Fatal("Payload() succeeded for a message without a body")
			}

			if headerErr != nil {
				t.Fatalf("full decode succeeded but header decode failed: %v", headerErr)
			}
		}

		if headerErr == nil {
			if _, err := header.Payload(); err == nil && header.BodyKind == lpptype.LPPMessageBodyC1PresentNothing {
				t.Fatal("header Payload() succeeded for a message without a body")
			}
		}

		if fullErr != nil || headerErr != nil {
			return
		}

		wantKind := header.BodyKind
		if wantKind >= lpptype.LPPMessageBodyC1PresentSpare7 {
			wantKind = lpptype.LPPMessageBodyC1PresentNothing
		}

		if full.BodyKind != wantKind {
			t.Fatalf("body kind: full %d, header %d", full.BodyKind, header.BodyKind)
		}

		if full.TransactionIDPresent != header.TransactionIDPresent || full.TransactionID != header.TransactionID || full.Initiator != header.Initiator {
			t.Fatalf("transaction ID: full %t/%d/%d, header %t/%d/%d",
				full.TransactionIDPresent, full.Initiator, full.TransactionID,
				header.TransactionIDPresent, header.Initiator, header.TransactionID)
		}

		if full.EndTransaction != header.EndTransaction || full.AckRequested != header.AckRequested {
			t.Fatal("endTransaction or ackRequested disagree")
		}

		if (full.SequenceNumber == nil) != (header.SequenceNumber == nil) ||
			(full.SequenceNumber != nil && *full.SequenceNumber != *header.SequenceNumber) {
			t.Fatal("sequence numbers disagree")
		}
	})
}
