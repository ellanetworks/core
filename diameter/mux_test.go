// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"context"
	"testing"
)

func TestMuxDispatchesOnApplicationAndCommand(t *testing.T) {
	var called bool

	m := NewMux()
	m.Handle(16777313, 8388645, HandlerFunc(func(_ context.Context, c *Conn, req *Message) *Message {
		called = true
		return c.Answer(req, ResultSuccess)
	}))

	c := &Conn{n: &Node{cfg: Config{Identity: Identity{OriginHost: "h", OriginRealm: "r"}}}}

	ans := m.ServeDiameter(context.Background(), c, &Message{CommandCode: 8388645, ApplicationID: 16777313})
	if !called || resultCode(t, ans) != ResultSuccess {
		t.Fatalf("registered pair: called=%v answer=%+v", called, ans)
	}

	called = false

	ans = m.ServeDiameter(context.Background(), c, &Message{CommandCode: 8388645, ApplicationID: 16777312})
	if called || ans.Flags&FlagError == 0 {
		t.Fatalf("same command under another application must not reach the handler: %+v", ans)
	}

	if resultCode(t, ans) != ResultCommandUnsupported {
		t.Fatalf("Result-Code = %d", resultCode(t, ans))
	}
}
