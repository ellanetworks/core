// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// QoSRuleOperation is a QoS rule operation code: bits 8-6 of the first octet of
// a rule's content (TS 24.501 §9.11.4.13, table 9.11.4.13.1). Codes 0 and 7 are
// reserved.
type QoSRuleOperation uint8

// QoS rule operation codes (TS 24.501 table 9.11.4.13.1).
const (
	QoSRuleOpCreate               QoSRuleOperation = 1 // create new QoS rule
	QoSRuleOpDelete               QoSRuleOperation = 2 // delete existing QoS rule
	QoSRuleOpModifyAddFilters     QoSRuleOperation = 3 // modify and add packet filters
	QoSRuleOpModifyReplaceFilters QoSRuleOperation = 4 // modify and replace all packet filters
	QoSRuleOpModifyDeleteFilters  QoSRuleOperation = 5 // modify and delete packet filters
	QoSRuleOpModifyWithoutFilters QoSRuleOperation = 6 // modify without modifying packet filters
)

var qosRuleOperationNames = map[uint8]string{
	uint8(QoSRuleOpCreate):               "create",
	uint8(QoSRuleOpDelete):               "delete",
	uint8(QoSRuleOpModifyReplaceFilters): "modify, replace all packet filters",
	uint8(QoSRuleOpModifyAddFilters):     "modify, add packet filters",
	uint8(QoSRuleOpModifyDeleteFilters):  "modify, delete packet filters",
	uint8(QoSRuleOpModifyWithoutFilters): "modify without modifying packet filters",
}

// Name returns the operation's spec description, or the empty string when the
// value is not one TS 24.501 assigns.
func (o QoSRuleOperation) Name() string { return qosRuleOperationNames[uint8(o)] }

func (o QoSRuleOperation) String() string { return enumString(uint8(o), qosRuleOperationNames) }

// PacketFilterDirection is the direction a packet filter applies in
// (TS 24.501 §9.11.4.13, figure 9.11.4.13.4, bits 6-5). Value 0 is reserved.
type PacketFilterDirection uint8

// Packet filter directions (TS 24.501 table 9.11.4.13.1).
const (
	PacketFilterDownlink      PacketFilterDirection = 1
	PacketFilterUplink        PacketFilterDirection = 2
	PacketFilterBidirectional PacketFilterDirection = 3
)

var packetFilterDirectionNames = map[uint8]string{
	uint8(PacketFilterDownlink):      "downlink only",
	uint8(PacketFilterUplink):        "uplink only",
	uint8(PacketFilterBidirectional): "bidirectional",
}

// Name returns the direction's spec description, or the empty string for the
// value TS 24.501 table 9.11.4.13.1 reserves.
func (d PacketFilterDirection) Name() string { return packetFilterDirectionNames[uint8(d)] }

func (d PacketFilterDirection) String() string {
	return enumString(uint8(d), packetFilterDirectionNames)
}

// PacketFilterComponentType is a packet filter component's type octet
// (TS 24.501 §9.11.4.13, table 9.11.4.13.1).
type PacketFilterComponentType uint8

// Packet filter component types (TS 24.501 table 9.11.4.13.1).
const (
	pfComponentTypeMatchAll            PacketFilterComponentType = 0x01
	pfComponentTypeIPv4RemoteAddress   PacketFilterComponentType = 0x10
	pfComponentTypeIPv4LocalAddress    PacketFilterComponentType = 0x11
	pfComponentTypeIPv6RemoteAddress   PacketFilterComponentType = 0x21
	pfComponentTypeIPv6LocalAddress    PacketFilterComponentType = 0x23
	pfComponentTypeProtocolIdentifier  PacketFilterComponentType = 0x30
	pfComponentTypeSingleLocalPort     PacketFilterComponentType = 0x40
	pfComponentTypeLocalPortRange      PacketFilterComponentType = 0x41
	pfComponentTypeSingleRemotePort    PacketFilterComponentType = 0x50
	pfComponentTypeRemotePortRange     PacketFilterComponentType = 0x51
	pfComponentTypeSecurityParamIndex  PacketFilterComponentType = 0x60
	pfComponentTypeTypeOfService       PacketFilterComponentType = 0x70
	pfComponentTypeFlowLabel           PacketFilterComponentType = 0x80
	pfComponentTypeDestinationMAC      PacketFilterComponentType = 0x81
	pfComponentTypeSourceMAC           PacketFilterComponentType = 0x82
	pfComponentTypeCTAGVID             PacketFilterComponentType = 0x83
	pfComponentTypeSTAGVID             PacketFilterComponentType = 0x84
	pfComponentTypeCTAGPCPDEI          PacketFilterComponentType = 0x85
	pfComponentTypeSTAGPCPDEI          PacketFilterComponentType = 0x86
	pfComponentTypeEthertype           PacketFilterComponentType = 0x87
	pfComponentTypeDestinationMACRange PacketFilterComponentType = 0x88
	pfComponentTypeSourceMACRange      PacketFilterComponentType = 0x89
)

