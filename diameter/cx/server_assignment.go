// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"slices"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type ServerAssignmentRequest struct {
	PrivateIdentity          string
	PublicIdentities         []string
	ServerName               string
	Type                     uint32
	UserDataAlreadyAvailable bool
	Features                 uint32
}

type ChargingInformation struct {
	PrimaryEventChargingFunction        string
	SecondaryEventChargingFunction      string
	PrimaryChargingCollectionFunction   string
	SecondaryChargingCollectionFunction string
}

type ServerAssignment struct {
	Result               tgpp.Result
	PrivateIdentity      string
	UserData             []byte
	Charging             *ChargingInformation
	AssociatedIdentities []string
	LooseRouteRequired   bool
	ServerName           string
	PriviledgedSender    bool
	Features             uint32
}

var sarRules = commonRequestRules.With(diameter.Rules{
	{Code: diameter.AVPUserName}:                                       {},
	{Code: AVPPublicIdentity, VendorID: tgpp.VendorID}:                 {Multiple: true},
	{Code: AVPWildcardedPublicIdentity, VendorID: tgpp.VendorID}:       {},
	{Code: AVPServerName, VendorID: tgpp.VendorID}:                     {Required: true},
	{Code: AVPServerAssignmentType, VendorID: tgpp.VendorID}:           {Required: true},
	{Code: AVPUserDataAlreadyAvailable, VendorID: tgpp.VendorID}:       {Required: true},
	{Code: AVPSCSCFRestorationInfo, VendorID: tgpp.VendorID}:           {},
	{Code: AVPMultipleRegistrationIndication, VendorID: tgpp.VendorID}: {},
	{Code: AVPSessionPriority, VendorID: tgpp.VendorID}:                {},
	{Code: AVPSARFlags, VendorID: tgpp.VendorID}:                       {},
	{Code: AVPFailedPCSCF, VendorID: tgpp.VendorID}:                    {},
})

var bulkDeregistrations = []uint32{
	AssignmentTimeoutDeregistration,
	AssignmentUserDeregistration,
	AssignmentDeregistrationTooMuchData,
	AssignmentTimeoutDeregistrationStoreServer,
	AssignmentUserDeregistrationStoreServer,
	AssignmentAdministrativeDeregistration,
}

func assignmentDownloadsProfile(t uint32) bool {
	return t <= AssignmentUnregisteredUser
}

type identityProblem int

const (
	identitiesValid identityProblem = iota
	missingPrivateIdentity
	missingPublicIdentity
	extraPublicIdentity
)

func checkIdentities(t uint32, private string, publics int) identityProblem {
	switch {
	case slices.Contains(bulkDeregistrations, t):
		if private == "" && publics == 0 {
			return missingPrivateIdentity
		}
	case publics == 0:
		return missingPublicIdentity
	case publics > 1:
		return extraPublicIdentity
	case private == "" && t != AssignmentUnregisteredUser && t != AssignmentNoAssignment:
		return missingPrivateIdentity
	}

	return identitiesValid
}

func NewServerAssignmentRequest(env tgpp.Envelope, r ServerAssignmentRequest) (*diameter.Message, error) {
	if r.Type > AssignmentDeregistrationTooMuchData {
		return nil, invalid("Server-Assignment-Type %d", r.Type)
	}

	switch checkIdentities(r.Type, r.PrivateIdentity, len(r.PublicIdentities)) {
	case missingPrivateIdentity:
		return nil, invalid("Server-Assignment-Type %d without a private identity", r.Type)
	case missingPublicIdentity, extraPublicIdentity:
		return nil, invalid("Server-Assignment-Type %d with %d public identities, want one", r.Type, len(r.PublicIdentities))
	}

	server, err := serverNameAVP(r.ServerName)
	if err != nil {
		return nil, err
	}

	var avps []diameter.AVP

	if r.PrivateIdentity != "" {
		avps = append(avps, userName(r.PrivateIdentity))
	}

	for _, p := range r.PublicIdentities {
		a, err := publicIdentityAVP(p)
		if err != nil {
			return nil, err
		}

		avps = append(avps, a)
	}

	available := userDataNotAvailable
	if r.UserDataAlreadyAvailable {
		available = userDataAlreadyAvailable
	}

	avps = append(avps,
		server,
		vendorUnsigned(AVPServerAssignmentType, r.Type),
		vendorUnsigned(AVPUserDataAlreadyAvailable, available),
	)

	return newRequest(env, CommandServerAssignment, r.Features, avps...), nil
}

