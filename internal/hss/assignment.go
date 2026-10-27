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

	af := tgpp.ParseEnvelope(req).Origin

	var p *profile

	if carriesProfile(r.Type) {
		if p, err = h.profileOf(ctx, sub, sub.requestedSet()); err != nil {
			return h.errorAnswer(req, id, "server assignment", err)
		}
	}

	var outcome tgpp.Result

	err = h.updateRegistration(ctx, sub.IMSI, func(current *Registration) (*Registration, error) {
		next, result, err := assign(current, r, sub.requested, af.OriginHost, af.OriginRealm, p)
		outcome = result

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

	if outcome.Failure() {
		return cx.NewAnswer(req, id, outcome, 0)
	}

	a := cx.ServerAssignment{Result: outcome, PrivateIdentity: sub.privateIdentity()}

	if p != nil {
		a.UserData = p.userData
	}

	ans, err := cx.NewServerAssignmentAnswer(req, id, a)
	if err != nil {
		return h.errorAnswer(req, id, "server assignment", err)
	}

	return ans
}

func carriesProfile(t cx.AssignmentType) bool {
	switch t {
	case cx.AssignmentNoAssignment, cx.AssignmentRegistration, cx.AssignmentReRegistration, cx.AssignmentUnregisteredUser:
		return true
	default:
		return false
	}
}

func delivered(s SetRegistration, p *profile) SetRegistration {
	s.DefaultIdentity, s.ProfileDigest = p.defaultIdentity, p.digest

	return s
}

func assignedTo(current *Registration, serverName, host, realm string, p *profile) *Registration {
	next := &Registration{}
	if current != nil {
		next = current.clone()
	}

	next.ServerName, next.OriginHost, next.OriginRealm, next.PrivateIdentity = serverName, host, realm, p.privateIdentity

	return next
}

func assign(current *Registration, r cx.ServerAssignmentRequest, sets []RegistrationSet, host, realm string, p *profile) (*Registration, tgpp.Result, error) {
	success := tgpp.Result{Code: diameter.ResultSuccess}

	if r.Type != cx.AssignmentNoAssignment && assignedElsewhere(current, r.ServerName) {
		return nil, tgpp.Result{}, &assignedToAnother{serverName: current.ServerName}
	}

	set := MMTel
	if len(sets) > 0 {
		set = sets[0]
	}

	previous, known := current.set(set)

	switch r.Type {
	case cx.AssignmentNoAssignment:
		if current == nil || current.ServerName != r.ServerName {
			return current, tgpp.Result{Code: diameter.ResultUnableToComply}, nil
		}

		if !known {
			return current, success, nil
		}

		next := current.with(set, delivered(previous, p))
		next.PrivateIdentity = p.privateIdentity

		return next, success, nil
	case cx.AssignmentRegistration, cx.AssignmentReRegistration:
		return assignedTo(current, r.ServerName, host, realm, p).with(set, delivered(SetRegistration{State: Registered}, p)), success, nil
	case cx.AssignmentUnregisteredUser:
		next := SetRegistration{State: Unregistered, AuthPending: previous.AuthPending, Emergency: previous.Emergency}

		return assignedTo(current, r.ServerName, host, realm, p).with(set, delivered(next, p)), success, nil
	case cx.AssignmentTimeoutDeregistration, cx.AssignmentUserDeregistration,
		cx.AssignmentAdministrativeDeregistration, cx.AssignmentDeregistrationTooMuchData:
		return current.without(sets), success, nil
	case cx.AssignmentTimeoutDeregistrationStoreServer, cx.AssignmentUserDeregistrationStoreServer:
		next := current.without(sets)
		if next == nil {
			return nil, tgpp.Experimental(tgpp.ResultSuccessServerNameNotStored), nil
		}

		return next, success, nil
	case cx.AssignmentAuthenticationFailure, cx.AssignmentAuthenticationTimeout:
		if !known || previous.State == NotRegistered {
			return current.without([]RegistrationSet{set}), success, nil
		}

		previous.AuthPending = false

		return current.with(set, previous), success, nil
	default:
		return current, tgpp.Experimental(tgpp.ResultErrorInAssignmentType), nil
	}
}
