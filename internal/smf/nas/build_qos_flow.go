// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
)

// TS 24.501 Table 9.11.4.12
/*
-	01H (5QI);
-	02H (GFBR uplink);
-	03H (GFBR downlink);
-	04H (MFBR uplink);
-	05H (MFBR downlink);
-	06H (Averaging window); and
-	07H (EPS bearer identity).
*/
const (
	QFDParameterID5Qi     uint8 = 0x01
	QFDParameterIDGfbrUl  uint8 = 0x02
	QFDParameterIDGfbrDl  uint8 = 0x03
	QFDParameterIDMfbrUl  uint8 = 0x04
	QFDParameterIDMfbrDl  uint8 = 0x05
	QFDParameterIDAvgWind uint8 = 0x06
	QFDParameterIDEpsBId  uint8 = 0x07
)

const (
	QFBitRate1Kbps uint8 = 0x01
	QFBitRate1Mbps uint8 = 0x06
	QFBitRate1Gbps uint8 = 0x0B
)

const (
	QFDOpCreate uint8 = 0x20
	QFDOpModify uint8 = 0x40
	QFDOpDelete uint8 = 0x60
)

const (
	QFDQfiBitmask    uint8 = 0x3f // bits 6 to 1 of octet
	QFDOpCodeBitmask uint8 = 0xe0 // bits 8 to 6 of octet
	QFDEbit          uint8 = 0x40 // 7th bit of param length octet
)

const (
	QFDFixLen uint8 = 0x03
)

type QosFlowDescriptionsAuthorized struct {
	Content []byte
	IeType  uint8
	IeLen   uint16
}

type QoSFlowDescription struct {
	ParamList  []QosFlowParameter
	Qfi        uint8
	OpCode     uint8
	NumOfParam uint8
	QFDLen     uint8
}

// Qos Flow Description Parameter
type QosFlowParameter struct {
	ParamContent []byte
	ParamID      uint8
	ParamLen     uint8
}

// Build Qos Flow Description to be sent to UE
func BuildAuthorizedQosFlowDescription(qosData *models.QosData) (*QosFlowDescriptionsAuthorized, error) {
	if qosData == nil {
		return nil, fmt.Errorf("qos data is nil")
	}

	QFDescriptions := QosFlowDescriptionsAuthorized{
		IeType:  nasMessage.PDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsType,
		Content: make([]byte, 0),
	}

	err := QFDescriptions.BuildAddQosFlowDescFromQoSDesc(qosData)
	if err != nil {
		return nil, fmt.Errorf("error building QoS Flow Description from QoS Data %d: %v", qosData.QFI, err)
	}

	return &QFDescriptions, nil
}

func (d *QosFlowDescriptionsAuthorized) BuildAddQosFlowDescFromQoSDesc(qosData *models.QosData) error {
	qfd := QoSFlowDescription{QFDLen: QFDFixLen}

	qfd.SetQoSFlowDescQfi(qosData.QFI)

	// Operation Code
	qfd.SetQoSFlowDescOpCode(QFDOpCreate)

	// Create Params
	// 5QI
	qfd.AddQosFlowParam5Qi(uint8(qosData.Var5qi))

	// Set E-Bit of QFD for the "create new QoS flow description" operation
	qfd.SetQFDEBitCreateNewQFD()

	// Add QFD to Authorised QFD IE
	d.AddQFD(&qfd)

	return nil
}

// bits 6 to 1 of octet(00xxxxxx)
func (q *QoSFlowDescription) SetQoSFlowDescQfi(val uint8) {
	q.Qfi = QFDQfiBitmask & val
}

// Operation code -bits 8 to 6 of octet(xxx00000)
func (q *QoSFlowDescription) SetQoSFlowDescOpCode(val uint8) {
	q.OpCode = QFDOpCodeBitmask & val
}

// E-Bit Encoding
// For the "create new QoS flow description" operation,
// 1:	parameters list is included
func (q *QoSFlowDescription) SetQFDEBitCreateNewQFD() {
	q.NumOfParam |= QFDEbit
}

// Encode QoSFlowDescriptions IE
func (d *QosFlowDescriptionsAuthorized) AddQFD(qfd *QoSFlowDescription) {
	// Add QFI byte
	d.Content = append(d.Content, qfd.Qfi)

	// Add Operation Code byte
	d.Content = append(d.Content, qfd.OpCode)

	// Add Num of Param byte
	d.Content = append(d.Content, qfd.NumOfParam)

	// Iterate through Qos Flow Description's parameters
	for _, param := range qfd.ParamList {
		// Add Param Id
		d.Content = append(d.Content, param.ParamID)

		// Add Param Length
		d.Content = append(d.Content, param.ParamLen)

		// Add Param Content
		d.Content = append(d.Content, param.ParamContent...)
	}

	// Add QFD Len
	d.IeLen += uint16(qfd.QFDLen)
}

func (q *QoSFlowDescription) AddQosFlowParam5Qi(val uint8) {
	qfp := QosFlowParameter{}
	qfp.ParamID = QFDParameterID5Qi
	qfp.ParamLen = 1 // 1 Octet
	qfp.ParamContent = []byte{val}

	// Add to QosFlowDescription
	q.NumOfParam += 1
	q.ParamList = append(q.ParamList, qfp)

	q.QFDLen += 3 //(Id + Len + content)
}
