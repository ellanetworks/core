// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"bytes"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	randLen    = 16
	autnLen    = 16
	autsLen    = 14
	keyLen     = 16
	minXRESLen = 4
	maxXRESLen = 16
)

type Resync struct {
	RAND []byte
	AUTS []byte
}

type MultimediaAuthRequest struct {
	PrivateIdentity string
	PublicIdentity  string
	ServerName      string
	NumberOfItems   uint32
	Scheme          AuthenticationScheme
	Resync          *Resync
	Features        Features
}

type AKAVector struct {
	RAND []byte
	AUTN []byte
	XRES []byte
	CK   []byte
	IK   []byte
}

type AuthItem struct {
	ItemNumber uint32
	Scheme     AuthenticationScheme
	AKA        *AKAVector
}

type MultimediaAuth struct {
	Result          tgpp.Result
	PrivateIdentity string
	PublicIdentity  string
	Items           []AuthItem
	Features        Features
}

var marRules = commonRequestRules.With(diameter.Rules{
	{Code: diameter.AVPUserName}:                           {Required: true},
	{Code: AVPPublicIdentity, VendorID: tgpp.VendorID}:     {Required: true},
	{Code: AVPSIPAuthDataItem, VendorID: tgpp.VendorID}:    {Required: true},
	{Code: AVPSIPNumberAuthItems, VendorID: tgpp.VendorID}: {Required: true},
	{Code: AVPServerName, VendorID: tgpp.VendorID}:         {Required: true},
})

func NewMultimediaAuthRequest(env tgpp.Envelope, r MultimediaAuthRequest) (*diameter.Message, error) {
	switch {
	case r.PrivateIdentity == "":
		return nil, invalid("authentication request without a private identity")
	case r.Scheme == "":
		return nil, invalid("authentication request without a scheme")
	}

	public, err := publicIdentityAVP(r.PublicIdentity)
	if err != nil {
		return nil, err
	}

	server, err := serverNameAVP(r.ServerName)
	if err != nil {
		return nil, err
	}

	if r.NumberOfItems == 0 {
		r.NumberOfItems = 1
	}

	item := []diameter.AVP{vendorString(AVPSIPAuthenticationScheme, string(r.Scheme))}

	if r.Resync != nil {
		if !r.Scheme.IsAKA() {
			return nil, invalid("resynchronisation with the non-AKA scheme %q", r.Scheme)
		}

		if len(r.Resync.RAND) != randLen || len(r.Resync.AUTS) != autsLen {
			return nil, invalid("resynchronisation with a %d-octet RAND and a %d-octet AUTS", len(r.Resync.RAND), len(r.Resync.AUTS))
		}

		item = append(item, diameter.OctetString(AVPSIPAuthorization, diameter.AVPFlagMandatory, tgpp.VendorID,
			append(append([]byte(nil), r.Resync.RAND...), r.Resync.AUTS...)))
	}

	return newRequest(env, CommandMultimediaAuth, r.Features,
		userName(r.PrivateIdentity),
		public,
		vendorUnsigned(AVPSIPNumberAuthItems, r.NumberOfItems),
		diameter.Grouped(AVPSIPAuthDataItem, diameter.AVPFlagMandatory, tgpp.VendorID, item...),
		server,
	), nil
}

func CheckMultimediaAuth(req *diameter.Message) error {
	return marRules.Check(req)
}

