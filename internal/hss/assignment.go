// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
)

type assignedToAnother struct {
	serverName string
}

func (e *assignedToAnother) Error() string {
	return "subscriber assigned to " + e.serverName
}

type assignmentOutcome struct {
	result   tgpp.Result
	userData bool
}

func (h *HSS) ServerAssignment(ctx context.Context, id diameter.Identity, req *diameter.Message) *diameter.Message {
	r, err := cx.ParseServerAssignmentRequest(req)
	if err != nil {
		return cx.NewErrorAnswer(req, id, err, 0)
	}

	if cx.RequiresUnsupportedFeatures(req, 0) {
		return experimentalAnswer(req, id, tgpp.ResultErrorFeatureUnsupported)
	}

	if r.WildcardedPublicIdentity != "" {
		return experimentalAnswer(req, id, tgpp.ResultErrorUserUnknown)
	}

	sub, err := h.resolve(ctx, r.PrivateIdentity, r.PublicIdentities...)
	if err != nil {
		return h.errorAnswer(req, id, "server assignment", err)
	}

	host, realm := origin(req)

	var outcome assignmentOutcome

	err = h.updateRegistration(ctx, sub.IMSI, func(current *Registration) (*Registration, error) {
		next, o, err := assign(current, r, host, realm)
		outcome = o

		return next, err
	})

	var other *assignedToAnother

	switch {
	case errors.As(err, &other):
		ans, err := cx.NewServerAssignmentErrorAnswer(req, id, cx.ServerAssignmentError{
			ResultError:     cx.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorIdentityAlreadyRegistered)},
			PrivateIdentity: sub.privateIdentity(),
			ServerName:      other.serverName,
		})
		if err != nil {
			return h.errorAnswer(req, id, "server assignment", err)
		}

		return ans
	case err != nil:
		return h.errorAnswer(req, id, "server assignment", err)
	}

	if outcome.result.Failure() {
		return cx.NewAnswer(req, id, outcome.result, 0)
	}

	a := cx.ServerAssignment{Result: outcome.result, PrivateIdentity: sub.privateIdentity()}

	if outcome.userData && !r.UserDataAlreadyAvailable {
		if a.UserData, err = sub.userData(); err != nil {
			return h.errorAnswer(req, id, "server assignment", err)
		}
	}

	ans, err := cx.NewServerAssignmentAnswer(req, id, a)
	if err != nil {
		return h.errorAnswer(req, id, "server assignment", err)
	}

	return ans
}

func assign(current *Registration, r cx.ServerAssignmentRequest, host, realm string) (*Registration, assignmentOutcome, error) {
	success := tgpp.Result{Code: diameter.ResultSuccess}

	if r.Type != cx.AssignmentNoAssignment && assignedElsewhere(current, r.ServerName) {
		return nil, assignmentOutcome{}, &assignedToAnother{serverName: current.ServerName}
	}

	switch r.Type {
	case cx.AssignmentNoAssignment:
		if current == nil || current.ServerName != r.ServerName {
			return current, assignmentOutcome{result: tgpp.Result{Code: diameter.ResultUnableToComply}}, nil
		}

		return current, assignmentOutcome{result: success, userData: true}, nil
	case cx.AssignmentRegistration, cx.AssignmentReRegistration:
		return &Registration{State: Registered, ServerName: r.ServerName, OriginHost: host, OriginRealm: realm},
			assignmentOutcome{result: success, userData: true}, nil
	case cx.AssignmentUnregisteredUser:
		next := Registration{State: Unregistered, ServerName: r.ServerName, OriginHost: host, OriginRealm: realm}
		if current != nil {
			next.AuthPending = current.AuthPending
		}

		return &next, assignmentOutcome{result: success, userData: true}, nil
	case cx.AssignmentTimeoutDeregistration, cx.AssignmentUserDeregistration,
		cx.AssignmentAdministrativeDeregistration, cx.AssignmentDeregistrationTooMuchData:
		return nil, assignmentOutcome{result: success}, nil
	case cx.AssignmentTimeoutDeregistrationStoreServer, cx.AssignmentUserDeregistrationStoreServer:
		return nil, assignmentOutcome{result: tgpp.Experimental(tgpp.ResultSuccessServerNameNotStored)}, nil
	case cx.AssignmentAuthenticationFailure, cx.AssignmentAuthenticationTimeout:
		if current == nil || current.State == NotRegistered {
			return nil, assignmentOutcome{result: success}, nil
		}

		next := *current
		next.AuthPending = false

		return &next, assignmentOutcome{result: success}, nil
	default:
		return current, assignmentOutcome{result: tgpp.Experimental(tgpp.ResultErrorInAssignmentType)}, nil
	}
}
