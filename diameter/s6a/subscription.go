// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"bytes"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	WildcardAPN   = "*"
	maxRFSP       = 256
	maxZoneCodes  = 10
	maxServedIPs  = 2
	maxRAUTAUSecs = 1<<32 - 1
)

type SubscriptionData struct {
	SubscriberStatus                         *SubscriberStatus
	MSISDN                                   string
	AMSISDN                                  string
	STNSR                                    string
	NetworkAccessMode                        *NetworkAccessMode
	OperatorDeterminedBarring                *OperatorDeterminedBarring
	HPLMNODB                                 *HPLMNODB
	RegionalSubscriptionZoneCodes            [][]byte
	AccessRestrictionData                    *AccessRestrictionData
	APNOIReplacement                         string
	ChargingCharacteristics                  string
	AMBR                                     *AMBR
	APNConfigurationProfile                  *APNConfigurationProfile
	RATFrequencySelectionPriorityID          uint32
	RoamingRestrictedDueToUnsupportedFeature bool
	SubscribedPeriodicRAUTAUTimer            time.Duration
	UEUsageType                              *UEUsageType
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
	NonIPPDNType               bool
	NonIPDataDeliveryMechanism *NonIPDataDeliveryMechanism
	Interworking5GSIndicator   bool
	EthernetPDNType            bool
}

type SpecificAPNInfo struct {
	ServiceSelection         string
	PDNGateway               MIP6AgentInfo
	VisitedNetworkIdentifier string
}

func (c APNConfiguration) Wildcard() bool {
	return c.ServiceSelection == WildcardAPN
}

func (c APNConfiguration) IPBased() bool {
	return !c.NonIPPDNType && !c.EthernetPDNType
}

func (p APNConfigurationProfile) Default() (APNConfiguration, bool) {
	c, ok := p.Configuration(p.ContextIdentifier)
	if !ok || c.Wildcard() {
		return APNConfiguration{}, false
	}

	return c, true
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

func (p APNConfigurationProfile) Resolve(apn string) (APNConfiguration, bool) {
	if c, ok := p.Lookup(apn); ok {
		return c, true
	}

	return p.Lookup(WildcardAPN)
}

func (p APNConfigurationProfile) defaultContext(contextID uint32) bool {
	return contextID != 0 &&
		(contextID == p.ContextIdentifier || contextID == p.AdditionalContextIdentifier || contextID == p.ThirdContextIdentifier)
}

func (p APNConfigurationProfile) Merge(update APNConfigurationProfile) APNConfigurationProfile {
	if update.Included == AllAPNConfigurationsIncluded {
		update.Configurations = slices.Clone(update.Configurations)
		return update
	}

	merged := p
	merged.Configurations = slices.Clone(p.Configurations)
	merged.ContextIdentifier = update.ContextIdentifier

	if update.AdditionalContextIdentifier != 0 {
		merged.AdditionalContextIdentifier = update.AdditionalContextIdentifier
	}

	if update.ThirdContextIdentifier != 0 {
		merged.ThirdContextIdentifier = update.ThirdContextIdentifier
	}

	for _, c := range update.Configurations {
		i := slices.IndexFunc(merged.Configurations, func(s APNConfiguration) bool { return s.ContextIdentifier == c.ContextIdentifier })
		if i < 0 {
			merged.Configurations = append(merged.Configurations, c)
		} else {
			merged.Configurations[i] = c
		}
	}

	return merged
}

func (s SubscriptionData) Merge(update SubscriptionData) SubscriptionData {
	merged := s

	replace := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}

	if update.SubscriberStatus != nil {
		merged.SubscriberStatus = update.SubscriberStatus
	}

	replace(&merged.MSISDN, update.MSISDN)
	replace(&merged.AMSISDN, update.AMSISDN)
	replace(&merged.STNSR, update.STNSR)

	if update.NetworkAccessMode != nil {
		merged.NetworkAccessMode = update.NetworkAccessMode
	}

	if update.OperatorDeterminedBarring != nil {
		merged.OperatorDeterminedBarring = update.OperatorDeterminedBarring
	}

	if update.HPLMNODB != nil {
		merged.HPLMNODB = update.HPLMNODB
	}

	if update.RegionalSubscriptionZoneCodes != nil {
		merged.RegionalSubscriptionZoneCodes = slices.Clone(update.RegionalSubscriptionZoneCodes)
	}

	if update.AccessRestrictionData != nil {
		merged.AccessRestrictionData = update.AccessRestrictionData
	}

	replace(&merged.APNOIReplacement, update.APNOIReplacement)
	replace(&merged.ChargingCharacteristics, update.ChargingCharacteristics)

	if update.AMBR != nil {
		merged.AMBR = update.AMBR
	}

	if update.APNConfigurationProfile != nil {
		var profile APNConfigurationProfile
		if s.APNConfigurationProfile != nil {
			profile = *s.APNConfigurationProfile
		}

		profile = profile.Merge(*update.APNConfigurationProfile)
		merged.APNConfigurationProfile = &profile
	}

	if update.RATFrequencySelectionPriorityID != 0 {
		merged.RATFrequencySelectionPriorityID = update.RATFrequencySelectionPriorityID
	}

	merged.RoamingRestrictedDueToUnsupportedFeature = s.RoamingRestrictedDueToUnsupportedFeature || update.RoamingRestrictedDueToUnsupportedFeature

	if update.SubscribedPeriodicRAUTAUTimer != 0 {
		merged.SubscribedPeriodicRAUTAUTimer = update.SubscribedPeriodicRAUTAUTimer
	}

	if update.UEUsageType != nil {
		merged.UEUsageType = update.UEUsageType
	}

	merged.SubscribedVSRVCC = s.SubscribedVSRVCC || update.SubscribedVSRVCC

	return merged
}

