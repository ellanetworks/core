// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const ApplicationID uint32 = 16777251

const (
	CommandUpdateLocation            uint32 = 316
	CommandCancelLocation            uint32 = 317
	CommandAuthenticationInformation uint32 = 318
	CommandInsertSubscriberData      uint32 = 319
	CommandDeleteSubscriberData      uint32 = 320
	CommandPurgeUE                   uint32 = 321
	CommandReset                     uint32 = 322
	CommandNotify                    uint32 = 323
)

const (
	AVPServedPartyIPAddress                      uint32 = 848
	AVPSubscriptionData                          uint32 = 1400
	AVPTerminalInformation                       uint32 = 1401
	AVPIMEI                                      uint32 = 1402
	AVPSoftwareVersion                           uint32 = 1403
	AVPQoSSubscribed                             uint32 = 1404
	AVPULRFlags                                  uint32 = 1405
	AVPULAFlags                                  uint32 = 1406
	AVPVisitedPLMNID                             uint32 = 1407
	AVPRequestedEUTRANAuthenticationInfo         uint32 = 1408
	AVPRequestedUTRANGERANAuthenticationInfo     uint32 = 1409
	AVPNumberOfRequestedVectors                  uint32 = 1410
	AVPReSynchronizationInfo                     uint32 = 1411
	AVPImmediateResponsePreferred                uint32 = 1412
	AVPAuthenticationInfo                        uint32 = 1413
	AVPEUTRANVector                              uint32 = 1414
	AVPUTRANVector                               uint32 = 1415
	AVPGERANVector                               uint32 = 1416
	AVPNetworkAccessMode                         uint32 = 1417
	AVPHPLMNODB                                  uint32 = 1418
	AVPItemNumber                                uint32 = 1419
	AVPCancellationType                          uint32 = 1420
	AVPDSRFlags                                  uint32 = 1421
	AVPDSAFlags                                  uint32 = 1422
	AVPContextIdentifier                         uint32 = 1423
	AVPSubscriberStatus                          uint32 = 1424
	AVPOperatorDeterminedBarring                 uint32 = 1425
	AVPAccessRestrictionData                     uint32 = 1426
	AVPAPNOIReplacement                          uint32 = 1427
	AVPAllAPNConfigurationsIncludedIndicator     uint32 = 1428
	AVPAPNConfigurationProfile                   uint32 = 1429
	AVPAPNConfiguration                          uint32 = 1430
	AVPEPSSubscribedQoSProfile                   uint32 = 1431
	AVPVPLMNDynamicAddressAllowed                uint32 = 1432
	AVPSTNSR                                     uint32 = 1433
	AVPAlertReason                               uint32 = 1434
	AVPAMBR                                      uint32 = 1435
	AVPCSGSubscriptionData                       uint32 = 1436
	AVPPDNGWAllocationType                       uint32 = 1438
	AVPRATFrequencySelectionPriorityID           uint32 = 1440
	AVPIDAFlags                                  uint32 = 1441
	AVPPUAFlags                                  uint32 = 1442
	AVPNORFlags                                  uint32 = 1443
	AVPUserID                                    uint32 = 1444
	AVPRegionalSubscriptionZoneCode              uint32 = 1446
	AVPRAND                                      uint32 = 1447
	AVPXRES                                      uint32 = 1448
	AVPAUTN                                      uint32 = 1449
	AVPKASME                                     uint32 = 1450
	AVPKc                                        uint32 = 1453
	AVPSRES                                      uint32 = 1454
	AVPPDNType                                   uint32 = 1456
	AVPRoamingRestrictedDueToUnsupportedFeature  uint32 = 1457
	AVPTraceData                                 uint32 = 1458
	AVPTraceReference                            uint32 = 1459
	AVPGPRSSubscriptionData                      uint32 = 1467
	AVP3GPP2MEID                                 uint32 = 1471
	AVPSpecificAPNInfo                           uint32 = 1472
	AVPLCSInfo                                   uint32 = 1473
	AVPSSCode                                    uint32 = 1476
	AVPTeleserviceList                           uint32 = 1486
	AVPTSCode                                    uint32 = 1487
	AVPCallBarringInfo                           uint32 = 1488
	AVPIDRFlags                                  uint32 = 1490
	AVPICSIndicator                              uint32 = 1491
	AVPIMSVoiceOverPSSessionsSupported           uint32 = 1492
	AVPHomogeneousSupportOfIMSVoiceOverPSSession uint32 = 1493
	AVPLastUEActivityTime                        uint32 = 1494
	AVPEPSUserState                              uint32 = 1495
	AVPEPSLocationInformation                    uint32 = 1496
	AVPEmergencyServices                         uint32 = 1538
	AVPActiveAPN                                 uint32 = 1612
	AVPSIPTOPermission                           uint32 = 1613
	AVPErrorDiagnostic                           uint32 = 1614
	AVPUESRVCCCapability                         uint32 = 1615
	AVPMPSPriority                               uint32 = 1616
	AVPVPLMNLIPAAllowed                          uint32 = 1617
	AVPLIPAPermission                            uint32 = 1618
	AVPSubscribedPeriodicRAUTAUTimer             uint32 = 1619
	AVPRelayNodeIndicator                        uint32 = 1633
	AVPMDTUserConsent                            uint32 = 1634
	AVPPURFlags                                  uint32 = 1635
	AVPSubscribedVSRVCC                          uint32 = 1636
	AVPEquivalentPLMNList                        uint32 = 1637
	AVPCLRFlags                                  uint32 = 1638
	AVPTimeZone                                  uint32 = 1642
	AVPAMSISDN                                   uint32 = 1643
	AVPSMSRegisterRequest                        uint32 = 1648
	AVPLocalTimeZone                             uint32 = 1649
	AVPDaylightSavingTime                        uint32 = 1650
	AVPSubscriptionDataFlags                     uint32 = 1654
	AVPRestorationPriority                       uint32 = 1663
	AVPSGsMMEIdentity                            uint32 = 1664
	AVPSIPTOLocalNetworkPermission               uint32 = 1665
	AVPCoupledNodeDiameterID                     uint32 = 1666
	AVPWLANOffloadability                        uint32 = 1667
	AVPResetID                                   uint32 = 1670
	AVPAdjacentPLMNs                             uint32 = 1672
	AVPAdjacentAccessRestrictionData             uint32 = 1673
	AVPDLBufferingSuggestedPacketCount           uint32 = 1674
	AVPIMSIGroupID                               uint32 = 1675
	AVPAIRFlags                                  uint32 = 1679
	AVPUEUsageType                               uint32 = 1680
	AVPNonIPPDNTypeIndicator                     uint32 = 1681
	AVPNonIPDataDeliveryMechanism                uint32 = 1682
	AVPAdditionalContextIdentifier               uint32 = 1683
	AVPSCEFRealm                                 uint32 = 1684
	AVPSubscriptionDataDeletion                  uint32 = 1685
	AVPPreferredDataMode                         uint32 = 1686
	AVPEmergencyInfo                             uint32 = 1687
	AVPV2XSubscriptionData                       uint32 = 1688
	AVPPDNConnectionContinuity                   uint32 = 1690
	AVPEDRXCycleLength                           uint32 = 1691
	AVPRDSIndicator                              uint32 = 1697
	AVPServiceGapTime                            uint32 = 1698
	AVPAerialUESubscriptionInformation           uint32 = 1699
	AVPBroadcastLocationAssistanceDataTypes      uint32 = 1700
	AVPPagingTimeWindow                          uint32 = 1701
	AVPCoreNetworkRestrictions                   uint32 = 1704
	AVPEDRXRelatedRAT                            uint32 = 1705
	AVPInterworking5GSIndicator                  uint32 = 1706
	AVPEthernetPDNTypeIndicator                  uint32 = 1707
	AVPSubscribedARPI                            uint32 = 1708
	AVPIABOperationPermission                    uint32 = 1709
	AVPV2XSubscriptionDataNR                     uint32 = 1710
	AVPThirdContextIdentifier                    uint32 = 1719
	AVPPLMNRATUsageControl                       uint32 = 1728
	AVPSFULRTimestamp                            uint32 = 1729
	AVPSFProvisionalIndication                   uint32 = 1730
	AVPGMLCAddress                               uint32 = 2405
	AVPExternalIdentifier                        uint32 = 3111
	AVPAESECommunicationPattern                  uint32 = 3113
	AVPMonitoringEventConfiguration              uint32 = 3122
	AVPMonitoringEventReport                     uint32 = 3123
	AVPSCEFID                                    uint32 = 3125
	AVPMonitoringEventConfigStatus               uint32 = 3142
	AVPSupportedServices                         uint32 = 3143
	AVPProSeSubscriptionData                     uint32 = 3701
	AVPActiveTime                                uint32 = 4342
)

