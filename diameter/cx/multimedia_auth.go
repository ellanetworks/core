// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
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
	Scheme          string
	Resync          *Resync
	Features        uint32
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
	Scheme     string
	AKA        *AKAVector
}

type MultimediaAuth struct {
	Result          tgpp.Result
	PrivateIdentity string
	PublicIdentity  string
	Items           []AuthItem
	Features        uint32
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
	case r.NumberOfItems == 0:
		return nil, invalid("authentication request for no vectors")
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

	item := []diameter.AVP{vendorString(AVPSIPAuthenticationScheme, r.Scheme)}

	if r.Resync != nil {
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
	if !ok || scheme.UTF8String() == "" {
		return MultimediaAuthRequest{}, tgpp.InvalidAVP(itemAVP)
	}

	user, _ := req.Find(diameter.AVPUserName, 0)

	r := MultimediaAuthRequest{
		PrivateIdentity: user.UTF8String(),
		PublicIdentity:  public,
		ServerName:      server,
		NumberOfItems:   n,
		Scheme:          scheme.UTF8String(),
		Features:        features(req),
	}

	if a, ok := diameter.Find(item, AVPSIPAuthorization, tgpp.VendorID); ok {
		if len(a.Data) != randLen+autsLen {
			return MultimediaAuthRequest{}, tgpp.InvalidAVP(itemAVP)
		}

		r.Resync = &Resync{RAND: a.Data[:randLen], AUTS: a.Data[randLen:]}
	}

	return r, nil
}

func NewMultimediaAuthAnswer(req *diameter.Message, id diameter.Identity, a MultimediaAuth) (*diameter.Message, error) {
	if err := successResult(a.Result); err != nil {
		return nil, err
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

	ans := NewAnswer(req, id, a.Result, a.Features)
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
	if item.Scheme == "" {
		return diameter.AVP{}, invalid("authentication vector without a scheme")
	}

	var inner []diameter.AVP

	if item.ItemNumber != 0 {
		inner = append(inner, vendorUnsigned(AVPSIPItemNumber, item.ItemNumber))
	}

	inner = append(inner, vendorString(AVPSIPAuthenticationScheme, item.Scheme))

	if v := item.AKA; v != nil {
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
	}

	return diameter.Grouped(AVPSIPAuthDataItem, diameter.AVPFlagMandatory, tgpp.VendorID, inner...), nil
}

func ParseMultimediaAuthAnswer(ans *diameter.Message) (MultimediaAuth, error) {
	result, err := parseResult(ans)
	if err != nil {
		return MultimediaAuth{}, err
	}

	a := MultimediaAuth{
		Result:          result,
		PrivateIdentity: answerString(ans, diameter.AVPUserName, 0),
		PublicIdentity:  answerString(ans, AVPPublicIdentity, tgpp.VendorID),
		Features:        features(ans),
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
		return AuthItem{}, malformed("SIP-Auth-Data-Item: %v", err)
	}

	var item AuthItem

	if n, ok := diameter.Find(inner, AVPSIPItemNumber, tgpp.VendorID); ok {
		if item.ItemNumber, err = n.Unsigned32(); err != nil {
			return AuthItem{}, malformed("SIP-Item-Number")
		}
	}

	scheme, _ := diameter.Find(inner, AVPSIPAuthenticationScheme, tgpp.VendorID)
	if item.Scheme = scheme.UTF8String(); item.Scheme == "" {
		return AuthItem{}, malformed("SIP-Auth-Data-Item without a scheme")
	}

	authenticate, ok := diameter.Find(inner, AVPSIPAuthenticate, tgpp.VendorID)
	if !ok {
		return item, nil
	}

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
		RAND: authenticate.Data[:randLen],
		AUTN: authenticate.Data[randLen:],
		XRES: xres.Data,
		CK:   ck.Data,
		IK:   ik.Data,
	}

	return item, nil
}