func ParseMultimediaAuthRequest(req *diameter.Message) (MultimediaAuthRequest, error) {
	if err := CheckMultimediaAuth(req); err != nil {
		return MultimediaAuthRequest{}, err
	}

	public, err := requestPublicIdentity(req)
	if err != nil {
		return MultimediaAuthRequest{}, err
	}

	server, err := requestServerName(req)
	if err != nil {
		return MultimediaAuthRequest{}, err
	}

	count, _ := req.Find(AVPSIPNumberAuthItems, tgpp.VendorID)

	n, err := count.Unsigned32()
	if err != nil || n == 0 {
		return MultimediaAuthRequest{}, tgpp.InvalidAVP(count)
	}

	itemAVP, _ := req.Find(AVPSIPAuthDataItem, tgpp.VendorID)

	item, err := itemAVP.Grouped()
	if err != nil {
		return MultimediaAuthRequest{}, tgpp.InvalidAVP(itemAVP)
	}

	scheme, ok := diameter.Find(item, AVPSIPAuthenticationScheme, tgpp.VendorID)
	if !ok {
		return MultimediaAuthRequest{}, tgpp.MissingAVP(AVPSIPAuthenticationScheme, tgpp.VendorID)
	}

	if scheme.UTF8String() == "" {
		return MultimediaAuthRequest{}, tgpp.InvalidAVP(scheme)
	}

	user, _ := req.Find(diameter.AVPUserName, 0)

	r := MultimediaAuthRequest{
		PrivateIdentity: user.UTF8String(),
		PublicIdentity:  public,
		ServerName:      server,
		NumberOfItems:   n,
		Scheme:          AuthenticationScheme(scheme.UTF8String()),
		Features:        featureList(req),
	}

	if a, ok := diameter.Find(item, AVPSIPAuthorization, tgpp.VendorID); ok {
		if !r.Scheme.IsAKA() || len(a.Data) != randLen+autsLen {
			return MultimediaAuthRequest{}, tgpp.InvalidAVP(a)
		}

		r.Resync = &Resync{RAND: bytes.Clone(a.Data[:randLen]), AUTS: bytes.Clone(a.Data[randLen:])}
	}

	return r, nil
}

func NewMultimediaAuthAnswer(req *diameter.Message, id diameter.Identity, a MultimediaAuth) (*diameter.Message, error) {
	result, err := successResult(a.Result)
	if err != nil {
		return nil, err
	}

	if result.Experimental {
		return nil, invalid("authentication answer with the experimental %s", result)
	}

	if requested, err := answerUnsigned(req, AVPSIPNumberAuthItems, "SIP-Number-Auth-Items"); err == nil && requested != 0 && len(a.Items) > int(requested) {
		return nil, invalid("%d vectors for %d requested", len(a.Items), requested)
	}

	if a.PrivateIdentity == "" {
		a.PrivateIdentity = answerString(req, diameter.AVPUserName, 0)
	}

	if a.PublicIdentity == "" {
		a.PublicIdentity = answerString(req, AVPPublicIdentity, tgpp.VendorID)
	}

	switch {
	case a.PrivateIdentity == "":
		return nil, invalid("authentication answer without a private identity")
	case len(a.Items) == 0:
		return nil, invalid("authentication answer without vectors")
	}

	public, err := publicIdentityAVP(a.PublicIdentity)
	if err != nil {
		return nil, err
	}

	ans := NewAnswer(req, id, result, a.Features)
	ans.AVPs = append(ans.AVPs,
		userName(a.PrivateIdentity),
		public,
		vendorUnsigned(AVPSIPNumberAuthItems, uint32(len(a.Items))),
	)

	for _, item := range a.Items {
		avp, err := authItemAVP(item)
		if err != nil {
			return nil, err
		}

		ans.AVPs = append(ans.AVPs, avp)
	}

	return ans, nil
}

func authItemAVP(item AuthItem) (diameter.AVP, error) {
	switch {
	case !item.Scheme.IsAKA():
		return diameter.AVP{}, invalid("authentication vector for the unsupported scheme %q", item.Scheme)
	case item.AKA == nil:
		return diameter.AVP{}, invalid("%s vector without AKA material", item.Scheme)
	}

	var inner []diameter.AVP

	if item.ItemNumber != 0 {
		inner = append(inner, vendorUnsigned(AVPSIPItemNumber, item.ItemNumber))
	}

	inner = append(inner, vendorString(AVPSIPAuthenticationScheme, string(item.Scheme)))

	v := item.AKA

	switch {
	case len(v.RAND) != randLen || len(v.AUTN) != autnLen:
		return diameter.AVP{}, invalid("AKA vector with a %d-octet RAND and a %d-octet AUTN", len(v.RAND), len(v.AUTN))
	case len(v.XRES) < minXRESLen || len(v.XRES) > maxXRESLen:
		return diameter.AVP{}, invalid("AKA vector with a %d-octet XRES", len(v.XRES))
	case len(v.CK) != keyLen || len(v.IK) != keyLen:
		return diameter.AVP{}, invalid("AKA vector with a %d-octet CK and a %d-octet IK", len(v.CK), len(v.IK))
	}

	inner = append(inner,
		diameter.OctetString(AVPSIPAuthenticate, diameter.AVPFlagMandatory, tgpp.VendorID, append(append([]byte(nil), v.RAND...), v.AUTN...)),
		diameter.OctetString(AVPSIPAuthorization, diameter.AVPFlagMandatory, tgpp.VendorID, v.XRES),
		diameter.OctetString(AVPConfidentialityKey, diameter.AVPFlagMandatory, tgpp.VendorID, v.CK),
		diameter.OctetString(AVPIntegrityKey, diameter.AVPFlagMandatory, tgpp.VendorID, v.IK),
	)

	return diameter.Grouped(AVPSIPAuthDataItem, diameter.AVPFlagMandatory, tgpp.VendorID, inner...), nil
}

