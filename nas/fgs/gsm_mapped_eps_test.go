// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"reflect"
	"testing"
)

// mappedEPSGolden is one "create new EPS bearer" context for EBI 5 laid out from
// TS 24.501 figures 9.11.4.8.2 and 9.11.4.8.3:
//
//	50            EPS bearer identity 5 in bits 5-8, bits 1-4 spare
//	00 0a         length of the context, covering octet 7 onwards
//	52            operation code 01 (create), E bit set, 2 EPS parameters
//	01 03 0b 0c 0d   mapped EPS QoS parameters, 3 octets
//	04 02 aa bb      APN-AMBR, 2 octets
//
// The two-octet length is what makes this element type 6 rather than type 4, and
// it counts from the operation octet, not from the identity.
var (
	mappedEPSGolden = []byte{
		0x50, 0x00, 0x0a, 0x52,
		0x01, 0x03, 0x0b, 0x0c, 0x0d,
		0x04, 0x02, 0xaa, 0xbb,
	}
	mappedEPSContexts = MappedEPSBearerContexts{{
		EPSBearerIdentity: 5,
		Operation:         MappedEPSBearerOpCreate,
		EBit:              true,
		Parameters: []EPSParameter{
			{Identifier: EPSParameterMappedEPSQoS, Contents: []byte{0x0b, 0x0c, 0x0d}},
			{Identifier: EPSParameterAPNAMBR, Contents: []byte{0xaa, 0xbb}},
		},
	}}
)

func TestMappedEPSBearerContextsWire(t *testing.T) {
	raw, err := mappedEPSContexts.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(raw, mappedEPSGolden) {
		t.Fatalf("contexts = % x, want % x", raw, mappedEPSGolden)
	}

	back, err := ParseMappedEPSBearerContexts(mappedEPSGolden)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(back, mappedEPSContexts) {
		t.Fatalf("round trip = %+v, want %+v", back, mappedEPSContexts)
	}
}

// TestMappedEPSBearerContextsMinimumLength pins the 7-octet minimum TS 24.501
// §9.11.4.8 gives the whole element: an IEI, a two-octet length, and a context
// that is an identity, its own two-octet length and the operation octet. So the
// shortest value part this codec produces is four octets.
func TestMappedEPSBearerContextsMinimumLength(t *testing.T) {
	raw, err := MappedEPSBearerContexts{{
		EPSBearerIdentity: 15,
		Operation:         MappedEPSBearerOpDelete,
	}}.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if want := []byte{0xf0, 0x00, 0x01, 0x80}; !bytes.Equal(raw, want) {
		t.Fatalf("delete context = % x, want % x", raw, want)
	}

	back, err := ParseMappedEPSBearerContexts(raw)
	if err != nil {
		t.Fatal(err)
	}

	if len(back) != 1 || back[0].Operation != MappedEPSBearerOpDelete || back[0].EPSBearerIdentity != 15 {
		t.Fatalf("round trip = %+v", back)
	}

	if back[0].Parameters != nil {
		t.Errorf("parameters = %+v, want none", back[0].Parameters)
	}
}

// TestMappedEPSBearerContextsSeveral checks that the contexts of one IE are read
// back-to-back, each delimited by its own length rather than by the IE's.
func TestMappedEPSBearerContextsSeveral(t *testing.T) {
	want := MappedEPSBearerContexts{
		{EPSBearerIdentity: 5, Operation: MappedEPSBearerOpCreate, EBit: true, Parameters: []EPSParameter{
			{Identifier: EPSParameterAPNAMBR, Contents: []byte{0x01}},
		}},
		{EPSBearerIdentity: 6, Operation: MappedEPSBearerOpModify},
		{EPSBearerIdentity: 7, Operation: MappedEPSBearerOpDelete},
	}

	raw, err := want.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseMappedEPSBearerContexts(raw)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestMappedEPSBearerContextsRefusesMalformed(t *testing.T) {
	tests := []struct {
		name string
		b    []byte
	}{
		{"truncated length", []byte{0x50, 0x00}},
		{"length past the end", []byte{0x50, 0x00, 0x09, 0x52}},
		{"empty context", []byte{0x50, 0x00, 0x00}},
		{"truncated parameter contents", []byte{0x50, 0x00, 0x04, 0x51, 0x01, 0x03, 0x0b}},
		// The count claims one parameter and the length covers two, so the
		// element would re-encode shorter than it arrived.
		{"count below the length", []byte{0x50, 0x00, 0x07, 0x51, 0x04, 0x01, 0x01, 0x04, 0x01, 0x02}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseMappedEPSBearerContexts(tc.b); err == nil {
				t.Error("want an error, got none")
			}
		})
	}
}

