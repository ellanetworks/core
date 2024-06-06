package qos

import (
	"encoding/binary"
	"fmt"

	"github.com/omec-project/openapi/models"
)

func (obj *IPFilterRule) String() string {
	return fmt.Sprintf("IPFilter content: ProtocolId:[%v], Source:[Ip:[%v], Mask:[%v], Port:[%v] Port-range [%v-%v]],Destination [Ip [%v], Mask [%v], Port [%v], Port-range [%v-%v]]",
		obj.protoId, obj.sAddrv4.addr, obj.sAddrv4.mask, obj.sPort, obj.sPortRange.lowLimit, obj.sPortRange.highLimit, obj.dAddrv4.addr, obj.sAddrv4.mask, obj.dPort, obj.dPortRange.lowLimit, obj.dPortRange.highLimit)
}

func (obj QosRule) String() string {
	return fmt.Sprintf("QosRule:[Id:[%v], Precedence:[%v], OpCode:[%v]], DQR:[%v], QFI:[%v], PacketFilters:[%v]",
		obj.Identifier, obj.Precedence, RuleOperation(obj.OperationCode), obj.DQR, obj.QFI, obj.PacketFilterList)
}

func (obj PacketFilter) String() string {
	return fmt.Sprintf("\nPacketFilter:[Id:[%v], direction:[%v], content:[\n%v]]", obj.Identifier, PfDirectionString(obj.Direction), obj.Content)
}

func (obj PacketFilterComponent) String() string {
	switch obj.ComponentType {
	case PFComponentTypeSingleLocalPort:
		fallthrough
	case PFComponentTypeSingleRemotePort:
		return fmt.Sprintf("PFComponent content: type:[%v] value:[%v]\n",
			PfcString(obj.ComponentType), binary.BigEndian.Uint16(obj.ComponentValue))
	case PFComponentTypeLocalPortRange:
		fallthrough
	case PFComponentTypeRemotePortRange:
		return fmt.Sprintf("PFComponent content: type:[%v] value:[%v-%v]\n",
			PfcString(obj.ComponentType), binary.BigEndian.Uint16(obj.ComponentValue[:2]), binary.BigEndian.Uint16(obj.ComponentValue[2:]))
	default:
		return fmt.Sprintf("PFComponent content: type:[%v] value:[%v]\n", PfcString(obj.ComponentType), obj.ComponentValue)
	}
}

func RuleOperation(op uint8) string {
	switch op {
	case OperationCodeCreateNewQoSRule:
		return "CreateNewQoSRule"
	case OperationCodeDeleteExistingQoSRule:
		return "DeleteExistingQoSRule"
	case OperationCodeModifyExistingQoSRuleAndAddPacketFilters:
		return "ModifyExistingQoSRuleAndAddPacketFilters"
	case OperationCodeModifyExistingQoSRuleAndReplaceAllPacketFilters:
		return "ModifyExistingQoSRuleAndReplaceAllPacketFilters"
	case OperationCodeModifyExistingQoSRuleAndDeletePacketFilters:
		return "ModifyExistingQoSRuleAndDeletePacketFilters"
	case OperationCodeModifyExistingQoSRuleWithoutModifyingPacketFilters:
		return "ModifyExistingQoSRuleWithoutModifyingPacketFilters"
	default:
		return "invalid"
	}
}

func PfDirectionString(dir uint8) string {
	switch dir {
	case PacketFilterDirectionDownlink:
		return "Downlink"
	case PacketFilterDirectionUplink:
		return "Uplink"
	case PacketFilterDirectionBidirectional:
		return "Bidirectional"
	default:
		return "Unspecified"
	}
}

