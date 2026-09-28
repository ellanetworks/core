// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/sctp"
)

func TestSCTPAcceptsUnspecifiedPPID(t *testing.T) {
	_, p := ellaWithSMSC(t, TransportSCTP, Peer{ID: "smsc", Addresses: []netip.Addr{loopback2}, Applications: []Application{sgdApp}})

	b, err := cer("smsc.example.org", appAVP(sgdApp)).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	sc := p.tr.(*sctpTransport).sc
	if _, err := sc.WriteMsg(b, &sctp.SndRcvInfo{PPID: 0}); err != nil {
		t.Fatal(err)
	}

	if code := resultCode(t, p.recv()); code != ResultSuccess {
		t.Fatalf("Result-Code = %d", code)
	}
}

func TestOutboundTransportReleasedWhenPeerCloses(t *testing.T) {
	for _, kind := range transports {
		t.Run(kind.String(), func(t *testing.T) {
			hss, smsc, addr := pair(t, kind)

			if err := smsc.SetPeers([]Peer{{ID: "hss", Addresses: []netip.Addr{addr.Addr()}, Port: addr.Port(), Transport: kind, Applications: []Application{sgdApp}}}); err != nil {
				t.Fatal(err)
			}

			eventually(t, "the connection", func() bool { return doOK(smsc, "hss") })

			smsc.mu.Lock()
			c := smsc.byID["hss"].open
			smsc.mu.Unlock()

			for _, hc := range hss.allConns() {
				hc.abort("test")
			}

			<-c.done

			if err := c.t.abort(); !errors.Is(err, net.ErrClosed) {
				t.Fatalf("outbound transport still open after the peer closed: %v", err)
			}
		})
	}
}
