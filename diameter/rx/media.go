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
	maxCodecData       = 2
	codecDataSeparator = "\n"
)

type MediaComponent struct {
	Number                  uint32
	SubComponents           []MediaSubComponent
	AFApplicationIdentifier string
	Type                    *MediaType
	MaxRequestedBandwidthUL *Bandwidth
	MaxRequestedBandwidthDL *Bandwidth
	MaxSupportedBandwidthUL *Bandwidth
	MaxSupportedBandwidthDL *Bandwidth
	MinDesiredBandwidthUL   *Bandwidth
	MinDesiredBandwidthDL   *Bandwidth
	MinRequestedBandwidthUL *Bandwidth
	MinRequestedBandwidthDL *Bandwidth
	FlowStatus              *FlowStatus
	RSBandwidth             *uint32
	RRBandwidth             *uint32
	CodecData               []CodecData
	ContentVersion          *uint64
}

type MediaSubComponent struct {
	FlowNumber              uint32
	FlowDescriptions        []string
	FlowStatus              *FlowStatus
	FlowUsage               *FlowUsage
	MaxRequestedBandwidthUL *Bandwidth
	MaxRequestedBandwidthDL *Bandwidth
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
	ContentVersions      []uint64
	FinalUnitAction      *FinalUnitAction
	MediaComponentStatus *MediaComponentStatus
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
	MaxRequestedBandwidthUL *Bandwidth
	MaxRequestedBandwidthDL *Bandwidth
}

type AcceptableServiceInfo struct {
	MediaComponents         []MediaBandwidth
	MaxRequestedBandwidthUL *Bandwidth
	MaxRequestedBandwidthDL *Bandwidth
}

var mediaComponentRules = diameter.Rules{
	vendorKey(AVPMediaComponentNumber):                     {Required: true, MinLength: 4},
	vendorKey(AVPMediaSubComponent):                        {Multiple: true},
	vendorKey(AVPAFApplicationIdentifier):                  {},
	vendorKey(avpFLUSIdentifier):                           {},
	vendorKey(AVPMediaType):                                {},
	vendorKey(AVPMaxRequestedBandwidthUL):                  {},
	vendorKey(AVPMaxRequestedBandwidthDL):                  {},
	vendorKey(AVPMaxSupportedBandwidthUL):                  {},
	vendorKey(AVPMaxSupportedBandwidthDL):                  {},
	vendorKey(AVPMinDesiredBandwidthUL):                    {},
	vendorKey(AVPMinDesiredBandwidthDL):                    {},
	vendorKey(AVPMinRequestedBandwidthUL):                  {},
	vendorKey(AVPMinRequestedBandwidthDL):                  {},
	vendorKey(AVPExtendedMaxRequestedBWUL):                 {},
	vendorKey(AVPExtendedMaxRequestedBWDL):                 {},
	vendorKey(AVPExtendedMaxSupportedBWUL):                 {},
	vendorKey(AVPExtendedMaxSupportedBWDL):                 {},
	vendorKey(AVPExtendedMinDesiredBWUL):                   {},
	vendorKey(AVPExtendedMinDesiredBWDL):                   {},
	vendorKey(AVPExtendedMinRequestedBWUL):                 {},
	vendorKey(AVPExtendedMinRequestedBWDL):                 {},
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
	vendorKey(AVPContentVersion):                           {},
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
	vendorKey(AVPExtendedMaxRequestedBWUL): {},
	vendorKey(AVPExtendedMaxRequestedBWDL): {},
	vendorKey(AVPAFSignallingProtocol):     {},
	vendorKey(avpToSTrafficClass):          {},
}

var flowsRules = diameter.Rules{
	vendorKey(AVPMediaComponentNumber):  {Required: true, MinLength: 4},
	vendorKey(AVPFlowNumber):            {Multiple: true},
	vendorKey(AVPContentVersion):        {Multiple: true},
	{Code: diameter.AVPFinalUnitAction}: {},
	vendorKey(AVPMediaComponentStatus):  {},
}

var chargingIdentifierRules = diameter.Rules{
	vendorKey(AVPAccessNetworkChargingIdentifierValue): {Required: true},
	vendorKey(AVPFlows): {Multiple: true},
}

