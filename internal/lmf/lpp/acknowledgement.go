// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"fmt"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
)

// BuildAcknowledgement encodes a body-less LPP message acknowledging the message
// with the given sequence number (TS 37.355 §4.3.3): ackIndicator is set to the
// acknowledged sequence number, ackRequested is false so the acknowledgement is
// not itself acknowledged, and sequenceNumber carries this sender's own counter
// as §4.3.2 requires on every message.
func BuildAcknowledgement(sequenceNumber, ackedSequenceNumber byte) ([]byte, error) {
	seq := int64(sequenceNumber)
	ind := int64(ackedSequenceNumber)

	msg := &lpptype.LPPMessage{
		EndTransaction: false,
		SequenceNumber: &seq,
		Acknowledgement: &lpptype.Acknowledgement{
			AckRequested: false,
			AckIndicator: &ind,
		},
	}

	b, err := Encoder(msg)
	if err != nil {
		return nil, fmt.Errorf("encode acknowledgement: %w", err)
	}

	return b, nil
}