func PfcString(pfcType uint8) string {
	switch pfcType {
	case PFComponentTypeMatchAll:
		return "MatchAll"
	case PFComponentTypeIPv4RemoteAddress:
		return "IPv4RemoteAddress"
	case PFComponentTypeIPv4LocalAddress:
		return "IPv4LocalAddress"
	case PFComponentTypeIPv6RemoteAddress:
		return "IPv6RemoteAddress"
	case PFComponentTypeIPv6LocalAddress:
		return "IPv6LocalAddress"
	case PFComponentTypeProtocolIdentifierOrNextHeader:
		return "ProtocolIdentifierOrNextHeader"
	case PFComponentTypeSingleLocalPort:
		return "SingleLocalPort"
	case PFComponentTypeLocalPortRange:
		return "LocalPortRange"
	case PFComponentTypeSingleRemotePort:
		return "SingleRemotePort"
	case PFComponentTypeRemotePortRange:
		return "RemotePortRange"
	case PFComponentTypeSecurityParameterIndex:
		return "SecurityParameterIndex"
	case PFComponentTypeTypeOfServiceOrTrafficClass:
		return "TypeOfServiceOrTrafficClass"
	case PFComponentTypeFlowLabel:
		return "FlowLabel"
	case PFComponentTypeDestinationMACAddress:
		return "DestinationMACAddress"
	case PFComponentTypeSourceMACAddress:
		return "SourceMACAddress"
	case PFComponentType8021Q_CTAG_VID:
		return "8021Q_CTAG_VID"
	case PFComponentType8021Q_STAG_VID:
		return "8021Q_STAG_VID"
	case PFComponentType8021Q_CTAG_PCPOrDEI:
		return "8021Q_CTAG_PCPOrDEI"
	case PFComponentType8021Q_STAG_PCPOrDEI:
		return "8021Q_STAG_PCPOrDEI"
	case PFComponentTypeEthertype:
		return "Ethertype"
	default:
		return "invalid"
	}
}

func SmPolicyDecisionString(smPolicy *models.SmPolicyDecision) string {
	// PCC Rules
	str := "\nPCC Rules: "
	for name, rule := range smPolicy.PccRules {
		str = str + fmt.Sprintf("\n[name:[%v], %v]", name, PccRuleString(rule))
	}

	// Session Rules
	str = str + "\nSession Rules: "
	for name, rule := range smPolicy.SessRules {
		str = str + fmt.Sprintf("\n[name:[%v], %v]", name, SessRuleString(rule))
	}

	// Qos Data
	str = str + "\nQosData: "
	for name, qosData := range smPolicy.QosDecs {
		str = str + fmt.Sprintf("\n[name:[%v], %v]", name, QosDataString(qosData))
	}

	// TC Data
	str = str + "\nTCData: "
	for name, tcData := range smPolicy.TraffContDecs {
		str = str + fmt.Sprintf("\n[name:[%v], %v]", name, TCDataString(tcData))
	}
	return str
}

func QosDataString(q *models.QosData) string {
	if q == nil {
		return ""
	}
	return fmt.Sprintf("QosData:[QosId:[%v], Var5QI:[%v], MaxBrUl:[%v], MaxBrDl:[%v], GBrUl:[%v], GBrDl:[%v], PriorityLevel:[%v], ARP:[%v], DQFI:[%v]]",
		q.QosId, q.Var5qi, q.MaxbrUl, q.MaxbrDl, q.GbrUl, q.GbrDl, q.PriorityLevel, q.Arp, q.DefQosFlowIndication)
}

func SessRuleString(s *models.SessionRule) string {
	if s == nil {
		return ""
	}
	return fmt.Sprintf("SessRule:[RuleId:[%v], Ambr:[Dl:[%v], Ul:[%v]], AuthDefQos:[Var5QI:[%v], PriorityLevel:[%v], ARP:[%v]]]",
		s.SessRuleId, s.AuthSessAmbr.Downlink, s.AuthSessAmbr.Uplink, s.AuthDefQos.Var5qi, s.AuthDefQos.PriorityLevel, s.AuthDefQos.Arp)
}

