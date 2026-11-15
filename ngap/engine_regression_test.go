// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

func redundantTunnel(a byte, teid GTPTEID, flow QosFlowIdentifier, mapping *QosFlowMappingIndication) QosFlowPerTNLInformationItem {
	return QosFlowPerTNLInformationItem{QosFlowPerTNLInformation: QosFlowPerTNLInformation{
		UPTransportLayerInformation: UPTransportLayerInformation{GTPTunnel: &GTPTunnel{TransportLayerAddress: tlaOf(10, 0, 0, a), GTPTEID: teid}},
		AssociatedQosFlowList:       AssociatedQosFlowList{{QosFlowIdentifier: flow, QosFlowMappingIndication: mapping}},
	}}
}

func TestPartiallyNotComprehendedListKeepsEveryItemOnDecode(t *testing.T) {
	in := &PDUSessionResourceSetupResponseTransfer{
		DLQosFlowPerTNLInformation: redundantTunnel(1, 1, 1, nil).QosFlowPerTNLInformation,
		AdditionalRedundantDLQosFlowPerTNLInformation: QosFlowPerTNLInformationList{
			redundantTunnel(2, 2, 1, nil),
			redundantTunnel(3, 3, 2, Ptr(QosFlowMappingIndication(7))),
		},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodePDUSessionResourceSetupResponseTransfer(b)
	if err != nil {
		t.Fatal(err)
	}

	if got := decoded.AdditionalRedundantDLQosFlowPerTNLInformation; len(got) != 2 || got[1].QosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel.GTPTEID != 3 {
		t.Fatalf("decoded %d redundant tunnels, want both kept", len(got))
	}

	ies := decoded.Diagnostics().IEs
	if len(ies) != 1 || ies[0].ID != IDAdditionalRedundantDLQosFlowPerTNLInformation || ies[0].Kind != ap.KindNotComprehended || ies[0].Criticality != CriticalityIgnore {
		t.Fatalf("diagnostics = %v, want the ignore extension reported not comprehended", ies)
	}

	parsed, err := ParsePDUSessionResourceSetupResponseTransfer(b)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if parsed.AdditionalRedundantDLQosFlowPerTNLInformation != nil {
		t.Fatal("parse kept an extension that is partially not comprehended (TS 38.413 §10.3.2)")
	}

	if parsed.DLQosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel.GTPTEID != 1 {
		t.Fatal("parse lost the comprehended part of the transfer")
	}

	for name, v := range map[string]*PDUSessionResourceSetupResponseTransfer{"decoded": decoded, "parsed": parsed} {
		again, err := v.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(again, b) {
			t.Fatalf("%s re-encoded %x, want %x", name, again, b)
		}
	}
}

func initialUEMessageValue(t *testing.T, m *InitialUEMessage) []byte {
	t.Helper()

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	return pdu.value()
}

func TestNotComprehendedMandatoryIEIsNotAlsoMissing(t *testing.T) {
	value := initialUEMessageValue(t, &InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      NASPDU{0x7e, 0x00},
		UserLocationInformation: UserLocationInformation{NR: &UserLocationInformationNR{
			NRCGI: NRCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, NRCellIdentity: 1},
			TAI:   TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
		}},
		RRCEstablishmentCause: Ptr(RRCEstablishmentCause(40)),
	})

	decoded, err := DecodeInitialUEMessage(value)
	if err != nil {
		t.Fatal(err)
	}

	if decoded.RRCEstablishmentCause == nil || *decoded.RRCEstablishmentCause != 40 {
		t.Fatalf("RRCEstablishmentCause = %v, want the unknown value 40 kept", decoded.RRCEstablishmentCause)
	}

	ies := decoded.Diagnostics().IEs
	if len(ies) != 1 || ies[0].ID != IDRRCEstablishmentCause || ies[0].Kind != ap.KindNotComprehended {
		t.Fatalf("diagnostics = %v, want one not-comprehended entry and no missing entry", ies)
	}

	parsed, err := ParseInitialUEMessage(value)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.RRCEstablishmentCause != nil {
		t.Fatalf("parsed RRCEstablishmentCause = %v, want it treated as not received", *parsed.RRCEstablishmentCause)
	}

	for name, m := range map[string]*InitialUEMessage{"decoded": decoded, "parsed": parsed} {
		b, err := m.Marshal()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(pdu.value(), value) {
			t.Fatalf("%s re-encoded %x, want %x", name, pdu.value(), value)
		}
	}
}

