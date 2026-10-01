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
	AVPAFSignallingProtocol                 uint32 = 529
	AVPRxRequestType                        uint32 = 533
	AVPMinRequestedBandwidthDL              uint32 = 534
	AVPMinRequestedBandwidthUL              uint32 = 535
	AVPRequiredAccessInfo                   uint32 = 536
	AVPIPDomainID                           uint32 = 537
	AVPRetryInterval                        uint32 = 541
	AVPMaxSupportedBandwidthDL              uint32 = 543
	AVPMaxSupportedBandwidthUL              uint32 = 544
	AVPMinDesiredBandwidthDL                uint32 = 545
	AVPMinDesiredBandwidthUL                uint32 = 546
	AVPMediaComponentStatus                 uint32 = 549
	AVPContentVersion                       uint32 = 552
	AVPExtendedMaxRequestedBWDL             uint32 = 554
	AVPExtendedMaxRequestedBWUL             uint32 = 555
	AVPExtendedMaxSupportedBWDL             uint32 = 556
	AVPExtendedMaxSupportedBWUL             uint32 = 557
	AVPExtendedMinDesiredBWDL               uint32 = 558
	AVPExtendedMinDesiredBWUL               uint32 = 559
	AVPExtendedMinRequestedBWDL             uint32 = 560
	AVPExtendedMinRequestedBWUL             uint32 = 561
	AVPNID                                  uint32 = 569
	AVPServingSatelliteIdentity             uint32 = 583
	AVPPCSessionRecoveryStatus              uint32 = 584
)

const (
	avpMPSIdentifier            uint32 = 528
	avpSponsoredConnectivity    uint32 = 530
	avpGCSIdentifier            uint32 = 538
	avpSharingKeyDL             uint32 = 539
	avpSharingKeyUL             uint32 = 540
	avpMCPTTIdentifier          uint32 = 547
	avpPrioritySharingIndicator uint32 = 550
	avpAFRequestedData          uint32 = 551
	avpPreemptionControlInfo    uint32 = 553
	avpMCVideoIdentifier        uint32 = 562
	avpIMSContentIdentifier     uint32 = 563
	avpIMSContentType           uint32 = 564
	avpCalleeInformation        uint32 = 565
	avpFLUSIdentifier           uint32 = 566
	avpDesiredMaxLatency        uint32 = 567
	avpDesiredMaxLoss           uint32 = 568
	avpMAInformation            uint32 = 570
	avp5GSRANNASReleaseCause    uint32 = 572
	avpWirelineUserLocationInfo uint32 = 578
	avpMPSAction                uint32 = 582
)

const (
	avpReservationPriority     uint32 = 458
	avpOCSupportedFeatures     uint32 = 621
	avpCallingPartyAddress     uint32 = 831
	avpToSTrafficClass         uint32 = 1014
	avpPreemptionCapability    uint32 = 1047
	avpPreemptionVulnerability uint32 = 1048
	avpMaxPLRDL                uint32 = 2852
	avpMaxPLRUL                uint32 = 2853
	avpReferenceID             uint32 = 4202
	etsiVendorID               uint32 = 13019
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

func NewAnswer(req *diameter.Message, id diameter.Identity, r tgpp.Result, features Features) *diameter.Message {
	if r.Experimental && !experimentalAllowed(req.CommandCode) {
		r = baseResult(r)
	}

	var ans *diameter.Message
	if r.Experimental {
		ans = diameter.NewExperimentalAnswer(req, id, r.VendorID, r.Code)
	} else {
		ans = diameter.NewAnswer(req, id, r.Code)
	}

	return finishAnswer(req, ans, features)
}

func NewErrorAnswer(req *diameter.Message, id diameter.Identity, err error, features Features) *diameter.Message {
	var aaErr *AAError
	if req.CommandCode == CommandAA && errors.As(err, &aaErr) {
		if ans, buildErr := NewAAErrorAnswer(req, id, *aaErr); buildErr == nil {
			return ans
		}
	}

	if r, ok := tgpp.ResultOf(err); ok {
		if !r.Failure() {
			r = tgpp.Result{Code: diameter.ResultUnableToComply}
		}

		return NewAnswer(req, id, r, features)
	}

	return finishAnswer(req, diameter.NewErrorAnswer(req, id, err), features)
}

func experimentalAllowed(command uint32) bool {
	return command == CommandAA || command == CommandReAuth
}

func baseResult(r tgpp.Result) tgpp.Result {
	if r.Success() {
		return tgpp.Result{Code: diameter.ResultSuccess}
	}

	return tgpp.Result{Code: diameter.ResultUnableToComply}
}

func finishAnswer(req, ans *diameter.Message, features Features) *diameter.Message {
	if req.CommandCode != CommandAA {
		return ans
	}

	ans.AVPs = append(ans.AVPs, authApplicationID())

	if state, ok := req.Find(diameter.AVPAuthSessionState, 0); ok {
		if v, err := state.Unsigned32(); err == nil && v <= diameter.AuthSessionStateNoStateMaintained {
			ans.AVPs = append(ans.AVPs, diameter.Unsigned32(diameter.AVPAuthSessionState, diameter.AVPFlagMandatory, 0, v))
		}
	}

	ans.AVPs = append(ans.AVPs, featureAVPs(features&featureList(req), false)...)

	return ans
}

func authApplicationID() diameter.AVP {
	return diameter.Unsigned32(diameter.AVPAuthApplicationID, diameter.AVPFlagMandatory, 0, ApplicationID)
}

func newRequest(env tgpp.Envelope, command uint32, avps ...diameter.AVP) (*diameter.Message, error) {
	if err := env.Validate(); err != nil {
		return nil, invalidf("%w", err)
	}

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
	}, nil
}

