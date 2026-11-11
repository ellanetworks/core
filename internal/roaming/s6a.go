// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/metrics"
)

const s6aRequestTimeout = 5 * time.Second

var (
	ErrHSSUnreachable = errors.New("the home HSS cannot be reached")
	ErrNoVector       = errors.New("the home HSS returned no E-UTRAN vector")
)

var ULRFeatures = s6a.Features{List1: s6a.FeatureODBAllAPN | s6a.FeatureODBHPLMNAPN | s6a.FeatureODBVPLMNAPN}

type DiameterNode interface {
	Send(ctx context.Context, req *diameter.Message, opts ...diameter.RequestOption) (*diameter.Message, error)
	Identity() diameter.Identity
	NewSessionID() string
}

type HPLMNPeer struct {
	Realm string
	HSS   string
}

type S6a struct {
	node    func() DiameterNode
	timeout time.Duration
}

func NewS6a(node func() DiameterNode) *S6a {
	return &S6a{node: node, timeout: s6aRequestTimeout}
}

func (c *S6a) AuthenticationInformation(ctx context.Context, home HPLMNPeer, imsi string, visited tgpp.PLMN, resync *s6a.Resync) (s6a.EUTRANVector, string, error) {
	ans, err := c.send(ctx, home, func(env tgpp.Envelope) (*diameter.Message, error) {
		return s6a.NewAuthenticationInformationRequest(env, s6a.AuthenticationInformationRequest{
			IMSI: imsi,
			EUTRAN: &s6a.RequestedAuthenticationInfo{
				NumberOfVectors:            1,
				ImmediateResponsePreferred: true,
				Resync:                     resync,
			},
			VisitedPLMN: visited,
		})
	})
	if err != nil {
		return s6a.EUTRANVector{}, "", err
	}

	info, err := s6a.ParseAuthenticationInformationAnswer(ans)
	if err != nil {
		return s6a.EUTRANVector{}, "", err
	}

	if len(info.EUTRAN) == 0 {
		return s6a.EUTRANVector{}, "", ErrNoVector
	}

	return info.EUTRAN[0], info.HSS.OriginHost, nil
}

func (c *S6a) UpdateLocation(ctx context.Context, home HPLMNPeer, req s6a.UpdateLocationRequest) (s6a.UpdateLocation, error) {
	ans, err := c.send(ctx, home, func(env tgpp.Envelope) (*diameter.Message, error) {
		return s6a.NewUpdateLocationRequest(env, req)
	})
	if err != nil {
		return s6a.UpdateLocation{}, err
	}

	return s6a.ParseUpdateLocationAnswer(ans)
}

func (c *S6a) PurgeUE(ctx context.Context, home HPLMNPeer, req s6a.PurgeUERequest) (s6a.PurgeUE, error) {
	ans, err := c.send(ctx, home, func(env tgpp.Envelope) (*diameter.Message, error) {
		return s6a.NewPurgeUERequest(env, req)
	})
	if err != nil {
		return s6a.PurgeUE{}, err
	}

	return s6a.ParsePurgeUEAnswer(ans)
}

func (c *S6a) Notify(ctx context.Context, home HPLMNPeer, req s6a.NotifyRequest) error {
	ans, err := c.send(ctx, home, func(env tgpp.Envelope) (*diameter.Message, error) {
		return s6a.NewNotifyRequest(env, req)
	})
	if err != nil {
		return err
	}

	_, err = s6a.ParseNotifyAnswer(ans)

	return err
}

func (c *S6a) send(ctx context.Context, home HPLMNPeer, build func(tgpp.Envelope) (*diameter.Message, error)) (*diameter.Message, error) {
	node := c.node()
	if node == nil {
		return nil, fmt.Errorf("%w: the Diameter node is not running", ErrHSSUnreachable)
	}

	req, err := build(tgpp.Envelope{
		SessionID:        node.NewSessionID(),
		Origin:           node.Identity(),
		DestinationHost:  home.HSS,
		DestinationRealm: home.Realm,
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	start := time.Now()
	ans, err := node.Send(ctx, req)

	metrics.PeerRequest(metrics.PeerS6a, s6aResult(ans, err), time.Since(start))

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrHSSUnreachable, err)
	}

	return ans, nil
}
