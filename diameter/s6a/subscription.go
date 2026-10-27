// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"bytes"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const WildcardAPN = "*"

type SubscriptionData struct {
	SubscriberStatus                         *SubscriberStatus
	MSISDN                                   string
	AMSISDN                                  string
	STNSR                                    string
	NetworkAccessMode                        *NetworkAccessMode
	OperatorDeterminedBarring                *OperatorDeterminedBarring
	HPLMNODB                                 *uint32
	RegionalSubscriptionZoneCodes            [][]byte
	AccessRestrictionData                    *AccessRestrictionData
	APNOIReplacement                         string
	ChargingCharacteristics                  string
	AMBR                                     *AMBR
	APNConfigurationProfile                  *APNConfigurationProfile
	RATFrequencySelectionPriorityID          uint32
	RoamingRestrictedDueToUnsupportedFeature bool
	SubscribedPeriodicRAUTAUTimer            time.Duration
	UEUsageType                              *uint32
	SubscribedVSRVCC                         bool
}

type APNConfigurationProfile struct {
	ContextIdentifier           uint32
	AdditionalContextIdentifier uint32
	ThirdContextIdentifier      uint32
	Included                    APNConfigurationsIncluded
	Configurations              []APNConfiguration
}

type APNConfiguration struct {
	ContextIdentifier          uint32
	ServedPartyIPAddresses     []netip.Addr
	PDNType                    PDNType
	ServiceSelection           string
	QoS                        *EPSSubscribedQoSProfile
	VPLMNDynamicAddressAllowed bool
	PDNGateway                 *MIP6AgentInfo
	VisitedNetworkIdentifier   string
	PDNGWAllocationType        *PDNGWAllocationType
	ChargingCharacteristics    string
	AMBR                       *AMBR
	SpecificAPNInfo            []SpecificAPNInfo
	APNOIReplacement           string
	Interworking5GSIndicator   bool
}

type SpecificAPNInfo struct {
	ServiceSelection         string
	PDNGateway               MIP6AgentInfo
	VisitedNetworkIdentifier string
}

func (p APNConfigurationProfile) Default() (APNConfiguration, bool) {
	return p.Configuration(p.ContextIdentifier)
}

func (p APNConfigurationProfile) Configuration(contextID uint32) (APNConfiguration, bool) {
	for _, c := range p.Configurations {
		if c.ContextIdentifier == contextID {
			return c, true
		}
	}

	return APNConfiguration{}, false
}

func (p APNConfigurationProfile) Lookup(apn string) (APNConfiguration, bool) {
	for _, c := range p.Configurations {
		if strings.EqualFold(c.ServiceSelection, apn) {
			return c, true
		}
	}

	return APNConfiguration{}, false
}

func (c APNConfiguration) Wildcard() bool {
	return c.ServiceSelection == WildcardAPN
}