func CheckServerAssignment(req *diameter.Message) error {
	return sarRules.Check(req)
}

func ParseServerAssignmentRequest(req *diameter.Message) (ServerAssignmentRequest, error) {
	if err := CheckServerAssignment(req); err != nil {
		return ServerAssignmentRequest{}, err
	}

	server, err := requestServerName(req)
	if err != nil {
		return ServerAssignmentRequest{}, err
	}

	t, _, err := optionalUnsigned(req, AVPServerAssignmentType, AssignmentDeregistrationTooMuchData)
	if err != nil {
		return ServerAssignmentRequest{}, err
	}

	available, _, err := optionalUnsigned(req, AVPUserDataAlreadyAvailable, userDataAlreadyAvailable)
	if err != nil {
		return ServerAssignmentRequest{}, err
	}

	r := ServerAssignmentRequest{
		ServerName:               server,
		Type:                     t,
		UserDataAlreadyAvailable: available == userDataAlreadyAvailable,
		Features:                 features(req),
	}

	if user, ok := req.Find(diameter.AVPUserName, 0); ok {
		if r.PrivateIdentity = user.UTF8String(); r.PrivateIdentity == "" {
			return ServerAssignmentRequest{}, tgpp.InvalidAVP(user)
		}
	}

	publics := diameter.FindAll(req.AVPs, AVPPublicIdentity, tgpp.VendorID)

	for _, p := range publics {
		if !ValidPublicIdentity(p.UTF8String()) {
			return ServerAssignmentRequest{}, tgpp.InvalidAVP(p)
		}

		r.PublicIdentities = append(r.PublicIdentities, p.UTF8String())
	}

	switch checkIdentities(r.Type, r.PrivateIdentity, len(publics)) {
	case missingPrivateIdentity:
		return ServerAssignmentRequest{}, tgpp.MissingAVP(diameter.AVPUserName, 0)
	case missingPublicIdentity:
		return ServerAssignmentRequest{}, tgpp.MissingAVP(AVPPublicIdentity, tgpp.VendorID)
	case extraPublicIdentity:
		return ServerAssignmentRequest{}, diameter.NewAVPError(diameter.ResultAVPOccursTooManyTimes, publics[1])
	}

	return r, nil
}

func NewServerAssignmentAnswer(req *diameter.Message, id diameter.Identity, a ServerAssignment) (*diameter.Message, error) {
	if err := successResult(a.Result); err != nil {
		return nil, err
	}

	if err := validIdentityList(a.AssociatedIdentities); err != nil {
		return nil, err
	}

	t, _, err := optionalUnsigned(req, AVPServerAssignmentType, ^uint32(0))
	if err != nil {
		return nil, invalid("request Server-Assignment-Type")
	}

	if _, ok := req.Find(AVPServerAssignmentType, tgpp.VendorID); ok && assignmentDownloadsProfile(t) && len(a.UserData) == 0 {
		return nil, invalid("Server-Assignment-Type %d answered without user data", t)
	}

	ans := NewAnswer(req, id, a.Result, a.Features)

	if a.PrivateIdentity != "" {
		ans.AVPs = append(ans.AVPs, userName(a.PrivateIdentity))
	}

	if len(a.UserData) > 0 {
		ans.AVPs = append(ans.AVPs, diameter.OctetString(AVPUserData, diameter.AVPFlagMandatory, tgpp.VendorID, a.UserData))
	}

	if a.Charging != nil {
		charging, err := chargingAVP(*a.Charging)
		if err != nil {
			return nil, err
		}

		ans.AVPs = append(ans.AVPs, charging)
	}

	if len(a.AssociatedIdentities) > 0 {
		ans.AVPs = append(ans.AVPs, identityList(AVPAssociatedIdentities, a.AssociatedIdentities))
	}

	if a.LooseRouteRequired {
		ans.AVPs = append(ans.AVPs, diameter.Unsigned32(AVPLooseRouteIndication, 0, tgpp.VendorID, looseRouteRequired))
	}

	if a.ServerName != "" {
		server, err := serverNameAVP(a.ServerName)
		if err != nil {
			return nil, err
		}

		ans.AVPs = append(ans.AVPs, server)
	}

	if a.PriviledgedSender {
		ans.AVPs = append(ans.AVPs, diameter.Unsigned32(AVPPriviledgedSenderIndication, 0, tgpp.VendorID, priviledgedSender))
	}

	return ans, nil
}

