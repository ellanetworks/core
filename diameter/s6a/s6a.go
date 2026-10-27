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
	AVPSSCode                                    uint32 = 1476
	AVPTSCode                                    uint32 = 1487
	AVPIDRFlags                                  uint32 = 1490
	AVPICSIndicator                              uint32 = 1491
	AVPIMSVoiceOverPSSessionsSupported           uint32 = 1492
	AVPHomogeneousSupportOfIMSVoiceOverPSSession uint32 = 1493
	AVPLastUEActivityTime                        uint32 = 1494
	AVPEPSUserState                              uint32 = 1495
	AVPEPSLocationInformation                    uint32 = 1496
	AVPActiveAPN                                 uint32 = 1612
	AVPErrorDiagnostic                           uint32 = 1614
	AVPUESRVCCCapability                         uint32 = 1615
	AVPSubscribedPeriodicRAUTAUTimer             uint32 = 1619
	AVPPURFlags                                  uint32 = 1635
	AVPSubscribedVSRVCC                          uint32 = 1636
	AVPEquivalentPLMNList                        uint32 = 1637
	AVPCLRFlags                                  uint32 = 1638
	AVPTimeZone                                  uint32 = 1642
	AVPAMSISDN                                   uint32 = 1643
	AVPSMSRegisterRequest                        uint32 = 1648
	AVPLocalTimeZone                             uint32 = 1649
	AVPDaylightSavingTime                        uint32 = 1650
	AVPSGsMMEIdentity                            uint32 = 1664
	AVPCoupledNodeDiameterID                     uint32 = 1666
	AVPResetID                                   uint32 = 1670
	AVPAdjacentPLMNs                             uint32 = 1672
	AVPAIRFlags                                  uint32 = 1679
	AVPUEUsageType                               uint32 = 1680
	AVPAdditionalContextIdentifier               uint32 = 1683
	AVPSubscriptionDataDeletion                  uint32 = 1685
	AVPEDRXRelatedRAT                            uint32 = 1705
	AVPInterworking5GSIndicator                  uint32 = 1706
	AVPThirdContextIdentifier                    uint32 = 1719
	AVPSFULRTimestamp                            uint32 = 1729
	AVPSFProvisionalIndication                   uint32 = 1730
	AVPServedPartyIPAddress                      uint32 = 848
	AVPGMLCAddress                               uint32 = 2405
	AVPSupportedServices                         uint32 = 3143
	AVPMonitoringEventConfigStatus               uint32 = 3142
	AVPMonitoringEventReport                     uint32 = 3123
	AVPSCEFID                                    uint32 = 3125
	AVPExternalIdentifier                        uint32 = 3111
	AVPEmergencyServices                         uint32 = 1538
)

