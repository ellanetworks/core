// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode

import (
	"context"

	"github.com/ellanetworks/core/diameter"
)

func Handler(h func(context.Context, diameter.Identity, *diameter.Message) *diameter.Message) diameter.Handler {
	return HandlerFrom(func(ctx context.Context, id diameter.Identity, _ string, req *diameter.Message) *diameter.Message {
		return h(ctx, id, req)
	})
}

func HandlerFrom(h func(context.Context, diameter.Identity, string, *diameter.Message) *diameter.Message) diameter.Handler {
	return diameter.HandlerFunc(func(ctx context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		return h(ctx, c.LocalIdentity(), c.PeerHost(), req)
	})
}