const (
	FeatureListID1 uint32 = 1
	FeatureListID2 uint32 = 2
)

var (
	ErrMalformedAnswer = errors.New("s6a: malformed answer")
	ErrInvalidMessage  = errors.New("s6a: invalid message")
)

type ResultError struct {
	tgpp.Result

	ErrorDiagnostic *ErrorDiagnostic
	UEUsageType     *UEUsageType
	Features        Features
}

func (e *ResultError) Error() string {
	s := "s6a: request failed with " + e.String()
	if e.ErrorDiagnostic != nil {
		s += " (" + e.ErrorDiagnostic.String() + ")"
	}

	return s
}

type Features struct {
	List1 FeatureList1
	List2 FeatureList2
}

func (f Features) lists() tgpp.FeatureLists {
	return tgpp.FeatureLists{FeatureListID1: uint32(f.List1), FeatureListID2: uint32(f.List2)}
}

func (f Features) avps() []diameter.AVP {
	return f.lists().AVPs(false)
}

func featuresOf(m *diameter.Message) Features {
	lists := tgpp.ParseFeatureLists(m.AVPs)

	return Features{List1: FeatureList1(lists[FeatureListID1]), List2: FeatureList2(lists[FeatureListID2])}
}

func RequiresUnsupportedFeatures(req *diameter.Message, supported Features) bool {
	return supported.lists().Unsupported(req.AVPs)
}

