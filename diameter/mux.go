// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import "context"

type muxKey struct {
	applicationID uint32
	commandCode   uint32
}

type Mux struct {
	handlers map[muxKey]Handler
}

func NewMux() *Mux {
	return &Mux{handlers: make(map[muxKey]Handler)}
}

func (m *Mux) Handle(applicationID, commandCode uint32, h Handler) {
	m.handlers[muxKey{applicationID, commandCode}] = h
}

func (m *Mux) ServeDiameter(ctx context.Context, c *Conn, req *Message) *Message {
	h, ok := m.handlers[muxKey{req.ApplicationID, req.CommandCode}]
	if !ok {
		return c.Answer(req, ResultCommandUnsupported)
	}

	return h.ServeDiameter(ctx, c, req)
}