func PccRuleString(pcc *models.PccRule) string {
	if pcc == nil {
		return ""
	}

	return fmt.Sprintf("PccRule:[RuleId:[%v], Precdence:[%v], RefQosData:[%v], flow:[%v]]",
		pcc.PccRuleId, pcc.Precedence, pcc.RefQosData[0], PccFlowInfosString(pcc.FlowInfos))
}

func TCDataString(tcData *models.TrafficControlData) string {
	return fmt.Sprintf("TC Data:[Id:[%v], FlowStatus:[%v]]", tcData.TcId, tcData.FlowStatus)
}

func PccFlowInfosString(flows []models.FlowInformation) []string {
	var flowStrs []string
	for _, flow := range flows {
		str := fmt.Sprintf("\nFlowInfo:[flowDesc:[%v], PFId:[%v], direction:[%v]]",
			flow.FlowDescription, flow.PackFiltId, flow.FlowDirection)

		flowStrs = append(flowStrs, str)
	}
	return flowStrs
}

func (obj QoSFlowDescription) String() string {
	return fmt.Sprintf("QosFlowDesc:[QFI:[%v], OpCode:[%v], FlowParam:[%v]], ", obj.Qfi, obj.OpCode, obj.ParamList)
}

func (obj QosFlowParameter) String() string {
	return fmt.Sprintf("QFParam:[Id:[%v], Len:[%v], content:[%v]]", obj.ParamId, obj.ParamLen, obj.ParamContent)
}

func (obj PolicyUpdate) String() string {
	return fmt.Sprintf("Policy Update:[\nPccRule:[%v], \nSessRules:[%v], \nQosData:[%v], \nTcData:[%v]]",
		obj.PccRuleUpdate, obj.SessRuleUpdate, obj.QosFlowUpdate, obj.TCUpdate)
}

func (obj PccRulesUpdate) String() string {
	str := "\nPCC Rule Changes:"

	// To be added
	strAdd := ""
	for name, rule := range obj.add {
		strAdd += fmt.Sprintf("\n[name:[%v], %v", name, PccRuleString(rule))
	}
	str += fmt.Sprintf("\n[to add:[%v]]", strAdd)

	// To be modified
	strMod := ""
	for name, rule := range obj.mod {
		strMod += fmt.Sprintf("\n[name:[%v], %v", name, PccRuleString(rule))
	}
	str += fmt.Sprintf("\n[to mod:[%v]]", strMod)

	// To be deleted
	strDel := ""
	for name, rule := range obj.del {
		strDel += fmt.Sprintf("\n[name:[%v], %v", name, PccRuleString(rule))
	}
	str += fmt.Sprintf("\n[to del:[%v]]", strDel)

	return str
}

func (obj SessRulesUpdate) String() string {
	str := "\nSess Rule Changes:"

	// To be added
	strAdd := ""
	for name, rule := range obj.add {
		strAdd += fmt.Sprintf("\n[name:[%v], %v", name, SessRuleString(rule))
	}
	str += fmt.Sprintf("\n[to add:[%v]]", strAdd)

	// To be modified
	strMod := ""
	for name, rule := range obj.mod {
		strMod += fmt.Sprintf("\n[name:[%v], %v", name, SessRuleString(rule))
	}
	str += fmt.Sprintf("\n[to mod:[%v]]", strMod)

	// To be deleted
	strDel := ""
	for name, rule := range obj.del {
		strDel += fmt.Sprintf("\n[name:[%v], %v", name, SessRuleString(rule))
	}
	str += fmt.Sprintf("\n[to del:[%v]]", strDel)

	return str
}

func (obj QosFlowsUpdate) String() string {
	str := "\nQos Data Changes:"

	// To be added
	strAdd := ""
	for name, val := range obj.add {
		strAdd += fmt.Sprintf("\n[name:[%v], %v", name, QosDataString(val))
	}
	str += fmt.Sprintf("\n[to add:[%v]]", strAdd)

	// To be modified
	strMod := ""
	for name, val := range obj.mod {
		strMod += fmt.Sprintf("\n[name:[%v], %v", name, QosDataString(val))
	}
	str += fmt.Sprintf("\n[to mod:[%v]]", strMod)

	// To be deleted
	strDel := ""
	for name, val := range obj.del {
		strDel += fmt.Sprintf("\n[name:[%v], %v", name, QosDataString(val))
	}
	str += fmt.Sprintf("\n[to del:[%v]]", strDel)

	return str
}