func (s SubscriptionData) Withdraw(d SubscriptionDataDeletion) (SubscriptionData, error) {
	if d.Flags&DSRFlagCompleteAPNConfigurationProfileWithdrawal != 0 {
		return SubscriptionData{}, &ResultError{Result: tgpp.Result{Code: diameter.ResultUnableToComply}}
	}

	out := s

	if d.Flags&DSRFlagPDNSubscriptionContextsWithdrawal != 0 && s.APNConfigurationProfile != nil {
		profile := *s.APNConfigurationProfile

		for _, id := range d.ContextIdentifiers {
			if profile.defaultContext(id) {
				return SubscriptionData{}, &ResultError{Result: tgpp.Result{Code: diameter.ResultUnableToComply}}
			}
		}

		profile.Configurations = slices.DeleteFunc(slices.Clone(profile.Configurations), func(c APNConfiguration) bool {
			return slices.Contains(d.ContextIdentifiers, c.ContextIdentifier)
		})
		out.APNConfigurationProfile = &profile
	}

	for _, w := range []struct {
		flag  DSRFlags
		clear func()
	}{
		{DSRFlagRegionalSubscriptionWithdrawal, func() { out.RegionalSubscriptionZoneCodes = nil }},
		{DSRFlagSubscribedChargingCharacteristics, func() { out.ChargingCharacteristics = "" }},
		{DSRFlagSTNSR, func() { out.STNSR = "" }},
		{DSRFlagRoamingRestrictedDueToUnsupportedFeature, func() { out.RoamingRestrictedDueToUnsupportedFeature = false }},
		{DSRFlagAPNOIReplacement, func() { out.APNOIReplacement = "" }},
		{DSRFlagSubscribedPeriodicRAUTAUTimerWithdrawal, func() { out.SubscribedPeriodicRAUTAUTimer = 0 }},
		{DSRFlagSubscribedVSRVCCWithdrawal, func() { out.SubscribedVSRVCC = false }},
		{DSRFlagAMSISDNWithdrawal, func() { out.AMSISDN = "" }},
		{DSRFlagMSISDNWithdrawal, func() { out.MSISDN = "" }},
		{DSRFlagUEUsageTypeWithdrawal, func() { out.UEUsageType = nil }},
	} {
		if d.Flags&w.flag != 0 {
			w.clear()
		}
	}

	return out, nil
}

