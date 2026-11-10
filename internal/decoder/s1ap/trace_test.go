// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"slices"
	"testing"

	lib "github.com/ellanetworks/core/s1ap"
)

func TestDecodeS1APMessage_TraceStart(t *testing.T) {
	raw, _ := hex.DecodeString("001b4023000003000000020001000800020002001940100002f8390a0b0c1234f800f80a000001")

	msg := DecodeS1APMessage(raw)
	if msg.Value.Error != "" {
		t.Fatalf("decode error %q", msg.Value.Error)
	}

	if len(msg.Value.IEs) != 3 {
		t.Fatalf("decoded %+v", msg)
	}

	ta, ok := msg.Value.IEs[2].Value.(TraceActivation)
	if !ok {
		t.Fatalf("Trace Activation rendered as %T", msg.Value.IEs[2].Value)
	}

	if ta.EUTRANTraceID != "02f8390a0b0c1234" || ta.TraceDepth != "minimum" || ta.TraceCollectionEntityIPAddress != "10.0.0.1" ||
		!slices.Equal(ta.InterfacesToTrace, []string{"S1-MME", "X2", "Uu", "F1-C", "E1"}) {
		t.Fatalf("Trace Activation %+v", ta)
	}
}

func TestDecodeS1APMessage_TraceFailureIndication(t *testing.T) {
	id := lib.EUTRANTraceID{0x02, 0xf8, 0x39, 0x0a, 0x0b, 0x0c, 0x12, 0x34}
	cause := lib.Cause{Group: lib.CauseGroupRadioNetwork, Value: 5}

	raw, err := (&lib.TraceFailureIndication{MMEUES1APID: 1, ENBUES1APID: 2, EUTRANTraceID: &id, Cause: &cause}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	msg := DecodeS1APMessage(raw)
	if msg.Value.Error != "" {
		t.Fatalf("decode error %q", msg.Value.Error)
	}

	if len(msg.Value.IEs) != 4 || msg.Value.IEs[2].Value != "02f8390a0b0c1234" {
		t.Fatalf("decoded %+v", msg.Value.IEs)
	}
}