func (obj TrafficControlUpdate) String() string {
	str := "\nTC Data Changes:"

	// To be added
	strAdd := ""
	for name, val := range obj.add {
		strAdd += fmt.Sprintf("\n[name:[%v], %v", name, TCDataString(val))
	}
	str += fmt.Sprintf("\n[to add:[%v]]", strAdd)

	// To be modified
	strMod := ""
	for name, val := range obj.mod {
		strMod += fmt.Sprintf("\n[name:[%v], %v", name, TCDataString(val))
	}
	str += fmt.Sprintf("\n[to mod:[%v]]", strMod)

	// To be deleted
	strDel := ""
	for name, val := range obj.del {
		strDel += fmt.Sprintf("\n[name:[%v], %v", name, TCDataString(val))
	}
	str += fmt.Sprintf("\n[to del:[%v]]", strDel)

	return str
}

// TestMakeSamplePolicyDecision - Locally generate SM Policy Decision
func TestMakeSamplePolicyDecision() *models.SmPolicyDecision {
	smPolDec := &models.SmPolicyDecision{
		PccRules:      TestMakePccRules(),
		SessRules:     TestMakeSessionRule(),
		QosDecs:       TestMakeQosData(),
		TraffContDecs: TestMakeTrafficControlData(),
	}

	return smPolDec
}

// TestMakePccRules - Locally generate PCC Rule data
func TestMakePccRules() map[string]*models.PccRule {
	pccRuleDef := models.PccRule{
		PccRuleId:  "255",
		Precedence: 255,
		RefQosData: []string{"QosData1"},
		RefTcData:  []string{"TC1"},
		FlowInfos:  make([]models.FlowInformation, 0),
	}

	flowInfosDef := []models.FlowInformation{
		{
			FlowDescription:   "permit out ip from any to assigned",
			PackFiltId:        "1",
			PacketFilterUsage: true,
			FlowDirection:     models.FlowDirectionRm_BIDIRECTIONAL,
		},
	}

	pccRuleDef.FlowInfos = append(pccRuleDef.FlowInfos, flowInfosDef...)

	pccRule1 := models.PccRule{
		PccRuleId:  "1",
		Precedence: 111,
		RefQosData: []string{"QosData1"},
		RefTcData:  []string{"TC1"},
		FlowInfos:  make([]models.FlowInformation, 0),
	}

	flowInfos := []models.FlowInformation{
		{
			FlowDescription:   "permit out ip from 1.1.1.1 1000-1200 to assigned",
			PackFiltId:        "1",
			PacketFilterUsage: true,
			FlowDirection:     models.FlowDirectionRm_BIDIRECTIONAL,
		},
		{
			FlowDescription:   "permit out 17 from 3.3.3.3/24 3000 to 4.4.4.4/24 4000",
			PackFiltId:        "2",
			PacketFilterUsage: true,
			FlowDirection:     models.FlowDirectionRm_BIDIRECTIONAL,
		},
	}

	pccRule1.FlowInfos = append(pccRule1.FlowInfos, flowInfos...)

	pccRule2 := models.PccRule{
		PccRuleId:  "2",
		Precedence: 222,
		RefQosData: []string{"QosData2"},
		RefTcData:  []string{"TC2"},
		FlowInfos:  make([]models.FlowInformation, 0),
	}

	flowInfos1 := []models.FlowInformation{
		{
			FlowDescription:   "permit out ip from 5.5.5.5 1000-1200 to assigned",
			PackFiltId:        "1",
			PacketFilterUsage: true,
			FlowDirection:     models.FlowDirectionRm_BIDIRECTIONAL,
		},
		{
			FlowDescription:   "permit out 17 from 3.3.3.3/24 3000 to 4.4.4.4/24 4000",
			PackFiltId:        "2",
			PacketFilterUsage: true,
			FlowDirection:     models.FlowDirectionRm_BIDIRECTIONAL,
		},
	}

	pccRule2.FlowInfos = append(pccRule2.FlowInfos, flowInfos1...)

	return map[string]*models.PccRule{"PccRule1": &pccRule1, "PccRule2": &pccRule2, "PccRuleDef": &pccRuleDef}
}

