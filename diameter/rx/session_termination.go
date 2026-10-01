// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type SessionTerminationRequest struct {
	Cause TerminationCause
}

type SessionTerminationAnswer struct {
	Result tgpp.Result
}

var strRules = commonRequestRules.With(diameter.Rules{
	{Code: diameter.AVPTerminationCause}:  {Required: true, MinLength: 4},
	vendorKey(AVPRequiredAccessInfo):      {Multiple: true},
	{Code: diameter.AVPClass}:             {Multiple: true},
	vendorKey(AVPAFApplicationIdentifier): {},
})

func NewSessionTerminationRequest(env tgpp.Envelope, r SessionTerminationRequest) (*diameter.Message, error) {
	if !validTerminationCause(r.Cause) {
		return nil, invalid("Termination-Cause %d", uint32(r.Cause))
	}

	return newRequest(env, CommandSessionTermination,
		diameter.Unsigned32(diameter.AVPTerminationCause, diameter.AVPFlagMandatory, 0, uint32(r.Cause)),
	), nil
}

func CheckSessionTermination(req *diameter.Message) error {
	return checkRequest(strRules, req)
}

func ParseSessionTerminationRequest(req *diameter.Message) (SessionTerminationRequest, error) {
	if err := CheckSessionTermination(req); err != nil {
		return SessionTerminationRequest{}, err
	}

	cause, _, err := enum(req.AVPs, diameter.AVPTerminationCause, 0, validTerminationCause)
	if err != nil {
		return SessionTerminationRequest{}, err
	}

	return SessionTerminationRequest{Cause: cause}, nil
}

func NewSessionTerminationAnswer(req *diameter.Message, id diameter.Identity, a SessionTerminationAnswer) (*diameter.Message, error) {
	return successAnswer(req, id, a.Result)
}

func ParseSessionTerminationAnswer(ans *diameter.Message) (SessionTerminationAnswer, error) {
	result, err := parseResult(ans)
	if err != nil {
		return SessionTerminationAnswer{}, err
	}

	return SessionTerminationAnswer{Result: result}, nil
}

func validTerminationCause(c TerminationCause) bool {
	return c >= TerminationLogout && c <= maxTerminationCause
}