var packetFilterComponentTypeNames = map[PacketFilterComponentType]string{
	pfComponentTypeMatchAll:            "Match-all type",
	pfComponentTypeIPv4RemoteAddress:   "IPv4 remote address type",
	pfComponentTypeIPv4LocalAddress:    "IPv4 local address type",
	pfComponentTypeIPv6RemoteAddress:   "IPv6 remote address/prefix length type",
	pfComponentTypeIPv6LocalAddress:    "IPv6 local address/prefix length type",
	pfComponentTypeProtocolIdentifier:  "Protocol identifier/Next header type",
	pfComponentTypeSingleLocalPort:     "Single local port type",
	pfComponentTypeLocalPortRange:      "Local port range type",
	pfComponentTypeSingleRemotePort:    "Single remote port type",
	pfComponentTypeRemotePortRange:     "Remote port range type",
	pfComponentTypeSecurityParamIndex:  "Security parameter index type",
	pfComponentTypeTypeOfService:       "Type of service/Traffic class type",
	pfComponentTypeFlowLabel:           "Flow label type",
	pfComponentTypeDestinationMAC:      "Destination MAC address type",
	pfComponentTypeSourceMAC:           "Source MAC address type",
	pfComponentTypeCTAGVID:             "802.1Q C-TAG VID type",
	pfComponentTypeSTAGVID:             "802.1Q S-TAG VID type",
	pfComponentTypeCTAGPCPDEI:          "802.1Q C-TAG PCP/DEI type",
	pfComponentTypeSTAGPCPDEI:          "802.1Q S-TAG PCP/DEI type",
	pfComponentTypeEthertype:           "Ethertype type",
	pfComponentTypeDestinationMACRange: "Destination MAC address range type",
	pfComponentTypeSourceMACRange:      "Source MAC address range type",
}

// Name returns the component type's spec description, or the empty string for a
// value TS 24.501 table 9.11.4.13.1 reserves.
func (t PacketFilterComponentType) Name() string { return packetFilterComponentTypeNames[t] }

func (t PacketFilterComponentType) String() string {
	if name, ok := packetFilterComponentTypeNames[t]; ok {
		return name
	}

	return fmt.Sprintf("reserved packet filter component type (%#x)", uint8(t))
}

// QoS rule and packet-filter coding (TS 24.501 §9.11.4.13, table 9.11.4.13.1).
const (
	maxPacketFiltersPerRule = 15
	// maxQoSFlowParameters is the largest number of parameters a flow description
	// can declare: the count is a 6-bit field (TS 24.501 §9.11.4.12).
	maxQoSFlowParameters = 0x3F
	maxPacketFilterID    = 15
	maxPacketFilterDir   = 3
)

// ruleParametersRequired and ruleParametersForbidden report what TS 24.501
// §9.11.4.13 says about the QoS rule precedence and segregation/QFI octets for an
// operation code: "create new QoS rule" shall include them, "delete existing QoS
// rule" shall not, and the four modify operations may do either, which is why
// figure 9.11.4.13.2 marks octets m+1 and m+2 conditional.
func ruleParametersRequired(op QoSRuleOperation) bool  { return op == QoSRuleOpCreate }
func ruleParametersForbidden(op QoSRuleOperation) bool { return op == QoSRuleOpDelete }

// filtersAreIdentifiersOnly reports whether a rule's packet filter list is a
// sequence of bare one-octet packet filter identifiers rather than full filters.
// TS 24.501 figure 9.11.4.13.3 uses that form for the "modify existing QoS rule
// and delete packet filters" operation only.
func filtersAreIdentifiersOnly(op QoSRuleOperation) bool { return op == QoSRuleOpModifyDeleteFilters }

