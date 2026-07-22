// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// QoS rule and packet-filter coding (TS 24.501 §9.11.4.13, table 9.11.4.13.1).
const (
	qosOperationCreateNewQoSRule uint8 = 1
	packetFilterBidirectional    uint8 = 3
	pfComponentTypeMatchAll      uint8 = 0x01
)

// PacketFilter is one packet filter of a QoS rule (TS 24.501 §9.11.4.13).
type PacketFilter struct {
	ComponentType uint8
	Identifier    uint8 // 0-15
	Direction     uint8
}

// QoSRule is a single authorized QoS rule (TS 24.501 §9.11.4.13).
type QoSRule struct {
	Identifier    uint8 // QRI, 1-255
	OperationCode uint8
	DQR           uint8
	Precedence    uint8
	QFI           uint8
	Segregation   uint8
	Filters       []PacketFilter
}

// DefaultQoSRule builds the match-all default QoS rule bound to qfi.
func DefaultQoSRule(id, qfi uint8) QoSRule {
	return QoSRule{
		Identifier:    id,
		DQR:           0x01,
		OperationCode: qosOperationCreateNewQoSRule,
		Precedence:    255,
		QFI:           qfi,
		Filters: []PacketFilter{
			{ComponentType: pfComponentTypeMatchAll, Identifier: 1, Direction: packetFilterBidirectional},
		},
	}
}

func (f PacketFilter) marshal(w *common.Writer) {
	w.U8(f.Direction<<4 | f.Identifier)
	w.U8(0x01) // packet filter contents length (match-all: type octet only)
	w.U8(f.ComponentType)
}

func (r QoSRule) marshal(w *common.Writer) {
	var content common.Writer

	content.U8(r.OperationCode<<5 | r.DQR<<4 | uint8(len(r.Filters)))

	for _, f := range r.Filters {
		f.marshal(&content)
	}

	content.U8(r.Precedence)
	content.U8(r.Segregation<<6 | r.QFI)

	w.U8(r.Identifier)
	w.U16(uint16(content.Len()))
	w.Raw(content.Bytes())
}

// MarshalQoSRules encodes the authorized QoS rules IE content (the value of the
// LV-E / TLV-E, without IEI or length).
func MarshalQoSRules(rules []QoSRule) []byte {
	var w common.Writer

	for _, r := range rules {
		r.marshal(&w)
	}

	return w.Bytes()
}

// QoS flow description coding (TS 24.501 §9.11.4.12, table 9.11.4.12.1).
const (
	qfdParam5QI   uint8 = 0x01
	qfdOpCreate   uint8 = 0x20
	qfdOpModify   uint8 = 0x40
	qfdQFIBitmask uint8 = 0x3F
	qfdOpCodeMask uint8 = 0xE0
	qfdEBit       uint8 = 0x40
)

// marshalQoSFlowDescription encodes one QoS flow description carrying the 5QI
// parameter, with the given operation code. The E bit is set: the parameter
// list replaces the flow's entire parameter set (TS 24.501 §9.11.4.12).
func marshalQoSFlowDescription(w *common.Writer, qfi, fiveQI, opCode uint8) {
	w.U8(qfi & qfdQFIBitmask)
	w.U8(opCode & qfdOpCodeMask)
	w.U8(qfdEBit | 0x01) // E bit + one parameter
	w.U8(qfdParam5QI)
	w.U8(0x01) // parameter length
	w.U8(fiveQI)
}

// MarshalCreateQoSFlow encodes the authorized QoS flow descriptions IE content
// for a "create new QoS flow description" operation.
func MarshalCreateQoSFlow(qfi, fiveQI uint8) []byte {
	var w common.Writer

	marshalQoSFlowDescription(&w, qfi, fiveQI, qfdOpCreate)

	return w.Bytes()
}

// MarshalModifyQoSFlow encodes the authorized QoS flow descriptions IE content
// for a "modify existing QoS flow description" operation.
func MarshalModifyQoSFlow(qfi, fiveQI uint8) []byte {
	var w common.Writer

	marshalQoSFlowDescription(&w, qfi, fiveQI, qfdOpModify)

	return w.Bytes()
}
