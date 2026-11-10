// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"slices"
	"testing"

	lib "github.com/ellanetworks/core/ngap"
)

func TestDecodeNGAPMessage_TraceStart(t *testing.T) {
	raw, _ := hex.DecodeString("00274023000003000a00020001005500020002006c40100002f8390a0b0c1234f800f80a000001")

	msg := DecodeNGAPMessage(raw)
	if msg.Value.Error != "" {
		t.Fatalf("decode error %q", msg.Value.Error)
	}

	if msg.ProcedureCode.Value != int64(lib.ProcTraceStart) || len(msg.Value.IEs) != 3 {
		t.Fatalf("decoded %+v", msg)
	}

	ta, ok := msg.Value.IEs[2].Value.(TraceActivation)
	if !ok {
		t.Fatalf("Trace Activation rendered as %T", msg.Value.IEs[2].Value)
	}

	want := TraceActivation{
		NGRANTraceID:                   "02f8390a0b0c1234",
		InterfacesToTrace:              []string{"NG-C", "Xn-C", "Uu", "F1-C", "E1"},
		TraceDepth:                     "minimum",
		TraceCollectionEntityIPAddress: "10.0.0.1",
	}

	if ta.NGRANTraceID != want.NGRANTraceID || !slices.Equal(ta.InterfacesToTrace, want.InterfacesToTrace) ||
		ta.TraceDepth != want.TraceDepth || ta.TraceCollectionEntityIPAddress != want.TraceCollectionEntityIPAddress {
		t.Fatalf("Trace Activation %+v, want %+v", ta, want)
	}
}

func TestDecodeNGAPMessage_TraceFailureIndication(t *testing.T) {
	raw, _ := hex.DecodeString("00264021000004000a00020001005500020002002c400802f8390a0b0c1234000f40020140")

	msg := DecodeNGAPMessage(raw)
	if msg.Value.Error != "" {
		t.Fatalf("decode error %q", msg.Value.Error)
	}

	if msg.ProcedureCode.Value != int64(lib.ProcTraceFailureIndication) || len(msg.Value.IEs) != 4 {
		t.Fatalf("decoded %+v", msg)
	}

	if got := msg.Value.IEs[2].Value; got != "02f8390a0b0c1234" {
		t.Fatalf("NG-RAN Trace ID rendered as %v", got)
	}
}
