// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sh

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const ApplicationID uint32 = 16777217

const (
	CommandUserData               uint32 = 306
	CommandProfileUpdate          uint32 = 307
	CommandSubscribeNotifications uint32 = 308
	CommandPushNotification       uint32 = 309
)

func init() {
	diameter.RegisterMethod(ApplicationID, CommandUserData, "Sh/User-Data")
	diameter.RegisterMethod(ApplicationID, CommandProfileUpdate, "Sh/Profile-Update")
	diameter.RegisterMethod(ApplicationID, CommandSubscribeNotifications, "Sh/Subscribe-Notifications")
	diameter.RegisterMethod(ApplicationID, CommandPushNotification, "Sh/Push-Notification")
}

const (
	AVPPublicIdentity           uint32 = 601
	AVPServerName               uint32 = 602
	AVPSupportedApplications    uint32 = 631
	AVPWildcardedPublicIdentity uint32 = 634
	AVPWildcardedIMPU           uint32 = 636
	AVPSessionPriority          uint32 = 650
	AVPUserIdentity             uint32 = 700
	AVPMSISDN                   uint32 = 701
	AVPUserData                 uint32 = 702
	AVPDataReference            uint32 = 703
	AVPServiceIndication        uint32 = 704
	AVPSubsReqType              uint32 = 705
	AVPRequestedDomain          uint32 = 706
	AVPCurrentLocation          uint32 = 707
	AVPIdentitySet              uint32 = 708
	AVPExpiryTime               uint32 = 709
	AVPSendDataIndication       uint32 = 710
	AVPDSAITag                  uint32 = 711
	AVPOneTimeNotification      uint32 = 712
	AVPRequestedNodes           uint32 = 713
	AVPServingNodeIndication    uint32 = 714
	AVPRepositoryDataID         uint32 = 715
	AVPSequenceNumber           uint32 = 716
	AVPPrePagingSupported       uint32 = 717
	AVPLocalTimeZoneIndication  uint32 = 718
	AVPUDRFlags                 uint32 = 719
	AVPCallReferenceInfo        uint32 = 720
	AVPCallReferenceNumber      uint32 = 721
	AVPASNumber                 uint32 = 722
	AVPExternalIdentifier       uint32 = 3111
)

const (
	featureListID     uint32 = 1
	maxSequenceNumber uint32 = 65535
)

var (
	ErrMalformedAnswer = errors.New("sh: malformed answer")
	ErrInvalidMessage  = errors.New("sh: invalid message")
)

type ResultError struct {
	tgpp.Result

	Features Features
}

func (e *ResultError) Error() string {
	return "sh: request failed with " + e.String()
}

type UserIdentity struct {
	PublicIdentity     string
	MSISDN             string
	ExternalIdentifier string
}

var commonRequestRules = diameter.BaseRequestRules().With(diameter.Rules{
	{Code: tgpp.AVPSupportedFeatures, VendorID: tgpp.VendorID}: {Multiple: true},
	{Code: diameter.AVPOCSupportedFeatures}:                    {},
})

func NewAnswer(req *diameter.Message, id diameter.Identity, r tgpp.Result, features Features) *diameter.Message {
	return finishAnswer(tgpp.NewResultAnswer(req, id, r), features)
}

func NewErrorAnswer(req *diameter.Message, id diameter.Identity, err error, features Features) *diameter.Message {
	return finishAnswer(tgpp.NewErrorAnswer(req, id, err), features)
}

func finishAnswer(ans *diameter.Message, features Features) *diameter.Message {
	ans.AVPs = append(ans.AVPs, vendorSpecificApplicationID())
	ans.AVPs = append(ans.AVPs, featureAVPs(features, false)...)

	return ans
}

func vendorSpecificApplicationID() diameter.AVP {
	return diameter.Grouped(diameter.AVPVendorSpecificApplicationID, diameter.AVPFlagMandatory, 0,
		diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, tgpp.VendorID),
		diameter.Unsigned32(diameter.AVPAuthApplicationID, diameter.AVPFlagMandatory, 0, ApplicationID),
	)
}