func checkRequest(rules diameter.Rules, req *diameter.Message) error {
	if err := rules.Check(req); err != nil {
		return err
	}

	app, _ := req.Find(diameter.AVPAuthApplicationID, 0)

	v, err := tgpp.Unsigned32(app)
	if err != nil {
		return err
	}

	if v != ApplicationID {
		return tgpp.InvalidAVP(app)
	}

	return nil
}

func featureAVPs(features Features, mandatory bool) []diameter.AVP {
	var avps []diameter.AVP

	for _, id := range []uint32{featureList1, featureList2} {
		if list := features.list(id); list != 0 {
			avps = append(avps, tgpp.SupportedFeatures{
				VendorID: tgpp.VendorID, FeatureListID: id, FeatureList: list, Mandatory: mandatory,
			}.AVP())
		}
	}

	return avps
}

func featureList(m *diameter.Message) Features {
	return listFeatures(featureList1, tgpp.FeatureList(m.AVPs, tgpp.VendorID, featureList1)) |
		listFeatures(featureList2, tgpp.FeatureList(m.AVPs, tgpp.VendorID, featureList2))
}

func featuresRequired(m *diameter.Message) bool {
	return tgpp.FeaturesMandatory(m.AVPs, tgpp.VendorID)
}

func invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %w", ErrInvalidMessage, fmt.Errorf(format, args...))
}

func malformedf(format string, args ...any) error {
	return fmt.Errorf("%w: %w", ErrMalformedAnswer, fmt.Errorf(format, args...))
}

func parseResult(ans *diameter.Message) (tgpp.Result, error) {
	result, err := tgpp.ParseFinalResult(ans)
	if err != nil {
		return tgpp.Result{}, fmt.Errorf("%w: %w", ErrMalformedAnswer, err)
	}

	if result.Failure() {
		return result, &ResultError{Result: result}
	}

	return result, nil
}

func successResult(r tgpp.Result, command uint32) (tgpp.Result, error) {
	r = r.OrSuccess()

	switch {
	case !r.Success():
		return tgpp.Result{}, invalidf("answer with the non-success %s", r)
	case r.Experimental && !experimentalAllowed(command):
		return tgpp.Result{}, invalidf("command %d answered with the experimental %s", command, r)
	}

	return r, nil
}

func errorResult(r tgpp.Result) error {
	if !r.Failure() {
		return invalidf("error answer with the non-error %s", r)
	}

	return nil
}

func successAnswer(req *diameter.Message, id diameter.Identity, r tgpp.Result) (*diameter.Message, error) {
	result, err := successResult(r, req.CommandCode)
	if err != nil {
		return nil, err
	}

	return NewAnswer(req, id, result, 0), nil
}