var subscriptionDataRules = diameter.Rules{
	vendorKey(AVPSubscriberStatus):                         {},
	vendorKey(tgpp.AVPMSISDN):                              {},
	vendorKey(AVPAMSISDN):                                  {},
	vendorKey(AVPSTNSR):                                    {},
	vendorKey(AVPICSIndicator):                             {},
	vendorKey(AVPNetworkAccessMode):                        {},
	vendorKey(AVPOperatorDeterminedBarring):                {},
	vendorKey(AVPHPLMNODB):                                 {},
	vendorKey(AVPRegionalSubscriptionZoneCode):             {Multiple: true},
	vendorKey(AVPAccessRestrictionData):                    {},
	vendorKey(AVPAPNOIReplacement):                         {},
	vendorKey(AVPLCSInfo):                                  {},
	vendorKey(AVPTeleserviceList):                          {},
	vendorKey(AVPCallBarringInfo):                          {Multiple: true},
	vendorKey(tgpp.AVP3GPPChargingCharacteristics):         {},
	vendorKey(AVPAMBR):                                     {},
	vendorKey(AVPAPNConfigurationProfile):                  {},
	vendorKey(AVPRATFrequencySelectionPriorityID):          {},
	vendorKey(AVPTraceData):                                {},
	vendorKey(AVPGPRSSubscriptionData):                     {},
	vendorKey(AVPCSGSubscriptionData):                      {Multiple: true},
	vendorKey(AVPRoamingRestrictedDueToUnsupportedFeature): {},
	vendorKey(AVPSubscribedPeriodicRAUTAUTimer):            {},
	vendorKey(AVPMPSPriority):                              {},
	vendorKey(AVPVPLMNLIPAAllowed):                         {},
	vendorKey(AVPRelayNodeIndicator):                       {},
	vendorKey(AVPMDTUserConsent):                           {},
	vendorKey(AVPSubscribedVSRVCC):                         {},
	vendorKey(AVPProSeSubscriptionData):                    {},
	vendorKey(AVPSubscriptionDataFlags):                    {},
	vendorKey(AVPAdjacentAccessRestrictionData):            {Multiple: true},
	vendorKey(AVPDLBufferingSuggestedPacketCount):          {},
	vendorKey(AVPIMSIGroupID):                              {Multiple: true},
	vendorKey(AVPUEUsageType):                              {},
	vendorKey(AVPAESECommunicationPattern):                 {Multiple: true},
	vendorKey(AVPMonitoringEventConfiguration):             {Multiple: true},
	vendorKey(AVPEmergencyInfo):                            {},
	vendorKey(AVPV2XSubscriptionData):                      {},
	vendorKey(AVPV2XSubscriptionDataNR):                    {},
	vendorKey(AVPEDRXCycleLength):                          {Multiple: true},
	vendorKey(AVPExternalIdentifier):                       {},
	vendorKey(AVPActiveTime):                               {},
	vendorKey(AVPServiceGapTime):                           {},
	vendorKey(AVPBroadcastLocationAssistanceDataTypes):     {},
	vendorKey(AVPAerialUESubscriptionInformation):          {},
	vendorKey(AVPCoreNetworkRestrictions):                  {},
	vendorKey(AVPPagingTimeWindow):                         {Multiple: true},
	vendorKey(AVPSubscribedARPI):                           {},
	vendorKey(AVPIABOperationPermission):                   {},
	vendorKey(AVPPLMNRATUsageControl):                      {},
}

var apnConfigurationProfileRules = diameter.Rules{
	vendorKey(AVPContextIdentifier):                     {},
	vendorKey(AVPAdditionalContextIdentifier):           {},
	vendorKey(AVPThirdContextIdentifier):                {},
	vendorKey(AVPAllAPNConfigurationsIncludedIndicator): {},
	vendorKey(AVPAPNConfiguration):                      {Multiple: true},
}

