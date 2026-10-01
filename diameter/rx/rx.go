// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const ApplicationID uint32 = 16777236

const (
	CommandAA                 uint32 = 265
	CommandReAuth                    = diameter.CommandReAuth
	CommandAbortSession              = diameter.CommandAbortSession
	CommandSessionTermination        = diameter.CommandSessionTermination
)

const (
	AVPAbortCause                           uint32 = 500
	AVPAccessNetworkChargingAddress         uint32 = 501
	AVPAccessNetworkChargingIdentifier      uint32 = 502
	AVPAccessNetworkChargingIdentifierValue uint32 = 503
	AVPAFApplicationIdentifier              uint32 = 504
	AVPAFChargingIdentifier                 uint32 = 505
	AVPFlowDescription                      uint32 = 507
	AVPFlowNumber                           uint32 = 509
	AVPFlows                                uint32 = 510
	AVPFlowStatus                           uint32 = 511
	AVPFlowUsage                            uint32 = 512
	AVPSpecificAction                       uint32 = 513
	AVPMaxRequestedBandwidthDL              uint32 = 515
	AVPMaxRequestedBandwidthUL              uint32 = 516
	AVPMediaComponentDescription            uint32 = 517
	AVPMediaComponentNumber                 uint32 = 518
	AVPMediaSubComponent                    uint32 = 519
	AVPMediaType                            uint32 = 520
	AVPRRBandwidth                          uint32 = 521
	AVPRSBandwidth                          uint32 = 522
	AVPSIPForkingIndication                 uint32 = 523
	AVPCodecData                            uint32 = 524
	AVPServiceURN                           uint32 = 525
	AVPAcceptableServiceInfo                uint32 = 526
	AVPServiceInfoStatus                    uint32 = 527
	AVPMPSIdentifier                        uint32 = 528
	AVPAFSignallingProtocol                 uint32 = 529
	AVPSponsoredConnectivityData            uint32 = 530
	AVPRxRequestType                        uint32 = 533
	AVPRequiredAccessInfo                   uint32 = 536
	AVPIPDomainID                           uint32 = 537
	AVPGCSIdentifier                        uint32 = 538
	AVPRetryInterval                        uint32 = 541
	AVPMCPTTIdentifier                      uint32 = 547
	AVPAFRequestedData                      uint32 = 551
	AVPPreemptionControlInfo                uint32 = 553
	AVPMCVideoIdentifier                    uint32 = 562
	AVPIMSContentIdentifier                 uint32 = 563
	AVPIMSContentType                       uint32 = 564
	AVPCalleeInformation                    uint32 = 565
	AVPNID                                  uint32 = 569
	AVPMAInformation                        uint32 = 570
	AVP5GSRANNASReleaseCause                uint32 = 572
	AVPWirelineUserLocationInfo             uint32 = 578
	AVPMPSAction                            uint32 = 582
	AVPServingSatelliteIdentity             uint32 = 583
	AVPPCSessionRecoveryStatus              uint32 = 584
)

const (
	AVPFramedIPAddress      uint32 = 8
	AVPCalledStationID      uint32 = 30
	AVPFramedIPv6Prefix     uint32 = 97
	AVPSubscriptionID       uint32 = 443
	AVPSubscriptionIDData   uint32 = 444
	AVPSubscriptionIDType   uint32 = 450
	avp3GPPSGSNMCCMNC       uint32 = 18
	avp3GPPUserLocationInfo uint32 = 22
	avp3GPPMSTimeZone       uint32 = 23
	avpTWANIdentifier       uint32 = 29
	avpOCSupportedFeatures  uint32 = 621
	avpCallingPartyAddress  uint32 = 831
	avpIPCANType            uint32 = 1027
	avpRATType              uint32 = 1032
	avpANGWAddress          uint32 = 1050
	avpANTrusted            uint32 = 1503
	avpUELocalIPAddress     uint32 = 2805
	avpUDPSourcePort        uint32 = 2806
	avpUserLocationInfoTime uint32 = 2812
	avpRANNASReleaseCause   uint32 = 2819
	avpNetLocAccessSupport  uint32 = 2824
	avpTCPSourcePort        uint32 = 2843
	avpReferenceID          uint32 = 4202
	avpReservationPriority  uint32 = 458
	etsiVendorID            uint32 = 13019
)

const (
	featureList1 uint32 = 1
	featureList2 uint32 = 2
)

var (
	ErrMalformedAnswer = errors.New("rx: malformed answer")
	ErrInvalidMessage  = errors.New("rx: invalid message")
)

type ResultError struct {
	tgpp.Result
}

func (e *ResultError) Error() string {
	return "rx: request failed with " + e.String()
}

var commonRequestRules = diameter.BaseRequestRules().With(diameter.Rules{
	{Code: diameter.AVPAuthSessionState}:  {},
	{Code: diameter.AVPAuthApplicationID}: {Required: true, MinLength: 4},
	{Code: avpOCSupportedFeatures}:        {},
})