func (s SubscriptionData) avp() (diameter.AVP, error) {
	var (
		inner []diameter.AVP
		err   error
	)

	if inner, err = appendEnum(inner, AVPSubscriberStatus, "Subscriber-Status", s.SubscriberStatus); err != nil {
		return diameter.AVP{}, err
	}

	for _, n := range []struct {
		code         uint32
		name, number string
	}{
		{tgpp.AVPMSISDN, "MSISDN", s.MSISDN},
		{AVPAMSISDN, "A-MSISDN", s.AMSISDN},
		{AVPSTNSR, "STN-SR", s.STNSR},
	} {
		if inner, err = appendE164(inner, n.code, n.name, n.number); err != nil {
			return diameter.AVP{}, err
		}
	}

	if inner, err = appendEnum(inner, AVPNetworkAccessMode, "Network-Access-Mode", s.NetworkAccessMode); err != nil {
		return diameter.AVP{}, err
	}

	if s.OperatorDeterminedBarring != nil {
		inner = append(inner, unsignedAVP(AVPOperatorDeterminedBarring, uint32(*s.OperatorDeterminedBarring)))
	}

	if s.HPLMNODB != nil {
		inner = append(inner, unsignedAVP(AVPHPLMNODB, *s.HPLMNODB))
	}

	if len(s.RegionalSubscriptionZoneCodes) > maxZoneCodes {
		return diameter.AVP{}, invalidf("%d regional subscription zone codes", len(s.RegionalSubscriptionZoneCodes))
	}

	for _, zc := range s.RegionalSubscriptionZoneCodes {
		inner = append(inner, octetsAVP(AVPRegionalSubscriptionZoneCode, zc))
	}

	if s.AccessRestrictionData != nil {
		inner = append(inner, unsignedAVP(AVPAccessRestrictionData, uint32(*s.AccessRestrictionData)))
	}

	inner = appendString(inner, AVPAPNOIReplacement, s.APNOIReplacement)
	inner = appendString(inner, tgpp.AVP3GPPChargingCharacteristics, s.ChargingCharacteristics)

	if s.AMBR != nil {
		ambr, err := s.AMBR.avp()
		if err != nil {
			return diameter.AVP{}, err
		}

		inner = append(inner, ambr)
	}

	if s.APNConfigurationProfile != nil {
		profile, err := s.APNConfigurationProfile.avp()
		if err != nil {
			return diameter.AVP{}, err
		}

		inner = append(inner, profile)
	}

	if s.RATFrequencySelectionPriorityID != 0 {
		if s.RATFrequencySelectionPriorityID > maxRFSP {
			return diameter.AVP{}, invalidf("RAT-Frequency-Selection-Priority-ID %d", s.RATFrequencySelectionPriorityID)
		}

		inner = append(inner, unsignedAVP(AVPRATFrequencySelectionPriorityID, s.RATFrequencySelectionPriorityID))
	}

	if s.RoamingRestrictedDueToUnsupportedFeature {
		inner = append(inner, unsignedAVP(AVPRoamingRestrictedDueToUnsupportedFeature, roamingRestrictedValue))
	}

	if s.SubscribedPeriodicRAUTAUTimer != 0 {
		seconds := s.SubscribedPeriodicRAUTAUTimer / time.Second
		if s.SubscribedPeriodicRAUTAUTimer < 0 || s.SubscribedPeriodicRAUTAUTimer%time.Second != 0 || seconds > 1<<32-1 {
			return diameter.AVP{}, invalidf("subscribed periodic RAU/TAU timer %v", s.SubscribedPeriodicRAUTAUTimer)
		}

		inner = append(inner, unsignedAVP(AVPSubscribedPeriodicRAUTAUTimer, uint32(seconds)))
	}

	if s.SubscribedVSRVCC {
		inner = append(inner, unsignedAVP(AVPSubscribedVSRVCC, vsrvccSubscribedValue))
	}

	if s.UEUsageType != nil {
		if *s.UEUsageType > maxUEUsage {
			return diameter.AVP{}, invalidf("UE-Usage-Type %d", *s.UEUsageType)
		}

		inner = append(inner, unsignedAVP(AVPUEUsageType, *s.UEUsageType))
	}

	return groupedAVP(AVPSubscriptionData, inner...), nil
}