func newRequest(env tgpp.Envelope, command uint32, features Features, required bool, avps ...diameter.AVP) (*diameter.Message, error) {
	if required && features == 0 {
		return nil, invalidf("features required without any feature")
	}

	if err := env.Validate(); err != nil {
		return nil, invalidf("%w", err)
	}

	msg := &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   command,
		ApplicationID: ApplicationID,
		AVPs:          append(env.AVPs(), vendorSpecificApplicationID()),
	}

	msg.AVPs = append(msg.AVPs, featureAVPs(features, required)...)
	msg.AVPs = append(msg.AVPs, avps...)

	return msg, nil
}

func (f Features) lists() tgpp.FeatureLists {
	return tgpp.FeatureLists{featureListID: uint32(f)}
}

func featureAVPs(features Features, required bool) []diameter.AVP {
	return features.lists().AVPs(required)
}

func featureList(m *diameter.Message) Features {
	return Features(tgpp.ParseFeatureLists(m.AVPs)[featureListID])
}

func RequiresUnsupportedFeatures(req *diameter.Message, supported Features) bool {
	return supported.lists().Unsupported(req.AVPs)
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
		return result, &ResultError{Result: result, Features: featureList(ans)}
	}

	return result, nil
}

func errorResult(r tgpp.Result) error {
	if !r.Failure() {
		return invalidf("error answer with the non-error %s", r)
	}

	return nil
}

func successResult(r tgpp.Result) (tgpp.Result, error) {
	r = r.OrSuccess()
	if !r.Success() {
		return tgpp.Result{}, invalidf("answer with the non-success %s", r)
	}

	return r, nil
}

