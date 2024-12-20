package qos

import (
	"bytes"
	"encoding/binary"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/omec-project/openapi/models"
)

const (
	OperationCodeCreateNewQoSRule                                   uint8 = 1
	OperationCodeDeleteExistingQoSRule                              uint8 = 2
	OperationCodeModifyExistingQoSRuleAndAddPacketFilters           uint8 = 3
	OperationCodeModifyExistingQoSRuleAndReplaceAllPacketFilters    uint8 = 4
	OperationCodeModifyExistingQoSRuleAndDeletePacketFilters        uint8 = 5
	OperationCodeModifyExistingQoSRuleWithoutModifyingPacketFilters uint8 = 6
)

const (
	PacketFilterDirectionDownlink      uint8 = 1
	PacketFilterDirectionUplink        uint8 = 2
	PacketFilterDirectionBidirectional uint8 = 3
)

// TS 24.501 Table 9.11.4.13.1
const (
	PFComponentTypeMatchAll                       uint8 = 0x01
	PFComponentTypeIPv4RemoteAddress              uint8 = 0x10
	PFComponentTypeIPv4LocalAddress               uint8 = 0x11
	PFComponentTypeIPv6RemoteAddress              uint8 = 0x21
	PFComponentTypeIPv6LocalAddress               uint8 = 0x23
	PFComponentTypeProtocolIdentifierOrNextHeader uint8 = 0x30
	PFComponentTypeSingleLocalPort                uint8 = 0x40
	PFComponentTypeLocalPortRange                 uint8 = 0x41
	PFComponentTypeSingleRemotePort               uint8 = 0x50
	PFComponentTypeRemotePortRange                uint8 = 0x51
	PFComponentTypeSecurityParameterIndex         uint8 = 0x60
	PFComponentTypeTypeOfServiceOrTrafficClass    uint8 = 0x70
	PFComponentTypeFlowLabel                      uint8 = 0x80
	PFComponentTypeDestinationMACAddress          uint8 = 0x81
	PFComponentTypeSourceMACAddress               uint8 = 0x82
	PFComponentType8021Q_CTAG_VID                 uint8 = 0x83
	PFComponentType8021Q_STAG_VID                 uint8 = 0x84
	PFComponentType8021Q_CTAG_PCPOrDEI            uint8 = 0x85
	PFComponentType8021Q_STAG_PCPOrDEI            uint8 = 0x86
	PFComponentTypeEthertype                      uint8 = 0x87
)

const (
	PacketFilterIdBitmask uint8 = 0x0f
)

type IPFilterRulePortRange struct {
	lowLimit  string
	highLimit string
}

type IPFilterRuleIpAddrV4 struct {
	addr string
	mask string
}

type IPFilterRule struct {
	protoId                string
	sPort, dPort           string
	sPortRange, dPortRange IPFilterRulePortRange
	sAddrv4, dAddrv4       IPFilterRuleIpAddrV4
}

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

func BuildAddDefaultQosRule(defQFI uint8) *QosRule {
	defQosRule := &QosRule{
		Identifier:    255,
		DQR:           0x01,
		OperationCode: OperationCodeCreateNewQoSRule,
		Precedence:    255,
		QFI:           defQFI,
		PacketFilterList: []PacketFilter{
			{
				Identifier: 255,
				Direction:  PacketFilterDirectionBidirectional,
			},
		},
	}

	defPfc := PacketFilterComponent{
		ComponentType: PFComponentTypeMatchAll,
		// ComponentValue: NA for Match All
	}
	defQosRule.PacketFilterList[0].Content = []PacketFilterComponent{defPfc}
	defQosRule.PacketFilterList[0].ContentLength = 0x01

	return defQosRule
}

func BuildQosRules(smPolicyUpdates *PolicyUpdate) QoSRules {
	qosRules := QoSRules{}

	smPolicyDecision := smPolicyUpdates.SmPolicyDecision
	pccRulesUpdate := smPolicyUpdates.PccRuleUpdate

	// New Rules to be added
	if pccRulesUpdate != nil {
		for pccRuleName, pccRuleVal := range pccRulesUpdate.add {
			log.Printf("Building QoS Rule from PCC rule [%s]", pccRuleName)
			refQosData := GetQoSDataFromPolicyDecision(smPolicyDecision, pccRuleVal.RefQosData[0])
			qosRule := BuildAddQoSRuleFromPccRule(pccRuleVal, refQosData, OperationCodeCreateNewQoSRule)
			qosRules = append(qosRules, *qosRule)
		}
	}

	//Add default Matchall QosRule as well
	/*
		if smPolicyUpdates.SessRuleUpdate != nil {
			defQosRule := BuildAddDefaultQosRule(uint8(smPolicyUpdates.SessRuleUpdate.ActiveSessRule.AuthDefQos.Var5qi))
			qosRules = append(qosRules, *defQosRule)
		}
	*/
	return qosRules
}

