// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type PurgeUERequest struct {
	IMSI     string
	Features Features
	Flags    PURFlags
	Location *EPSLocationInformation
}

type PurgeUE struct {
	Features Features
	Flags    PUAFlags
}

var purRules = requestRules(false, userNameRule.With(diameter.Rules{
	vendorKey(AVPPURFlags):               {},
	vendorKey(AVPEPSLocationInformation): {},
}))

func NewPurgeUERequest(env tgpp.Envelope, r PurgeUERequest) (*diameter.Message, error) {
	userName, err := userNameAVP(r.IMSI)
	if err != nil {
		return nil, err
	}

	avps, err := appendFlags([]diameter.AVP{userName}, AVPPURFlags, "PUR-Flags", r.Flags)
	if err != nil {
		return nil, err
	}

	if r.Location != nil {
		loc, err := r.Location.avp()
		if err != nil {
			return nil, err
		}

		avps = append(avps, loc)
	}

	return newRequest(env, CommandPurgeUE, false, append(avps, r.Features.avps()...)...)
}

func CheckPurgeUE(req *diameter.Message) error {
	return purRules.Check(req)
}

func ParsePurgeUERequest(req *diameter.Message) (PurgeUERequest, error) {
	if err := CheckPurgeUE(req); err != nil {
		return PurgeUERequest{}, err
	}

	var (
		r   PurgeUERequest
		err error
	)

	if r.IMSI, err = requestIMSI(req); err != nil {
		return PurgeUERequest{}, err
	}

	r.Features = featuresOf(req)

	if r.Flags, err = requestFlags(req.AVPs, AVPPURFlags, purFlagsDefined); err != nil {
		return PurgeUERequest{}, err
	}

	if loc, ok := req.Find(AVPEPSLocationInformation, tgpp.VendorID); ok {
		if r.Location, err = requestDecoder.epsLocationInformation(loc); err != nil {
			return PurgeUERequest{}, err
		}
	}

	return r, nil
}

func NewPurgeUEAnswer(req *diameter.Message, id diameter.Identity, p PurgeUE) (*diameter.Message, error) {
	ans := newSuccessAnswer(req, id, p.Features)

	var err error

	if ans.AVPs, err = appendFlags(ans.AVPs, AVPPUAFlags, "PUA-Flags", p.Flags); err != nil {
		return nil, err
	}

	return ans, nil
}

func ParsePurgeUEAnswer(ans *diameter.Message) (PurgeUE, error) {
	features, err := parseAnswer(ans)
	if err != nil {
		return PurgeUE{}, err
	}

	p := PurgeUE{Features: features}

	if p.Flags, err = answerFlags[PUAFlags](ans.AVPs, AVPPUAFlags); err != nil {
		return PurgeUE{}, malformed(err)
	}

	return p, nil
}
