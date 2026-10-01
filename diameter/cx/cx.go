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

const (
	CommandUserAuthorization       uint32 = 300
	CommandServerAssignment        uint32 = 301
	CommandLocationInfo            uint32 = 302
	CommandMultimediaAuth          uint32 = 303
	CommandRegistrationTermination uint32 = 304
	CommandPushProfile             uint32 = 305
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
	AVPWebRTCAuthenticationFunctionName        uint32 = 657
	AVPWebRTCWebServerFunctionName             uint32 = 658
	AVPRTRFlags                                uint32 = 659
	AVPFailedPCSCF                             uint32 = 664
)

const avpOCSupportedFeatures uint32 = 621

const (
	uarFlagEmergencyRegistration    uint32 = 1 << 0
	liaFlagPSIDirectRouting         uint32 = 1 << 0
	rtrFlagReferenceLocationChanged uint32 = 1 << 0
	sarFlagPCSCFRestoration         uint32 = 1 << 0
	featureListID                   uint32 = 1
)

const (
	userDataNotAvailable     uint32 = 0
	userDataAlreadyAvailable uint32 = 1
	looseRouteRequired       uint32 = 1
	privilegedSender         uint32 = 1
	originating              uint32 = 0
)

var (
	ErrMalformedAnswer = errors.New("cx: malformed answer")
	ErrInvalidMessage  = errors.New("cx: invalid message")
)

type ResultError struct {
	tgpp.Result

	Features Features
}

func (e *ResultError) Error() string {
	return "cx: request failed with " + e.String()
}

type ServerAssignmentError struct {
	ResultError

	PrivateIdentity          string
	ServerName               string
	WildcardedPublicIdentity string
}

func (e *ServerAssignmentError) Unwrap() error {
	return &e.ResultError
}

type RegistrationTerminationError struct {
	ResultError

	AssociatedIdentities []string
	EmergencyIdentities  []EmergencyIdentity
}

func (e *RegistrationTerminationError) Unwrap() error {
	return &e.ResultError
}

type ServerCapabilities struct {
	Mandatory   []uint32
	Optional    []uint32
	ServerNames []string
}

func NewAnswer(req *diameter.Message, id diameter.Identity, r tgpp.Result, features Features) *diameter.Message {
	return finishAnswer(tgpp.NewResultAnswer(req, id, r), features)
}

func NewErrorAnswer(req *diameter.Message, id diameter.Identity, err error, features Features) *diameter.Message {
	return finishAnswer(tgpp.NewErrorAnswer(req, id, err), features)
}

func finishAnswer(ans *diameter.Message, features Features) *diameter.Message {
	ans.AVPs = append(ans.AVPs, vendorSpecificApplicationID())

	return withFeatures(ans, features)
}

func withFeatures(msg *diameter.Message, features Features) *diameter.Message {
	if features != 0 {
		msg.AVPs = append(msg.AVPs, tgpp.SupportedFeatures{VendorID: tgpp.VendorID, FeatureListID: featureListID, FeatureList: uint32(features)}.AVP())
	}

	return msg
}

func vendorSpecificApplicationID() diameter.AVP {
	return diameter.Grouped(diameter.AVPVendorSpecificApplicationID, diameter.AVPFlagMandatory, 0,
		diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, tgpp.VendorID),
		diameter.Unsigned32(diameter.AVPAuthApplicationID, diameter.AVPFlagMandatory, 0, ApplicationID),
	)
}

func newRequest(env tgpp.Envelope, command uint32, features Features, avps ...diameter.AVP) *diameter.Message {
	msg := withFeatures(&diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   command,
		ApplicationID: ApplicationID,
		AVPs:          append(env.AVPs(), vendorSpecificApplicationID()),
	}, features)

	msg.AVPs = append(msg.AVPs, avps...)

	return msg
}

func featureList(m *diameter.Message) Features {
	return Features(tgpp.FeatureList(m.AVPs, tgpp.VendorID, featureListID))
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
		return result, &ResultError{Result: result, Features: featureList(ans)}
	}

	return result, nil
}

func errorResult(r tgpp.Result) error {
	if r.Success() {
		return invalid("error answer with the success %s", r)
	}

	return nil
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

func wildcardedIdentityAVP(v string) (diameter.AVP, error) {
	if !ValidPublicIdentity(v) {
		return diameter.AVP{}, invalid("wildcarded public identity %q", v)
	}

	return diameter.UTF8String(AVPWildcardedPublicIdentity, 0, tgpp.VendorID, v), nil
}

func answerWildcardedIdentity(ans *diameter.Message) (string, error) {
	a, ok := ans.Find(AVPWildcardedPublicIdentity, tgpp.VendorID)
	if !ok {
		return "", nil
	}

	if !ValidPublicIdentity(a.UTF8String()) {
		return "", malformed("Wildcarded-Public-Identity %q", a.UTF8String())
	}

	return a.UTF8String(), nil
}

func answerServerName(ans *diameter.Message) (string, error) {
	a, ok := ans.Find(AVPServerName, tgpp.VendorID)
	if !ok {
		return "", nil
	}

	if !validServerName(a.UTF8String()) {
		return "", malformed("Server-Name %q", a.UTF8String())
	}

	return a.UTF8String(), nil
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

func optionalFlags(req *diameter.Message, code uint32) (uint32, error) {
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

	if c.Mandatory, err = unsignedList(inner, AVPMandatoryCapability); err != nil {
		return nil, err
	}

	if c.Optional, err = unsignedList(inner, AVPOptionalCapability); err != nil {
		return nil, err
	}

	for _, name := range diameter.FindAll(inner, AVPServerName, tgpp.VendorID) {
		if !validServerName(name.UTF8String()) {
			return nil, fmt.Errorf("Server-Name %q", name.UTF8String())
		}

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

func identityPairAVP(code uint32, private, public string) (diameter.AVP, error) {
	if private == "" {
		return diameter.AVP{}, invalid("identity pair without a private identity")
	}

	p, err := publicIdentityAVP(public)
	if err != nil {
		return diameter.AVP{}, err
	}

	return diameter.Grouped(code, 0, tgpp.VendorID, userName(private), p), nil
}

func unsignedList(avps []diameter.AVP, code uint32) ([]uint32, error) {
	var out []uint32

	for _, a := range diameter.FindAll(avps, code, tgpp.VendorID) {
		v, err := a.Unsigned32()
		if err != nil {
			return nil, err
		}

		out = append(out, v)
	}

	return out, nil
}