func NewAnswer(req *diameter.Message, id diameter.Identity, r tgpp.Result, features Features) *diameter.Message {
	ans := tgpp.NewResultAnswer(req, id, r)
	ans.AVPs = append(ans.AVPs, features.avps()...)

	return ans
}

func NewErrorAnswer(req *diameter.Message, id diameter.Identity, err error, features Features) *diameter.Message {
	var re *ResultError
	if !errors.As(err, &re) {
		ans := tgpp.NewErrorAnswer(req, id, err)
		ans.AVPs = append(ans.AVPs, features.avps()...)

		return ans
	}

	ans := NewAnswer(req, id, re.Result, features)

	if d := re.ErrorDiagnostic; d != nil && errorDiagnosticAllowed(req.CommandCode, re.Result, *d) {
		ans.AVPs = append(ans.AVPs, unsignedAVP(AVPErrorDiagnostic, uint32(*d)))
	}

	if u := re.UEUsageType; u != nil && req.CommandCode == CommandAuthenticationInformation &&
		re.IsExperimental(tgpp.ResultAuthenticationDataUnavailable) {
		ans.AVPs = append(ans.AVPs, u.avp())
	}

	return ans
}

func errorDiagnosticAllowed(command uint32, r tgpp.Result, d ErrorDiagnostic) bool {
	switch {
	case r.IsExperimental(tgpp.ResultErrorUnknownEPSSubscription):
		return (command == CommandUpdateLocation || command == CommandAuthenticationInformation) &&
			(d == ErrorDiagnosticGPRSDataSubscribed || d == ErrorDiagnosticNoGPRSDataSubscribed)
	case r.IsExperimental(tgpp.ResultErrorRoamingNotAllowed):
		return command == CommandUpdateLocation && d >= ErrorDiagnosticODBAllAPN && d <= ErrorDiagnosticODBVPLMNAPN
	default:
		return false
	}
}

