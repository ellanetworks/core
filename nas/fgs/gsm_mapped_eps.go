// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"fmt"

	"github.com/ellanetworks/core/nas"
)

// MappedEPSBearerOperation is a mapped EPS bearer context operation code, held
// unshifted; it occupies bits 8-7 of the context's operation octet
// (TS 24.501 §9.11.4.8, table 9.11.4.8.1). Code 0 is reserved.
type MappedEPSBearerOperation uint8

// Mapped EPS bearer context operation codes (TS 24.501 table 9.11.4.8.1).
const (
	MappedEPSBearerOpCreate MappedEPSBearerOperation = 1 // create new EPS bearer
	MappedEPSBearerOpDelete MappedEPSBearerOperation = 2 // delete existing EPS bearer
	MappedEPSBearerOpModify MappedEPSBearerOperation = 3 // modify existing EPS bearer
)

func (o MappedEPSBearerOperation) String() string {
	return enumString(uint8(o), map[uint8]string{
		uint8(MappedEPSBearerOpCreate): "create",
		uint8(MappedEPSBearerOpDelete): "delete",
		uint8(MappedEPSBearerOpModify): "modify",
	})
}

// EPSParameterIdentifier names one EPS parameter of a mapped EPS bearer context
// (TS 24.501 §9.11.4.8, table 9.11.4.8.1). A receiver discards a parameter whose
// identifier it does not support, which is why decoding keeps the ones this
// codec does not interpret rather than failing on them.
type EPSParameterIdentifier uint8

// EPS parameter identifiers (TS 24.501 table 9.11.4.8.1). The contents of each
// are coded by the EPS specification the identifier names, so this codec carries
// them verbatim: TS 24.301 §9.9.4.3 for the mapped EPS QoS parameters (see
// eps.EPSQoS), §9.9.4.30 for the extended form, TS 24.008 figure 10.5.144 from
// octet 2 for the traffic flow template, and TS 24.301 §9.9.4.2 for the APN-AMBR
// (see eps.APNAMBR), §9.9.4.29 for its extended form.
const (
	EPSParameterMappedEPSQoS         EPSParameterIdentifier = 0x01
	EPSParameterMappedExtendedEPSQoS EPSParameterIdentifier = 0x02
	EPSParameterTrafficFlowTemplate  EPSParameterIdentifier = 0x03
	EPSParameterAPNAMBR              EPSParameterIdentifier = 0x04
	// EPSParameterExtendedAPNAMBR is assigned but TS 24.501 table 9.11.4.8.1
	// forbids its use in this version of the protocol.
	EPSParameterExtendedAPNAMBR EPSParameterIdentifier = 0x05
)

func (i EPSParameterIdentifier) String() string {
	return enumString(uint8(i), map[uint8]string{
		uint8(EPSParameterMappedEPSQoS):         "mapped EPS QoS parameters",
		uint8(EPSParameterMappedExtendedEPSQoS): "mapped extended EPS QoS parameters",
		uint8(EPSParameterTrafficFlowTemplate):  "traffic flow template",
		uint8(EPSParameterAPNAMBR):              "APN-AMBR",
		uint8(EPSParameterExtendedAPNAMBR):      "extended APN-AMBR",
	})
}

// EPSParameter is one entry of a mapped EPS bearer context's EPS parameters list
// (TS 24.501 figure 9.11.4.8.3): an identifier, a one-octet content length, and
// the contents.
type EPSParameter struct {
	Identifier EPSParameterIdentifier
	Contents   []byte
}

// MappedEPSBearerContext is one EPS bearer context mapped from a PDU session's
// QoS flows (TS 24.501 §9.11.4.8, figure 9.11.4.8.2). The SMF builds these and
// the AMF relays them, so the UE and the EPC agree on the bearers a handover to
// EPS will produce (TS 23.502 §4.11.1.4.1 step 9a).
type MappedEPSBearerContext struct {
	// EPSBearerIdentity occupies bits 5-8 of octet 4 and is coded as in
	// TS 24.301 §9.3.2.
	EPSBearerIdentity uint8

	Operation MappedEPSBearerOperation

	// EBit is bit 5 of the operation octet, whose meaning depends on Operation
	// (TS 24.501 table 9.11.4.8.1). Under "create new EPS bearer" it reports
	// whether the parameters list is included; under "modify existing EPS bearer"
	// it chooses between extending the previously provided list (false) and
	// replacing all of it (true). The same table then says the bit is ignored for
	// the create and delete operations, so it is carried as it arrived rather
	// than derived.
	EBit bool

	// Parameters is the EPS parameters list. Its length is what the four-bit
	// number-of-EPS-parameters field carries, so encoding derives that field and
	// caps the list at maxEPSParameters.
	Parameters []EPSParameter
}

// MappedEPSBearerContexts is the Mapped EPS bearer contexts IE value: the
// content of the TLV-E, without IEI or length (TS 24.501 §9.11.4.8).
type MappedEPSBearerContexts []MappedEPSBearerContext

const (
	// maxEPSParameters is the largest number of parameters one context can
	// declare: the count is a four-bit field (TS 24.501 figure 9.11.4.8.2).
	maxEPSParameters = 0x0F
	// maxMappedEPSBearerIdentity is the widest EPS bearer identity the four bits
	// of octet 4 hold (TS 24.301 §9.3.2).
	maxMappedEPSBearerIdentity = 0x0F
	// maxMappedEPSBearerContextLen is what one context's two-octet length field
	// can cover: the operation octet and the parameters list.
	maxMappedEPSBearerContextLen = 0xFFFF
	// mappedEPSBearerOpShift is the position of the operation code in its octet.
	mappedEPSBearerOpShift = 6
	// mappedEPSBearerEBit is bit 5 of the operation octet.
	mappedEPSBearerEBit uint8 = 0x10
)