func ParseServerAssignmentAnswer(ans *diameter.Message) (ServerAssignment, error) {
	result, err := parseResult(ans)
	if err != nil {
		return ServerAssignment{}, err
	}

	a := ServerAssignment{
		Result:          result,
		PrivateIdentity: answerString(ans, diameter.AVPUserName, 0),
		ServerName:      answerString(ans, AVPServerName, tgpp.VendorID),
		Features:        features(ans),
	}

	if data, ok := ans.Find(AVPUserData, tgpp.VendorID); ok {
		a.UserData = data.Data
	}

	if c, ok := ans.Find(AVPChargingInformation, tgpp.VendorID); ok {
		if a.Charging, err = parseCharging(c); err != nil {
			return ServerAssignment{}, err
		}
	}

	if ids, ok := ans.Find(AVPAssociatedIdentities, tgpp.VendorID); ok {
		if a.AssociatedIdentities, err = parseIdentityList(ids); err != nil {
			return ServerAssignment{}, malformed("Associated-Identities: %v", err)
		}
	}

	looseRoute, err := answerUnsigned(ans, AVPLooseRouteIndication, "Loose-Route-Indication")
	if err != nil {
		return ServerAssignment{}, err
	}

	priviledged, err := answerUnsigned(ans, AVPPriviledgedSenderIndication, "Priviledged-Sender-Indication")
	if err != nil {
		return ServerAssignment{}, err
	}

	a.LooseRouteRequired = looseRoute == looseRouteRequired
	a.PriviledgedSender = priviledged == priviledgedSender

	return a, nil
}

func (c *ChargingInformation) fields() []struct {
	code uint32
	dst  *string
} {
	return []struct {
		code uint32
		dst  *string
	}{
		{AVPPrimaryEventChargingFunctionName, &c.PrimaryEventChargingFunction},
		{AVPSecondaryEventChargingFunctionName, &c.SecondaryEventChargingFunction},
		{AVPPrimaryChargingCollectionFunctionName, &c.PrimaryChargingCollectionFunction},
		{AVPSecondaryChargingCollectionFunctionName, &c.SecondaryChargingCollectionFunction},
	}
}

func chargingAVP(c ChargingInformation) (diameter.AVP, error) {
	if c.PrimaryEventChargingFunction == "" && c.PrimaryChargingCollectionFunction == "" {
		return diameter.AVP{}, invalid("charging information without a primary charging function")
	}

	var inner []diameter.AVP

	for _, f := range c.fields() {
		if *f.dst != "" {
			inner = append(inner, vendorString(f.code, *f.dst))
		}
	}

	return diameter.Grouped(AVPChargingInformation, diameter.AVPFlagMandatory, tgpp.VendorID, inner...), nil
}

func parseCharging(a diameter.AVP) (*ChargingInformation, error) {
	inner, err := a.Grouped()
	if err != nil {
		return nil, malformed("Charging-Information: %v", err)
	}

	var c ChargingInformation

	for _, f := range c.fields() {
		if v, ok := diameter.Find(inner, f.code, tgpp.VendorID); ok {
			*f.dst = v.UTF8String()
		}
	}

	return &c, nil
}