// TestMakeQosData - Locally generate Qos Flow data
func TestMakeQosData() map[string]*models.QosData {
	qosData1 := models.QosData{
		QosId:                "1",
		Var5qi:               9,
		MaxbrUl:              "101 Mbps",
		MaxbrDl:              "201 Mbps",
		GbrUl:                "11 Mbps",
		GbrDl:                "21 Mbps",
		PriorityLevel:        5,
		DefQosFlowIndication: true,
		Arp: &models.Arp{
			PriorityLevel: 3,
			PreemptCap:    models.PreemptionCapability_MAY_PREEMPT,
			PreemptVuln:   models.PreemptionVulnerability_PREEMPTABLE,
		},
	}

	qosData2 := models.QosData{
		QosId:                "2",
		Var5qi:               9,
		MaxbrUl:              "301 Mbps",
		MaxbrDl:              "401 Mbps",
		GbrUl:                "31 Mbps",
		GbrDl:                "41 Mbps",
		PriorityLevel:        3,
		DefQosFlowIndication: false,
		Arp: &models.Arp{
			PriorityLevel: 3,
			PreemptCap:    models.PreemptionCapability_NOT_PREEMPT,
			PreemptVuln:   models.PreemptionVulnerability_NOT_PREEMPTABLE,
		},
	}

	return map[string]*models.QosData{
		"QosData1": &qosData1,
		"QosData2": &qosData2,
	}
}

// TestMakeSessionRule - Locally generate Qos Flow data
func TestMakeSessionRule() map[string]*models.SessionRule {
	sessRule1 := models.SessionRule{
		SessRuleId: "RuleId-1",
		AuthSessAmbr: &models.Ambr{
			Uplink:   "77 Mbps",
			Downlink: "99 Mbps",
		},
		AuthDefQos: &models.AuthorizedDefaultQos{
			Var5qi: 9,
			Arp: &models.Arp{
				PriorityLevel: 8,
				PreemptCap:    models.PreemptionCapability_MAY_PREEMPT,
				PreemptVuln:   models.PreemptionVulnerability_NOT_PREEMPTABLE,
			},
			PriorityLevel: 8,
		},
	}
	sessRule2 := models.SessionRule{
		SessRuleId: "RuleId-2",
		AuthSessAmbr: &models.Ambr{
			Uplink:   "55 Mbps",
			Downlink: "33 Mbps",
		},
		AuthDefQos: &models.AuthorizedDefaultQos{
			Var5qi: 9,
			Arp: &models.Arp{
				PriorityLevel: 7,
				PreemptCap:    models.PreemptionCapability_MAY_PREEMPT,
				PreemptVuln:   models.PreemptionVulnerability_NOT_PREEMPTABLE,
			},
			PriorityLevel: 7,
		},
	}

	return map[string]*models.SessionRule{
		"SessRule1": &sessRule1,
		"SessRule2": &sessRule2,
	}
}

// TestMakeTrafficControlData - Locally generate Traffic Control data
func TestMakeTrafficControlData() map[string]*models.TrafficControlData {
	tc1 := models.TrafficControlData{
		TcId:       "TC1",
		FlowStatus: models.FlowStatus_ENABLED,
	}

	tc2 := models.TrafficControlData{
		TcId:       "TC2",
		FlowStatus: models.FlowStatus_ENABLED,
	}

	return map[string]*models.TrafficControlData{"TC1": &tc1, "TC2": &tc2}
}
