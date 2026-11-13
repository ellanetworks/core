// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux && !386

package gnb_test

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/sctp"
)

func TestUEContextReleaseClearsSelectedPLMN(t *testing.T) {
	skipIfNoSCTP(t)

	const ranUEID = 3

	acceptedCh := make(chan *sctp.SCTPConn, 1)

	srv := sctp.NewServer(sctp.Config{PPID: 60, Name: "TEST"}, sctp.Callbacks{
		Dispatch: func(_ context.Context, conn *sctp.SCTPConn, _ []byte) {
			select {
			case acceptedCh <- conn:
			default:
			}
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	ln, err := new(sctp.ListenConfig).Listen(ctx, &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.IPv4(127, 0, 0, 1)}}})
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}

	srv.Serve(ctx, ln)

	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		srv.Shutdown(shutdownCtx)
	})

	loopback := &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}}

	gnbConn, err := sctp.Dial(ctx, "sctp", loopback, &sctp.SCTPAddr{IPAddrs: loopback.IPAddrs, Port: ln.Addr().(*sctp.SCTPAddr).Port}, sctp.InitMsg{NumOstreams: 2, MaxInstreams: 2})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	g := gnb.NewGnodeB("000102", "001", "01", 1, "", "internet", "000001", "test", gnbConn, nil, netip.Addr{})
	t.Cleanup(g.Close)

	g.AdditionalPLMNs = []gnb.BroadcastPLMN{{Mcc: "310", Mnc: "260"}}
	g.AddUE(ranUEID, &recordingDownlinkSender{})

	if err := g.SelectPLMN(ranUEID, "310", "260"); err != nil {
		t.Fatalf("SelectPLMN: %v", err)
	}

	if _, err := gnbConn.WriteMsg(marshalNGSetupRequest(t), &sctp.SndRcvInfo{PPID: sctp.PPIDWireOrder(60), Stream: 0}); err != nil {
		t.Fatalf("NG Setup trigger write: %v", err)
	}

	var amfConn *sctp.SCTPConn
	select {
	case amfConn = <-acceptedCh:
	case <-time.After(8 * time.Second):
		t.Fatal("server never dispatched the association")
	}

	cmd := &ngap.UEContextReleaseCommand{
		UENGAPIDs: ngap.UENGAPIDs{AMFUENGAPID: 1, RANUENGAPID: ranUEID, Pair: true},
		Cause:     &ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASNormalRelease},
	}

	pdu, err := cmd.Marshal()
	if err != nil {
		t.Fatalf("marshal UEContextReleaseCommand: %v", err)
	}

	if _, err := amfConn.WriteMsg(pdu, &sctp.SndRcvInfo{PPID: sctp.PPIDWireOrder(60), Stream: 1}); err != nil {
		t.Fatalf("write UEContextReleaseCommand: %v", err)
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if mcc, mnc := g.SelectedPLMN(ranUEID); mcc == "001" && mnc == "01" {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	mcc, mnc := g.SelectedPLMN(ranUEID)
	t.Fatalf("SelectedPLMN after release = %s/%s, want the serving 001/01", mcc, mnc)
}
