// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type CancelLocationRequest struct {
	IMSI     string
	Features Features
	Type     CancellationType
	Flags    CLRFlags
}

var clrRules = requestRules(true, userNameRule.With(diameter.Rules{
	vendorKey(AVPCancellationType): {Required: true, MinLength: 4},
	vendorKey(AVPCLRFlags):         {},
}))

func NewCancelLocationRequest(env tgpp.Envelope, r CancelLocationRequest) (*diameter.Message, error) {
	userName, err := userNameAVP(r.IMSI)
	if err != nil {
		return nil, err
	}

	if !r.Type.valid() {
		return nil, invalidf("Cancellation-Type %d", r.Type)
	}

	avps := append([]diameter.AVP{userName}, r.Features.avps()...)
	avps = append(avps, unsignedAVP(AVPCancellationType, uint32(r.Type)))

	if avps, err = appendFlags(avps, AVPCLRFlags, "CLR-Flags", r.Flags, clrFlagsDefined); err != nil {
		return nil, err
	}

	return newRequest(env, CommandCancelLocation, true, avps...)
}

func CheckCancelLocation(req *diameter.Message) error {
	return clrRules.Check(req)
}

func ParseCancelLocationRequest(req *diameter.Message) (CancelLocationRequest, error) {
	if err := CheckCancelLocation(req); err != nil {
		return CancelLocationRequest{}, err
	}

	var (
		r   CancelLocationRequest
		err error
	)

	if r.IMSI, err = requestIMSI(req); err != nil {
		return CancelLocationRequest{}, err
	}

	r.Features = featuresOf(req)

	if r.Type, err = requiredEnum[CancellationType](req.AVPs, AVPCancellationType); err != nil {
		return CancelLocationRequest{}, err
	}

	if r.Flags, err = optionalFlags(req.AVPs, AVPCLRFlags, clrFlagsDefined); err != nil {
		return CancelLocationRequest{}, err
	}

	return r, nil
}

func NewCancelLocationAnswer(req *diameter.Message, id diameter.Identity, features Features) *diameter.Message {
	return newSuccessAnswer(req, id, features)
}

func ParseCancelLocationAnswer(ans *diameter.Message) (Features, error) {
	return parseAnswer(ans)
}
