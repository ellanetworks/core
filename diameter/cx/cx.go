// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const ApplicationID uint32 = 16777216

var Application = diameter.Application{ID: ApplicationID, VendorID: tgpp.VendorID}

const (
	CommandUserAuthorization       uint32 = 300
	CommandServerAssignment        uint32 = 301
	CommandLocationInfo            uint32 = 302
	CommandMultimediaAuth          uint32 = 303
	CommandRegistrationTermination uint32 = 304
)

const (
	AVPVisitedNetworkIdentifier                uint32 = 600
	AVPPublicIdentity                          uint32 = 601
	AVPServerName                              uint32 = 602
	AVPServerCapabilities                      uint32 = 603
	AVPMandatoryCapability                     uint32 = 604
	AVPOptionalCapability                      uint32 = 605
	AVPUserData                                uint32 = 606
	AVPSIPNumberAuthItems                      uint32 = 607
	AVPSIPAuthenticationScheme                 uint32 = 608
	AVPSIPAuthenticate                         uint32 = 609
	AVPSIPAuthorization                        uint32 = 610
	AVPSIPAuthenticationContext                uint32 = 611
	AVPSIPAuthDataItem                         uint32 = 612
	AVPSIPItemNumber                           uint32 = 613
	AVPServerAssignmentType                    uint32 = 614
	AVPDeregistrationReason                    uint32 = 615
	AVPReasonCode                              uint32 = 616
	AVPReasonInfo                              uint32 = 617
	AVPChargingInformation                     uint32 = 618
	AVPPrimaryEventChargingFunctionName        uint32 = 619
	AVPSecondaryEventChargingFunctionName      uint32 = 620
	AVPPrimaryChargingCollectionFunctionName   uint32 = 621
	AVPSecondaryChargingCollectionFunctionName uint32 = 622
	AVPUserAuthorizationType                   uint32 = 623
	AVPUserDataAlreadyAvailable                uint32 = 624
	AVPConfidentialityKey                      uint32 = 625
	AVPIntegrityKey                            uint32 = 626
	AVPAssociatedIdentities                    uint32 = 632
	AVPOriginatingRequest                      uint32 = 633
	AVPWildcardedPublicIdentity                uint32 = 634
	AVPUARFlags                                uint32 = 637
	AVPLooseRouteIndication                    uint32 = 638
	AVPSCSCFRestorationInfo                    uint32 = 639
	AVPAssociatedRegisteredIdentities          uint32 = 647
	AVPMultipleRegistrationIndication          uint32 = 648
	AVPSessionPriority                         uint32 = 650
	AVPIdentityWithEmergencyRegistration       uint32 = 651
	AVPPriviledgedSenderIndication             uint32 = 652
	AVPLIAFlags                                uint32 = 653
	AVPSARFlags                                uint32 = 655
	AVPAllowedWAFWWSFIdentities                uint32 = 656
	AVPRTRFlags                                uint32 = 659
	AVPFailedPCSCF                             uint32 = 664
)

const (
	avpOCSupportedFeatures uint32 = 621
	avpOCOLR               uint32 = 623
	avpLoad                uint32 = 650
)

const (
	AuthorizationRegistration                uint32 = 0
	AuthorizationDeregistration              uint32 = 1
	AuthorizationRegistrationAndCapabilities uint32 = 2
)

const (
	AssignmentNoAssignment                     uint32 = 0
	AssignmentRegistration                     uint32 = 1
	AssignmentReRegistration                   uint32 = 2
	AssignmentUnregisteredUser                 uint32 = 3
	AssignmentTimeoutDeregistration            uint32 = 4
	AssignmentUserDeregistration               uint32 = 5
	AssignmentTimeoutDeregistrationStoreServer uint32 = 6
	AssignmentUserDeregistrationStoreServer    uint32 = 7
	AssignmentAdministrativeDeregistration     uint32 = 8
	AssignmentAuthenticationFailure            uint32 = 9
	AssignmentAuthenticationTimeout            uint32 = 10
	AssignmentDeregistrationTooMuchData        uint32 = 11
)

const (
	ReasonPermanentTermination uint32 = 0
	ReasonNewServerAssigned    uint32 = 1
	ReasonServerChange         uint32 = 2
	ReasonRemoveSCSCF          uint32 = 3
)

