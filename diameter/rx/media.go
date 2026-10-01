// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"bytes"
	"strings"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	avpMinRequestedBandwidthDL  uint32 = 534
	avpMinRequestedBandwidthUL  uint32 = 535
	avpSharingKeyDL             uint32 = 539
	avpSharingKeyUL             uint32 = 540
	avpMaxSupportedBandwidthDL  uint32 = 543
	avpMaxSupportedBandwidthUL  uint32 = 544
	avpMinDesiredBandwidthDL    uint32 = 545
	avpMinDesiredBandwidthUL    uint32 = 546
	avpMediaComponentStatus     uint32 = 549
	avpPrioritySharingIndicator uint32 = 550
	avpContentVersion           uint32 = 552
	avpExtendedMaxRequestedBWDL uint32 = 554
	avpExtendedMaxRequestedBWUL uint32 = 555
	avpExtendedMaxSupportedBWDL uint32 = 556
	avpExtendedMaxSupportedBWUL uint32 = 557
	avpExtendedMinDesiredBWDL   uint32 = 558
	avpExtendedMinDesiredBWUL   uint32 = 559
	avpExtendedMinRequestedBWDL uint32 = 560
	avpExtendedMinRequestedBWUL uint32 = 561
	avpFLUSIdentifier           uint32 = 566
	avpDesiredMaxLatency        uint32 = 567
	avpDesiredMaxLoss           uint32 = 568
	avpFinalUnitAction          uint32 = 449
	avpToSTrafficClass          uint32 = 1014
	avpPreemptionCapability     uint32 = 1047
	avpPreemptionVulnerability  uint32 = 1048
	avpMaxPLRDL                 uint32 = 2852
	avpMaxPLRUL                 uint32 = 2853
	maxCodecData                       = 2
	codecDataSeparator                 = "\n"
)

type MediaComponent struct {
	Number                  uint32
	SubComponents           []MediaSubComponent
	AFApplicationIdentifier string
	Type                    *MediaType
	MaxRequestedBandwidthUL *uint32
	MaxRequestedBandwidthDL *uint32
	FlowStatus              *FlowStatus
	RSBandwidth             *uint32
	RRBandwidth             *uint32
	CodecData               []CodecData
}

type MediaSubComponent struct {
	FlowNumber              uint32
	FlowDescriptions        []string
	FlowStatus              *FlowStatus
	FlowUsage               FlowUsage
	MaxRequestedBandwidthUL *uint32
	MaxRequestedBandwidthDL *uint32
	SignallingProtocol      AFSignallingProtocol
}

type CodecData struct {
	Direction CodecDirection
	Kind      CodecKind
	SDP       string
}

type Flows struct {
	MediaComponentNumber uint32
	FlowNumbers          []uint32
}

type AccessNetworkChargingIdentifier struct {
	Value []byte
	Flows []Flows
}

type SubscriptionID struct {
	Type SubscriptionIDType
	Data string
}

type MediaBandwidth struct {
	MediaComponentNumber    uint32
	MaxRequestedBandwidthUL *uint32
	MaxRequestedBandwidthDL *uint32
}

type AcceptableServiceInfo struct {
	MediaComponents         []MediaBandwidth
	MaxRequestedBandwidthUL *uint32
	MaxRequestedBandwidthDL *uint32
}