func ParseMultimediaAuthAnswer(ans *diameter.Message) (MultimediaAuth, error) {
	result, err := parseResult(ans)
	if err != nil {
		return MultimediaAuth{}, err
	}

	if result.Experimental {
		return MultimediaAuth{}, malformed("authentication answer with the experimental %s", result)
	}

	a := MultimediaAuth{
		Result:          result,
		PrivateIdentity: answerString(ans, diameter.AVPUserName, 0),
		PublicIdentity:  answerString(ans, AVPPublicIdentity, tgpp.VendorID),
		Features:        featureList(ans),
	}

	switch {
	case a.PrivateIdentity == "":
		return MultimediaAuth{}, malformed("no User-Name")
	case !ValidPublicIdentity(a.PublicIdentity):
		return MultimediaAuth{}, malformed("Public-Identity %q", a.PublicIdentity)
	}

	for _, avp := range ans.AVPs {
		if avp.Code != AVPSIPAuthDataItem || avp.VendorID != tgpp.VendorID {
			continue
		}

		item, err := parseAuthItem(avp)
		if err != nil {
			return MultimediaAuth{}, err
		}

		a.Items = append(a.Items, item)
	}

	if len(a.Items) == 0 {
		return MultimediaAuth{}, malformed("no SIP-Auth-Data-Item")
	}

	return a, nil
}

func parseAuthItem(avp diameter.AVP) (AuthItem, error) {
	inner, err := avp.Grouped()
	if err != nil {
		return AuthItem{}, malformed("SIP-Auth-Data-Item: %w", err)
	}

	var item AuthItem

	if n, ok := diameter.Find(inner, AVPSIPItemNumber, tgpp.VendorID); ok {
		if item.ItemNumber, err = n.Unsigned32(); err != nil {
			return AuthItem{}, malformed("SIP-Item-Number")
		}
	}

	scheme, _ := diameter.Find(inner, AVPSIPAuthenticationScheme, tgpp.VendorID)
	if item.Scheme = AuthenticationScheme(scheme.UTF8String()); item.Scheme == "" {
		return AuthItem{}, malformed("SIP-Auth-Data-Item without a scheme")
	}

	if !item.Scheme.IsAKA() {
		return item, nil
	}

	authenticate, _ := diameter.Find(inner, AVPSIPAuthenticate, tgpp.VendorID)
	xres, _ := diameter.Find(inner, AVPSIPAuthorization, tgpp.VendorID)
	ck, _ := diameter.Find(inner, AVPConfidentialityKey, tgpp.VendorID)
	ik, _ := diameter.Find(inner, AVPIntegrityKey, tgpp.VendorID)

	switch {
	case len(authenticate.Data) != randLen+autnLen:
		return AuthItem{}, malformed("SIP-Authenticate of %d octets", len(authenticate.Data))
	case len(xres.Data) < minXRESLen || len(xres.Data) > maxXRESLen:
		return AuthItem{}, malformed("XRES of %d octets", len(xres.Data))
	case len(ck.Data) != keyLen || len(ik.Data) != keyLen:
		return AuthItem{}, malformed("CK of %d octets and IK of %d octets", len(ck.Data), len(ik.Data))
	}

	item.AKA = &AKAVector{
		RAND: bytes.Clone(authenticate.Data[:randLen]),
		AUTN: bytes.Clone(authenticate.Data[randLen:]),
		XRES: bytes.Clone(xres.Data),
		CK:   bytes.Clone(ck.Data),
		IK:   bytes.Clone(ik.Data),
	}

	return item, nil
}
