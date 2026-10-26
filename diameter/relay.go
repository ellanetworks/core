// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"errors"
	"slices"
	"strings"
)

var ErrLoopDetected = errors.New("diameter: forwarding loop detected")

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