var mediaComponentRules = diameter.Rules{
	vendorKey(AVPMediaComponentNumber):                     {Required: true, MinLength: 4},
	vendorKey(AVPMediaSubComponent):                        {Multiple: true},
	vendorKey(AVPAFApplicationIdentifier):                  {},
	vendorKey(avpFLUSIdentifier):                           {},
	vendorKey(AVPMediaType):                                {},
	vendorKey(AVPMaxRequestedBandwidthUL):                  {},
	vendorKey(AVPMaxRequestedBandwidthDL):                  {},
	vendorKey(avpMaxSupportedBandwidthUL):                  {},
	vendorKey(avpMaxSupportedBandwidthDL):                  {},
	vendorKey(avpMinDesiredBandwidthUL):                    {},
	vendorKey(avpMinDesiredBandwidthDL):                    {},
	vendorKey(avpMinRequestedBandwidthUL):                  {},
	vendorKey(avpMinRequestedBandwidthDL):                  {},
	vendorKey(avpExtendedMaxRequestedBWUL):                 {},
	vendorKey(avpExtendedMaxRequestedBWDL):                 {},
	vendorKey(avpExtendedMaxSupportedBWUL):                 {},
	vendorKey(avpExtendedMaxSupportedBWDL):                 {},
	vendorKey(avpExtendedMinDesiredBWUL):                   {},
	vendorKey(avpExtendedMinDesiredBWDL):                   {},
	vendorKey(avpExtendedMinRequestedBWUL):                 {},
	vendorKey(avpExtendedMinRequestedBWDL):                 {},
	vendorKey(AVPFlowStatus):                               {},
	vendorKey(avpPrioritySharingIndicator):                 {},
	vendorKey(avpPreemptionCapability):                     {},
	vendorKey(avpPreemptionVulnerability):                  {},
	{Code: avpReservationPriority, VendorID: etsiVendorID}: {},
	vendorKey(AVPRSBandwidth):                              {},
	vendorKey(AVPRRBandwidth):                              {},
	vendorKey(AVPCodecData):                                {Multiple: true},
	vendorKey(avpSharingKeyDL):                             {},
	vendorKey(avpSharingKeyUL):                             {},
	vendorKey(avpContentVersion):                           {},
	vendorKey(avpMaxPLRDL):                                 {},
	vendorKey(avpMaxPLRUL):                                 {},
	vendorKey(avpDesiredMaxLatency):                        {},
	vendorKey(avpDesiredMaxLoss):                           {},
}

var mediaSubComponentRules = diameter.Rules{
	vendorKey(AVPFlowNumber):               {Required: true, MinLength: 4},
	vendorKey(AVPFlowDescription):          {Multiple: true},
	vendorKey(AVPFlowStatus):               {},
	vendorKey(AVPFlowUsage):                {},
	vendorKey(AVPMaxRequestedBandwidthUL):  {},
	vendorKey(AVPMaxRequestedBandwidthDL):  {},
	vendorKey(avpExtendedMaxRequestedBWUL): {},
	vendorKey(avpExtendedMaxRequestedBWDL): {},
	vendorKey(AVPAFSignallingProtocol):     {},
	vendorKey(avpToSTrafficClass):          {},
}

var flowsRules = diameter.Rules{
	vendorKey(AVPMediaComponentNumber): {Required: true, MinLength: 4},
	vendorKey(AVPFlowNumber):           {Multiple: true},
	vendorKey(avpContentVersion):       {Multiple: true},
	{Code: avpFinalUnitAction}:         {},
	vendorKey(avpMediaComponentStatus): {},
}

var chargingIdentifierRules = diameter.Rules{
	vendorKey(AVPAccessNetworkChargingIdentifierValue): {Required: true},
	vendorKey(AVPFlows): {Multiple: true},
}

var subscriptionIDRules = diameter.Rules{
	{Code: AVPSubscriptionIDType}: {Required: true, MinLength: 4},
	{Code: AVPSubscriptionIDData}: {Required: true},
}

var acceptableServiceInfoRules = diameter.Rules{
	vendorKey(AVPMediaComponentDescription): {Multiple: true},
	vendorKey(AVPMaxRequestedBandwidthDL):   {},
	vendorKey(AVPMaxRequestedBandwidthUL):   {},
	vendorKey(avpExtendedMaxRequestedBWDL):  {},
	vendorKey(avpExtendedMaxRequestedBWUL):  {},
}

func vendorKey(code uint32) diameter.AVPKey {
	return diameter.AVPKey{Code: code, VendorID: tgpp.VendorID}
}

func vendorUnsigned(code, v uint32) diameter.AVP {
	return diameter.Unsigned32(code, diameter.AVPFlagMandatory, tgpp.VendorID, v)
}

func vendorOctets(code uint32, v []byte) diameter.AVP {
	return diameter.OctetString(code, diameter.AVPFlagMandatory, tgpp.VendorID, v)
}

func vendorGrouped(code uint32, avps ...diameter.AVP) diameter.AVP {
	return diameter.Grouped(code, diameter.AVPFlagMandatory, tgpp.VendorID, avps...)
}

func appendOptional(avps []diameter.AVP, code uint32, v *uint32) []diameter.AVP {
	if v != nil {
		avps = append(avps, vendorUnsigned(code, *v))
	}

	return avps
}

