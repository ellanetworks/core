// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/nas/nastest"
)

// roundTrip asserts that a built message encodes, that its encoding decodes, and
// that encoding it again yields the same octets. This test package is external —
// nastest imports fgs, so the in-package helper of the same name is out of reach —
// and value identity is left to the in-package targets, which can normalize the
// order encoding gives preserved elements before comparing.
func roundTrip[T interface{ MarshalBinary() ([]byte, error) }](
	t *testing.T, name string, parse func([]byte) (T, error), b []byte,
) {
	t.Helper()

	msg, err := parse(b)
	if err != nil && !nas.SoftOnly(err) {
		return
	}

	raw, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("%s: decoded % x but would not encode: %v", name, b, err)
	}

	again, err := parse(raw)
	if err != nil && !nas.SoftOnly(err) {
		t.Fatalf("%s: own encoding % x did not decode: %v (from % x)", name, raw, err, b)
	}

	stable, err := again.MarshalBinary()
	if err != nil {
		t.Fatalf("%s: re-encode failed: %v", name, err)
	}

	if !bytes.Equal(stable, raw) {
		t.Fatalf("%s: encoding is not idempotent\n first % x\nsecond % x", name, raw, stable)
	}
}

// FuzzBuildRegistrationRequest is structure-aware: it frames a valid 5GMM header,
// registration-type octet, and LV-E mobile identity, then appends fuzz-chosen bytes
// as the optional-IE part. This drives the IE table and optional-IE walker far
// deeper than random input, and asserts the round-trip properties hold for the result.
func FuzzBuildRegistrationRequest(f *testing.F) {
	f.Add([]byte{0xf2, 0x00, 0xf1, 0x10, 0x01}, []byte{0x2e, 0x02, 0xe0, 0xe0})
	f.Add([]byte{}, []byte{0x10, 0x01, 0xff})
	f.Add([]byte{0x01}, []byte{})

	f.Fuzz(func(t *testing.T, mobileID, optionalIEs []byte) {
		msg := nastest.BuildGMM(fgs.MsgRegistrationRequest).U8(0x01).LVE(mobileID).Raw(optionalIEs...).Bytes()
		roundTrip(t, "RegistrationRequest", fgs.ParseRegistrationRequest, msg)
	})
}

// FuzzBuildULNASTransport frames a valid header and LV-E payload container, then
// fuzzes the optional-IE part (PDU session id, request type, S-NSSAI, DNN, …),
// asserting the round-trip properties hold for the result.
func FuzzBuildULNASTransport(f *testing.F) {
	f.Add([]byte{0x2e, 0x01}, []byte{0x12, 0x05, 0x81, 0x22, 0x01, 0x01})
	f.Add([]byte{}, []byte{0x25, 0x08, 0x08, 'i', 'n', 't', 'e', 'r', 'n', 'e', 't'})

	f.Fuzz(func(t *testing.T, payloadContainer, optionalIEs []byte) {
		msg := nastest.BuildGMM(fgs.MsgULNASTransport).U8(0x01).LVE(payloadContainer).Raw(optionalIEs...).Bytes()
		roundTrip(t, "ULNASTransport", fgs.ParseULNASTransport, msg)
	})
}

// FuzzMarshalQoS drives the encoders from field values rather than from wire
// bytes, which is the direction every other target reaches only through a
// successful parse. A count or length field narrower than its Go field is the
// defect class it looks for: the encoder must report the value it cannot frame,
// never truncate it into a message that re-decodes as something else.
func FuzzMarshalQoS(f *testing.F) {
	f.Add(uint8(1), uint8(1), 1, 4, uint8(1), 1, 4)
	f.Add(uint8(1), uint8(1), 20, 300, uint8(1), 70, 8)
	f.Add(uint8(0), uint8(0), 0, 0, uint8(0), 0, 0)

	f.Fuzz(func(t *testing.T, qri, ruleOp uint8, filters, filterLen int, qfi uint8, params, paramLen int) {
		// Bound the shapes so a case stays a message rather than an allocation
		// test; the encoders' own limits are well inside these.
		filters, params = clamp(filters, 64), clamp(params, 96)
		filterLen, paramLen = clamp(filterLen, 512), clamp(paramLen, 512)

		rule := fgs.QoSRule{Identifier: qri, OperationCode: fgs.QoSRuleOperation(ruleOp & 0x07)}
		for range filters {
			// 0x30 is the protocol identifier component, whose value is one octet.
			rule.Filters = append(rule.Filters, fgs.PacketFilter{
				Identifier: 1,
				Direction:  fgs.PacketFilterBidirectional,
				Components: []fgs.PacketFilterComponent{{Type: 0x30, Value: []byte{uint8(filterLen)}}},
			})
		}

		flow := fgs.QoSFlowDescription{QFI: qfi, OperationCode: fgs.QoSFlowOpCreate}
		for range params {
			flow.Parameters = append(flow.Parameters, fgs.QoSFlowParameter{Value: make([]byte, paramLen)})
		}

		roundTripEncoded(t, "QoSRules", fgs.QoSRules{rule}, fgs.ParseQoSRules)
		roundTripEncoded(t, "QoSFlowDescriptions", fgs.QoSFlowDescriptions{flow}, fgs.ParseQoSFlowDescriptions)
	})
}

func clamp(v, limit int) int {
	if v < 0 {
		v = -v
	}

	if v > limit {
		return limit
	}

	return v
}

// roundTripEncoded asserts that a value either refuses to encode or encodes to
// octets that decode back to it. Silently emitting a truncated field is the
// failure it rules out.
func roundTripEncoded[T interface{ MarshalBinary() ([]byte, error) }](
	t *testing.T, name string, value T, parse func([]byte) (T, error),
) {
	t.Helper()

	raw, err := value.MarshalBinary()
	if err != nil {
		return
	}

	again, err := parse(raw)
	if err != nil && !nas.SoftOnly(err) {
		t.Fatalf("%s: encoded % x but would not decode: %v", name, raw, err)
	}

	stable, err := again.MarshalBinary()
	if err != nil {
		t.Fatalf("%s: re-encode failed: %v", name, err)
	}

	if !bytes.Equal(stable, raw) {
		t.Fatalf("%s: encoding is not idempotent\n first % x\nsecond % x", name, raw, stable)
	}
}
