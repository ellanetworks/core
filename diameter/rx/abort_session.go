// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type AbortSessionRequest struct {
	Cause AbortCause
}

type AbortSessionAnswer struct {
	Result tgpp.Result
}

var asrRules = commonRequestRules.With(diameter.Rules{
	{Code: diameter.AVPDestinationHost}: {Required: true},
	vendorKey(AVPAbortCause):            {Required: true, MinLength: 4},
})

func NewAbortSessionRequest(env tgpp.Envelope, r AbortSessionRequest) (*diameter.Message, error) {
	switch {
	case env.DestinationHost == "":
		return nil, invalid("abort session without a destination host")
	case r.Cause > maxAbortCause:
		return nil, invalid("Abort-Cause %d", uint32(r.Cause))
	}

	return newRequest(env, CommandAbortSession, vendorUnsigned(AVPAbortCause, uint32(r.Cause))), nil
}

func CheckAbortSession(req *diameter.Message) error {
	return checkRequest(asrRules, req)
}

func ParseAbortSessionRequest(req *diameter.Message) (AbortSessionRequest, error) {
	if err := CheckAbortSession(req); err != nil {
		return AbortSessionRequest{}, err
	}

	cause, _, err := enum(req.AVPs, AVPAbortCause, tgpp.VendorID, upTo(maxAbortCause))
	if err != nil {
		return AbortSessionRequest{}, err
	}

	return AbortSessionRequest{Cause: cause}, nil
}

func NewAbortSessionAnswer(req *diameter.Message, id diameter.Identity, a AbortSessionAnswer) (*diameter.Message, error) {
	return successAnswer(req, id, a.Result)
}

func ParseAbortSessionAnswer(ans *diameter.Message) (AbortSessionAnswer, error) {
	result, err := parseResult(ans)
	if err != nil {
		return AbortSessionAnswer{}, err
	}

	return AbortSessionAnswer{Result: result}, nil
}