func ValidPublicIdentity(s string) bool {
	return hasScheme(s, "sip:", "sips:", "tel:")
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

func userIdentityAVP(u UserIdentity) (diameter.AVP, error) {
	var inner []diameter.AVP

	if u.PublicIdentity != "" {
		if !ValidPublicIdentity(u.PublicIdentity) {
			return diameter.AVP{}, invalidf("public identity %q", u.PublicIdentity)
		}

		inner = append(inner, vendorString(AVPPublicIdentity, u.PublicIdentity))
	}

	if u.MSISDN != "" {
		msisdn, err := tgpp.EncodeE164(u.MSISDN)
		if err != nil {
			return diameter.AVP{}, invalidf("MSISDN: %w", err)
		}

		inner = append(inner, diameter.OctetString(AVPMSISDN, diameter.AVPFlagMandatory, tgpp.VendorID, msisdn))
	}

	if u.ExternalIdentifier != "" {
		if !validExternalIdentifier(u.ExternalIdentifier) {
			return diameter.AVP{}, invalidf("external identifier %q", u.ExternalIdentifier)
		}

		inner = append(inner, vendorString(AVPExternalIdentifier, u.ExternalIdentifier))
	}

	if len(inner) != 1 {
		return diameter.AVP{}, invalidf("user identity with %d of a public identity, an MSISDN and an external identifier", len(inner))
	}

	return diameter.Grouped(AVPUserIdentity, diameter.AVPFlagMandatory, tgpp.VendorID, inner...), nil
}

func parseUserIdentity(a diameter.AVP) (UserIdentity, error) {
	inner, err := a.Grouped()
	if err != nil {
		return UserIdentity{}, tgpp.InvalidAVP(a)
	}

	var (
		u     UserIdentity
		found int
	)

	if p, ok := diameter.Find(inner, AVPPublicIdentity, tgpp.VendorID); ok {
		if !ValidPublicIdentity(p.UTF8String()) {
			return UserIdentity{}, tgpp.InvalidAVP(p)
		}

		u.PublicIdentity = p.UTF8String()
		found++
	}

	if m, ok := diameter.Find(inner, AVPMSISDN, tgpp.VendorID); ok {
		if u.MSISDN, err = tgpp.DecodeE164(m.Data); err != nil {
			return UserIdentity{}, tgpp.InvalidAVP(m)
		}

		found++
	}

	if e, ok := diameter.Find(inner, AVPExternalIdentifier, tgpp.VendorID); ok {
		if !validExternalIdentifier(e.UTF8String()) {
			return UserIdentity{}, tgpp.InvalidAVP(e)
		}

		u.ExternalIdentifier = e.UTF8String()
		found++
	}

	if found != 1 {
		return UserIdentity{}, tgpp.InvalidAVP(a)
	}

	return u, nil
}

func validExternalIdentifier(s string) bool {
	local, domain, ok := strings.Cut(s, "@")

	return ok && local != "" && domain != "" && !strings.Contains(domain, "@")
}

func wildcardedAVPs(publicIdentity, impu string) ([]diameter.AVP, error) {
	if publicIdentity != "" && impu != "" {
		return nil, invalidf("both a wildcarded PSI and a wildcarded public user identity")
	}

	var avps []diameter.AVP

	for _, w := range []struct {
		code  uint32
		value string
	}{
		{AVPWildcardedPublicIdentity, publicIdentity},
		{AVPWildcardedIMPU, impu},
	} {
		if w.value == "" {
			continue
		}

		if !ValidPublicIdentity(w.value) {
			return nil, invalidf("wildcarded identity %q", w.value)
		}

		avps = append(avps, diameter.UTF8String(w.code, 0, tgpp.VendorID, w.value))
	}

	return avps, nil
}

func requestWildcarded(req *diameter.Message) (string, string, error) {
	psi, psiFound := req.Find(AVPWildcardedPublicIdentity, tgpp.VendorID)
	impu, impuFound := req.Find(AVPWildcardedIMPU, tgpp.VendorID)

	for _, w := range []struct {
		avp   diameter.AVP
		found bool
	}{{psi, psiFound}, {impu, impuFound}} {
		if w.found && !ValidPublicIdentity(w.avp.UTF8String()) {
			return "", "", tgpp.InvalidAVP(w.avp)
		}
	}

	if psiFound && impuFound {
		return "", "", diameter.NewAVPError(diameter.ResultContradictingAVPs, psi, impu)
	}

	return psi.UTF8String(), impu.UTF8String(), nil
}

func answerWildcarded(ans *diameter.Message) (string, string, error) {
	var values [2]string

	for i, f := range []struct {
		code uint32
		name string
	}{
		{AVPWildcardedPublicIdentity, "Wildcarded-Public-Identity"},
		{AVPWildcardedIMPU, "Wildcarded-IMPU"},
	} {
		a, ok := ans.Find(f.code, tgpp.VendorID)
		if !ok {
			continue
		}

		if !ValidPublicIdentity(a.UTF8String()) {
			return "", "", malformedf("%s %q", f.name, a.UTF8String())
		}

		values[i] = a.UTF8String()
	}

	if values[0] != "" && values[1] != "" {
		return "", "", malformedf("both Wildcarded-Public-Identity and Wildcarded-IMPU")
	}

	return values[0], values[1], nil
}

func dataReferenceAVP(r DataReference) (diameter.AVP, error) {
	if !r.Valid() {
		return diameter.AVP{}, invalidf("Data-Reference %d", uint32(r))
	}

	return diameter.Unsigned32(AVPDataReference, diameter.AVPFlagMandatory, tgpp.VendorID, uint32(r)), nil
}

func parseDataReference(a diameter.AVP) (DataReference, error) {
	v, err := tgpp.Unsigned32(a)
	if err != nil {
		return 0, err
	}

	if r := DataReference(v); r.Valid() {
		return r, nil
	}

	return 0, tgpp.InvalidAVP(a)
}