func NewAnswer(req *diameter.Message, id diameter.Identity, r tgpp.Result) *diameter.Message {
	var ans *diameter.Message
	if r.Experimental {
		ans = diameter.NewExperimentalAnswer(req, id, r.VendorID, r.Code)
	} else {
		ans = diameter.NewAnswer(req, id, r.Code)
	}

	return finishAnswer(req, ans)
}

func NewErrorAnswer(req *diameter.Message, id diameter.Identity, err error) *diameter.Message {
	if r, ok := tgpp.ResultOf(err); ok {
		return NewAnswer(req, id, r)
	}

	return finishAnswer(req, diameter.NewErrorAnswer(req, id, err))
}

func finishAnswer(req, ans *diameter.Message) *diameter.Message {
	if req.CommandCode != CommandAA {
		return ans
	}

	ans.AVPs = append(ans.AVPs, authApplicationID())

	if state, ok := req.Find(diameter.AVPAuthSessionState, 0); ok {
		if v, err := state.Unsigned32(); err == nil && v <= diameter.AuthSessionStateNoStateMaintained {
			ans.AVPs = append(ans.AVPs, diameter.Unsigned32(diameter.AVPAuthSessionState, diameter.AVPFlagMandatory, 0, v))
		}
	}

	return ans
}

func authApplicationID() diameter.AVP {
	return diameter.Unsigned32(diameter.AVPAuthApplicationID, diameter.AVPFlagMandatory, 0, ApplicationID)
}

func newRequest(env tgpp.Envelope, command uint32, avps ...diameter.AVP) *diameter.Message {
	head := []diameter.AVP{
		diameter.UTF8String(diameter.AVPSessionID, diameter.AVPFlagMandatory, 0, env.SessionID),
		authApplicationID(),
		diameter.UTF8String(diameter.AVPOriginHost, diameter.AVPFlagMandatory, 0, env.Origin.OriginHost),
		diameter.UTF8String(diameter.AVPOriginRealm, diameter.AVPFlagMandatory, 0, env.Origin.OriginRealm),
		diameter.UTF8String(diameter.AVPDestinationRealm, diameter.AVPFlagMandatory, 0, env.DestinationRealm),
	}

	if env.DestinationHost != "" {
		head = append(head, diameter.UTF8String(diameter.AVPDestinationHost, diameter.AVPFlagMandatory, 0, env.DestinationHost))
	}

	return &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   command,
		ApplicationID: ApplicationID,
		AVPs:          append(head, avps...),
	}
}

func checkRequest(rules diameter.Rules, req *diameter.Message) error {
	if err := rules.Check(req); err != nil {
		return err
	}

	app, _ := req.Find(diameter.AVPAuthApplicationID, 0)
	if v, err := app.Unsigned32(); err != nil || v != ApplicationID {
		return tgpp.InvalidAVP(app)
	}

	return nil
}

func featureAVPs(features, required Features) ([]diameter.AVP, error) {
	if required&^features != 0 {
		return nil, invalid("required features %s not among the advertised %s", required, features)
	}

	var avps []diameter.AVP

	for _, id := range []uint32{featureList1, featureList2} {
		avps = append(avps, tgpp.FeatureAVPs(tgpp.VendorID, id, features.list(id), required.list(id))...)
	}

	return avps, nil
}

func featureList(m *diameter.Message) Features {
	return listFeatures(featureList1, tgpp.FeatureList(m.AVPs, tgpp.VendorID, featureList1)) |
		listFeatures(featureList2, tgpp.FeatureList(m.AVPs, tgpp.VendorID, featureList2))
}

func requiredFeatures(m *diameter.Message) Features {
	return listFeatures(featureList1, tgpp.RequiredFeatureList(m.AVPs, tgpp.VendorID, featureList1)) |
		listFeatures(featureList2, tgpp.RequiredFeatureList(m.AVPs, tgpp.VendorID, featureList2))
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidMessage}, args...)...)
}

func malformed(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrMalformedAnswer}, args...)...)
}

func parseResult(ans *diameter.Message) (tgpp.Result, error) {
	result, err := tgpp.ParseResult(ans)
	if err != nil {
		return tgpp.Result{}, fmt.Errorf("%w: %w", ErrMalformedAnswer, err)
	}

	if !result.Success() {
		return result, &ResultError{Result: result}
	}

	return result, nil
}

func successResult(r tgpp.Result) (tgpp.Result, error) {
	if r == (tgpp.Result{}) {
		return tgpp.Result{Code: diameter.ResultSuccess}, nil
	}

	if !r.Success() {
		return tgpp.Result{}, invalid("answer with the non-success %s", r)
	}

	return r, nil
}

func errorResult(r tgpp.Result) error {
	if r.Success() {
		return invalid("error answer with the success %s", r)
	}

	return nil
}

func successAnswer(req *diameter.Message, id diameter.Identity, r tgpp.Result) (*diameter.Message, error) {
	result, err := successResult(r)
	if err != nil {
		return nil, err
	}

	return NewAnswer(req, id, result), nil
}