// PacketFilterComponent is one component of a packet filter: a component type
// and its fixed-length value (TS 24.501 §9.11.4.13, table 9.11.4.13.1). The
// match-all component (type 0x01) carries no value.
type PacketFilterComponent struct {
	Type  PacketFilterComponentType
	Value []byte
}

// PacketFilter is one packet filter of a QoS rule (TS 24.501 §9.11.4.13).
type PacketFilter struct {
	Identifier uint8 // 0-15
	Direction  PacketFilterDirection
	Components []PacketFilterComponent
}

// QoSRuleParameters are the QoS rule precedence and segregation/QFI octets,
// which TS 24.501 figure 9.11.4.13.2 shows as octets m+1 and m+2. They are
// present or absent as a pair.
type QoSRuleParameters struct {
	Precedence  uint8
	QFI         uint8
	Segregation uint8
}

// QoSRule is a single authorized QoS rule (TS 24.501 §9.11.4.13). Parameters is
// nil when the rule carries neither the precedence nor the segregation/QFI octet,
// which every operation but "create new QoS rule" is allowed to do.
type QoSRule struct {
	Identifier    uint8 // QRI, 1-255
	OperationCode QoSRuleOperation
	DQR           uint8
	Parameters    *QoSRuleParameters
	Filters       []PacketFilter
}

// DefaultQoSRule builds the match-all default QoS rule bound to qfi.
func DefaultQoSRule(id, qfi uint8) QoSRule {
	return QoSRule{
		Identifier:    id,
		DQR:           0x01,
		OperationCode: QoSRuleOpCreate,
		Parameters:    &QoSRuleParameters{Precedence: 255, QFI: qfi},
		Filters: []PacketFilter{
			{Identifier: 1, Direction: PacketFilterBidirectional, Components: []PacketFilterComponent{{Type: pfComponentTypeMatchAll}}},
		},
	}
}

func (f PacketFilter) marshal(w *nas.Writer) {
	// Direction is 2 bits at bits 6-5 and the identifier 4 bits at bits 4-1
	// (TS 24.501 figure 9.11.4.13.4); masking keeps an out-of-range field from
	// silently corrupting its neighbour.
	w.U8(uint8(f.Direction)&maxPacketFilterDir<<4 | f.Identifier&maxPacketFilterID)
	w.LVFunc(func(c *nas.Writer) {
		for _, comp := range f.Components {
			c.U8(uint8(comp.Type))
			c.Raw(comp.Value)
		}
	})
}

func (r QoSRule) marshal(w *nas.Writer) error {
	if len(r.Filters) > maxPacketFiltersPerRule {
		return fmt.Errorf("nas/fgs: QoS rule %d has %d packet filters, the field holds %d",
			r.Identifier, len(r.Filters), maxPacketFiltersPerRule)
	}

	if ruleParametersForbidden(r.OperationCode) && len(r.Filters) > 0 {
		return fmt.Errorf("nas/fgs: QoS rule %d deletes the rule, so its packet filter list must be empty", r.Identifier)
	}

	if ruleParametersForbidden(r.OperationCode) && r.Parameters != nil {
		return fmt.Errorf("nas/fgs: QoS rule %d deletes the rule, so it carries no precedence and no QFI", r.Identifier)
	}

	if ruleParametersRequired(r.OperationCode) && r.Parameters == nil {
		return fmt.Errorf("nas/fgs: QoS rule %d creates a rule, so it needs a precedence and a QFI", r.Identifier)
	}

	w.U8(r.Identifier)
	w.LVEFunc(func(c *nas.Writer) {
		c.U8(uint8(r.OperationCode)&0x07<<5 | r.DQR&0x01<<4 | uint8(len(r.Filters)))

		for _, f := range r.Filters {
			if filtersAreIdentifiersOnly(r.OperationCode) {
				c.U8(f.Identifier & maxPacketFilterID)
				continue
			}

			f.marshal(c)
		}

		if r.Parameters == nil {
			return
		}

		c.U8(r.Parameters.Precedence)
		c.U8(r.Parameters.Segregation&0x01<<6 | r.Parameters.QFI&0x3F)
	})

	return nil
}

// QoSRules is the authorized QoS rules IE value: the content of the LV-E / TLV-E,
// without IEI or length (TS 24.501 §9.11.4.13).
type QoSRules []QoSRule

