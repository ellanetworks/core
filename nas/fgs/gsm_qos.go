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
	QoSRuleOpModifyReplaceFilters QoSRuleOperation = 3 // modify and replace all packet filters
	QoSRuleOpModifyAddFilters     QoSRuleOperation = 4 // modify and add packet filters
	QoSRuleOpModifyDeleteFilters  QoSRuleOperation = 5 // modify and delete packet filters
	QoSRuleOpModifyWithoutFilters QoSRuleOperation = 6 // modify without modifying packet filters
)

func (o QoSRuleOperation) String() string {
	return enumString(uint8(o), map[uint8]string{
		uint8(QoSRuleOpCreate):               "create",
		uint8(QoSRuleOpDelete):               "delete",
		uint8(QoSRuleOpModifyReplaceFilters): "modify, replace all packet filters",
		uint8(QoSRuleOpModifyAddFilters):     "modify, add packet filters",
		uint8(QoSRuleOpModifyDeleteFilters):  "modify, delete packet filters",
		uint8(QoSRuleOpModifyWithoutFilters): "modify without modifying packet filters",
	})
}

// PacketFilterDirection is the direction a packet filter applies in
// (TS 24.501 §9.11.4.13, figure 9.11.4.13.4, bits 6-5). Value 0 is reserved.
type PacketFilterDirection uint8

// Packet filter directions (TS 24.501 table 9.11.4.13.1).
const (
	PacketFilterDownlink      PacketFilterDirection = 1
	PacketFilterUplink        PacketFilterDirection = 2
	PacketFilterBidirectional PacketFilterDirection = 3
)

func (d PacketFilterDirection) String() string {
	return enumString(uint8(d), map[uint8]string{
		uint8(PacketFilterDownlink):      "downlink only",
		uint8(PacketFilterUplink):        "uplink only",
		uint8(PacketFilterBidirectional): "bidirectional",
	})
}

// PacketFilterComponentType is a packet filter component's type octet
// (TS 24.501 §9.11.4.13, table 9.11.4.13.1).
type PacketFilterComponentType uint8

// QoS rule and packet-filter coding (TS 24.501 §9.11.4.13, table 9.11.4.13.1).
const (
	pfComponentTypeMatchAll PacketFilterComponentType = 0x01

	maxPacketFiltersPerRule = 15
	// maxQoSFlowParameters is the largest number of parameters a flow description
	// can declare: the count is a 6-bit field (TS 24.501 §9.11.4.12).
	maxQoSFlowParameters = 0x3F
	maxPacketFilterID    = 15
	maxPacketFilterDir   = 3
)

// carriesRuleBody reports whether an operation code includes the QoS rule
// precedence and the segregation/QFI octets. The "delete existing QoS rule"
// operation omits both: TS 24.501 §9.11.4.13 sets its rule length to one, so the
// content is the operation octet alone.
func carriesRuleBody(op QoSRuleOperation) bool { return op != QoSRuleOpDelete }

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

// QoSRule is a single authorized QoS rule (TS 24.501 §9.11.4.13).
type QoSRule struct {
	Identifier    uint8 // QRI, 1-255
	OperationCode QoSRuleOperation
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
		OperationCode: QoSRuleOpCreate,
		Precedence:    255,
		QFI:           qfi,
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

	if !carriesRuleBody(r.OperationCode) && len(r.Filters) > 0 {
		return fmt.Errorf("nas/fgs: QoS rule %d deletes the rule, so its packet filter list must be empty", r.Identifier)
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

		if !carriesRuleBody(r.OperationCode) {
			return
		}

		c.U8(r.Precedence)
		c.U8(r.Segregation&0x01<<6 | r.QFI&0x3F)
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

func (o QoSFlowOperation) String() string {
	return enumString(uint8(o), map[uint8]string{
		uint8(QoSFlowOpCreate): "create",
		uint8(QoSFlowOpDelete): "delete",
		uint8(QoSFlowOpModify): "modify",
	})
}

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
	case 0x01: // match-all — no value
		return 0, true
	case 0x10, 0x11: // IPv4 remote/local address + mask
		return 8, true
	case 0x21, 0x23: // IPv6 remote/local address + prefix length
		return 17, true
	case 0x30: // protocol identifier / next header
		return 1, true
	case 0x40, 0x50: // single local/remote port
		return 2, true
	case 0x41, 0x51: // local/remote port range
		return 4, true
	case 0x60: // security parameter index
		return 4, true
	case 0x70: // type of service / traffic class (value + mask)
		return 2, true
	case 0x80: // IPv6 flow label (20 bits, 3 octets)
		return 3, true
	case 0x81, 0x82: // destination/source MAC address
		return 6, true
	case 0x83, 0x84: // 802.1Q C-TAG/S-TAG VID
		return 2, true
	case 0x85, 0x86: // 802.1Q C-TAG/S-TAG PCP/DEI
		return 1, true
	case 0x87: // Ethertype type
		return 2, true
	case 0x88, 0x89: // destination/source MAC address range (low and high limits)
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
	if !carriesRuleBody(rule.OperationCode) && hdr&0x0F != 0 {
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

	// A rule that only deletes carries no precedence and no segregation/QFI.
	if !carriesRuleBody(rule.OperationCode) {
		return rule, nil
	}

	prec, err := cr.U8()
	if err != nil {
		return QoSRule{}, err
	}

	segQFI, err := cr.U8()
	if err != nil {
		return QoSRule{}, err
	}

	rule.Precedence = prec
	rule.Segregation = segQFI >> 6 & 0x01
	rule.QFI = segQFI & 0x3F

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