func TestEmptyOctetStringIsPresent(t *testing.T) {
	value := initialUEMessageValue(t, &InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      NASPDU{},
		UserLocationInformation: UserLocationInformation{NR: &UserLocationInformationNR{
			NRCGI: NRCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, NRCellIdentity: 1},
			TAI:   TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
		}},
		RRCEstablishmentCause: Ptr(RRCCauseEmergency),
	})

	m, err := DecodeInitialUEMessage(value)
	if err != nil {
		t.Fatal(err)
	}

	if m.NASPDU == nil || len(m.NASPDU) != 0 {
		t.Fatalf("NASPDU = %#v, want present and empty", m.NASPDU)
	}

	if !m.Diagnostics().Empty() {
		t.Fatalf("diagnostics = %v, want none: the NAS-PDU was received", m.Diagnostics().IEs)
	}

	assertLossless(t, func(b []byte) (typedMessage, error) { return DecodeInitialUEMessage(b) }, value)
}

func TestTrailingOctetsAreATransferSyntaxError(t *testing.T) {
	in := &HandoverCommandTransfer{}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := DecodeHandoverCommandTransfer(append(b, make([]byte, 49)...)); !errors.Is(err, per.ErrTrailing) {
		t.Fatalf("decode with trailing octets = %v, want per.ErrTrailing", err)
	}

	value := initialUEMessageValue(t, &InitialUEMessage{
		RANUENGAPID: 1,
		NASPDU:      NASPDU{0x7e},
		UserLocationInformation: UserLocationInformation{NR: &UserLocationInformationNR{
			NRCGI: NRCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, NRCellIdentity: 1},
			TAI:   TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
		}},
		RRCEstablishmentCause: Ptr(RRCCauseEmergency),
	})

	_, err = DecodeInitialUEMessage(append(value, 0x00))

	var tse *TransferSyntaxError
	if !errors.As(err, &tse) || !errors.Is(err, per.ErrTrailing) || tse.Procedure != ProcInitialUEMessage {
		t.Fatalf("decode with a trailing octet = %v, want a *TransferSyntaxError for InitialUEMessage", err)
	}
}

func TestUnsetTransportLayerAddressDoesNotEncode(t *testing.T) {
	if _, err := per.Marshal(&GTPTunnel{GTPTEID: 1}, per.Aligned); !errors.Is(err, ap.ErrTransportLayerAddress) {
		t.Fatalf("encoding a GTPTunnel without an address = %v, want %v", err, ap.ErrTransportLayerAddress)
	}
}

func TestRawPDUErrorsAreTransferSyntaxErrors(t *testing.T) {
	for name, b := range map[string][]byte{
		"empty":     nil,
		"truncated": {0x00, 0x0f, 0x40, 0x10},
		"trailing":  append(mustHexBytes(goldenInitialUEMessage), 0x00),
	} {
		_, err := ParseRawPDU(b)

		var tse *TransferSyntaxError
		if !errors.As(err, &tse) {
			t.Errorf("%s: ParseRawPDU = %v (%T), want *TransferSyntaxError", name, err, err)
		}
	}
}

func assertLossless(t *testing.T, decode func([]byte) (typedMessage, error), value []byte) {
	t.Helper()

	m, err := decode(value)
	if err != nil {
		t.Fatalf("re-decode: %v", err)
	}

	b, err := m.Marshal()
	if err != nil {
		t.Fatalf("a decoded message does not re-encode: %v", err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(pdu.value(), value) {
		t.Fatalf("re-encoded %x, want %x", pdu.value(), value)
	}
}

var losslessDecoders = []func([]byte) (typedMessage, error){
	func(b []byte) (typedMessage, error) { return DecodeInitialUEMessage(b) },
	func(b []byte) (typedMessage, error) { return DecodeUplinkNASTransport(b) },
	func(b []byte) (typedMessage, error) { return DecodeDownlinkNASTransport(b) },
	func(b []byte) (typedMessage, error) { return DecodeHandoverRequest(b) },
	func(b []byte) (typedMessage, error) { return DecodePDUSessionResourceSetupRequest(b) },
	func(b []byte) (typedMessage, error) { return DecodeInitialContextSetupRequest(b) },
}

func FuzzMessageLossless(f *testing.F) {
	f.Add([]byte("0\x00\x03\x00%0\x02\x000\x00%00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"))

	for _, g := range []string{goldenInitialUEMessage, goldenUplinkNASTransportNR, goldenPDUSessionResourceSetupRequest, goldenInitialContextSetupRequest} {
		pdu, err := Unmarshal(mustHexBytes(g))
		if err != nil {
			f.Fatal(err)
		}

		f.Add(pdu.value())
	}

	f.Fuzz(func(t *testing.T, value []byte) {
		for _, decode := range losslessDecoders {
			m, err := decode(value)
			if err != nil {
				continue
			}

			b, err := m.Marshal()
			if err != nil {
				t.Fatalf("a decoded message does not re-encode: %v", err)
			}

			pdu, err := Unmarshal(b)
			if err != nil {
				t.Fatal(err)
			}

			assertLossless(t, decode, pdu.value())
		}
	})
}