func parseSubscriptionData(a diameter.AVP) (*SubscriptionData, error) {
	inner, err := grouped(a)
	if err != nil {
		return nil, err
	}

	var s SubscriptionData

	if s.SubscriberStatus, err = optionalEnum[SubscriberStatus](inner, AVPSubscriberStatus); err != nil {
		return nil, err
	}

	for _, n := range []struct {
		code uint32
		dst  *string
	}{
		{tgpp.AVPMSISDN, &s.MSISDN},
		{AVPAMSISDN, &s.AMSISDN},
		{AVPSTNSR, &s.STNSR},
	} {
		if *n.dst, err = optionalE164(inner, n.code); err != nil {
			return nil, err
		}
	}

	if s.NetworkAccessMode, err = optionalEnum[NetworkAccessMode](inner, AVPNetworkAccessMode); err != nil {
		return nil, err
	}

	if v, ok, err := optionalUnsigned(inner, AVPOperatorDeterminedBarring); err != nil {
		return nil, err
	} else if ok {
		odb := OperatorDeterminedBarring(v)
		s.OperatorDeterminedBarring = &odb
	}

	if v, ok, err := optionalUnsigned(inner, AVPHPLMNODB); err != nil {
		return nil, err
	} else if ok {
		s.HPLMNODB = &v
	}

	zoneCodes := diameter.FindAll(inner, AVPRegionalSubscriptionZoneCode, tgpp.VendorID)
	if len(zoneCodes) > maxZoneCodes {
		return nil, diameter.NewAVPError(diameter.ResultAVPOccursTooManyTimes, zoneCodes[maxZoneCodes])
	}

	for _, zc := range zoneCodes {
		s.RegionalSubscriptionZoneCodes = append(s.RegionalSubscriptionZoneCodes, bytes.Clone(zc.Data))
	}

	if v, ok, err := optionalUnsigned(inner, AVPAccessRestrictionData); err != nil {
		return nil, err
	} else if ok {
		ard := AccessRestrictionData(v) & ardDefined
		s.AccessRestrictionData = &ard
	}

	if s.APNOIReplacement, err = optionalString(inner, AVPAPNOIReplacement, tgpp.VendorID); err != nil {
		return nil, err
	}

	if s.ChargingCharacteristics, err = optionalString(inner, tgpp.AVP3GPPChargingCharacteristics, tgpp.VendorID); err != nil {
		return nil, err
	}

	if ambr, ok := diameter.Find(inner, AVPAMBR, tgpp.VendorID); ok {
		if s.AMBR, err = parseAMBR(ambr); err != nil {
			return nil, err
		}
	}

	if profile, ok := diameter.Find(inner, AVPAPNConfigurationProfile, tgpp.VendorID); ok {
		if s.APNConfigurationProfile, err = parseAPNConfigurationProfile(profile); err != nil {
			return nil, err
		}
	}

	if rfsp, ok := diameter.Find(inner, AVPRATFrequencySelectionPriorityID, tgpp.VendorID); ok {
		if s.RATFrequencySelectionPriorityID, err = tgpp.Unsigned32(rfsp); err != nil {
			return nil, err
		}

		if s.RATFrequencySelectionPriorityID < 1 || s.RATFrequencySelectionPriorityID > maxRFSP {
			return nil, tgpp.InvalidAVP(rfsp)
		}
	}

	if s.RoamingRestrictedDueToUnsupportedFeature, err = optionalPresence(inner, AVPRoamingRestrictedDueToUnsupportedFeature, roamingRestrictedValue); err != nil {
		return nil, err
	}

	if v, ok, err := optionalUnsigned(inner, AVPSubscribedPeriodicRAUTAUTimer); err != nil {
		return nil, err
	} else if ok {
		s.SubscribedPeriodicRAUTAUTimer = time.Duration(v) * time.Second
	}

	if s.SubscribedVSRVCC, err = optionalPresence(inner, AVPSubscribedVSRVCC, vsrvccSubscribedValue); err != nil {
		return nil, err
	}

	if usage, ok := diameter.Find(inner, AVPUEUsageType, tgpp.VendorID); ok {
		v, err := tgpp.Unsigned32(usage)
		if err != nil {
			return nil, err
		}

		if v > maxUEUsage {
			return nil, tgpp.InvalidAVP(usage)
		}

		s.UEUsageType = &v
	}

	return &s, nil
}

func (p APNConfigurationProfile) avp() (diameter.AVP, error) {
	switch {
	case p.ContextIdentifier == 0:
		return diameter.AVP{}, invalidf("APN configuration profile without a default context identifier")
	case !p.Included.valid():
		return diameter.AVP{}, invalidf("All-APN-Configurations-Included-Indicator %d", p.Included)
	case len(p.Configurations) == 0:
		return diameter.AVP{}, invalidf("APN configuration profile without an APN configuration")
	}

	if _, ok := p.Default(); !ok && p.Included == AllAPNConfigurationsIncluded {
		return diameter.AVP{}, invalidf("default context identifier %d matches no APN configuration", p.ContextIdentifier)
	}

	if err := validateConfigurations(p.Configurations); err != nil {
		return diameter.AVP{}, invalidf("%w", err)
	}

	inner := []diameter.AVP{unsignedAVP(AVPContextIdentifier, p.ContextIdentifier)}

	if p.AdditionalContextIdentifier != 0 {
		inner = append(inner, unsignedAVP(AVPAdditionalContextIdentifier, p.AdditionalContextIdentifier))
	}

	if p.ThirdContextIdentifier != 0 {
		inner = append(inner, unsignedAVP(AVPThirdContextIdentifier, p.ThirdContextIdentifier))
	}

	inner = append(inner, unsignedAVP(AVPAllAPNConfigurationsIncludedIndicator, uint32(p.Included)))

	for _, c := range p.Configurations {
		avp, err := c.avp()
		if err != nil {
			return diameter.AVP{}, err
		}

		inner = append(inner, avp)
	}

	return groupedAVP(AVPAPNConfigurationProfile, inner...), nil
}

