// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"slices"
	"strings"
)

// ErrLoopDetected is of a request that passed the node already: its identity is in a Route-Record of the request
// (RFC 6733 §6.1.3). The handler answers it DIAMETER_LOOP_DETECTED.
var ErrLoopDetected = errors.New("diameter: forwarding loop detected")

// Relay forwards req, a request the node received from the peer of identity from, to the peer of host in realm, as a
// proxy agent (RFC 6733 §6.1.9): with from appended in a Route-Record, its Destination-Host and
// Destination-Realm those of host and realm, its End-to-End Identifier, Origin-Host and other AVPs as received. It
// returns the answer with the Hop-by-Hop and End-to-End Identifiers of req, for the handler to return as it is
// (§6.2.2). A request whose Route-Records have the identity of the node gets ErrLoopDetected.
func (n *Node) Relay(ctx context.Context, req *Message, from, host, realm string, opts ...RequestOption) (*Message, error) {
	own := n.Identity().OriginHost

	for _, rr := range FindAll(req.AVPs, AVPRouteRecord, 0) {
		if strings.EqualFold(rr.UTF8String(), own) {
			return nil, ErrLoopDetected
		}
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, n.cfg.RequestTimeout)
		defer cancel()
	}

	m := *req
	m.Flags = (m.Flags | FlagRequest) &^ FlagError
	m.AVPs = withDestinationHost(slices.DeleteFunc(slices.Clone(req.AVPs), func(a AVP) bool {
		return a.VendorID == 0 && a.Code == AVPDestinationRealm
	}), host)
	m.AVPs = append(m.AVPs,
		UTF8String(AVPDestinationRealm, AVPFlagMandatory, 0, realm),
		UTF8String(AVPRouteRecord, AVPFlagMandatory, 0, from),
	)

	s := &sending{
		n:     n,
		o:     applyOptions(opts),
		m:     m,
		host:  strings.ToLower(host),
		realm: strings.ToLower(realm),
		tried: make(map[*peer]bool),
	}

	defer s.release()

	ans, err := s.run(ctx)
	if err != nil {
		return nil, err
	}

	out := *ans
	out.HopByHopID, out.EndToEndID = req.HopByHopID, req.EndToEndID

	return &out, nil
}