func BuildAddQoSRuleFromPccRule(pccRule *models.PccRule, qosData *models.QosData, pccRuleOpCode uint8) *QosRule {
	qRule := QosRule{
		Identifier:    GetQosRuleIdFromPccRuleId(pccRule.PccRuleId),
		DQR:           btou(qosData.DefQosFlowIndication),
		OperationCode: pccRuleOpCode,
		Precedence:    uint8(pccRule.Precedence),
		QFI:           GetQosFlowIdFromQosId(qosData.QosId),
	}

	qRule.BuildPacketFilterListFromPccRule(pccRule)

	return &qRule
}

func BuildModifyQosRuleFromPccRule(pccRule *models.PccRule) *QosRule {
	return nil
}

func BuildDeleteQosRuleFromPccRule(pccRule *models.PccRule) *QosRule {
	return nil
}

func btou(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

func GetQosRuleIdFromPccRuleId(pccRuleId string) uint8 {
	if id, err := strconv.Atoi(pccRuleId); err != nil {
		return 0
	} else {
		return uint8(id)
	}
}

func (q *QosRule) BuildPacketFilterListFromPccRule(pccRule *models.PccRule) {
	pfList := []PacketFilter{}

	// Iterate through
	for _, flow := range pccRule.FlowInfos {
		pf := GetPacketFilterFromFlowInfo(&flow)
		pfList = append(pfList, pf)
	}
	q.PacketFilterList = pfList
}

func GetPacketFilterFromFlowInfo(flowInfo *models.FlowInformation) PacketFilter {
	pf := &PacketFilter{
		Identifier: GetPfId(flowInfo.PackFiltId),
		Direction:  GetPfDirectionFromPccFlowInfo(flowInfo.FlowDirection),
	}

	// Fill PF component contents
	pf.GetPfContent(flowInfo.FlowDescription)

	return *pf
}

func GetPfId(ids string) uint8 {
	if id, err := strconv.Atoi(ids); err != nil {
		return 0
	} else {
		return (uint8(id) & PacketFilterIdBitmask)
	}
}

// Get Packet Filter Directions
func GetPfDirectionFromPccFlowInfo(flowDir models.FlowDirectionRm) uint8 {
	switch flowDir {
	case models.FlowDirectionRm_UPLINK:
		return PacketFilterDirectionUplink
	case models.FlowDirectionRm_DOWNLINK:
		return PacketFilterDirectionDownlink
	case models.FlowDirectionRm_BIDIRECTIONAL:
		return PacketFilterDirectionBidirectional
	default:
		return PacketFilterDirectionBidirectional
	}
}

// e.x. permit out ip-proto from x.x.x.x/maskbits port/port-range to assigned(x.x.x.x/maskbits) port/port-range

//	0		1 	2		3	  4   				5   		   6 	7						8

// See spec 29212-5.4.2 / 29512-5.6.3.2
func DecodeFlowDescToIPFilters(flowDesc string) *IPFilterRule {
	// Tokenize flow desc and make PF components
	pfcTags := strings.Fields(flowDesc)

	// get PF tags into IP filter components
	ipfRule := &IPFilterRule{}

	// Protocol Id/Next Header
	ipfRule.protoId = pfcTags[2]

	// decode source IP/mask
	ipfRule.decodeIpFilterAddrv4(true, pfcTags[4])

	// decode source port/port-range (optional)
	if pfcTags[6] == "to" {
		// decode source port/port-range
		ipfRule.decodeIpFilterPortInfo(true, pfcTags[5])

		// decode destination IP/mask
		ipfRule.decodeIpFilterAddrv4(false, pfcTags[7])

		// decode destination port/port-range(optional), if any
		if len(pfcTags) == 9 {
			ipfRule.decodeIpFilterPortInfo(false, pfcTags[8])
		}
	} else {
		// decode destination IP/mask
		ipfRule.decodeIpFilterAddrv4(false, pfcTags[6])

		// decode destination port/port-range(optional), if any
		if len(pfcTags) == 8 {
			ipfRule.decodeIpFilterPortInfo(false, pfcTags[7])
		}
	}

	return ipfRule
}

func (ipf *IPFilterRule) IsMatchAllIPFilter() bool {
	if ipf.sAddrv4.addr == "any" && ipf.dAddrv4.addr == "assigned" {
		return true
	}
	return false
}

func (ipfRule *IPFilterRule) decodeIpFilterPortInfo(source bool, tag string) {
	// check if it is single port or range
	ports := strings.Split(tag, "-")

	if len(ports) > 1 { // port range
		if source {
			ipfRule.sPortRange.lowLimit = ports[0]
			ipfRule.sPortRange.highLimit = ports[1]
		} else {
			ipfRule.dPortRange.lowLimit = ports[0]
			ipfRule.dPortRange.highLimit = ports[1]
		}
	} else {
		if source {
			ipfRule.sPort = ports[0]
		} else {
			ipfRule.dPort = ports[0]
		}
	}
}

func (ipfRule *IPFilterRule) decodeIpFilterAddrv4(source bool, tag string) {
	ipAndMask := strings.Split(tag, "/")
	if source {
		ipfRule.sAddrv4.addr = ipAndMask[0] // can be x.x.x.x or "any"
	} else {
		ipfRule.dAddrv4.addr = ipAndMask[0] // can be x.x.x.x or "assigned"
	}

	// mask can be nil
	if len(ipAndMask) > 1 {
		if source {
			ipfRule.sAddrv4.mask = ipAndMask[1]
		} else {
			ipfRule.dAddrv4.mask = ipAndMask[1]
		}
	}
}

func (pf *PacketFilter) GetPfContent(flowDesc string) {
	pfcList := []PacketFilterComponent{}

	ipf := DecodeFlowDescToIPFilters(flowDesc)

	// Make Packet Filter Component from decoded IPFilters

	// MatchAll Packet Filter
	if ipf.IsMatchAllIPFilter() {
		pfc := &PacketFilterComponent{
			ComponentType: PFComponentTypeMatchAll,
		}

		pfcList = append(pfcList, *pfc)
		pf.ContentLength += 1
		pf.Content = pfcList
		return
	}

	// Protocol identifier/Next header type
	if pfc, protocolIdLen := BuildPFCompProtocolId(ipf.protoId); pfc != nil {
		pfcList = append(pfcList, *pfc)
		pf.ContentLength += protocolIdLen
	}

	// Remote Addr
	if pfc, addrLen := buildPFCompAddr(false, ipf.sAddrv4); pfc != nil {
		pfcList = append(pfcList, *pfc)
		pf.ContentLength += addrLen
	}

	// Remote Port
	if pfc, portLen := buildPFCompPort(false, ipf.sPort); pfc != nil {
		pfcList = append(pfcList, *pfc)
		pf.ContentLength += portLen
	}

	// Remote Port range
	if pfc, portRangeLen := buildPFCompPortRange(false, ipf.sPortRange); pfc != nil {
		pfcList = append(pfcList, *pfc)
		pf.ContentLength += portRangeLen
	}

	// Local Addr
	if pfc, addrLen := buildPFCompAddr(true, ipf.dAddrv4); pfc != nil {
		pfcList = append(pfcList, *pfc)
		pf.ContentLength += addrLen
	}

	// Local Port
	if pfc, portLen := buildPFCompPort(true, ipf.dPort); pfc != nil {
		pfcList = append(pfcList, *pfc)
		pf.ContentLength += portLen
	}

	// Local Port range
	if pfc, portRangeLen := buildPFCompPortRange(true, ipf.dPortRange); pfc != nil {
		pfcList = append(pfcList, *pfc)
		pf.ContentLength += portRangeLen
	}

	pf.Content = pfcList
}

func buildPFCompAddr(local bool, val IPFilterRuleIpAddrV4) (*PacketFilterComponent, uint8) {
	component := PFComponentTypeIPv4RemoteAddress

	if local {
		component = PFComponentTypeIPv4LocalAddress
		// if local address value- "assigned" then don't need to set it
		if val.addr == "assigned" {
			return nil, 0
		}
	} else {
		// if remote address value- "any" then don't need to set it
		if val.addr == "any" {
			return nil, 0
		}
	}

	pfc := &PacketFilterComponent{
		ComponentType:  component,
		ComponentValue: make([]byte, 0),
	}

	var addr, mask []byte

	if ipAddr := net.ParseIP(val.addr); ipAddr == nil {
		return nil, 0
	} else {
		// check if it is valid v4 addr
		if v4addr := ipAddr.To4(); v4addr == nil {
			return nil, 0
		} else {
			addr = []byte(v4addr)
			pfc.ComponentValue = append(pfc.ComponentValue, addr...)
		}
	}

	if val.mask != "" {
		maskInt, _ := strconv.Atoi(val.mask)
		mask = net.CIDRMask(maskInt, 32)
		pfc.ComponentValue = append(pfc.ComponentValue, mask...)
	} else {
		mask = net.CIDRMask(32, 32)
		pfc.ComponentValue = append(pfc.ComponentValue, mask...)
	}

	return pfc, 9
}

func buildPFCompPort(local bool, val string) (*PacketFilterComponent, uint8) {
	if val == "" {
		return nil, 0
	}

	component := PFComponentTypeSingleRemotePort
	if local {
		component = PFComponentTypeSingleLocalPort
	}

	pfc := &PacketFilterComponent{
		ComponentType:  component,
		ComponentValue: make([]byte, 2),
	}

	if port, err := strconv.Atoi(val); err == nil {
		port16 := uint16(port)
		pfc.ComponentValue = []byte{byte(port16 >> 8), byte(port16 & 0xff)}
	}
	return pfc, 3
}

func buildPFCompPortRange(local bool, val IPFilterRulePortRange) (*PacketFilterComponent, uint8) {
	if val.lowLimit == "" || val.highLimit == "" {
		return nil, 0
	}

	component := PFComponentTypeRemotePortRange
	if local {
		component = PFComponentTypeLocalPortRange
	}

	pfc := &PacketFilterComponent{
		ComponentType:  component,
		ComponentValue: make([]byte, 4),
	}

	// low port value
	if port, err := strconv.Atoi(val.lowLimit); err == nil {
		port16 := uint16(port)
		pfc.ComponentValue = []byte{byte(port16 >> 8), byte(port16 & 0xff)}
	}

	// high port value
	if port, err := strconv.Atoi(val.highLimit); err == nil {
		port16 := uint16(port)
		pfc.ComponentValue = append(pfc.ComponentValue, byte(port16>>8), byte(port16&0xff))
	}
	return pfc, 5
}

func BuildPFCompProtocolId(val string) (*PacketFilterComponent, uint8) {
	if val == "ip" {
		return nil, 0
	}

	pfc := &PacketFilterComponent{
		ComponentType:  PFComponentTypeProtocolIdentifierOrNextHeader,
		ComponentValue: make([]byte, 1),
	}

	if pfcVal, err := strconv.Atoi(val); err == nil {
		bs := make([]byte, 4)
		binary.BigEndian.PutUint32(bs, uint32(pfcVal))
		pfc.ComponentValue = []byte{bs[3]}
	} else {
		return nil, 0
	}

	return pfc, 2
}

func (pf *PacketFilter) MarshalBinary() (data []byte, err error) {
	packetFilterBuffer := bytes.NewBuffer(nil)
	header := 0 | pf.Direction<<4 | pf.Identifier
	// write header
	err = packetFilterBuffer.WriteByte(header)
	if err != nil {
		return nil, err
	}
	// write length of packet filter
	err = packetFilterBuffer.WriteByte(pf.ContentLength) // uint8(len(pf.Content)))
	if err != nil {
		return nil, err
	}

	for _, content := range pf.Content {
		err = packetFilterBuffer.WriteByte(content.ComponentType)
		if err != nil {
			return nil, err
		}
		_, err = packetFilterBuffer.Write(content.ComponentValue)
		if err != nil {
			return nil, err
		}
	}
	/*
		err = packetFilterBuffer.WriteByte(pf.Content)
		if err != nil {
			return nil, err
		}

		if pf.ComponentType == PacketFilterComponentTypeMatchAll || pf.Component == nil {
			_, err = packetFilterBuffer.Write(pf.Component)
			if err != nil {
				return nil, err
			}
		}
	*/
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
		if retPacketFilterByte, err := pf.MarshalBinary(); err != nil {
			return nil, err
		} else {
			packetFilterBytes = retPacketFilterByte
		}

		if _, err := packetFilterListBuffer.Write(packetFilterBytes); err != nil {
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
		if retRuleBytes, err := rule.MarshalBinary(); err != nil {
			return nil, err
		} else {
			ruleBytes = retRuleBytes
		}

		if _, err := qosRulesBuffer.Write(ruleBytes); err != nil {
			return nil, err
		}
	}
	return qosRulesBuffer.Bytes(), nil
}