func checkGrouped(rules diameter.Rules, a diameter.AVP) ([]diameter.AVP, error) {
	inner, err := a.Grouped()
	if err != nil {
		return nil, tgpp.InvalidAVP(a)
	}

	if err := rules.Check(&diameter.Message{AVPs: inner}); err != nil {
		return nil, err
	}

	return inner, nil
}

func unsigned(avps []diameter.AVP, code uint32) (uint32, error) {
	a, _ := diameter.Find(avps, code, tgpp.VendorID)

	v, err := a.Unsigned32()
	if err != nil {
		return 0, tgpp.InvalidAVP(a)
	}

	return v, nil
}

func optionalUnsigned(avps []diameter.AVP, code uint32) (*uint32, error) {
	if _, ok := diameter.Find(avps, code, tgpp.VendorID); !ok {
		return nil, nil
	}

	v, err := unsigned(avps, code)
	if err != nil {
		return nil, err
	}

	return &v, nil
}

func enum[T ~uint32](avps []diameter.AVP, code, vendorID uint32, valid func(T) bool) (T, bool, error) {
	a, ok := diameter.Find(avps, code, vendorID)
	if !ok {
		return 0, false, nil
	}

	v, err := a.Unsigned32()
	if err != nil || !valid(T(v)) {
		return 0, false, tgpp.InvalidAVP(a)
	}

	return T(v), true, nil
}

func optionalEnum[T ~uint32](avps []diameter.AVP, code uint32, valid func(T) bool) (*T, error) {
	v, ok, err := enum(avps, code, tgpp.VendorID, valid)
	if err != nil || !ok {
		return nil, err
	}

	return &v, nil
}

func upTo[T ~uint32](limit T) func(T) bool {
	return func(v T) bool { return v <= limit }
}

func validFlowStatus(s *FlowStatus) bool {
	return s == nil || *s <= maxFlowStatus
}

func mediaComponentAVP(c MediaComponent) (diameter.AVP, error) {
	switch {
	case c.Type != nil && !c.Type.valid():
		return diameter.AVP{}, invalid("Media-Type %d", uint32(*c.Type))
	case !validFlowStatus(c.FlowStatus):
		return diameter.AVP{}, invalid("Flow-Status %d", uint32(*c.FlowStatus))
	case len(c.CodecData) > maxCodecData:
		return diameter.AVP{}, invalid("media component %d with %d Codec-Data", c.Number, len(c.CodecData))
	}

	avps := []diameter.AVP{vendorUnsigned(AVPMediaComponentNumber, c.Number)}

	for _, sub := range c.SubComponents {
		a, err := mediaSubComponentAVP(sub)
		if err != nil {
			return diameter.AVP{}, err
		}

		avps = append(avps, a)
	}

	if c.AFApplicationIdentifier != "" {
		avps = append(avps, vendorOctets(AVPAFApplicationIdentifier, []byte(c.AFApplicationIdentifier)))
	}

	if c.Type != nil {
		avps = append(avps, vendorUnsigned(AVPMediaType, uint32(*c.Type)))
	}

	avps = appendOptional(avps, AVPMaxRequestedBandwidthUL, c.MaxRequestedBandwidthUL)
	avps = appendOptional(avps, AVPMaxRequestedBandwidthDL, c.MaxRequestedBandwidthDL)
	avps = appendOptional(avps, AVPFlowStatus, (*uint32)(c.FlowStatus))
	avps = appendOptional(avps, AVPRSBandwidth, c.RSBandwidth)
	avps = appendOptional(avps, AVPRRBandwidth, c.RRBandwidth)

	for _, d := range c.CodecData {
		a, err := codecDataAVP(d)
		if err != nil {
			return diameter.AVP{}, err
		}

		avps = append(avps, a)
	}

	return vendorGrouped(AVPMediaComponentDescription, avps...), nil
}

