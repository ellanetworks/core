// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"context"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func NewStubHandler() diameter.Handler {
	mux := diameter.NewMux()

	unableToComply := diameter.HandlerFunc(func(_ context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		return tgpp.NewAnswer(req, c.LocalIdentity(), diameter.ResultUnableToComply)
	})

	mux.Handle(s6c.ApplicationID, s6c.CommandSendRoutingInfoForSM, unableToComply)
	mux.Handle(s6c.ApplicationID, s6c.CommandReportSMDeliveryStatus, unableToComply)
	mux.Handle(sgd.ApplicationID, sgd.CommandMTForwardShortMessage, unableToComply)

	return mux
}
