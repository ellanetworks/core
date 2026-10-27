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

	avps, err := appendFlags([]diameter.AVP{userName}, AVPPURFlags, "PUR-Flags", r.Flags, purFlagsDefined)
	if err != nil {
		return nil, err
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

	if r.Flags, err = optionalFlags(req.AVPs, AVPPURFlags, purFlagsDefined); err != nil {
		return PurgeUERequest{}, err
	}

	return r, nil
}

func NewPurgeUEAnswer(req *diameter.Message, id diameter.Identity, p PurgeUE) (*diameter.Message, error) {
	ans := newSuccessAnswer(req, id, p.Features)

	var err error

	if ans.AVPs, err = appendFlags(ans.AVPs, AVPPUAFlags, "PUA-Flags", p.Flags, puaFlagsDefined); err != nil {
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

	if p.Flags, err = optionalFlags(ans.AVPs, AVPPUAFlags, puaFlagsDefined); err != nil {
		return PurgeUE{}, malformed(err)
	}

	return p, nil
}