func parseMediaComponent(a diameter.AVP) (MediaComponent, error) {
	inner, err := checkGrouped(mediaComponentRules, a)
	if err != nil {
		return MediaComponent{}, err
	}

	var c MediaComponent

	if c.Number, err = unsigned(inner, AVPMediaComponentNumber); err != nil {
		return MediaComponent{}, err
	}

	for _, sub := range diameter.FindAll(inner, AVPMediaSubComponent, tgpp.VendorID) {
		s, err := parseMediaSubComponent(sub)
		if err != nil {
			return MediaComponent{}, err
		}

		c.SubComponents = append(c.SubComponents, s)
	}

	if id, ok := diameter.Find(inner, AVPAFApplicationIdentifier, tgpp.VendorID); ok {
		if c.AFApplicationIdentifier = string(id.Data); c.AFApplicationIdentifier == "" {
			return MediaComponent{}, tgpp.InvalidAVP(id)
		}
	}

	if c.Type, err = optionalEnum(inner, AVPMediaType, MediaType.valid); err != nil {
		return MediaComponent{}, err
	}

	if c.FlowStatus, err = optionalEnum(inner, AVPFlowStatus, upTo(maxFlowStatus)); err != nil {
		return MediaComponent{}, err
	}

	for _, f := range []struct {
		code uint32
		dst  **uint32
	}{
		{AVPMaxRequestedBandwidthUL, &c.MaxRequestedBandwidthUL},
		{AVPMaxRequestedBandwidthDL, &c.MaxRequestedBandwidthDL},
		{AVPRSBandwidth, &c.RSBandwidth},
		{AVPRRBandwidth, &c.RRBandwidth},
	} {
		if *f.dst, err = optionalUnsigned(inner, f.code); err != nil {
			return MediaComponent{}, err
		}
	}

	codecs := diameter.FindAll(inner, AVPCodecData, tgpp.VendorID)
	if len(codecs) > maxCodecData {
		return MediaComponent{}, diameter.NewAVPError(diameter.ResultAVPOccursTooManyTimes, codecs[maxCodecData])
	}

	for _, codec := range codecs {
		d, err := parseCodecData(codec)
		if err != nil {
			return MediaComponent{}, err
		}

		c.CodecData = append(c.CodecData, d)
	}

	return c, nil
}

func mediaSubComponentAVP(s MediaSubComponent) (diameter.AVP, error) {
	switch {
	case !validFlowStatus(s.FlowStatus):
		return diameter.AVP{}, invalid("Flow-Status %d", uint32(*s.FlowStatus))
	case s.FlowUsage > maxFlowUsage:
		return diameter.AVP{}, invalid("Flow-Usage %d", uint32(s.FlowUsage))
	case s.SignallingProtocol > maxAFSignallingProtocol:
		return diameter.AVP{}, invalid("AF-Signalling-Protocol %d", uint32(s.SignallingProtocol))
	case s.SignallingProtocol != SignallingNoInformation && s.FlowUsage != FlowUsageAFSignalling:
		return diameter.AVP{}, invalid("AF-Signalling-Protocol on flow %d without AF_SIGNALLING usage", s.FlowNumber)
	}

	avps := []diameter.AVP{vendorUnsigned(AVPFlowNumber, s.FlowNumber)}

	for _, d := range s.FlowDescriptions {
		if d == "" {
			return diameter.AVP{}, invalid("empty Flow-Description on flow %d", s.FlowNumber)
		}

		avps = append(avps, vendorOctets(AVPFlowDescription, []byte(d)))
	}

	avps = appendOptional(avps, AVPFlowStatus, (*uint32)(s.FlowStatus))

	if s.FlowUsage != FlowUsageNoInformation {
		avps = append(avps, vendorUnsigned(AVPFlowUsage, uint32(s.FlowUsage)))
	}

	avps = appendOptional(avps, AVPMaxRequestedBandwidthUL, s.MaxRequestedBandwidthUL)
	avps = appendOptional(avps, AVPMaxRequestedBandwidthDL, s.MaxRequestedBandwidthDL)

	if s.SignallingProtocol != SignallingNoInformation {
		avps = append(avps, diameter.Unsigned32(AVPAFSignallingProtocol, 0, tgpp.VendorID, uint32(s.SignallingProtocol)))
	}

	return vendorGrouped(AVPMediaSubComponent, avps...), nil
}

