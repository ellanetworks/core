// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package qos

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasMessage"
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

type QosFlowsUpdate struct {
	add, mod, del map[string]*models.QosData
}

func GetQosFlowIDFromQosID(qosID string) (uint8, error) {
	id, err := strconv.Atoi(qosID)
	if err != nil {
		return 0, fmt.Errorf("qos ID cannot be converted to integer: %v", err)
	}
	if id < 0 || id > 255 {
		return 0, fmt.Errorf("integer value out of uint8 range: %d", id)
	}
	return uint8(id), nil
}

// Build Qos Flow Description to be sent to UE
func BuildAuthorizedQosFlowDescriptions(smPolicyUpdates *PolicyUpdate) (*QosFlowDescriptionsAuthorized, error) {
	QFDescriptions := QosFlowDescriptionsAuthorized{
		IeType:  nasMessage.PDUSessionEstablishmentAcceptAuthorizedQosFlowDescriptionsType,
		Content: make([]byte, 0),
	}

	qosFlowUpdate := smPolicyUpdates.QosFlowUpdate

	// QoS Flow Description to be Added
	if qosFlowUpdate != nil {
		for _, qosFlow := range qosFlowUpdate.add {
			err := QFDescriptions.BuildAddQosFlowDescFromQoSDesc(qosFlow)
			if err != nil {
				return nil, fmt.Errorf("failed to build Qos Flow Description: %v", err)
			}
		}
	}

	return &QFDescriptions, nil
}

func (d *QosFlowDescriptionsAuthorized) BuildAddQosFlowDescFromQoSDesc(qosData *models.QosData) error {
	qfd := QoSFlowDescription{QFDLen: QFDFixLen}

	// Set QFI
	flowID, err := GetQosFlowIDFromQosID(qosData.QosID)
	if err != nil {
		return fmt.Errorf("failed to get QFI from QosID: %v", err)
	}
	qfd.SetQoSFlowDescQfi(flowID)

	// Operation Code
	qfd.SetQoSFlowDescOpCode(QFDOpCreate)

	// Create Params
	// 5QI
	qfd.AddQosFlowParam5Qi(uint8(qosData.Var5qi))

	// MFBR uplink
	if qosData.MaxbrUl != "" {
		qfd.addQosFlowRateParam(qosData.MaxbrUl, QFDParameterIDMfbrUl)
	}

	// MFBR downlink
	if qosData.MaxbrDl != "" {
		qfd.addQosFlowRateParam(qosData.MaxbrDl, QFDParameterIDMfbrDl)
	}

	// GFBR uplink
	if qosData.GbrUl != "" {
		qfd.addQosFlowRateParam(qosData.GbrUl, QFDParameterIDGfbrUl)
	}

	// GFBR downlink
	if qosData.GbrDl != "" {
		qfd.addQosFlowRateParam(qosData.GbrDl, QFDParameterIDGfbrDl)
	}

	// Set E-Bit of QFD for the "create new QoS flow description" operation
	qfd.SetQFDEBitCreateNewQFD()

	// Add QFD to Authorised QFD IE
	d.AddQFD(&qfd)
	return nil
}

func GetBitRate(sBitRate string) (val uint16, unit uint8) {
	sl := strings.Fields(sBitRate)

	// rate
	if rate, err := strconv.ParseUint(sl[0], 10, 16); err != nil {
		log.Printf("invalid bit rate [%v]", sBitRate)
	} else {
		val = uint16(rate)
	}

	// Unit
	switch sl[1] {
	case "Kbps":
		unit = QFBitRate1Kbps
	case "Mbps":
		unit = QFBitRate1Mbps
	case "Gbps":
		unit = QFBitRate1Gbps
	default:
		unit = QFBitRate1Mbps
	}
	return
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

func (p *QosFlowParameter) SetQosFlowParamBitRate(rateType, rateUnit uint8, rateVal uint16) {
	p.ParamID = rateType //(i.e. QosFlowDescriptionParameterIDGfbrUl)
	p.ParamLen = 0x03    //(Length is rate unit(1 byte) + rate value(2 bytes))
	p.ParamContent = []byte{rateUnit}
	p.ParamContent = append(p.ParamContent, byte(rateVal>>8), byte(rateVal&0xff))
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

func (q *QoSFlowDescription) addQosFlowRateParam(rate string, rateType uint8) {
	flowParam := QosFlowParameter{}
	bitRate, unit := GetBitRate(rate)
	flowParam.SetQosFlowParamBitRate(rateType, unit, bitRate)
	// Add to QosFlowDescription
	q.NumOfParam += 1
	q.ParamList = append(q.ParamList, flowParam)

	q.QFDLen += 5 //(Id-1 + len-1 + Content-3)
}

func GetQosFlowDescUpdate(pcfQosData, ctxtQosData map[string]*models.QosData) *QosFlowsUpdate {
	if len(pcfQosData) == 0 {
		return nil
	}

	update := QosFlowsUpdate{
		add: make(map[string]*models.QosData),
		mod: make(map[string]*models.QosData),
		del: make(map[string]*models.QosData),
	}

	// Iterate through pcf qos data to identify find add/mod/del qos flows
	for name, pcfQF := range pcfQosData {
		// if pcfQF is null then rule is deleted
		if pcfQF == nil {
			update.del[name] = pcfQF // nil
			continue
		}

		// Flows to add
		if ctxtQF := ctxtQosData[name]; ctxtQF == nil {
			update.add[name] = pcfQF
		} else if GetQosDataChanges(pcfQF, ctxtQF) {
			update.mod[name] = pcfQF
		}
	}

	return &update
}

func CommitQosFlowDescUpdate(smCtxtPolData *SmCtxtPolicyData, update *QosFlowsUpdate) {
	// Iterate through Add/Mod/Del Qos Flows

	// Add new Flows
	if len(update.add) > 0 {
		for name, qosData := range update.add {
			smCtxtPolData.SmCtxtQosData.QosData[name] = qosData
		}
	}

	// Del flows
	if len(update.del) > 0 {
		for name := range update.del {
			delete(smCtxtPolData.SmCtxtQosData.QosData, name)
		}
	}
}

// Compare if any change in QoS Data
func GetQosDataChanges(qf1, qf2 *models.QosData) bool {
	return false
}

func GetQoSDataFromPolicyDecision(smPolicyDecision *models.SmPolicyDecision, refQosData string) *models.QosData {
	return smPolicyDecision.QosDecs[refQosData]
}

func (upd *QosFlowsUpdate) GetAddQosFlowUpdate() map[string]*models.QosData {
	return upd.add
}

func GetDefaultQoSDataFromPolicyDecision(smPolicyDecision *models.SmPolicyDecision) *models.QosData {
	for _, qosData := range smPolicyDecision.QosDecs {
		if qosData.DefQosFlowIndication {
			return qosData
		}
	}

	return nil
}