var apnConfigurationRules = diameter.Rules{
	vendorKey(AVPContextIdentifier):                {},
	vendorKey(AVPServedPartyIPAddress):             {Multiple: true},
	vendorKey(AVPPDNType):                          {},
	ietfKey(diameter.AVPServiceSelection):          {},
	vendorKey(AVPEPSSubscribedQoSProfile):          {},
	vendorKey(AVPVPLMNDynamicAddressAllowed):       {},
	ietfKey(diameter.AVPMIP6AgentInfo):             {},
	vendorKey(tgpp.AVPVisitedNetworkIdentifier):    {},
	vendorKey(AVPPDNGWAllocationType):              {},
	vendorKey(tgpp.AVP3GPPChargingCharacteristics): {},
	vendorKey(AVPAMBR):                             {},
	vendorKey(AVPSpecificAPNInfo):                  {Multiple: true},
	vendorKey(AVPAPNOIReplacement):                 {},
	vendorKey(AVPSIPTOPermission):                  {},
	vendorKey(AVPLIPAPermission):                   {},
	vendorKey(AVPRestorationPriority):              {},
	vendorKey(AVPSIPTOLocalNetworkPermission):      {},
	vendorKey(AVPWLANOffloadability):               {},
	vendorKey(AVPNonIPPDNTypeIndicator):            {},
	vendorKey(AVPNonIPDataDeliveryMechanism):       {},
	vendorKey(AVPSCEFID):                           {},
	vendorKey(AVPSCEFRealm):                        {},
	vendorKey(AVPPreferredDataMode):                {},
	vendorKey(AVPPDNConnectionContinuity):          {},
	vendorKey(AVPRDSIndicator):                     {},
	vendorKey(AVPInterworking5GSIndicator):         {},
	vendorKey(AVPEthernetPDNTypeIndicator):         {},
}

var specificAPNInfoRules = diameter.Rules{
	ietfKey(diameter.AVPServiceSelection):       {},
	ietfKey(diameter.AVPMIP6AgentInfo):          {},
	vendorKey(tgpp.AVPVisitedNetworkIdentifier): {},
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
		if inner, err = appendPresentFlags(inner, AVPOperatorDeterminedBarring, "Operator-Determined-Barring", *s.OperatorDeterminedBarring); err != nil {
			return diameter.AVP{}, err
		}
	}

	if s.HPLMNODB != nil {
		if inner, err = appendPresentFlags(inner, AVPHPLMNODB, "HPLMN-ODB", *s.HPLMNODB); err != nil {
			return diameter.AVP{}, err
		}
	}

	if len(s.RegionalSubscriptionZoneCodes) > maxZoneCodes {
		return diameter.AVP{}, invalidf("%d regional subscription zone codes", len(s.RegionalSubscriptionZoneCodes))
	}

	for _, zc := range s.RegionalSubscriptionZoneCodes {
		inner = append(inner, octetsAVP(AVPRegionalSubscriptionZoneCode, zc))
	}

	if s.AccessRestrictionData != nil {
		if inner, err = appendPresentFlags(inner, AVPAccessRestrictionData, "Access-Restriction-Data", *s.AccessRestrictionData); err != nil {
			return diameter.AVP{}, err
		}
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

	inner = appendIndicator(inner, AVPRoamingRestrictedDueToUnsupportedFeature, s.RoamingRestrictedDueToUnsupportedFeature, roamingRestrictedValue)

	if s.SubscribedPeriodicRAUTAUTimer != 0 {
		seconds := s.SubscribedPeriodicRAUTAUTimer / time.Second
		if s.SubscribedPeriodicRAUTAUTimer < 0 || s.SubscribedPeriodicRAUTAUTimer%time.Second != 0 || seconds > maxRAUTAUSecs {
			return diameter.AVP{}, invalidf("subscribed periodic RAU/TAU timer %v", s.SubscribedPeriodicRAUTAUTimer)
		}

		inner = append(inner, unsignedAVP(AVPSubscribedPeriodicRAUTAUTimer, uint32(seconds)))
	}

	inner = appendIndicator(inner, AVPSubscribedVSRVCC, s.SubscribedVSRVCC, vsrvccSubscribedValue)

	if s.UEUsageType != nil {
		inner = append(inner, s.UEUsageType.avp())
	}

	return groupedAVP(AVPSubscriptionData, inner...), nil
}