func parseMediaSubComponent(a diameter.AVP) (MediaSubComponent, error) {
	inner, err := checkGrouped(mediaSubComponentRules, a)
	if err != nil {
		return MediaSubComponent{}, err
	}

	var s MediaSubComponent

	if s.FlowNumber, err = unsigned(inner, AVPFlowNumber); err != nil {
		return MediaSubComponent{}, err
	}

	for _, d := range diameter.FindAll(inner, AVPFlowDescription, tgpp.VendorID) {
		if len(d.Data) == 0 {
			return MediaSubComponent{}, tgpp.InvalidAVP(d)
		}

		s.FlowDescriptions = append(s.FlowDescriptions, string(d.Data))
	}

	if s.FlowStatus, err = optionalEnum(inner, AVPFlowStatus, upTo(maxFlowStatus)); err != nil {
		return MediaSubComponent{}, err
	}

	if s.FlowUsage, _, err = enum(inner, AVPFlowUsage, tgpp.VendorID, upTo(maxFlowUsage)); err != nil {
		return MediaSubComponent{}, err
	}

	if s.MaxRequestedBandwidthUL, err = optionalUnsigned(inner, AVPMaxRequestedBandwidthUL); err != nil {
		return MediaSubComponent{}, err
	}

	if s.MaxRequestedBandwidthDL, err = optionalUnsigned(inner, AVPMaxRequestedBandwidthDL); err != nil {
		return MediaSubComponent{}, err
	}

	if s.SignallingProtocol, _, err = enum(inner, AVPAFSignallingProtocol, tgpp.VendorID, upTo(maxAFSignallingProtocol)); err != nil {
		return MediaSubComponent{}, err
	}

	if s.SignallingProtocol != SignallingNoInformation && s.FlowUsage != FlowUsageAFSignalling {
		protocol, _ := diameter.Find(inner, AVPAFSignallingProtocol, tgpp.VendorID)
		return MediaSubComponent{}, tgpp.InvalidAVP(protocol)
	}

	return s, nil
}

func codecDataAVP(d CodecData) (diameter.AVP, error) {
	if d.Direction > CodecDownlink || d.Kind > CodecDescription {
		return diameter.AVP{}, invalid("Codec-Data %s %s", d.Direction, d.Kind)
	}

	return vendorOctets(AVPCodecData, []byte(d.Direction.String()+codecDataSeparator+d.Kind.String()+codecDataSeparator+d.SDP)), nil
}

func parseCodecData(a diameter.AVP) (CodecData, error) {
	lines := strings.SplitN(string(a.Data), codecDataSeparator, 3)
	if len(lines) != 3 {
		return CodecData{}, tgpp.InvalidAVP(a)
	}

	d := CodecData{SDP: lines[2]}

	switch lines[0] {
	case CodecUplink.String():
		d.Direction = CodecUplink
	case CodecDownlink.String():
		d.Direction = CodecDownlink
	default:
		return CodecData{}, tgpp.InvalidAVP(a)
	}

	switch lines[1] {
	case CodecOffer.String():
		d.Kind = CodecOffer
	case CodecAnswer.String():
		d.Kind = CodecAnswer
	case CodecDescription.String():
		d.Kind = CodecDescription
	default:
		return CodecData{}, tgpp.InvalidAVP(a)
	}

	return d, nil
}

func flowsAVP(f Flows) diameter.AVP {
	avps := []diameter.AVP{vendorUnsigned(AVPMediaComponentNumber, f.MediaComponentNumber)}
	for _, n := range f.FlowNumbers {
		avps = append(avps, vendorUnsigned(AVPFlowNumber, n))
	}

	return vendorGrouped(AVPFlows, avps...)
}

func parseFlows(a diameter.AVP) (Flows, error) {
	inner, err := checkGrouped(flowsRules, a)
	if err != nil {
		return Flows{}, err
	}

	var f Flows

	if f.MediaComponentNumber, err = unsigned(inner, AVPMediaComponentNumber); err != nil {
		return Flows{}, err
	}

	for _, n := range diameter.FindAll(inner, AVPFlowNumber, tgpp.VendorID) {
		v, err := n.Unsigned32()
		if err != nil {
			return Flows{}, tgpp.InvalidAVP(n)
		}

		f.FlowNumbers = append(f.FlowNumbers, v)
	}

	return f, nil
}

func flowsList(avps []diameter.AVP) ([]Flows, error) {
	var flows []Flows

	for _, a := range diameter.FindAll(avps, AVPFlows, tgpp.VendorID) {
		f, err := parseFlows(a)
		if err != nil {
			return nil, err
		}

		flows = append(flows, f)
	}

	return flows, nil
}

