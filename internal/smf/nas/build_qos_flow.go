// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
)

// QoS flow description parameter identifiers (TS 24.501 Table 9.11.4.12).
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

type QosFlowParameter struct {
	ParamContent []byte
	ParamID      uint8
	ParamLen     uint8
}

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

// BuildModifyQosFlowDescription builds a QoS Flow Description with the "modify
// existing QoS flow description" operation code (TS 24.501 Table 9.11.4.12).
func BuildModifyQosFlowDescription(qosData *models.QosData) (*QosFlowDescriptionsAuthorized, error) {
	if qosData == nil {
		return nil, fmt.Errorf("qos data is nil")
	}

	qfDescriptions := QosFlowDescriptionsAuthorized{
		IeType:  nasMessage.PDUSessionModificationCommandAuthorizedQosFlowDescriptionsType,
		Content: make([]byte, 0),
	}

	qfd := QoSFlowDescription{QFDLen: QFDFixLen}
	qfd.SetQoSFlowDescQfi(qosData.QFI)
	qfd.SetQoSFlowDescOpCode(QFDOpModify)
	qfd.AddQosFlowParam5Qi(uint8(qosData.Var5qi))

	// E-bit set: the parameters list replaces the flow's entire parameter set
	// (TS 24.501 Table 9.11.4.12).
	qfd.NumOfParam |= QFDEbit

	qfDescriptions.AddQFD(&qfd)

	return &qfDescriptions, nil
}

func (d *QosFlowDescriptionsAuthorized) BuildAddQosFlowDescFromQoSDesc(qosData *models.QosData) error {
	qfd := QoSFlowDescription{QFDLen: QFDFixLen}

	qfd.SetQoSFlowDescQfi(qosData.QFI)
	qfd.SetQoSFlowDescOpCode(QFDOpCreate)
	qfd.AddQosFlowParam5Qi(uint8(qosData.Var5qi))
	qfd.SetQFDEBitCreateNewQFD()
	d.AddQFD(&qfd)

	return nil
}

func (q *QoSFlowDescription) SetQoSFlowDescQfi(val uint8) {
	q.Qfi = QFDQfiBitmask & val
}

func (q *QoSFlowDescription) SetQoSFlowDescOpCode(val uint8) {
	q.OpCode = QFDOpCodeBitmask & val
}

// E-bit set to 1 signals that the parameters list is included, for the "create
// new QoS flow description" operation (TS 24.501 Table 9.11.4.12).
func (q *QoSFlowDescription) SetQFDEBitCreateNewQFD() {
	q.NumOfParam |= QFDEbit
}

func (d *QosFlowDescriptionsAuthorized) AddQFD(qfd *QoSFlowDescription) {
	d.Content = append(d.Content, qfd.Qfi)
	d.Content = append(d.Content, qfd.OpCode)
	d.Content = append(d.Content, qfd.NumOfParam)

	for _, param := range qfd.ParamList {
		d.Content = append(d.Content, param.ParamID)
		d.Content = append(d.Content, param.ParamLen)
		d.Content = append(d.Content, param.ParamContent...)
	}

	d.IeLen += uint16(qfd.QFDLen)
}

func (q *QoSFlowDescription) AddQosFlowParam5Qi(val uint8) {
	qfp := QosFlowParameter{}
	qfp.ParamID = QFDParameterID5Qi
	qfp.ParamLen = 1
	qfp.ParamContent = []byte{val}

	q.NumOfParam += 1
	q.ParamList = append(q.ParamList, qfp)

	q.QFDLen += 3 // Id + Len + content
}