func appendPresentFlags[T known](avps []diameter.AVP, code uint32, name string, v T) ([]diameter.AVP, error) {
	if !v.Known() {
		return nil, invalidf("%s with undefined bits %#x", name, uint32(v))
	}

	return append(avps, unsignedAVP(code, uint32(v))), nil
}

func optionalBits[T ~uint32](avps []diameter.AVP, code uint32) (*T, error) {
	v, ok, err := optionalUnsigned(avps, code)
	if err != nil || !ok {
		return nil, err
	}

	b := T(v)

	return &b, nil
}

func (d decoder) subscriptionData(a diameter.AVP) (*SubscriptionData, error) {
	inner, err := d.group(a, subscriptionDataRules)
	if err != nil {
		return nil, err
	}

	s, err := d.subscriptionDataFields(inner)
	if err != nil {
		return nil, diameter.WithinGrouped(a, err)
	}

	return s, nil
}

func (d decoder) subscriptionDataFields(inner []diameter.AVP) (*SubscriptionData, error) {
	var (
		s   SubscriptionData
		err error
	)

	if s.SubscriberStatus, err = optionalEnum[SubscriberStatus](inner, AVPSubscriberStatus, false); err != nil {
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

	if s.NetworkAccessMode, err = optionalEnum[NetworkAccessMode](inner, AVPNetworkAccessMode, false); err != nil {
		return nil, err
	}

	if s.OperatorDeterminedBarring, err = optionalBits[OperatorDeterminedBarring](inner, AVPOperatorDeterminedBarring); err != nil {
		return nil, err
	}

	if s.HPLMNODB, err = optionalBits[HPLMNODB](inner, AVPHPLMNODB); err != nil {
		return nil, err
	}

	zoneCodes := diameter.FindAll(inner, AVPRegionalSubscriptionZoneCode, tgpp.VendorID)
	if len(zoneCodes) > maxZoneCodes {
		return nil, diameter.NewAVPError(diameter.ResultAVPOccursTooManyTimes, zoneCodes[maxZoneCodes])
	}

	for _, zc := range zoneCodes {
		s.RegionalSubscriptionZoneCodes = append(s.RegionalSubscriptionZoneCodes, bytes.Clone(zc.Data))
	}

	if s.AccessRestrictionData, err = optionalBits[AccessRestrictionData](inner, AVPAccessRestrictionData); err != nil {
		return nil, err
	}

	if s.APNOIReplacement, err = optionalString(inner, AVPAPNOIReplacement, tgpp.VendorID); err != nil {
		return nil, err
	}

	if s.ChargingCharacteristics, err = optionalString(inner, tgpp.AVP3GPPChargingCharacteristics, tgpp.VendorID); err != nil {
		return nil, err
	}

	if ambr, ok := diameter.Find(inner, AVPAMBR, tgpp.VendorID); ok {
		if s.AMBR, err = d.ambr(ambr); err != nil {
			return nil, err
		}
	}

	if profile, ok := diameter.Find(inner, AVPAPNConfigurationProfile, tgpp.VendorID); ok {
		if s.APNConfigurationProfile, err = d.apnConfigurationProfile(profile); err != nil {
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

	if s.RoamingRestrictedDueToUnsupportedFeature, err = optionalIndicator(inner, AVPRoamingRestrictedDueToUnsupportedFeature, roamingRestrictedValue); err != nil {
		return nil, err
	}

	if v, ok, err := optionalUnsigned(inner, AVPSubscribedPeriodicRAUTAUTimer); err != nil {
		return nil, err
	} else if ok {
		s.SubscribedPeriodicRAUTAUTimer = time.Duration(v) * time.Second
	}

	if s.SubscribedVSRVCC, err = optionalIndicator(inner, AVPSubscribedVSRVCC, vsrvccSubscribedValue); err != nil {
		return nil, err
	}

	if usage, ok := diameter.Find(inner, AVPUEUsageType, tgpp.VendorID); ok {
		u, err := parseUEUsageType(usage)
		if err != nil {
			return nil, err
		}

		s.UEUsageType = &u
	}

	return &s, nil
}

func (p APNConfigurationProfile) avp() (diameter.AVP, error) {
	switch {
	case p.ContextIdentifier == 0:
		return diameter.AVP{}, invalidf("APN configuration profile without a default context identifier")
	case !p.Included.Known():
		return diameter.AVP{}, invalidf("All-APN-Configurations-Included-Indicator %d", p.Included)
	case len(p.Configurations) == 0:
		return diameter.AVP{}, invalidf("APN configuration profile without an APN configuration")
	}

	def, hasDefault := p.Configuration(p.ContextIdentifier)

	switch {
	case !hasDefault && p.Included == AllAPNConfigurationsIncluded:
		return diameter.AVP{}, invalidf("default context identifier %d matches no APN configuration", p.ContextIdentifier)
	case hasDefault && def.Wildcard():
		return diameter.AVP{}, invalidf("default APN configuration %d is the wildcard APN", p.ContextIdentifier)
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

func (d decoder) apnConfigurationProfile(a diameter.AVP) (*APNConfigurationProfile, error) {
	inner, err := d.group(a, apnConfigurationProfileRules)
	if err != nil {
		return nil, err
	}

	p, err := d.apnConfigurationProfileFields(a, inner)
	if err != nil {
		return nil, diameter.WithinGrouped(a, err)
	}

	return p, nil
}

func (d decoder) apnConfigurationProfileFields(a diameter.AVP, inner []diameter.AVP) (*APNConfigurationProfile, error) {
	var (
		p   APNConfigurationProfile
		err error
	)

	if p.ContextIdentifier, err = requiredUnsigned(inner, AVPContextIdentifier); err != nil {
		return nil, err
	}

	if p.ContextIdentifier == 0 {
		id, _ := diameter.Find(inner, AVPContextIdentifier, tgpp.VendorID)
		return nil, tgpp.InvalidAVP(id)
	}

	if p.AdditionalContextIdentifier, _, err = optionalUnsigned(inner, AVPAdditionalContextIdentifier); err != nil {
		return nil, err
	}

	if p.ThirdContextIdentifier, _, err = optionalUnsigned(inner, AVPThirdContextIdentifier); err != nil {
		return nil, err
	}

	if p.Included, err = requiredEnum[APNConfigurationsIncluded](inner, AVPAllAPNConfigurationsIncludedIndicator, false); err != nil {
		return nil, err
	}

	configs := diameter.FindAll(inner, AVPAPNConfiguration, tgpp.VendorID)
	if len(configs) == 0 {
		return nil, tgpp.MissingAVP(AVPAPNConfiguration, tgpp.VendorID, 0)
	}

	for _, ca := range configs {
		c, err := d.apnConfiguration(ca)
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
	case !c.PDNType.Known():
		return diameter.AVP{}, invalidf("PDN-Type %d", c.PDNType)
	case len(c.ServedPartyIPAddresses) > maxServedIPs:
		return diameter.AVP{}, invalidf("%d served party IP addresses", len(c.ServedPartyIPAddresses))
	case c.PDNGWAllocationType != nil && c.PDNGateway == nil:
		return diameter.AVP{}, invalidf("PDN-GW-Allocation-Type without MIP6-Agent-Info")
	case c.NonIPDataDeliveryMechanism != nil && !c.NonIPPDNType:
		return diameter.AVP{}, invalidf("Non-IP-Data-Delivery-Mechanism on an APN without a Non-IP PDN type")
	case c.NonIPPDNType && c.EthernetPDNType:
		return diameter.AVP{}, invalidf("APN configuration %d with both a Non-IP and an Ethernet PDN type", c.ContextIdentifier)
	}

	inner := []diameter.AVP{unsignedAVP(AVPContextIdentifier, c.ContextIdentifier)}

	var families [2]bool

	for _, addr := range c.ServedPartyIPAddresses {
		family := 0
		if !addr.Is4() {
			family = 1
		}

		if !addr.IsValid() || addr.Zone() != "" || families[family] {
			return diameter.AVP{}, invalidf("served party IP address %v", addr)
		}

		families[family] = true

		inner = append(inner, diameter.Address(AVPServedPartyIPAddress, avpFlags(AVPServedPartyIPAddress), tgpp.VendorID, addr))
	}

	inner = append(inner, unsignedAVP(AVPPDNType, uint32(c.PDNType)), ietfString(diameter.AVPServiceSelection, c.ServiceSelection))

	if c.QoS != nil {
		qos, err := c.QoS.avp()
		if err != nil {
			return diameter.AVP{}, err
		}

		inner = append(inner, qos)
	}

	inner = appendIndicator(inner, AVPVPLMNDynamicAddressAllowed, c.VPLMNDynamicAddressAllowed, vplmnDynamicAddressAllowed)

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
	inner = appendIndicator(inner, AVPNonIPPDNTypeIndicator, c.NonIPPDNType, enumTrue)

	if inner, err = appendEnum(inner, AVPNonIPDataDeliveryMechanism, "Non-IP-Data-Delivery-Mechanism", c.NonIPDataDeliveryMechanism); err != nil {
		return diameter.AVP{}, err
	}

	inner = appendIndicator(inner, AVPInterworking5GSIndicator, c.Interworking5GSIndicator, interworking5GSSubscribed)
	inner = appendIndicator(inner, AVPEthernetPDNTypeIndicator, c.EthernetPDNType, enumTrue)

	return groupedAVP(AVPAPNConfiguration, inner...), nil
}

func (d decoder) apnConfiguration(a diameter.AVP) (APNConfiguration, error) {
	inner, err := d.group(a, apnConfigurationRules)
	if err != nil {
		return APNConfiguration{}, err
	}

	c, err := d.apnConfigurationFields(inner)
	if err != nil {
		return APNConfiguration{}, diameter.WithinGrouped(a, err)
	}

	return c, nil
}

func (d decoder) apnConfigurationFields(inner []diameter.AVP) (APNConfiguration, error) {
	var (
		c   APNConfiguration
		err error
	)

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

	if c.PDNType, err = requiredEnum[PDNType](inner, AVPPDNType, false); err != nil {
		return APNConfiguration{}, err
	}

	apn, ok := diameter.Find(inner, diameter.AVPServiceSelection, 0)
	if !ok {
		return APNConfiguration{}, tgpp.MissingAVP(diameter.AVPServiceSelection, 0, 0)
	}

	if c.ServiceSelection = apn.UTF8String(); c.ServiceSelection == "" {
		return APNConfiguration{}, tgpp.InvalidAVP(apn)
	}

	if qos, ok := diameter.Find(inner, AVPEPSSubscribedQoSProfile, tgpp.VendorID); ok {
		if c.QoS, err = d.qosProfile(qos); err != nil {
			return APNConfiguration{}, err
		}
	}

	if c.VPLMNDynamicAddressAllowed, err = optionalIndicator(inner, AVPVPLMNDynamicAddressAllowed, vplmnDynamicAddressAllowed); err != nil {
		return APNConfiguration{}, err
	}

	if gw, ok := diameter.Find(inner, diameter.AVPMIP6AgentInfo, 0); ok {
		if c.PDNGateway, err = d.mip6AgentInfo(gw); err != nil {
			return APNConfiguration{}, err
		}
	}

	if c.VisitedNetworkIdentifier, err = optionalString(inner, tgpp.AVPVisitedNetworkIdentifier, tgpp.VendorID); err != nil {
		return APNConfiguration{}, err
	}

	if c.PDNGWAllocationType, err = optionalEnum[PDNGWAllocationType](inner, AVPPDNGWAllocationType, false); err != nil {
		return APNConfiguration{}, err
	}

	if c.ChargingCharacteristics, err = optionalString(inner, tgpp.AVP3GPPChargingCharacteristics, tgpp.VendorID); err != nil {
		return APNConfiguration{}, err
	}

	if ambr, ok := diameter.Find(inner, AVPAMBR, tgpp.VendorID); ok {
		if c.AMBR, err = d.ambr(ambr); err != nil {
			return APNConfiguration{}, err
		}
	}

	for _, info := range diameter.FindAll(inner, AVPSpecificAPNInfo, tgpp.VendorID) {
		s, err := d.specificAPNInfo(info)
		if err != nil {
			return APNConfiguration{}, err
		}

		c.SpecificAPNInfo = append(c.SpecificAPNInfo, s)
	}

	if c.APNOIReplacement, err = optionalString(inner, AVPAPNOIReplacement, tgpp.VendorID); err != nil {
		return APNConfiguration{}, err
	}

	if c.NonIPPDNType, err = optionalIndicator(inner, AVPNonIPPDNTypeIndicator, enumTrue); err != nil {
		return APNConfiguration{}, err
	}

	if c.NonIPDataDeliveryMechanism, err = optionalEnum[NonIPDataDeliveryMechanism](inner, AVPNonIPDataDeliveryMechanism, false); err != nil {
		return APNConfiguration{}, err
	}

	if c.Interworking5GSIndicator, err = optionalIndicator(inner, AVPInterworking5GSIndicator, interworking5GSSubscribed); err != nil {
		return APNConfiguration{}, err
	}

	if c.EthernetPDNType, err = optionalIndicator(inner, AVPEthernetPDNTypeIndicator, enumTrue); err != nil {
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

	inner := []diameter.AVP{ietfString(diameter.AVPServiceSelection, s.ServiceSelection), gw}

	return groupedAVP(AVPSpecificAPNInfo, appendString(inner, tgpp.AVPVisitedNetworkIdentifier, s.VisitedNetworkIdentifier)...), nil
}

func (d decoder) specificAPNInfo(a diameter.AVP) (SpecificAPNInfo, error) {
	inner, err := d.group(a, specificAPNInfoRules)
	if err != nil {
		return SpecificAPNInfo{}, err
	}

	apn, ok := diameter.Find(inner, diameter.AVPServiceSelection, 0)

	switch {
	case !ok:
		return SpecificAPNInfo{}, diameter.WithinGrouped(a, tgpp.MissingAVP(diameter.AVPServiceSelection, 0, 0))
	case len(apn.Data) == 0:
		return SpecificAPNInfo{}, diameter.WithinGrouped(a, tgpp.InvalidAVP(apn))
	}

	gwAVP, ok := diameter.Find(inner, diameter.AVPMIP6AgentInfo, 0)
	if !ok {
		return SpecificAPNInfo{}, diameter.WithinGrouped(a, tgpp.MissingAVP(diameter.AVPMIP6AgentInfo, 0, 0))
	}

	gw, err := d.mip6AgentInfo(gwAVP)
	if err != nil {
		return SpecificAPNInfo{}, diameter.WithinGrouped(a, err)
	}

	s := SpecificAPNInfo{ServiceSelection: apn.UTF8String(), PDNGateway: *gw}

	if s.VisitedNetworkIdentifier, err = optionalString(inner, tgpp.AVPVisitedNetworkIdentifier, tgpp.VendorID); err != nil {
		return SpecificAPNInfo{}, diameter.WithinGrouped(a, err)
	}

	return s, nil
}
