// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	OperationCodeCreateNewQoSRule uint8 = 1
)

const (
	PacketFilterDirectionBidirectional uint8 = 3
)

// TS 24.501 Table 9.11.4.13.1
const (
	PFComponentTypeMatchAll uint8 = 0x01
)

type PacketFilterComponent struct {
	ComponentValue []byte
	ComponentType  uint8
}

type PacketFilter struct {
	Content       []PacketFilterComponent
	Direction     uint8
	Identifier    uint8 // only 0-15
	ContentLength uint8
}

type QosRule struct {
	PacketFilterList []PacketFilter
	Identifier       uint8 // 0 0 0 0 0 0 0 1	QRI 1 to 1 1 1 1 1 1 1 1	QRI 255
	OperationCode    uint8
	DQR              uint8
	Segregation      uint8
	Precedence       uint8
	QFI              uint8
	Length           uint8
}

func BuildDefaultQosRule(id uint8, qfi uint8) QosRule {
	rule := QosRule{
		Identifier:    id,
		DQR:           0x01,
		OperationCode: OperationCodeCreateNewQoSRule,
		Precedence:    255,
		QFI:           qfi,
		PacketFilterList: []PacketFilter{
			{
				Identifier: 1,
				Direction:  PacketFilterDirectionBidirectional,
				Content: []PacketFilterComponent{{
					ComponentType: PFComponentTypeMatchAll,
				}},
				ContentLength: 0x01,
			},
		},
	}

	return rule
}

func (pf *PacketFilter) MarshalBinary() ([]byte, error) {
	packetFilterBuffer := bytes.NewBuffer(nil)
	header := 0 | pf.Direction<<4 | pf.Identifier

	// write header
	err := packetFilterBuffer.WriteByte(header)
	if err != nil {
		return nil, fmt.Errorf("error writing packet filter header: %v", err)
	}

	// write length of packet filter
	err = packetFilterBuffer.WriteByte(pf.ContentLength) // uint8(len(pf.Content)))
	if err != nil {
		return nil, fmt.Errorf("error writing packet filter length: %v", err)
	}

	for _, content := range pf.Content {
		err = packetFilterBuffer.WriteByte(content.ComponentType)
		if err != nil {
			return nil, fmt.Errorf("error writing packet filter component type: %v", err)
		}

		_, err = packetFilterBuffer.Write(content.ComponentValue)
		if err != nil {
			return nil, fmt.Errorf("error writing packet filter component value: %v", err)
		}
	}

	return packetFilterBuffer.Bytes(), nil
}

func (r *QosRule) MarshalBinary() ([]byte, error) {
	ruleContentBuffer := bytes.NewBuffer(nil)

	// write rule content Header
	ruleContentHeader := r.OperationCode<<5 | r.DQR<<4 | uint8(len(r.PacketFilterList))
	ruleContentBuffer.WriteByte(ruleContentHeader)

	packetFilterListBuffer := &bytes.Buffer{}

	for _, pf := range r.PacketFilterList {
		var packetFilterBytes []byte

		retPacketFilterByte, err := pf.MarshalBinary()
		if err != nil {
			return nil, err
		}

		packetFilterBytes = retPacketFilterByte

		_, err = packetFilterListBuffer.Write(packetFilterBytes)
		if err != nil {
			return nil, err
		}
	}

	// write QoS
	if _, err := ruleContentBuffer.ReadFrom(packetFilterListBuffer); err != nil {
		return nil, err
	}

	// write precedence
	if err := ruleContentBuffer.WriteByte(r.Precedence); err != nil {
		return nil, err
	}

	// write Segregation and QFI
	segregationAndQFIByte := r.Segregation<<6 | r.QFI
	if err := ruleContentBuffer.WriteByte(segregationAndQFIByte); err != nil {
		return nil, err
	}

	ruleBuffer := bytes.NewBuffer(nil)
	// write QoS rule identifier
	if err := ruleBuffer.WriteByte(r.Identifier); err != nil {
		return nil, err
	}

	// write QoS rule length
	if err := binary.Write(ruleBuffer, binary.BigEndian, uint16(ruleContentBuffer.Len())); err != nil {
		return nil, err
	}

	// write QoS rule Content
	if _, err := ruleBuffer.ReadFrom(ruleContentBuffer); err != nil {
		return nil, err
	}

	return ruleBuffer.Bytes(), nil
}

type QoSRules []QosRule

func (rs QoSRules) MarshalBinary() (data []byte, err error) {
	qosRulesBuffer := bytes.NewBuffer(nil)

	for _, rule := range rs {
		var ruleBytes []byte

		retRuleBytes, err := rule.MarshalBinary()
		if err != nil {
			return nil, err
		}

		ruleBytes = retRuleBytes

		if _, err := qosRulesBuffer.Write(ruleBytes); err != nil {
			return nil, err
		}
	}

	return qosRulesBuffer.Bytes(), nil
}