func validateConfigurations(configs []APNConfiguration) error {
	ids := make(map[uint32]bool, len(configs))
	apns := make(map[string]bool, len(configs))

	for _, c := range configs {
		apn := strings.ToLower(c.ServiceSelection)

		switch {
		case ids[c.ContextIdentifier]:
			return fmt.Errorf("duplicate context identifier %d", c.ContextIdentifier)
		case apns[apn]:
			return fmt.Errorf("duplicate APN %q", c.ServiceSelection)
		}

		ids[c.ContextIdentifier], apns[apn] = true, true
	}

	return nil
}

func parseAPNConfigurationProfile(a diameter.AVP) (*APNConfigurationProfile, error) {
	inner, err := grouped(a)
	if err != nil {
		return nil, err
	}

	var p APNConfigurationProfile

	if p.ContextIdentifier, err = requiredUnsigned(inner, AVPContextIdentifier); err != nil {
		return nil, err
	}

	if p.AdditionalContextIdentifier, _, err = optionalUnsigned(inner, AVPAdditionalContextIdentifier); err != nil {
		return nil, err
	}

	if p.ThirdContextIdentifier, _, err = optionalUnsigned(inner, AVPThirdContextIdentifier); err != nil {
		return nil, err
	}

	if p.Included, err = requiredEnum[APNConfigurationsIncluded](inner, AVPAllAPNConfigurationsIncludedIndicator); err != nil {
		return nil, err
	}

	configs := diameter.FindAll(inner, AVPAPNConfiguration, tgpp.VendorID)
	if len(configs) == 0 {
		return nil, tgpp.MissingAVP(AVPAPNConfiguration, tgpp.VendorID, 0)
	}

	for _, ca := range configs {
		c, err := parseAPNConfiguration(ca)
		if err != nil {
			return nil, err
		}

		p.Configurations = append(p.Configurations, c)
	}

	if err := validateConfigurations(p.Configurations); err != nil {
		return nil, tgpp.InvalidAVP(a)
	}

	return &p, nil
}