func (p EPSParameter) marshal(w *nas.Writer) error {
	if len(p.Contents) > 0xFF {
		return fmt.Errorf(
			"nas/fgs: EPS parameter %s has %d octets of contents, the length field holds %d",
			p.Identifier, len(p.Contents), 0xFF)
	}

	w.U8(uint8(p.Identifier))
	w.LV(p.Contents)

	return nil
}

func (c MappedEPSBearerContext) marshal(w *nas.Writer) error {
	if c.EPSBearerIdentity > maxMappedEPSBearerIdentity {
		return fmt.Errorf("nas/fgs: mapped EPS bearer context: EPS bearer identity %d does not fit four bits", c.EPSBearerIdentity)
	}

	if len(c.Parameters) > maxEPSParameters {
		return fmt.Errorf("nas/fgs: mapped EPS bearer context %d has %d EPS parameters, the field holds %d",
			c.EPSBearerIdentity, len(c.Parameters), maxEPSParameters)
	}

	body := nas.NewWriter(nil)

	body.U8(uint8(c.Operation)&0x03<<mappedEPSBearerOpShift |
		boolBit(c.EBit, 4) |
		uint8(len(c.Parameters))&maxEPSParameters)

	for _, p := range c.Parameters {
		if err := p.marshal(body); err != nil {
			return err
		}
	}

	raw, err := body.Bytes()
	if err != nil {
		return err
	}

	if len(raw) > maxMappedEPSBearerContextLen {
		return fmt.Errorf("nas/fgs: mapped EPS bearer context %d is %d octets, the length field holds %d",
			c.EPSBearerIdentity, len(raw), maxMappedEPSBearerContextLen)
	}

	// Octet 4 is the EPS bearer identity in bits 5-8 with bits 1-4 spare, then a
	// two-octet length covering octet 7 onwards (TS 24.501 figure 9.11.4.8.2).
	w.U8(c.EPSBearerIdentity << 4)
	w.U16(uint16(len(raw)))
	w.Raw(raw)

	return nil
}

// AppendBinary encodes the Mapped EPS bearer contexts IE value onto b.
func (cs MappedEPSBearerContexts) AppendBinary(b []byte) ([]byte, error) {
	w := nas.NewWriter(b)

	for _, c := range cs {
		if err := c.marshal(w); err != nil {
			return b, err
		}
	}

	return w.Result(b)
}

// MarshalBinary encodes the Mapped EPS bearer contexts IE value.
func (cs MappedEPSBearerContexts) MarshalBinary() ([]byte, error) { return cs.AppendBinary(nil) }

// ParseMappedEPSBearerContexts decodes the Mapped EPS bearer contexts IE value —
// a sequence of mapped EPS bearer contexts (TS 24.501 §9.11.4.8).
func ParseMappedEPSBearerContexts(b []byte) (MappedEPSBearerContexts, error) {
	r := nas.NewReader(b)

	out := MappedEPSBearerContexts{}

	for r.Remaining() > 0 {
		c, err := parseMappedEPSBearerContext(r)
		if err != nil {
			return nil, err
		}

		out = append(out, c)
	}

	return out, nil
}

func parseMappedEPSBearerContext(r *nas.Reader) (MappedEPSBearerContext, error) {
	ebi, err := r.U8()
	if err != nil {
		return MappedEPSBearerContext{}, fmt.Errorf("nas/fgs: mapped EPS bearer context: EPS bearer identity: %w", err)
	}

	length, err := r.U16()
	if err != nil {
		return MappedEPSBearerContext{}, fmt.Errorf("nas/fgs: mapped EPS bearer context: length: %w", err)
	}

	body, err := r.Bytes(int(length))
	if err != nil {
		return MappedEPSBearerContext{}, fmt.Errorf("nas/fgs: mapped EPS bearer context: contents: %w", err)
	}

	if len(body) < 1 {
		return MappedEPSBearerContext{}, fmt.Errorf("nas/fgs: mapped EPS bearer context: empty contents")
	}

	out := MappedEPSBearerContext{
		EPSBearerIdentity: ebi >> 4,
		Operation:         MappedEPSBearerOperation(body[0] >> mappedEPSBearerOpShift & 0x03),
		EBit:              body[0]&mappedEPSBearerEBit != 0,
	}

	count := int(body[0] & maxEPSParameters)
	params := nas.NewReader(body[1:])

	if count > 0 {
		out.Parameters = make([]EPSParameter, 0, count)
	}

	for range count {
		p, err := parseEPSParameter(params)
		if err != nil {
			return MappedEPSBearerContext{}, err
		}

		out.Parameters = append(out.Parameters, p)
	}

	// The count and the length have to agree: a context whose length covers
	// octets the count does not claim is malformed, and accepting it would
	// re-encode to something shorter than what arrived.
	if params.Remaining() > 0 {
		return MappedEPSBearerContext{}, fmt.Errorf(
			"nas/fgs: mapped EPS bearer context %d declares %d EPS parameters but carries %d further octets",
			out.EPSBearerIdentity, count, params.Remaining())
	}

	return out, nil
}

func parseEPSParameter(r *nas.Reader) (EPSParameter, error) {
	id, err := r.U8()
	if err != nil {
		return EPSParameter{}, fmt.Errorf("nas/fgs: EPS parameter: identifier: %w", err)
	}

	contents, err := r.LV()
	if err != nil {
		return EPSParameter{}, fmt.Errorf("nas/fgs: EPS parameter %s: contents: %w", EPSParameterIdentifier(id), err)
	}

	return EPSParameter{Identifier: EPSParameterIdentifier(id), Contents: bytes.Clone(contents)}, nil
}
