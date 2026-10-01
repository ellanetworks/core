// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6c

import (
	"fmt"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type NodeAddress struct {
	Name   string
	Realm  string
	Number string
}

type ServingNode struct {
	MME       *NodeAddress
	SGSN      *NodeAddress
	MSCNumber string
	IPSMGW    *NodeAddress
}

func (s ServingNode) empty() bool {
	return s.MME == nil && s.SGSN == nil && s.MSCNumber == "" && s.IPSMGW == nil
}

type ServingNodes struct {
	Serving     *ServingNode
	Additional  *ServingNode
	SMSF3GPP    *NodeAddress
	SMSFNon3GPP *NodeAddress
}

func (n ServingNodes) empty() bool {
	return n.Serving == nil && n.Additional == nil && n.SMSF3GPP == nil && n.SMSFNon3GPP == nil
}

func (n ServingNodes) hasSMSF() bool {
	return n.SMSF3GPP != nil || n.SMSFNon3GPP != nil
}

func (s ServingNode) validate(additional bool) error {
	sgsn, mme, msc, ipsmgw := s.SGSN != nil, s.MME != nil, s.MSCNumber != "", s.IPSMGW != nil

	switch {
	case sgsn && !mme && !msc && !ipsmgw:
		if s.SGSN.Number == "" || (s.SGSN.Name == "") != (s.SGSN.Realm == "") {
			return invalidf("SGSN needs a number, and a name and realm together")
		}
	case mme && !sgsn && !msc && !ipsmgw:
		if s.MME.Name == "" || s.MME.Realm == "" || s.MME.Number == "" {
			return invalidf("MME needs a name, a realm and an MME number for MT SMS")
		}
	case msc && !sgsn && !mme && !ipsmgw:
	case msc && mme && !sgsn && !ipsmgw:
		if s.MME.Name == "" || s.MME.Realm == "" || s.MME.Number != "" {
			return invalidf("MSC with an MME needs the MME name and realm and no MME number")
		}
	case ipsmgw && !sgsn && !mme && !msc && !additional:
		if s.IPSMGW.Number == "" || (s.IPSMGW.Realm != "" && s.IPSMGW.Name == "") {
			return invalidf("IP-SM-GW needs a number, and a name with any realm")
		}
	default:
		return invalidf("serving node combination not allowed by TS 29.338 §5.3.3.6/§5.3.3.7")
	}

	return nil
}

func validateSMSF(n *NodeAddress) error {
	if n != nil && (n.Name == "" || n.Realm == "" || n.Number == "") {
		return invalidf("SMSF address needs a name, a realm and a number")
	}

	return nil
}

func (n ServingNodes) avps(smsfSupport bool) ([]diameter.AVP, error) {
	if !smsfSupport && n.hasSMSF() {
		return nil, invalidf("SMSF address for a peer without SMSF-Support")
	}

	var avps []diameter.AVP

	for _, sn := range []struct {
		code       uint32
		node       *ServingNode
		additional bool
	}{
		{AVPServingNode, n.Serving, false},
		{AVPAdditionalServingNode, n.Additional, true},
	} {
		if sn.node == nil {
			continue
		}

		if err := sn.node.validate(sn.additional); err != nil {
			return nil, err
		}

		a, err := servingNodeAVP(sn.code, *sn.node)
		if err != nil {
			return nil, err
		}

		avps = append(avps, a)
	}

	for _, smsf := range []struct {
		node                                       *NodeAddress
		groupCode, nameCode, realmCode, numberCode uint32
	}{
		{n.SMSF3GPP, AVPSMSF3GPPAddress, AVPSMSF3GPPName, AVPSMSF3GPPRealm, AVPSMSF3GPPNumber},
		{n.SMSFNon3GPP, AVPSMSFNon3GPPAddress, AVPSMSFNon3GPPName, AVPSMSFNon3GPPRealm, AVPSMSFNon3GPPNumber},
	} {
		if smsf.node == nil {
			continue
		}

		if err := validateSMSF(smsf.node); err != nil {
			return nil, err
		}

		inner, err := nodeAVPs(*smsf.node, smsf.nameCode, 0, smsf.realmCode, 0, smsf.numberCode, 0)
		if err != nil {
			return nil, err
		}

		avps = append(avps, diameter.Grouped(smsf.groupCode, 0, tgpp.VendorID, inner...))
	}

	return avps, nil
}

func servingNodeAVP(code uint32, n ServingNode) (diameter.AVP, error) {
	var inner []diameter.AVP

	if n.SGSN != nil {
		avps, err := nodeAVPs(*n.SGSN, AVPSGSNName, 0, AVPSGSNRealm, 0, tgpp.AVPSGSNNumber, diameter.AVPFlagMandatory)
		if err != nil {
			return diameter.AVP{}, err
		}

		inner = append(inner, avps...)
	}

	if n.MME != nil {
		avps, err := nodeAVPs(*n.MME, AVPMMEName, diameter.AVPFlagMandatory, AVPMMERealm, diameter.AVPFlagMandatory,
			tgpp.AVPMMENumberForMTSMS, diameter.AVPFlagMandatory)
		if err != nil {
			return diameter.AVP{}, err
		}

		inner = append(inner, avps...)
	}

	if n.MSCNumber != "" {
		number, err := tgpp.EncodeE164(n.MSCNumber)
		if err != nil {
			return diameter.AVP{}, invalidf("MSC number: %w", err)
		}

		inner = append(inner, diameter.OctetString(AVPMSCNumber, diameter.AVPFlagMandatory, tgpp.VendorID, number))
	}

	if n.IPSMGW != nil {
		number, err := tgpp.EncodeE164(n.IPSMGW.Number)
		if err != nil {
			return diameter.AVP{}, invalidf("IP-SM-GW number: %w", err)
		}

		inner = append(inner, diameter.OctetString(AVPIPSMGWNumber, diameter.AVPFlagMandatory, tgpp.VendorID, number))

		if n.IPSMGW.Name != "" {
			inner = append(inner, diameter.UTF8String(AVPIPSMGWName, diameter.AVPFlagMandatory, tgpp.VendorID, n.IPSMGW.Name))
		}

		if n.IPSMGW.Realm != "" {
			inner = append(inner, diameter.UTF8String(AVPIPSMGWRealm, diameter.AVPFlagMandatory, tgpp.VendorID, n.IPSMGW.Realm))
		}
	}

	return diameter.Grouped(code, diameter.AVPFlagMandatory, tgpp.VendorID, inner...), nil
}

func nodeAVPs(n NodeAddress, nameCode uint32, nameFlags uint8, realmCode uint32, realmFlags uint8, numberCode uint32, numberFlags uint8) ([]diameter.AVP, error) {
	var avps []diameter.AVP

	if n.Name != "" {
		avps = append(avps, diameter.UTF8String(nameCode, nameFlags, tgpp.VendorID, n.Name))
	}

	if n.Realm != "" {
		avps = append(avps, diameter.UTF8String(realmCode, realmFlags, tgpp.VendorID, n.Realm))
	}

	if n.Number != "" {
		number, err := tgpp.EncodeE164(n.Number)
		if err != nil {
			return nil, invalidf("node number: %w", err)
		}

		avps = append(avps, diameter.OctetString(numberCode, numberFlags, tgpp.VendorID, number))
	}

	return avps, nil
}

func parseServingNodes(m *diameter.Message) (ServingNodes, error) {
	var nodes ServingNodes

	for _, sn := range []struct {
		code uint32
		dst  **ServingNode
	}{
		{AVPServingNode, &nodes.Serving},
		{AVPAdditionalServingNode, &nodes.Additional},
	} {
		a, ok := m.Find(sn.code, tgpp.VendorID)
		if !ok {
			continue
		}

		node, err := parseServingNode(a)
		if err != nil {
			return ServingNodes{}, &avpParseError{avp: a, err: err}
		}

		*sn.dst = node
	}

	for _, smsf := range []struct {
		groupCode, nameCode, realmCode, numberCode uint32
		dst                                        **NodeAddress
	}{
		{AVPSMSF3GPPAddress, AVPSMSF3GPPName, AVPSMSF3GPPRealm, AVPSMSF3GPPNumber, &nodes.SMSF3GPP},
		{AVPSMSFNon3GPPAddress, AVPSMSFNon3GPPName, AVPSMSFNon3GPPRealm, AVPSMSFNon3GPPNumber, &nodes.SMSFNon3GPP},
	} {
		a, ok := m.Find(smsf.groupCode, tgpp.VendorID)
		if !ok {
			continue
		}

		node, err := parseSMSFAddress(a, smsf.nameCode, smsf.realmCode, smsf.numberCode)
		if err != nil {
			return ServingNodes{}, &avpParseError{avp: a, err: err}
		}

		*smsf.dst = node
	}

	return nodes, nil
}

type avpParseError struct {
	avp diameter.AVP
	err error
}

func (e *avpParseError) Error() string {
	return e.err.Error()
}

func (e *avpParseError) Unwrap() error {
	return e.err
}

func parseServingNode(a diameter.AVP) (*ServingNode, error) {
	inner, err := a.Grouped()
	if err != nil {
		return nil, fmt.Errorf("serving node: %w", err)
	}

	identity := func(code uint32) string {
		v, _ := diameter.Find(inner, code, tgpp.VendorID)
		return v.UTF8String()
	}

	number := func(code uint32) (string, error) {
		v, ok := diameter.Find(inner, code, tgpp.VendorID)
		if !ok {
			return "", nil
		}

		digits, err := tgpp.DecodeE164(v.Data)
		if err != nil {
			return "", fmt.Errorf("serving node number: %w", err)
		}

		return digits, nil
	}

	numbers := make(map[uint32]string, 4)

	for _, code := range []uint32{tgpp.AVPMMENumberForMTSMS, tgpp.AVPSGSNNumber, AVPMSCNumber, AVPIPSMGWNumber} {
		if numbers[code], err = number(code); err != nil {
			return nil, err
		}
	}

	node := &ServingNode{MSCNumber: numbers[AVPMSCNumber]}

	if name := identity(AVPMMEName); name != "" && numbers[tgpp.AVPMMENumberForMTSMS] != "" {
		node.MME = &NodeAddress{Name: name, Realm: identity(AVPMMERealm), Number: numbers[tgpp.AVPMMENumberForMTSMS]}
	}

	if n := numbers[tgpp.AVPSGSNNumber]; n != "" {
		node.SGSN = &NodeAddress{Name: identity(AVPSGSNName), Realm: identity(AVPSGSNRealm), Number: n}
	}

	if n := numbers[AVPIPSMGWNumber]; n != "" {
		node.IPSMGW = &NodeAddress{Name: identity(AVPIPSMGWName), Realm: identity(AVPIPSMGWRealm), Number: n}
	}

	if node.empty() {
		return nil, fmt.Errorf("serving node with no delivery target")
	}

	return node, nil
}

func parseSMSFAddress(a diameter.AVP, nameCode, realmCode, numberCode uint32) (*NodeAddress, error) {
	inner, err := a.Grouped()
	if err != nil {
		return nil, fmt.Errorf("SMSF address: %w", err)
	}

	node := &NodeAddress{}

	if v, ok := diameter.Find(inner, nameCode, tgpp.VendorID); ok {
		node.Name = v.UTF8String()
	}

	if v, ok := diameter.Find(inner, realmCode, tgpp.VendorID); ok {
		node.Realm = v.UTF8String()
	}

	if v, ok := diameter.Find(inner, numberCode, tgpp.VendorID); ok {
		if node.Number, err = tgpp.DecodeE164(v.Data); err != nil {
			return nil, fmt.Errorf("SMSF number: %w", err)
		}
	}

	return node, nil
}