func (c APNConfiguration) avp() (diameter.AVP, error) {
	switch {
	case c.ContextIdentifier == 0:
		return diameter.AVP{}, invalidf("APN configuration with a zero context identifier")
	case c.ServiceSelection == "":
		return diameter.AVP{}, invalidf("APN configuration %d without an APN", c.ContextIdentifier)
	case !c.PDNType.valid():
		return diameter.AVP{}, invalidf("PDN-Type %d", c.PDNType)
	case len(c.ServedPartyIPAddresses) > maxServedIPs:
		return diameter.AVP{}, invalidf("%d served party IP addresses", len(c.ServedPartyIPAddresses))
	case c.PDNGWAllocationType != nil && c.PDNGateway == nil:
		return diameter.AVP{}, invalidf("PDN-GW-Allocation-Type without MIP6-Agent-Info")
	}

	inner := []diameter.AVP{unsignedAVP(AVPContextIdentifier, c.ContextIdentifier)}

	var families [2]bool

	for _, addr := range c.ServedPartyIPAddresses {
		addr = addr.Unmap()

		family := 0
		if addr.Is6() {
			family = 1
		}

		if !addr.IsValid() || families[family] {
			return diameter.AVP{}, invalidf("served party IP address %v", addr)
		}

		families[family] = true

		inner = append(inner, diameter.Address(AVPServedPartyIPAddress, avpFlags(AVPServedPartyIPAddress), tgpp.VendorID, addr))
	}

	inner = append(inner,
		unsignedAVP(AVPPDNType, uint32(c.PDNType)),
		diameter.UTF8String(AVPServiceSelection, diameter.AVPFlagMandatory, 0, c.ServiceSelection),
	)

	if c.QoS != nil {
		qos, err := c.QoS.avp()
		if err != nil {
			return diameter.AVP{}, err
		}

		inner = append(inner, qos)
	}

	if c.VPLMNDynamicAddressAllowed {
		inner = append(inner, unsignedAVP(AVPVPLMNDynamicAddressAllowed, vplmnDynamicAddressAllowed))
	}

	if c.PDNGateway != nil {
		gw, err := c.PDNGateway.avp()
		if err != nil {
			return diameter.AVP{}, err
		}

		inner = append(inner, gw)
	}

	inner = appendString(inner, tgpp.AVPVisitedNetworkIdentifier, c.VisitedNetworkIdentifier)

	var err error

	if inner, err = appendEnum(inner, AVPPDNGWAllocationType, "PDN-GW-Allocation-Type", c.PDNGWAllocationType); err != nil {
		return diameter.AVP{}, err
	}

	inner = appendString(inner, tgpp.AVP3GPPChargingCharacteristics, c.ChargingCharacteristics)

	if c.AMBR != nil {
		ambr, err := c.AMBR.avp()
		if err != nil {
			return diameter.AVP{}, err
		}

		inner = append(inner, ambr)
	}

	for _, info := range c.SpecificAPNInfo {
		avp, err := info.avp()
		if err != nil {
			return diameter.AVP{}, err
		}

		inner = append(inner, avp)
	}

	inner = appendString(inner, AVPAPNOIReplacement, c.APNOIReplacement)

	if c.Interworking5GSIndicator {
		inner = append(inner, unsignedAVP(AVPInterworking5GSIndicator, enumTrue))
	}

	return groupedAVP(AVPAPNConfiguration, inner...), nil
}