func chargingIdentifierAVP(c AccessNetworkChargingIdentifier) (diameter.AVP, error) {
	if len(c.Value) == 0 {
		return diameter.AVP{}, invalid("empty Access-Network-Charging-Identifier-Value")
	}

	avps := []diameter.AVP{vendorOctets(AVPAccessNetworkChargingIdentifierValue, c.Value)}
	for _, f := range c.Flows {
		avps = append(avps, flowsAVP(f))
	}

	return vendorGrouped(AVPAccessNetworkChargingIdentifier, avps...), nil
}

func parseChargingIdentifier(a diameter.AVP) (AccessNetworkChargingIdentifier, error) {
	inner, err := checkGrouped(chargingIdentifierRules, a)
	if err != nil {
		return AccessNetworkChargingIdentifier{}, err
	}

	value, _ := diameter.Find(inner, AVPAccessNetworkChargingIdentifierValue, tgpp.VendorID)
	c := AccessNetworkChargingIdentifier{Value: bytes.Clone(value.Data)}

	if c.Flows, err = flowsList(inner); err != nil {
		return AccessNetworkChargingIdentifier{}, err
	}

	return c, nil
}

func subscriptionIDAVP(s SubscriptionID) (diameter.AVP, error) {
	switch {
	case s.Type > maxSubscriptionIDType:
		return diameter.AVP{}, invalid("Subscription-Id-Type %d", uint32(s.Type))
	case s.Data == "":
		return diameter.AVP{}, invalid("empty Subscription-Id-Data")
	}

	return diameter.Grouped(AVPSubscriptionID, diameter.AVPFlagMandatory, 0,
		diameter.Unsigned32(AVPSubscriptionIDType, diameter.AVPFlagMandatory, 0, uint32(s.Type)),
		diameter.UTF8String(AVPSubscriptionIDData, diameter.AVPFlagMandatory, 0, s.Data),
	), nil
}

func parseSubscriptionID(a diameter.AVP) (SubscriptionID, error) {
	inner, err := checkGrouped(subscriptionIDRules, a)
	if err != nil {
		return SubscriptionID{}, err
	}

	t, _, err := enum(inner, AVPSubscriptionIDType, 0, upTo(maxSubscriptionIDType))
	if err != nil {
		return SubscriptionID{}, err
	}

	data, _ := diameter.Find(inner, AVPSubscriptionIDData, 0)

	return SubscriptionID{Type: t, Data: data.UTF8String()}, nil
}

func acceptableServiceInfoAVP(s AcceptableServiceInfo) diameter.AVP {
	var avps []diameter.AVP

	for _, m := range s.MediaComponents {
		inner := []diameter.AVP{vendorUnsigned(AVPMediaComponentNumber, m.MediaComponentNumber)}
		inner = appendOptional(inner, AVPMaxRequestedBandwidthUL, m.MaxRequestedBandwidthUL)
		inner = appendOptional(inner, AVPMaxRequestedBandwidthDL, m.MaxRequestedBandwidthDL)
		avps = append(avps, vendorGrouped(AVPMediaComponentDescription, inner...))
	}

	avps = appendOptional(avps, AVPMaxRequestedBandwidthDL, s.MaxRequestedBandwidthDL)
	avps = appendOptional(avps, AVPMaxRequestedBandwidthUL, s.MaxRequestedBandwidthUL)

	return vendorGrouped(AVPAcceptableServiceInfo, avps...)
}

func parseAcceptableServiceInfo(a diameter.AVP) (*AcceptableServiceInfo, error) {
	inner, err := checkGrouped(acceptableServiceInfoRules, a)
	if err != nil {
		return nil, err
	}

	var s AcceptableServiceInfo

	for _, m := range diameter.FindAll(inner, AVPMediaComponentDescription, tgpp.VendorID) {
		c, err := parseMediaComponent(m)
		if err != nil {
			return nil, err
		}

		s.MediaComponents = append(s.MediaComponents, MediaBandwidth{
			MediaComponentNumber:    c.Number,
			MaxRequestedBandwidthUL: c.MaxRequestedBandwidthUL,
			MaxRequestedBandwidthDL: c.MaxRequestedBandwidthDL,
		})
	}

	if s.MaxRequestedBandwidthUL, err = optionalUnsigned(inner, AVPMaxRequestedBandwidthUL); err != nil {
		return nil, err
	}

	if s.MaxRequestedBandwidthDL, err = optionalUnsigned(inner, AVPMaxRequestedBandwidthDL); err != nil {
		return nil, err
	}

	return &s, nil
}