var subscriptionIDRules = diameter.Rules{
	{Code: diameter.AVPSubscriptionIDType}: {Required: true, MinLength: 4},
	{Code: diameter.AVPSubscriptionIDData}: {Required: true},
}

var acceptableServiceInfoRules = diameter.Rules{
	vendorKey(AVPMediaComponentDescription): {Multiple: true},
	vendorKey(AVPMaxRequestedBandwidthDL):   {},
	vendorKey(AVPMaxRequestedBandwidthUL):   {},
	vendorKey(AVPExtendedMaxRequestedBWDL):  {},
	vendorKey(AVPExtendedMaxRequestedBWUL):  {},
}

func mediaComponentAVP(c MediaComponent) (diameter.AVP, error) {
	switch {
	case !validEnum(c.Type):
		return diameter.AVP{}, invalidf("Media-Type %s", *c.Type)
	case !validEnum(c.FlowStatus):
		return diameter.AVP{}, invalidf("Flow-Status %s", *c.FlowStatus)
	case len(c.CodecData) > maxCodecData:
		return diameter.AVP{}, invalidf("media component %d with %d Codec-Data", c.Number, len(c.CodecData))
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

	avps = appendOptional(avps, AVPMediaType, diameter.AVPFlagMandatory, (*uint32)(c.Type))

	var err error

	for _, b := range []struct {
		codes bandwidthCodes
		value *Bandwidth
	}{
		{maxRequestedUL, c.MaxRequestedBandwidthUL},
		{maxRequestedDL, c.MaxRequestedBandwidthDL},
		{maxSupportedUL, c.MaxSupportedBandwidthUL},
		{maxSupportedDL, c.MaxSupportedBandwidthDL},
		{minDesiredUL, c.MinDesiredBandwidthUL},
		{minDesiredDL, c.MinDesiredBandwidthDL},
		{minRequestedUL, c.MinRequestedBandwidthUL},
		{minRequestedDL, c.MinRequestedBandwidthDL},
	} {
		if avps, err = appendBandwidth(avps, b.codes, b.value); err != nil {
			return diameter.AVP{}, err
		}
	}

	avps = appendOptional(avps, AVPFlowStatus, diameter.AVPFlagMandatory, (*uint32)(c.FlowStatus))
	avps = appendOptional(avps, AVPRSBandwidth, diameter.AVPFlagMandatory, c.RSBandwidth)
	avps = appendOptional(avps, AVPRRBandwidth, diameter.AVPFlagMandatory, c.RRBandwidth)

	for _, d := range c.CodecData {
		a, err := codecDataAVP(d)
		if err != nil {
			return diameter.AVP{}, err
		}

		avps = append(avps, a)
	}

	if c.ContentVersion != nil {
		avps = append(avps, diameter.Unsigned64(AVPContentVersion, 0, tgpp.VendorID, *c.ContentVersion))
	}

	return vendorGrouped(AVPMediaComponentDescription, avps...), nil
}

func parseMediaComponent(a diameter.AVP) (MediaComponent, error) {
	c, err := parseMediaComponentData(a)
	return c, withinGrouped(a, err)
}

func parseMediaComponentData(a diameter.AVP) (MediaComponent, error) {
	inner, err := checkGrouped(mediaComponentRules, a)
	if err != nil {
		return MediaComponent{}, err
	}

	var c MediaComponent

	if c.Number, err = requiredUint32(inner, AVPMediaComponentNumber, tgpp.VendorID); err != nil {
		return MediaComponent{}, err
	}

	for _, sub := range diameter.FindAll(inner, AVPMediaSubComponent, tgpp.VendorID) {
		s, err := parseMediaSubComponent(sub)
		if err != nil {
			return MediaComponent{}, err
		}

		c.SubComponents = append(c.SubComponents, s)
	}

	if c.AFApplicationIdentifier, err = optionalString(inner, AVPAFApplicationIdentifier, tgpp.VendorID); err != nil {
		return MediaComponent{}, err
	}

	if c.Type, err = optionalEnum[MediaType](inner, AVPMediaType, tgpp.VendorID); err != nil {
		return MediaComponent{}, err
	}

	for _, b := range []struct {
		codes bandwidthCodes
		dst   **Bandwidth
	}{
		{maxRequestedUL, &c.MaxRequestedBandwidthUL},
		{maxRequestedDL, &c.MaxRequestedBandwidthDL},
		{maxSupportedUL, &c.MaxSupportedBandwidthUL},
		{maxSupportedDL, &c.MaxSupportedBandwidthDL},
		{minDesiredUL, &c.MinDesiredBandwidthUL},
		{minDesiredDL, &c.MinDesiredBandwidthDL},
		{minRequestedUL, &c.MinRequestedBandwidthUL},
		{minRequestedDL, &c.MinRequestedBandwidthDL},
	} {
		if *b.dst, err = parseBandwidth(inner, b.codes); err != nil {
			return MediaComponent{}, err
		}
	}

	if c.FlowStatus, err = optionalEnum[FlowStatus](inner, AVPFlowStatus, tgpp.VendorID); err != nil {
		return MediaComponent{}, err
	}

	if c.RSBandwidth, err = optionalUint32(inner, AVPRSBandwidth, tgpp.VendorID); err != nil {
		return MediaComponent{}, err
	}

	if c.RRBandwidth, err = optionalUint32(inner, AVPRRBandwidth, tgpp.VendorID); err != nil {
		return MediaComponent{}, err
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

	if v, ok := diameter.Find(inner, AVPContentVersion, tgpp.VendorID); ok {
		version, err := uint64Value(v)
		if err != nil {
			return MediaComponent{}, err
		}

		c.ContentVersion = &version
	}

	return c, nil
}

func signallingProtocolAllowed(s MediaSubComponent) bool {
	return s.SignallingProtocol == SignallingProtocolNoInformation ||
		(s.FlowUsage != nil && *s.FlowUsage == FlowUsageAFSignalling)
}

func mediaSubComponentAVP(s MediaSubComponent) (diameter.AVP, error) {
	switch {
	case !validEnum(s.FlowStatus):
		return diameter.AVP{}, invalidf("Flow-Status %s", *s.FlowStatus)
	case !validEnum(s.FlowUsage):
		return diameter.AVP{}, invalidf("Flow-Usage %s", *s.FlowUsage)
	case !s.SignallingProtocol.valid():
		return diameter.AVP{}, invalidf("AF-Signalling-Protocol %s", s.SignallingProtocol)
	case !signallingProtocolAllowed(s):
		return diameter.AVP{}, invalidf("AF-Signalling-Protocol on flow %d without AF_SIGNALLING usage", s.FlowNumber)
	}

	avps := []diameter.AVP{vendorUnsigned(AVPFlowNumber, s.FlowNumber)}

	for _, d := range s.FlowDescriptions {
		if d == "" {
			return diameter.AVP{}, invalidf("empty Flow-Description on flow %d", s.FlowNumber)
		}

		avps = append(avps, vendorOctets(AVPFlowDescription, []byte(d)))
	}

	avps = appendOptional(avps, AVPFlowStatus, diameter.AVPFlagMandatory, (*uint32)(s.FlowStatus))
	avps = appendOptional(avps, AVPFlowUsage, diameter.AVPFlagMandatory, (*uint32)(s.FlowUsage))

	avps, err := appendBandwidth(avps, maxRequestedUL, s.MaxRequestedBandwidthUL)
	if err != nil {
		return diameter.AVP{}, err
	}

	if avps, err = appendBandwidth(avps, maxRequestedDL, s.MaxRequestedBandwidthDL); err != nil {
		return diameter.AVP{}, err
	}

	if s.SignallingProtocol != SignallingProtocolNoInformation {
		avps = append(avps, diameter.Unsigned32(AVPAFSignallingProtocol, 0, tgpp.VendorID, uint32(s.SignallingProtocol)))
	}

	return vendorGrouped(AVPMediaSubComponent, avps...), nil
}

func parseMediaSubComponent(a diameter.AVP) (MediaSubComponent, error) {
	s, err := parseMediaSubComponentData(a)
	return s, withinGrouped(a, err)
}

func parseMediaSubComponentData(a diameter.AVP) (MediaSubComponent, error) {
	inner, err := checkGrouped(mediaSubComponentRules, a)
	if err != nil {
		return MediaSubComponent{}, err
	}

	var s MediaSubComponent

	if s.FlowNumber, err = requiredUint32(inner, AVPFlowNumber, tgpp.VendorID); err != nil {
		return MediaSubComponent{}, err
	}

	for _, d := range diameter.FindAll(inner, AVPFlowDescription, tgpp.VendorID) {
		if len(d.Data) == 0 {
			return MediaSubComponent{}, tgpp.InvalidAVP(d)
		}

		s.FlowDescriptions = append(s.FlowDescriptions, string(d.Data))
	}

	if s.FlowStatus, err = optionalEnum[FlowStatus](inner, AVPFlowStatus, tgpp.VendorID); err != nil {
		return MediaSubComponent{}, err
	}

	if s.FlowUsage, err = optionalEnum[FlowUsage](inner, AVPFlowUsage, tgpp.VendorID); err != nil {
		return MediaSubComponent{}, err
	}

	if s.MaxRequestedBandwidthUL, err = parseBandwidth(inner, maxRequestedUL); err != nil {
		return MediaSubComponent{}, err
	}

	if s.MaxRequestedBandwidthDL, err = parseBandwidth(inner, maxRequestedDL); err != nil {
		return MediaSubComponent{}, err
	}

	if s.SignallingProtocol, err = defaultEnum[AFSignallingProtocol](inner, AVPAFSignallingProtocol, tgpp.VendorID); err != nil {
		return MediaSubComponent{}, err
	}

	if !signallingProtocolAllowed(s) {
		protocol, _ := diameter.Find(inner, AVPAFSignallingProtocol, tgpp.VendorID)
		return MediaSubComponent{}, tgpp.InvalidAVP(protocol)
	}

	return s, nil
}

func codecDataAVP(d CodecData) (diameter.AVP, error) {
	if !d.Direction.valid() || !d.Kind.valid() {
		return diameter.AVP{}, invalidf("Codec-Data %s %s", d.Direction, d.Kind)
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

func flowsAVP(f Flows) (diameter.AVP, error) {
	switch {
	case !validEnum(f.FinalUnitAction):
		return diameter.AVP{}, invalidf("Final-Unit-Action %s", *f.FinalUnitAction)
	case !validEnum(f.MediaComponentStatus):
		return diameter.AVP{}, invalidf("Media-Component-Status %s", *f.MediaComponentStatus)
	}

	avps := []diameter.AVP{vendorUnsigned(AVPMediaComponentNumber, f.MediaComponentNumber)}
	for _, n := range f.FlowNumbers {
		avps = append(avps, vendorUnsigned(AVPFlowNumber, n))
	}

	for _, v := range f.ContentVersions {
		avps = append(avps, diameter.Unsigned64(AVPContentVersion, 0, tgpp.VendorID, v))
	}

	if f.FinalUnitAction != nil {
		avps = append(avps, diameter.Unsigned32(diameter.AVPFinalUnitAction, diameter.AVPFlagMandatory, 0, uint32(*f.FinalUnitAction)))
	}

	avps = appendOptional(avps, AVPMediaComponentStatus, 0, (*uint32)(f.MediaComponentStatus))

	return vendorGrouped(AVPFlows, avps...), nil
}

func parseFlows(a diameter.AVP) (Flows, error) {
	f, err := parseFlowsData(a)
	return f, withinGrouped(a, err)
}

func parseFlowsData(a diameter.AVP) (Flows, error) {
	inner, err := checkGrouped(flowsRules, a)
	if err != nil {
		return Flows{}, err
	}

	var f Flows

	if f.MediaComponentNumber, err = requiredUint32(inner, AVPMediaComponentNumber, tgpp.VendorID); err != nil {
		return Flows{}, err
	}

	for _, n := range diameter.FindAll(inner, AVPFlowNumber, tgpp.VendorID) {
		v, err := tgpp.Unsigned32(n)
		if err != nil {
			return Flows{}, err
		}

		f.FlowNumbers = append(f.FlowNumbers, v)
	}

	for _, n := range diameter.FindAll(inner, AVPContentVersion, tgpp.VendorID) {
		v, err := uint64Value(n)
		if err != nil {
			return Flows{}, err
		}

		f.ContentVersions = append(f.ContentVersions, v)
	}

	if f.FinalUnitAction, err = optionalEnum[FinalUnitAction](inner, diameter.AVPFinalUnitAction, 0); err != nil {
		return Flows{}, err
	}

	if f.MediaComponentStatus, err = optionalEnum[MediaComponentStatus](inner, AVPMediaComponentStatus, tgpp.VendorID); err != nil {
		return Flows{}, err
	}

	return f, nil
}

func chargingIdentifierAVP(c AccessNetworkChargingIdentifier) (diameter.AVP, error) {
	if len(c.Value) == 0 {
		return diameter.AVP{}, invalidf("empty Access-Network-Charging-Identifier-Value")
	}

	flows, err := flowsAVPs(c.Flows)
	if err != nil {
		return diameter.AVP{}, err
	}

	avps := append([]diameter.AVP{vendorOctets(AVPAccessNetworkChargingIdentifierValue, c.Value)}, flows...)

	return vendorGrouped(AVPAccessNetworkChargingIdentifier, avps...), nil
}

func parseChargingIdentifier(a diameter.AVP) (AccessNetworkChargingIdentifier, error) {
	c, err := parseChargingIdentifierData(a)
	return c, withinGrouped(a, err)
}

func parseChargingIdentifierData(a diameter.AVP) (AccessNetworkChargingIdentifier, error) {
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
	case !s.Type.valid():
		return diameter.AVP{}, invalidf("Subscription-Id-Type %s", s.Type)
	case s.Data == "":
		return diameter.AVP{}, invalidf("empty Subscription-Id-Data")
	}

	return diameter.Grouped(diameter.AVPSubscriptionID, diameter.AVPFlagMandatory, 0,
		diameter.Unsigned32(diameter.AVPSubscriptionIDType, diameter.AVPFlagMandatory, 0, uint32(s.Type)),
		diameter.UTF8String(diameter.AVPSubscriptionIDData, diameter.AVPFlagMandatory, 0, s.Data),
	), nil
}

func parseSubscriptionID(a diameter.AVP) (SubscriptionID, error) {
	s, err := parseSubscriptionIDData(a)
	return s, withinGrouped(a, err)
}

func parseSubscriptionIDData(a diameter.AVP) (SubscriptionID, error) {
	inner, err := checkGrouped(subscriptionIDRules, a)
	if err != nil {
		return SubscriptionID{}, err
	}

	t, err := defaultEnum[SubscriptionIDType](inner, diameter.AVPSubscriptionIDType, 0)
	if err != nil {
		return SubscriptionID{}, err
	}

	data, _ := diameter.Find(inner, diameter.AVPSubscriptionIDData, 0)

	return SubscriptionID{Type: t, Data: data.UTF8String()}, nil
}

func acceptableServiceInfoAVP(s AcceptableServiceInfo) (diameter.AVP, error) {
	var avps []diameter.AVP

	for _, m := range s.MediaComponents {
		inner, err := bandwidthPair([]diameter.AVP{vendorUnsigned(AVPMediaComponentNumber, m.MediaComponentNumber)},
			m.MaxRequestedBandwidthUL, m.MaxRequestedBandwidthDL)
		if err != nil {
			return diameter.AVP{}, err
		}

		avps = append(avps, vendorGrouped(AVPMediaComponentDescription, inner...))
	}

	avps, err := bandwidthPair(avps, s.MaxRequestedBandwidthUL, s.MaxRequestedBandwidthDL)
	if err != nil {
		return diameter.AVP{}, err
	}

	return vendorGrouped(AVPAcceptableServiceInfo, avps...), nil
}

func bandwidthPair(avps []diameter.AVP, ul, dl *Bandwidth) ([]diameter.AVP, error) {
	avps, err := appendBandwidth(avps, maxRequestedUL, ul)
	if err != nil {
		return nil, err
	}

	return appendBandwidth(avps, maxRequestedDL, dl)
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

	if s.MaxRequestedBandwidthUL, err = parseBandwidth(inner, maxRequestedUL); err != nil {
		return nil, err
	}

	if s.MaxRequestedBandwidthDL, err = parseBandwidth(inner, maxRequestedDL); err != nil {
		return nil, err
	}

	return &s, nil
}