// AppendBinary encodes the authorized QoS rules IE value onto b.
func (rs QoSRules) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	for _, r := range rs {
		if err := r.marshal(w); err != nil {
			return b, err
		}
	}

	return w.Result(b)
}

// MarshalBinary encodes the authorized QoS rules IE value.
func (rs QoSRules) MarshalBinary() ([]byte, error) { return rs.AppendBinary(nil) }

// QoSFlowOperation is a QoS flow description operation code, held unshifted; it
// occupies bits 8-6 of the operation octet (TS 24.501 §9.11.4.12,
// table 9.11.4.12.1).
type QoSFlowOperation uint8

// QoS flow description operation codes (TS 24.501 table 9.11.4.12.1).
const (
	QoSFlowOpCreate QoSFlowOperation = 1 // create new QoS flow description
	QoSFlowOpDelete QoSFlowOperation = 2 // delete existing QoS flow description
	QoSFlowOpModify QoSFlowOperation = 3 // modify existing QoS flow description
)

var qosFlowOperationNames = map[uint8]string{
	uint8(QoSFlowOpCreate): "create",
	uint8(QoSFlowOpDelete): "delete",
	uint8(QoSFlowOpModify): "modify",
}

// Name returns the operation's spec description, or the empty string when the
// value is not one TS 24.501 assigns.
func (o QoSFlowOperation) Name() string { return qosFlowOperationNames[uint8(o)] }

func (o QoSFlowOperation) String() string { return enumString(uint8(o), qosFlowOperationNames) }

// qosFlowOpShift is the position of the operation code in its octet.
const qosFlowOpShift = 5

// QoS flow description coding (TS 24.501 §9.11.4.12, table 9.11.4.12.1).
const (
	qfdParam5QI   uint8 = 0x01
	qfdQFIBitmask uint8 = 0x3F
	qfdEBit       uint8 = 0x40
)

// FiveQIQoSFlow builds a QoS flow description carrying the 5QI parameter with
// the given operation code. The E bit is set: the parameter list replaces the
// flow's entire parameter set (TS 24.501 §9.11.4.12).
func FiveQIQoSFlow(qfi, fiveQI uint8, opCode QoSFlowOperation) QoSFlowDescription {
	return QoSFlowDescription{
		QFI:           qfi & qfdQFIBitmask,
		OperationCode: opCode,
		EBit:          true,
		Parameters:    []QoSFlowParameter{{ID: QoSFlowParam5QI, Value: []byte{fiveQI}}},
	}
}

// valueLength returns the fixed value-field length in octets of this component
// type, and whether the type is one TS 24.501 table 9.11.4.13.1 assigns.
func (t PacketFilterComponentType) valueLength() (int, bool) {
	switch t {
	case pfComponentTypeMatchAll:
		return 0, true
	case pfComponentTypeIPv4RemoteAddress, pfComponentTypeIPv4LocalAddress: // address + mask
		return 8, true
	case pfComponentTypeIPv6RemoteAddress, pfComponentTypeIPv6LocalAddress: // address + prefix length
		return 17, true
	case pfComponentTypeProtocolIdentifier:
		return 1, true
	case pfComponentTypeSingleLocalPort, pfComponentTypeSingleRemotePort:
		return 2, true
	case pfComponentTypeLocalPortRange, pfComponentTypeRemotePortRange:
		return 4, true
	case pfComponentTypeSecurityParamIndex:
		return 4, true
	case pfComponentTypeTypeOfService: // value + mask
		return 2, true
	case pfComponentTypeFlowLabel: // 20 bits
		return 3, true
	case pfComponentTypeDestinationMAC, pfComponentTypeSourceMAC:
		return 6, true
	case pfComponentTypeCTAGVID, pfComponentTypeSTAGVID:
		return 2, true
	case pfComponentTypeCTAGPCPDEI, pfComponentTypeSTAGPCPDEI:
		return 1, true
	case pfComponentTypeEthertype:
		return 2, true
	case pfComponentTypeDestinationMACRange, pfComponentTypeSourceMACRange: // low and high limits
		return 12, true
	default:
		return 0, false
	}
}