const (
	AVPMIPHomeAgentAddress uint32 = 334
	AVPMIPHomeAgentHost    uint32 = 348
	AVPMIP6AgentInfo       uint32 = 486
	AVPServiceSelection    uint32 = 493
	avpMIP6HomeLinkPrefix  uint32 = 125
	avpOCSupportedFeatures uint32 = 621
	avpOCOLR               uint32 = 623
	avpLoad                uint32 = 650
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

func (f Features) avps() []diameter.AVP {
	var avps []diameter.AVP

	if f.List1 != 0 {
		avps = append(avps, tgpp.SupportedFeatures{VendorID: tgpp.VendorID, FeatureListID: FeatureListID1, FeatureList: uint32(f.List1)}.AVP())
	}

	if f.List2 != 0 {
		avps = append(avps, tgpp.SupportedFeatures{VendorID: tgpp.VendorID, FeatureListID: FeatureListID2, FeatureList: uint32(f.List2)}.AVP())
	}

	return avps
}

func featuresOf(m *diameter.Message) Features {
	return Features{
		List1: FeatureList1(tgpp.FeatureList(m.AVPs, tgpp.VendorID, FeatureListID1)),
		List2: FeatureList2(tgpp.FeatureList(m.AVPs, tgpp.VendorID, FeatureListID2)),
	}
}

func RequiresUnsupportedFeatures(req *diameter.Message, supported Features) bool {
	return tgpp.UnsupportedRequiredFeatures(req.AVPs,
		tgpp.SupportedFeatures{VendorID: tgpp.VendorID, FeatureListID: FeatureListID1, FeatureList: uint32(supported.List1)},
		tgpp.SupportedFeatures{VendorID: tgpp.VendorID, FeatureListID: FeatureListID2, FeatureList: uint32(supported.List2)},
	)
}

func NewAnswer(req *diameter.Message, id diameter.Identity, r tgpp.Result, features Features) *diameter.Message {
	ans := tgpp.NewResultAnswer(req, id, r)
	ans.AVPs = append(ans.AVPs, features.avps()...)

	return ans
}

func NewErrorAnswer(req *diameter.Message, id diameter.Identity, err error) *diameter.Message {
	var re *ResultError
	if !errors.As(err, &re) {
		return tgpp.NewErrorAnswer(req, id, err)
	}

	ans := NewAnswer(req, id, re.Result, re.Features)

	if re.ErrorDiagnostic != nil && carriesErrorDiagnostic(req.CommandCode) {
		ans.AVPs = append(ans.AVPs, unsignedAVP(AVPErrorDiagnostic, uint32(*re.ErrorDiagnostic)))
	}

	return ans
}

func carriesErrorDiagnostic(command uint32) bool {
	return command == CommandUpdateLocation || command == CommandAuthenticationInformation
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

	return e
}

func parseAnswer(ans *diameter.Message) (Features, error) {
	if err := parseResult(ans); err != nil {
		return Features{}, err
	}

	return featuresOf(ans), nil
}

func avpFlags(code uint32) uint8 {
	switch code {
	case AVPUserID:
		return 0
	case tgpp.AVPMSISDN, tgpp.AVPQoSClassIdentifier, tgpp.AVPMaxRequestedBandwidthDL, tgpp.AVPMaxRequestedBandwidthUL,
		tgpp.AVPVisitedNetworkIdentifier, tgpp.AVPConfidentialityKey, tgpp.AVPIntegrityKey,
		tgpp.AVP3GPPChargingCharacteristics, AVPServedPartyIPAddress:
		return diameter.AVPFlagMandatory
	}

	if code >= AVPSubscriptionData && code <= AVPIDRFlags {
		return diameter.AVPFlagMandatory
	}

	return 0
}

func unsignedAVP(code, v uint32) diameter.AVP {
	return diameter.Unsigned32(code, avpFlags(code), tgpp.VendorID, v)
}

func octetsAVP(code uint32, v []byte) diameter.AVP {
	return diameter.OctetString(code, avpFlags(code), tgpp.VendorID, v)
}

func stringAVP(code uint32, v string) diameter.AVP {
	return diameter.UTF8String(code, avpFlags(code), tgpp.VendorID, v)
}

func groupedAVP(code uint32, avps ...diameter.AVP) diameter.AVP {
	return diameter.Grouped(code, avpFlags(code), tgpp.VendorID, avps...)
}

func userNameAVP(imsi string) (diameter.AVP, error) {
	if !tgpp.ValidIMSI(imsi) {
		return diameter.AVP{}, invalidf("IMSI %q", imsi)
	}

	return diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, imsi), nil
}

func visitedPLMNAVP(p tgpp.PLMN) (diameter.AVP, error) {
	b, err := p.Encode()
	if err != nil {
		return diameter.AVP{}, invalidf("%w", err)
	}

	return octetsAVP(AVPVisitedPLMNID, b), nil
}

func vendorKey(code uint32) diameter.AVPKey {
	return diameter.AVPKey{Code: code, VendorID: tgpp.VendorID}
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

	p, err := tgpp.DecodePLMN(a.Data)
	if err != nil {
		return tgpp.PLMN{}, tgpp.InvalidAVP(a)
	}

	return p, nil
}

var commonRules = diameter.Rules{
	{Code: avpOCSupportedFeatures}:                             {},
	{Code: tgpp.AVPSupportedFeatures, VendorID: tgpp.VendorID}: {Multiple: true},
}

var userNameRule = diameter.Rules{{Code: diameter.AVPUserName}: {Required: true}}

func requestRules(fromHSS bool, extra diameter.Rules) diameter.Rules {
	rules := diameter.BaseRequestRules().With(commonRules).With(extra)

	if fromHSS {
		rules[diameter.AVPKey{Code: diameter.AVPDestinationHost}] = diameter.Rule{Required: true}
	}

	return rules
}
