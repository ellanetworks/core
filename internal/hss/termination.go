// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"errors"
	"maps"
	"slices"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/tracing"
	"go.uber.org/zap"
)

const terminationTimeout = 10 * time.Second

var ErrDiameterUnavailable = errors.New("the Diameter node is not running")

type Peers interface {
	Node() *diameter.Node
}

type Termination struct {
	IMSI             string
	PrivateIdentity  string
	PublicIdentities []string
	ServerName       string
	ServerHost       string
	ServerRealm      string
	Reason           cx.ReasonCode
	sets             []RegistrationSet
	expected         Registration
}

func (h *HSS) Termination(ctx context.Context, imsi string) (*Termination, error) {
	reg, err := h.store.Registration(ctx, imsi)
	if err != nil || !reg.registered() {
		return nil, err
	}

	return termination(imsi, reg, cx.ReasonPermanentTermination, nil), nil
}

func termination(imsi string, reg *Registration, reason cx.ReasonCode, sets []RegistrationSet) *Termination {
	t := &Termination{
		IMSI:            imsi,
		PrivateIdentity: reg.PrivateIdentity,
		ServerName:      reg.ServerName,
		ServerHost:      reg.OriginHost,
		ServerRealm:     reg.OriginRealm,
		Reason:          reason,
		sets:            sets,
		expected:        *reg.clone(),
	}

	for _, set := range sets {
		s, ok := reg.Sets[set]

		switch {
		case !ok:
		case s.DefaultIdentity != "":
			t.PublicIdentities = append(t.PublicIdentities, s.DefaultIdentity)
		case set == MMTel && reg.PrivateIdentity != "":
			t.PublicIdentities = append(t.PublicIdentities, "sip:"+reg.PrivateIdentity)
		}
	}

	return t
}

func (h *HSS) Terminate(ctx context.Context, t *Termination) (err error) {
	ctx, span := tracer.Start(ctx, "hss/registration-termination")
	defer func() { tracing.End(span, err) }()

	if t.Reason == cx.ReasonServerChange {
		if err := h.clearRegistration(ctx, t); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, terminationTimeout)
	defer cancel()

	emergency, err := h.terminate(ctx, t)
	if err != nil {
		h.log.Warn("Cx registration termination failed", zap.String("imsi", t.IMSI), zap.String("serverHost", t.ServerHost), zap.Error(err))

		return err
	}

	h.log.Info("IMS registration terminated", zap.String("imsi", t.IMSI), zap.Strings("publicIdentities", t.PublicIdentities),
		zap.String("serverHost", t.ServerHost), zap.Strings("emergencyRegistered", emergency))

	if t.Reason == cx.ReasonPermanentTermination {
		return h.permanentlyTerminated(ctx, t, emergency)
	}

	return nil
}

func (h *HSS) permanentlyTerminated(ctx context.Context, t *Termination, emergency []string) error {
	return h.updateRegistration(ctx, t.IMSI, func(current *Registration) (*Registration, error) {
		if !t.holds(current) {
			return current, nil
		}

		affected := t.sets
		if len(affected) == 0 {
			affected = slices.Collect(maps.Keys(current.Sets))
		}

		next := current.clone()

		for _, set := range affected {
			s := next.Sets[set]

			if s.State != NotRegistered && slices.ContainsFunc(emergency, func(identity string) bool { return identitySet(identity) == set }) {
				s.State, s.Emergency = Unregistered, true
				next.Sets[set] = s

				continue
			}

			delete(next.Sets, set)
		}

		if len(next.Sets) == 0 {
			return nil, nil
		}

		return next, nil
	})
}

func (h *HSS) terminate(ctx context.Context, t *Termination) ([]string, error) {
	var node *diameter.Node
	if h.peers != nil {
		node = h.peers.Node()
	}

	if node == nil {
		return nil, ErrDiameterUnavailable
	}

	req, err := cx.NewRegistrationTerminationRequest(tgpp.Envelope{
		SessionID:        node.NewSessionID(),
		Origin:           node.Identity(),
		DestinationHost:  t.ServerHost,
		DestinationRealm: t.ServerRealm,
	}, cx.RegistrationTerminationRequest{
		PrivateIdentity:  t.PrivateIdentity,
		PublicIdentities: t.PublicIdentities,
		Reason:           cx.DeregistrationReason{Code: t.Reason},
	})
	if err != nil {
		return nil, err
	}

	ans, err := node.Send(ctx, req, diameter.FailFast())
	if err != nil {
		return nil, err
	}

	a, err := cx.ParseRegistrationTerminationAnswer(ans)

	var re *cx.RegistrationTerminationError
	if errors.As(err, &re) && re.Code == diameter.ResultUnableToComply && len(re.EmergencyIdentities) > 0 {
		return emergencyIdentities(re.EmergencyIdentities), nil
	}

	if err != nil {
		return nil, err
	}

	return emergencyIdentities(a.EmergencyIdentities), nil
}

func emergencyIdentities(pairs []cx.EmergencyIdentity) []string {
	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, p.PublicIdentity)
	}

	return out
}

func (t *Termination) holds(current *Registration) bool {
	if current == nil || !sameAssignment(current, &t.expected) {
		return false
	}

	if len(t.sets) == 0 {
		return true
	}

	for _, set := range t.sets {
		c, ok := current.Sets[set]
		e, expected := t.expected.Sets[set]

		if ok != expected || c != e {
			return false
		}
	}

	return true
}

func (h *HSS) clearRegistration(ctx context.Context, t *Termination) error {
	return h.updateRegistration(ctx, t.IMSI, func(current *Registration) (*Registration, error) {
		if !t.holds(current) {
			return current, nil
		}

		return current.without(t.sets), nil
	})
}