func TestMappedEPSBearerContextsRefusesUnencodableValues(t *testing.T) {
	tests := []struct {
		name string
		c    MappedEPSBearerContext
	}{
		{
			"EPS bearer identity wider than four bits",
			MappedEPSBearerContext{EPSBearerIdentity: 16, Operation: MappedEPSBearerOpDelete},
		},
		{
			"more parameters than the count field holds",
			MappedEPSBearerContext{
				EPSBearerIdentity: 5,
				Operation:         MappedEPSBearerOpCreate,
				Parameters:        make([]EPSParameter, maxEPSParameters+1),
			},
		},
		{
			"parameter contents longer than its length field",
			MappedEPSBearerContext{
				EPSBearerIdentity: 5,
				Operation:         MappedEPSBearerOpCreate,
				Parameters:        []EPSParameter{{Identifier: EPSParameterTrafficFlowTemplate, Contents: make([]byte, 256)}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := (MappedEPSBearerContexts{tc.c}).MarshalBinary(); err == nil {
				t.Error("want an error, got none")
			}
		})
	}
}

// TestMappedEPSBearerContextsInMessages checks the element reaches the three
// messages TS 24.501 tables 8.3.2.1.1, 8.3.7.1.1 and 8.3.9.1.1 carry it in,
// rather than falling through to Unrecognized.
func TestMappedEPSBearerContextsInMessages(t *testing.T) {
	t.Run("PDUSessionEstablishmentAccept", func(t *testing.T) {
		m := &PDUSessionEstablishmentAccept{
			PDUSessionID:            1,
			QoSRules:                QoSRules{DefaultQoSRule(1, 1)},
			SessionAMBR:             SessionAMBR{DownlinkUnit: 6, Downlink: 1, UplinkUnit: 6, Uplink: 1},
			MappedEPSBearerContexts: mappedEPSContexts,
		}

		raw, err := m.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		back, err := ParsePDUSessionEstablishmentAccept(raw)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(back.MappedEPSBearerContexts, mappedEPSContexts) {
			t.Fatalf("contexts = %+v, want %+v", back.MappedEPSBearerContexts, mappedEPSContexts)
		}

		if len(back.Unrecognized) != 0 {
			t.Errorf("unrecognized = %+v, want none", back.Unrecognized)
		}
	})

	t.Run("PDUSessionModificationCommand", func(t *testing.T) {
		m := &PDUSessionModificationCommand{PDUSessionID: 1, MappedEPSBearerContexts: mappedEPSContexts}

		raw, err := m.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		back, err := ParsePDUSessionModificationCommand(raw)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(back.MappedEPSBearerContexts, mappedEPSContexts) {
			t.Fatalf("contexts = %+v, want %+v", back.MappedEPSBearerContexts, mappedEPSContexts)
		}

		if len(back.Unrecognized) != 0 {
			t.Errorf("unrecognized = %+v, want none", back.Unrecognized)
		}
	})

	t.Run("PDUSessionModificationRequest", func(t *testing.T) {
		m := &PDUSessionModificationRequest{PDUSessionID: 1, MappedEPSBearerContexts: mappedEPSContexts}

		raw, err := m.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		back, err := ParsePDUSessionModificationRequest(raw)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(back.MappedEPSBearerContexts, mappedEPSContexts) {
			t.Fatalf("contexts = %+v, want %+v", back.MappedEPSBearerContexts, mappedEPSContexts)
		}

		if len(back.Unrecognized) != 0 {
			t.Errorf("unrecognized = %+v, want none", back.Unrecognized)
		}
	})
}

// TestMappedEPSBearerContextsMaximumLength pins the 65538-octet cap TS 24.501
// §9.11.4.8 gives the element: an IEI, a two-octet length and 65535 octets of
// value. A value past that has to refuse rather than truncate, since the
// two-octet length would wrap and the UE would read a context boundary that is
// not there.
func TestMappedEPSBearerContextsMaximumLength(t *testing.T) {
	// One context whose parameters overflow the element's own length field.
	contexts := MappedEPSBearerContexts{{
		EPSBearerIdentity: 5,
		Operation:         MappedEPSBearerOpCreate,
		EBit:              true,
		Parameters: []EPSParameter{
			{Identifier: EPSParameterTrafficFlowTemplate, Contents: make([]byte, 255)},
		},
	}}

	// 15 parameters of 255 octets each is 15*(1+1+255) = 3855 octets, well
	// inside the element; the cap is reached by repeating whole contexts.
	for len(contexts[0].Parameters) < maxEPSParameters {
		contexts[0].Parameters = append(contexts[0].Parameters, contexts[0].Parameters[0])
	}

	for len(contexts) < 20 {
		contexts = append(contexts, contexts[0])
	}

	raw, err := contexts.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if len(raw) <= 0xFFFF {
		t.Fatalf("built %d octets, want more than the length field holds", len(raw))
	}

	// The element value itself encodes; the framing is what refuses it, because
	// the TLV-E length is two octets.
	m := &PDUSessionEstablishmentAccept{
		PDUSessionID:            1,
		QoSRules:                QoSRules{DefaultQoSRule(1, 1)},
		SessionAMBR:             SessionAMBR{DownlinkUnit: 6, Downlink: 1, UplinkUnit: 6, Uplink: 1},
		MappedEPSBearerContexts: contexts,
	}

	if _, err := m.MarshalBinary(); err == nil {
		t.Fatal("an oversized Mapped EPS bearer contexts encoded, want an error")
	}
}