// ParseQoSRules decodes the authorized QoS rules IE content — a sequence of QoS
// rules (TS 24.501 §9.11.4.13, table 9.11.4.13.1).
func ParseQoSRules(b []byte) (QoSRules, error) {
	r := nas.NewReader(b)

	out := QoSRules{}

	for r.Remaining() > 0 {
		rule, err := parseQoSRule(r)
		if err != nil {
			return nil, err
		}

		out = append(out, rule)
	}

	return out, nil
}

func parseQoSRule(r *nas.Reader) (QoSRule, error) {
	id, err := r.U8()
	if err != nil {
		return QoSRule{}, err
	}

	length, err := r.U16()
	if err != nil {
		return QoSRule{}, err
	}

	content, err := r.Bytes(int(length))
	if err != nil {
		return QoSRule{}, err
	}

	cr := nas.NewReader(content)

	hdr, err := cr.U8()
	if err != nil {
		return QoSRule{}, err
	}

	rule := QoSRule{Identifier: id, OperationCode: QoSRuleOperation(hdr >> 5 & 0x07), DQR: hdr >> 4 & 0x01}

	// TS 24.501 §9.11.4.13: the "delete existing QoS rule" operation sets the rule
	// length to one, so it can carry no packet filters.
	if ruleParametersForbidden(rule.OperationCode) && hdr&0x0F != 0 {
		return QoSRule{}, fmt.Errorf("nas/fgs: QoS rule %d deletes the rule but lists %d packet filters", id, hdr&0x0F)
	}

	for i := 0; i < int(hdr&0x0F); i++ {
		if filtersAreIdentifiersOnly(rule.OperationCode) {
			// Figure 9.11.4.13.3: the list is bare packet filter identifiers.
			pfid, err := cr.U8()
			if err != nil {
				return QoSRule{}, fmt.Errorf("nas/fgs: packet filter identifier %d: %w", i, err)
			}

			rule.Filters = append(rule.Filters, PacketFilter{Identifier: pfid & maxPacketFilterID})

			continue
		}

		pf, err := parsePacketFilter(cr)
		if err != nil {
			return QoSRule{}, fmt.Errorf("nas/fgs: packet filter %d: %w", i, err)
		}

		rule.Filters = append(rule.Filters, pf)
	}

	if ruleParametersForbidden(rule.OperationCode) {
		if cr.Remaining() != 0 {
			return QoSRule{}, fmt.Errorf("nas/fgs: QoS rule %d deletes the rule but carries %d further octets", id, cr.Remaining())
		}

		return rule, nil
	}

	if cr.Remaining() == 0 && !ruleParametersRequired(rule.OperationCode) {
		return rule, nil
	}

	prec, err := cr.U8()
	if err != nil {
		return QoSRule{}, fmt.Errorf("nas/fgs: QoS rule %d precedence: %w", id, err)
	}

	segQFI, err := cr.U8()
	if err != nil {
		return QoSRule{}, fmt.Errorf("nas/fgs: QoS rule %d segregation and QFI: %w", id, err)
	}

	rule.Parameters = &QoSRuleParameters{
		Precedence:  prec,
		QFI:         segQFI & 0x3F,
		Segregation: segQFI >> 6 & 0x01,
	}

	return rule, nil
}

func parsePacketFilter(r *nas.Reader) (PacketFilter, error) {
	h, err := r.U8()
	if err != nil {
		return PacketFilter{}, err
	}

	length, err := r.U8()
	if err != nil {
		return PacketFilter{}, err
	}

	content, err := r.Bytes(int(length))
	if err != nil {
		return PacketFilter{}, err
	}

	// Direction is bits 6-5; bits 8-7 are spare (figure 9.11.4.13.4).
	pf := PacketFilter{Direction: PacketFilterDirection(h >> 4 & maxPacketFilterDir), Identifier: h & maxPacketFilterID}
	cr := nas.NewReader(content)

	for cr.Remaining() > 0 {
		t, err := cr.U8()
		if err != nil {
			return PacketFilter{}, err
		}

		valLen, known := PacketFilterComponentType(t).valueLength()
		if !known {
			// An unknown component type has no derivable length; keep the remainder opaque.
			rest, err := cr.Bytes(cr.Remaining())
			if err != nil {
				return PacketFilter{}, err
			}

			pf.Components = append(pf.Components, PacketFilterComponent{Type: PacketFilterComponentType(t), Value: rest})

			break
		}

		var val []byte
		if valLen > 0 {
			if val, err = cr.Bytes(valLen); err != nil {
				return PacketFilter{}, err
			}
		}

		pf.Components = append(pf.Components, PacketFilterComponent{Type: PacketFilterComponentType(t), Value: val})
	}

	return pf, nil
}