const (
	SchemeDigestAKAv1MD5   = "Digest-AKAv1-MD5"
	SchemeSIPDigest        = "SIP Digest"
	SchemeNASSBundled      = "NASS-Bundled"
	SchemeEarlyIMSSecurity = "Early-IMS-Security"
	SchemeUnknown          = "Unknown"
)

const (
	UARFlagEmergencyRegistration    uint32 = 1 << 0
	LIAFlagPSIDirectRouting         uint32 = 1 << 0
	RTRFlagReferenceLocationChanged uint32 = 1 << 0
)

const (
	userDataNotAvailable     uint32 = 0
	userDataAlreadyAvailable uint32 = 1
	looseRouteRequired       uint32 = 1
	priviledgedSender        uint32 = 1
	originating              uint32 = 0
)

const (
	FeatureListID           uint32 = 1
	FeatureSharedIFCSets    uint32 = 1 << 0
	FeatureAliasIndication  uint32 = 1 << 1
	FeatureIMSRestoration   uint32 = 1 << 2
	FeaturePCSCFRestoration uint32 = 1 << 3
)

var (
	ErrMalformedAnswer = errors.New("cx: malformed answer")
	ErrInvalidMessage  = errors.New("cx: invalid message")
)

type ResultError struct {
	tgpp.Result
}

func (e *ResultError) Error() string {
	return "cx: request failed with " + e.String()
}

type ServerCapabilities struct {
	Mandatory   []uint32
	Optional    []uint32
	ServerNames []string
}

func NewAnswer(req *diameter.Message, id diameter.Identity, r tgpp.Result, features uint32) *diameter.Message {
	return finishAnswer(tgpp.NewResultAnswer(req, id, r), features)
}

func NewErrorAnswer(req *diameter.Message, id diameter.Identity, err error, features uint32) *diameter.Message {
	return finishAnswer(tgpp.NewErrorAnswer(req, id, err), features)
}

func finishAnswer(ans *diameter.Message, features uint32) *diameter.Message {
	ans.AVPs = append(ans.AVPs, vendorSpecificApplicationID())

	return withFeatures(ans, features)
}

func withFeatures(msg *diameter.Message, features uint32) *diameter.Message {
	if features != 0 {
		msg.AVPs = append(msg.AVPs, tgpp.SupportedFeatures{VendorID: tgpp.VendorID, FeatureListID: FeatureListID, FeatureList: features}.AVP())
	}

	return msg
}

func vendorSpecificApplicationID() diameter.AVP {
	return diameter.Grouped(diameter.AVPVendorSpecificApplicationID, diameter.AVPFlagMandatory, 0,
		diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, tgpp.VendorID),
		diameter.Unsigned32(diameter.AVPAuthApplicationID, diameter.AVPFlagMandatory, 0, ApplicationID),
	)
}

func newRequest(env tgpp.Envelope, command uint32, features uint32, avps ...diameter.AVP) *diameter.Message {
	msg := withFeatures(&diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   command,
		ApplicationID: ApplicationID,
		AVPs:          append(env.AVPs(), vendorSpecificApplicationID()),
	}, features)

	msg.AVPs = append(msg.AVPs, avps...)

	return msg
}

func features(m *diameter.Message) uint32 {
	return tgpp.FeatureList(m.AVPs, tgpp.VendorID, FeatureListID)
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

func successResult(r tgpp.Result) error {
	if !r.Success() {
		return invalid("answer with the non-success %s", r)
	}

	return nil
}

func ValidPublicIdentity(s string) bool {
	return hasScheme(s, "sip:", "sips:", "tel:")
}

func validServerName(s string) bool {
	return hasScheme(s, "sip:", "sips:")
}

func hasScheme(s string, schemes ...string) bool {
	for _, scheme := range schemes {
		if len(s) > len(scheme) && strings.EqualFold(s[:len(scheme)], scheme) {
			return true
		}
	}

	return false
}

func vendorString(code uint32, v string) diameter.AVP {
	return diameter.UTF8String(code, diameter.AVPFlagMandatory, tgpp.VendorID, v)
}

func vendorUnsigned(code uint32, v uint32) diameter.AVP {
	return diameter.Unsigned32(code, diameter.AVPFlagMandatory, tgpp.VendorID, v)
}

func userName(v string) diameter.AVP {
	return diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, v)
}