func newRequest(env tgpp.Envelope, command uint32, fromHSS bool, avps ...diameter.AVP) (*diameter.Message, error) {
	if err := env.Validate(); err != nil {
		return nil, invalidf("%w", err)
	}

	if fromHSS && env.DestinationHost == "" {
		return nil, invalidf("HSS-initiated request without a destination host")
	}

	return &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   command,
		ApplicationID: ApplicationID,
		AVPs:          append(env.AVPs(), avps...),
	}, nil
}

func newSuccessAnswer(req *diameter.Message, id diameter.Identity, features Features) *diameter.Message {
	return NewAnswer(req, id, tgpp.Result{Code: diameter.ResultSuccess}, features)
}

func invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %w", ErrInvalidMessage, fmt.Errorf(format, args...))
}

func malformed(err error) error {
	return fmt.Errorf("%w: %w", ErrMalformedAnswer, err)
}

func malformedf(format string, args ...any) error {
	return malformed(fmt.Errorf(format, args...))
}

func parseResult(ans *diameter.Message) error {
	result, err := tgpp.ParseFinalResult(ans)
	if err != nil {
		return malformed(err)
	}

	if result.Success() {
		return nil
	}

	e := &ResultError{Result: result, Features: featuresOf(ans)}

	if a, ok := ans.Find(AVPErrorDiagnostic, tgpp.VendorID); ok {
		if v, err := a.Unsigned32(); err == nil {
			d := ErrorDiagnostic(v)
			e.ErrorDiagnostic = &d
		}
	}

	if a, ok := ans.Find(AVPUEUsageType, tgpp.VendorID); ok {
		if u, err := parseUEUsageType(a); err == nil {
			e.UEUsageType = &u
		}
	}

	return e
}

func parseAnswer(ans *diameter.Message) (Features, error) {
	if err := parseResult(ans); err != nil {
		return Features{}, err
	}

	return featuresOf(ans), nil
}

func origin(m *diameter.Message) diameter.Identity {
	e := tgpp.ParseEnvelope(m)

	return e.Origin
}

func userNameAVP(imsi string) (diameter.AVP, error) {
	if !tgpp.ValidIMSI(imsi) {
		return diameter.AVP{}, invalidf("IMSI %q", imsi)
	}

	return diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, imsi), nil
}

func visitedPLMNAVP(p tgpp.PLMN) (diameter.AVP, error) {
	b, err := p.MarshalBinary()
	if err != nil {
		return diameter.AVP{}, invalidf("%w", err)
	}

	return octetsAVP(AVPVisitedPLMNID, b), nil
}

func parsePLMN(a diameter.AVP) (tgpp.PLMN, error) {
	p, err := tgpp.ParsePLMN(a.Data)
	if err != nil {
		return tgpp.PLMN{}, tgpp.InvalidAVP(a)
	}

	return p, nil
}

func vendorKey(code uint32) diameter.AVPKey {
	return diameter.AVPKey{Code: code, VendorID: tgpp.VendorID}
}

func ietfKey(code uint32) diameter.AVPKey {
	return diameter.AVPKey{Code: code}
}

func requestIMSI(req *diameter.Message) (string, error) {
	a, _ := req.Find(diameter.AVPUserName, 0)
	if !tgpp.ValidIMSI(a.UTF8String()) {
		return "", tgpp.InvalidAVP(a)
	}

	return a.UTF8String(), nil
}

func requestVisitedPLMN(req *diameter.Message) (tgpp.PLMN, error) {
	a, _ := req.Find(AVPVisitedPLMNID, tgpp.VendorID)

	return parsePLMN(a)
}

var commonRules = diameter.Rules{
	ietfKey(diameter.AVPOCSupportedFeatures): {},
	vendorKey(tgpp.AVPSupportedFeatures):     {Multiple: true},
}

var userNameRule = diameter.Rules{ietfKey(diameter.AVPUserName): {Required: true, MinLength: 1}}

func requestRules(fromHSS bool, extra diameter.Rules) diameter.Rules {
	rules := diameter.BaseRequestRules().With(commonRules).With(extra)

	if fromHSS {
		rules[ietfKey(diameter.AVPDestinationHost)] = diameter.Rule{Required: true, MinLength: 1}
	}

	return rules
}