// QoSFlowParameterID identifies one parameter of a QoS flow description
// (TS 24.501 §9.11.4.12, table 9.11.4.12.1).
type QoSFlowParameterID uint8

// QoS flow description parameter identifiers (TS 24.501 table 9.11.4.12.1).
const (
	QoSFlowParam5QI             QoSFlowParameterID = 0x01
	QoSFlowParamGFBRUplink      QoSFlowParameterID = 0x02
	QoSFlowParamGFBRDownlink    QoSFlowParameterID = 0x03
	QoSFlowParamMFBRUplink      QoSFlowParameterID = 0x04
	QoSFlowParamMFBRDownlink    QoSFlowParameterID = 0x05
	QoSFlowParamAveragingWindow QoSFlowParameterID = 0x06
	QoSFlowParamEPSBearerID     QoSFlowParameterID = 0x07
)

var qosFlowParameterIDNames = map[uint8]string{
	uint8(QoSFlowParam5QI):             "5QI",
	uint8(QoSFlowParamGFBRUplink):      "GFBR uplink",
	uint8(QoSFlowParamGFBRDownlink):    "GFBR downlink",
	uint8(QoSFlowParamMFBRUplink):      "MFBR uplink",
	uint8(QoSFlowParamMFBRDownlink):    "MFBR downlink",
	uint8(QoSFlowParamAveragingWindow): "Averaging window",
	uint8(QoSFlowParamEPSBearerID):     "EPS bearer identity",
}

// Name returns the parameter's spec description, or the empty string when the
// identifier is not one TS 24.501 assigns.
func (i QoSFlowParameterID) Name() string { return qosFlowParameterIDNames[uint8(i)] }

func (i QoSFlowParameterID) String() string { return enumString(uint8(i), qosFlowParameterIDNames) }

// QoS flow bit-rate units (TS 24.501 §9.11.4.12): the unit octet of a GFBR/MFBR
// parameter value.
const (
	qosRateUnit1Kbps uint8 = 0x01
	qosRateUnit1Mbps uint8 = 0x06
	qosRateUnit1Gbps uint8 = 0x0B
)

// QoSFlowParameter is one parameter of a QoS flow description: its identifier and
// raw value (TS 24.501 §9.11.4.12).
type QoSFlowParameter struct {
	ID    QoSFlowParameterID
	Value []byte
}

// QoSFlowDescription is one authorized QoS flow description (TS 24.501 §9.11.4.12).
type QoSFlowDescription struct {
	QFI           uint8
	OperationCode QoSFlowOperation
	EBit          bool
	Parameters    []QoSFlowParameter
}

// QoSFlowDescriptions is the authorized QoS flow descriptions IE value: the
// content of the LV-E / TLV-E, without IEI or length (TS 24.501 §9.11.4.12).
type QoSFlowDescriptions []QoSFlowDescription

// epsBearerIDShift is the position of the EPS bearer identity in its parameter
// octet: TS 24.501 §9.11.4.12 puts it in bits 5 to 8, with bits 1 to 4 spare, so
// it sits in the high nibble and not the low one.
const epsBearerIDShift = 4

// EPSBearerIDQoSFlowParameter builds the EPS bearer identity parameter of a QoS
// flow description (TS 24.501 §9.11.4.12): one octet carrying the EBI in bits 5
// to 8, bits 1 to 4 coded as zero.
func EPSBearerIDQoSFlowParameter(ebi uint8) (QoSFlowParameter, error) {
	if ebi > 0x0F {
		return QoSFlowParameter{}, fmt.Errorf("nas/fgs: EPS bearer identity %d does not fit four bits", ebi)
	}

	return QoSFlowParameter{ID: QoSFlowParamEPSBearerID, Value: []byte{ebi << epsBearerIDShift}}, nil
}

// EPSBearerID returns the EPS bearer identity the flow's parameters carry and
// whether one was present and well formed.
func (d QoSFlowDescription) EPSBearerID() (uint8, bool) {
	for _, p := range d.Parameters {
		if p.ID == QoSFlowParamEPSBearerID && len(p.Value) == 1 {
			return p.Value[0] >> epsBearerIDShift, true
		}
	}

	return 0, false
}