func publicIdentityAVP(v string) (diameter.AVP, error) {
	if !ValidPublicIdentity(v) {
		return diameter.AVP{}, invalid("public identity %q", v)
	}

	return vendorString(AVPPublicIdentity, v), nil
}

func serverNameAVP(v string) (diameter.AVP, error) {
	if !validServerName(v) {
		return diameter.AVP{}, invalid("server name %q", v)
	}

	return vendorString(AVPServerName, v), nil
}

func requestPublicIdentity(req *diameter.Message) (string, error) {
	a, _ := req.Find(AVPPublicIdentity, tgpp.VendorID)
	if !ValidPublicIdentity(a.UTF8String()) {
		return "", tgpp.InvalidAVP(a)
	}

	return a.UTF8String(), nil
}

func requestServerName(req *diameter.Message) (string, error) {
	a, _ := req.Find(AVPServerName, tgpp.VendorID)
	if !validServerName(a.UTF8String()) {
		return "", tgpp.InvalidAVP(a)
	}

	return a.UTF8String(), nil
}

func optionalUnsigned(req *diameter.Message, code uint32, maxValue uint32) (uint32, bool, error) {
	a, ok := req.Find(code, tgpp.VendorID)
	if !ok {
		return 0, false, nil
	}

	v, err := a.Unsigned32()
	if err != nil || v > maxValue {
		return 0, false, tgpp.InvalidAVP(a)
	}

	return v, true, nil
}

func flags(req *diameter.Message, code uint32) (uint32, error) {
	v, _, err := optionalUnsigned(req, code, ^uint32(0))
	return v, err
}

func identityList(code uint32, names []string) diameter.AVP {
	inner := make([]diameter.AVP, 0, len(names))
	for _, n := range names {
		inner = append(inner, userName(n))
	}

	return diameter.Grouped(code, 0, tgpp.VendorID, inner...)
}

func parseIdentityList(a diameter.AVP) ([]string, error) {
	inner, err := a.Grouped()
	if err != nil {
		return nil, err
	}

	var names []string

	for _, n := range diameter.FindAll(inner, diameter.AVPUserName, 0) {
		if n.UTF8String() == "" {
			return nil, errors.New("empty User-Name")
		}

		names = append(names, n.UTF8String())
	}

	return names, nil
}

func validIdentityList(names []string) error {
	for _, n := range names {
		if n == "" {
			return invalid("empty private identity")
		}
	}

	return nil
}

func capabilitiesAVP(c ServerCapabilities) (diameter.AVP, error) {
	var inner []diameter.AVP

	for _, v := range c.Mandatory {
		inner = append(inner, vendorUnsigned(AVPMandatoryCapability, v))
	}

	for _, v := range c.Optional {
		inner = append(inner, vendorUnsigned(AVPOptionalCapability, v))
	}

	for _, name := range c.ServerNames {
		a, err := serverNameAVP(name)
		if err != nil {
			return diameter.AVP{}, err
		}

		inner = append(inner, a)
	}

	return diameter.Grouped(AVPServerCapabilities, diameter.AVPFlagMandatory, tgpp.VendorID, inner...), nil
}

func parseCapabilities(a diameter.AVP) (*ServerCapabilities, error) {
	inner, err := a.Grouped()
	if err != nil {
		return nil, err
	}

	var c ServerCapabilities

	for _, f := range []struct {
		code uint32
		dst  *[]uint32
	}{
		{AVPMandatoryCapability, &c.Mandatory},
		{AVPOptionalCapability, &c.Optional},
	} {
		for _, v := range diameter.FindAll(inner, f.code, tgpp.VendorID) {
			n, err := v.Unsigned32()
			if err != nil {
				return nil, err
			}

			*f.dst = append(*f.dst, n)
		}
	}

	for _, name := range diameter.FindAll(inner, AVPServerName, tgpp.VendorID) {
		c.ServerNames = append(c.ServerNames, name.UTF8String())
	}

	return &c, nil
}

func answerString(ans *diameter.Message, code uint32, vendorID uint32) string {
	a, _ := ans.Find(code, vendorID)
	return a.UTF8String()
}

func answerUnsigned(ans *diameter.Message, code uint32, name string) (uint32, error) {
	a, ok := ans.Find(code, tgpp.VendorID)
	if !ok {
		return 0, nil
	}

	v, err := a.Unsigned32()
	if err != nil {
		return 0, malformed("%s", name)
	}

	return v, nil
}