func parseAPNConfiguration(a diameter.AVP) (APNConfiguration, error) {
	inner, err := grouped(a)
	if err != nil {
		return APNConfiguration{}, err
	}

	var c APNConfiguration

	id, ok := diameter.Find(inner, AVPContextIdentifier, tgpp.VendorID)
	if !ok {
		return APNConfiguration{}, tgpp.MissingAVP(AVPContextIdentifier, tgpp.VendorID, 4)
	}

	if c.ContextIdentifier, err = tgpp.Unsigned32(id); err != nil {
		return APNConfiguration{}, err
	}

	if c.ContextIdentifier == 0 {
		return APNConfiguration{}, tgpp.InvalidAVP(id)
	}

	addrs := diameter.FindAll(inner, AVPServedPartyIPAddress, tgpp.VendorID)
	if len(addrs) > maxServedIPs {
		return APNConfiguration{}, diameter.NewAVPError(diameter.ResultAVPOccursTooManyTimes, addrs[maxServedIPs])
	}

	for _, addrAVP := range addrs {
		addr, err := addrAVP.Address()
		if err != nil {
			return APNConfiguration{}, tgpp.InvalidAVP(addrAVP)
		}

		c.ServedPartyIPAddresses = append(c.ServedPartyIPAddresses, addr)
	}

	if c.PDNType, err = requiredEnum[PDNType](inner, AVPPDNType); err != nil {
		return APNConfiguration{}, err
	}

	apn, ok := diameter.Find(inner, AVPServiceSelection, 0)
	if !ok {
		return APNConfiguration{}, tgpp.MissingAVP(AVPServiceSelection, 0, 0)
	}

	if c.ServiceSelection = apn.UTF8String(); c.ServiceSelection == "" {
		return APNConfiguration{}, tgpp.InvalidAVP(apn)
	}

	if qos, ok := diameter.Find(inner, AVPEPSSubscribedQoSProfile, tgpp.VendorID); ok {
		if c.QoS, err = parseEPSSubscribedQoSProfile(qos); err != nil {
			return APNConfiguration{}, err
		}
	}

	if dyn, ok := diameter.Find(inner, AVPVPLMNDynamicAddressAllowed, tgpp.VendorID); ok {
		v, err := tgpp.Unsigned32(dyn)
		if err != nil {
			return APNConfiguration{}, err
		}

		switch v {
		case vplmnDynamicAddressNotAllowed:
		case vplmnDynamicAddressAllowed:
			c.VPLMNDynamicAddressAllowed = true
		default:
			return APNConfiguration{}, tgpp.InvalidAVP(dyn)
		}
	}

	if gw, ok := diameter.Find(inner, AVPMIP6AgentInfo, 0); ok {
		if c.PDNGateway, err = parseMIP6AgentInfo(gw); err != nil {
			return APNConfiguration{}, err
		}
	}

	if c.VisitedNetworkIdentifier, err = optionalString(inner, tgpp.AVPVisitedNetworkIdentifier, tgpp.VendorID); err != nil {
		return APNConfiguration{}, err
	}

	if c.PDNGWAllocationType, err = optionalEnum[PDNGWAllocationType](inner, AVPPDNGWAllocationType); err != nil {
		return APNConfiguration{}, err
	}

	if c.ChargingCharacteristics, err = optionalString(inner, tgpp.AVP3GPPChargingCharacteristics, tgpp.VendorID); err != nil {
		return APNConfiguration{}, err
	}

	if ambr, ok := diameter.Find(inner, AVPAMBR, tgpp.VendorID); ok {
		if c.AMBR, err = parseAMBR(ambr); err != nil {
			return APNConfiguration{}, err
		}
	}

	for _, info := range diameter.FindAll(inner, AVPSpecificAPNInfo, tgpp.VendorID) {
		s, err := parseSpecificAPNInfo(info)
		if err != nil {
			return APNConfiguration{}, err
		}

		c.SpecificAPNInfo = append(c.SpecificAPNInfo, s)
	}

	if c.APNOIReplacement, err = optionalString(inner, AVPAPNOIReplacement, tgpp.VendorID); err != nil {
		return APNConfiguration{}, err
	}

	if c.Interworking5GSIndicator, err = optionalBool(inner, AVPInterworking5GSIndicator); err != nil {
		return APNConfiguration{}, err
	}

	return c, nil
}

func (s SpecificAPNInfo) avp() (diameter.AVP, error) {
	if s.ServiceSelection == "" {
		return diameter.AVP{}, invalidf("specific APN info without an APN")
	}

	gw, err := s.PDNGateway.avp()
	if err != nil {
		return diameter.AVP{}, err
	}

	inner := []diameter.AVP{diameter.UTF8String(AVPServiceSelection, diameter.AVPFlagMandatory, 0, s.ServiceSelection), gw}

	return groupedAVP(AVPSpecificAPNInfo, appendString(inner, tgpp.AVPVisitedNetworkIdentifier, s.VisitedNetworkIdentifier)...), nil
}

func parseSpecificAPNInfo(a diameter.AVP) (SpecificAPNInfo, error) {
	inner, err := grouped(a)
	if err != nil {
		return SpecificAPNInfo{}, err
	}

	apn, ok := diameter.Find(inner, AVPServiceSelection, 0)
	if !ok {
		return SpecificAPNInfo{}, tgpp.MissingAVP(AVPServiceSelection, 0, 0)
	}

	if len(apn.Data) == 0 {
		return SpecificAPNInfo{}, tgpp.InvalidAVP(apn)
	}

	gwAVP, ok := diameter.Find(inner, AVPMIP6AgentInfo, 0)
	if !ok {
		return SpecificAPNInfo{}, tgpp.MissingAVP(AVPMIP6AgentInfo, 0, 0)
	}

	gw, err := parseMIP6AgentInfo(gwAVP)
	if err != nil {
		return SpecificAPNInfo{}, err
	}

	s := SpecificAPNInfo{ServiceSelection: apn.UTF8String(), PDNGateway: *gw}

	if s.VisitedNetworkIdentifier, err = optionalString(inner, tgpp.AVPVisitedNetworkIdentifier, tgpp.VendorID); err != nil {
		return SpecificAPNInfo{}, err
	}

	return s, nil
}