// AppendBinary encodes the authorized QoS flow descriptions IE value onto b.
func (ds QoSFlowDescriptions) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	for _, d := range ds {
		if len(d.Parameters) > maxQoSFlowParameters {
			return b, fmt.Errorf("nas/fgs: QoS flow %d has %d parameters, the field holds %d",
				d.QFI, len(d.Parameters), maxQoSFlowParameters)
		}

		w.U8(d.QFI & qfdQFIBitmask)
		w.U8(uint8(d.OperationCode) & 0x07 << qosFlowOpShift)

		num := uint8(len(d.Parameters))
		if d.EBit {
			num |= qfdEBit
		}

		w.U8(num)

		for _, p := range d.Parameters {
			w.U8(uint8(p.ID))
			w.LV(p.Value)
		}
	}

	return w.Result(b)
}

// MarshalBinary encodes the authorized QoS flow descriptions IE value.
func (ds QoSFlowDescriptions) MarshalBinary() ([]byte, error) { return ds.AppendBinary(nil) }

// ParseQoSFlowDescriptions decodes the authorized QoS flow descriptions
// IE content (TS 24.501 §9.11.4.12, table 9.11.4.12.1).
func ParseQoSFlowDescriptions(b []byte) (QoSFlowDescriptions, error) {
	r := nas.NewReader(b)

	// Non-nil even when empty: nil is how a message field records an absent
	// element, so an element present with a zero-length value must not decode
	// to it.
	out := QoSFlowDescriptions{}

	for r.Remaining() > 0 {
		qfiOctet, err := r.U8()
		if err != nil {
			return nil, err
		}

		opOctet, err := r.U8()
		if err != nil {
			return nil, err
		}

		numOctet, err := r.U8()
		if err != nil {
			return nil, err
		}

		d := QoSFlowDescription{
			QFI:           qfiOctet & qfdQFIBitmask,
			OperationCode: QoSFlowOperation(opOctet >> qosFlowOpShift & 0x07),
			EBit:          numOctet&qfdEBit != 0,
		}

		for p := 0; p < int(numOctet&0x3F); p++ {
			id, err := r.U8()
			if err != nil {
				return nil, err
			}

			plen, err := r.U8()
			if err != nil {
				return nil, err
			}

			val, err := r.Bytes(int(plen))
			if err != nil {
				return nil, err
			}

			d.Parameters = append(d.Parameters, QoSFlowParameter{ID: QoSFlowParameterID(id), Value: val})
		}

		out = append(out, d)
	}

	return out, nil
}

// qosRateUnitMax is the largest unit code TS 24.501 §9.11.4.12 assigns
// (256 Pbps); anything above it means the same.
const qosRateUnitMax uint8 = 0x19

// qosRateUnitKbps returns the kbps one unit of a GFBR/MFBR value stands for.
//
// The units climb 1, 4, 16, 64, 256 within each decimal decade and then move to
// the next one — Kbps, Mbps, Gbps, Tbps, Pbps — so code n counts (n-1)/5 decades
// up from the step (n-1)%5. Code 0 is unused and reads as 1 Kbps (NOTE 2), and
// codes above 0x19 read as 256 Pbps.
func qosRateUnitKbps(unit uint8) uint64 {
	if unit == 0 {
		return 1
	}

	if unit > qosRateUnitMax {
		unit = qosRateUnitMax
	}

	steps := [5]uint64{1, 4, 16, 64, 256}
	kbps := steps[(unit-1)%5]

	for range (unit - 1) / 5 {
		kbps *= 1000
	}

	return kbps
}

// Kbps decodes a GFBR or MFBR parameter value (a unit octet then a 16-bit
// value) into kbps, and reports whether this parameter carries one
// (TS 24.501 §9.11.4.12). It is false for any other parameter identifier and for
// a value too short to hold a rate.
func (p QoSFlowParameter) Kbps() (uint64, bool) {
	switch p.ID {
	case QoSFlowParamGFBRUplink, QoSFlowParamGFBRDownlink, QoSFlowParamMFBRUplink, QoSFlowParamMFBRDownlink:
	default:
		return 0, false
	}

	if len(p.Value) < 3 {
		return 0, false
	}

	v := uint64(p.Value[1])<<8 | uint64(p.Value[2])

	return v * qosRateUnitKbps(p.Value[0]), true
}
