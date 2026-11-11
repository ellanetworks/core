// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	ellaIMSPeerName      = "ella-ims"
	probeIMSPeerName     = "integration-probe"
	probeIMSPeerRealm    = "probe.example.org"
	rogueDiameterHost    = "rogue.example.net"
	rogueDiameterRealm   = "example.net"
	diameterProbeTimeout = 15 * time.Second
)

var anySourceAddress = []string{"0.0.0.0/0", "::/0"}

type diameterProbe struct {
	conn  net.Conn
	host  string
	realm string
	next  uint32
}

func dialDiameterProbe(ctx context.Context, host, realm string) (*diameterProbe, uint32, error) {
	d := net.Dialer{Timeout: diameterProbeTimeout}
	address := net.JoinHostPort(N2Address(0), strconv.Itoa(coreDiameterPort))
	deadline := time.Now().Add(time.Minute)

	var (
		conn net.Conn
		err  error
	)

	for {
		conn, err = d.DialContext(ctx, "tcp", address)
		if err == nil {
			break
		}

		if time.Now().After(deadline) {
			return nil, 0, fmt.Errorf("dial the Diameter listener of Ella Core at %s: %w", address, err)
		}

		time.Sleep(time.Second)
	}

	p := &diameterProbe{conn: conn, host: host, realm: realm, next: uint32(time.Now().UnixNano())}

	local, err := netip.ParseAddrPort(conn.LocalAddr().String())
	if err != nil {
		p.close()
		return nil, 0, err
	}

	cer := &diameter.Message{
		Flags:       diameter.FlagRequest,
		CommandCode: diameter.CommandCapabilitiesExchange,
		AVPs: []diameter.AVP{
			diameter.UTF8String(diameter.AVPOriginHost, diameter.AVPFlagMandatory, 0, host),
			diameter.UTF8String(diameter.AVPOriginRealm, diameter.AVPFlagMandatory, 0, realm),
			diameter.Address(diameter.AVPHostIPAddress, diameter.AVPFlagMandatory, 0, local.Addr().Unmap()),
			diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, 0),
			diameter.UTF8String(diameter.AVPProductName, 0, 0, "ella-core-integration"),
			diameter.Unsigned32(diameter.AVPSupportedVendorID, diameter.AVPFlagMandatory, 0, tgpp.VendorID),
			diameter.Grouped(diameter.AVPVendorSpecificApplicationID, diameter.AVPFlagMandatory, 0,
				diameter.Unsigned32(diameter.AVPVendorID, diameter.AVPFlagMandatory, 0, tgpp.VendorID),
				diameter.Unsigned32(diameter.AVPAuthApplicationID, diameter.AVPFlagMandatory, 0, cx.ApplicationID),
			),
		},
	}

	cea, err := p.request(cer)
	if err != nil {
		p.close()
		return nil, 0, fmt.Errorf("capabilities exchange as %s: %w", host, err)
	}

	code, err := diameterResultCode(cea)
	if err != nil {
		p.close()
		return nil, 0, err
	}

	if code != diameter.ResultSuccess {
		p.close()
		return nil, code, nil
	}

	return p, code, nil
}

func dialDiameterProbeUntil(ctx context.Context, host, realm string, want uint32) (*diameterProbe, error) {
	deadline := time.Now().Add(time.Minute)

	for {
		probe, code, err := dialDiameterProbe(ctx, host, realm)
		if err != nil {
			return nil, err
		}

		if code == want {
			return probe, nil
		}

		if probe != nil {
			probe.close()
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("CER of %s realm %s: Result-Code %d, want %d", host, realm, code, want)
		}

		time.Sleep(time.Second)
	}
}

func diameterResultCode(m *diameter.Message) (uint32, error) {
	a, ok := m.Find(diameter.AVPResultCode, 0)
	if !ok {
		return 0, fmt.Errorf("command %d has no Result-Code", m.CommandCode)
	}

	return a.Unsigned32()
}

func (p *diameterProbe) close() {
	_ = p.conn.Close()
}

func (p *diameterProbe) envelope(destinationRealm string) tgpp.Envelope {
	p.next++

	return tgpp.Envelope{
		SessionID:        p.host + ";" + strconv.FormatUint(uint64(p.next), 10),
		Origin:           diameter.Identity{OriginHost: p.host, OriginRealm: p.realm},
		DestinationRealm: destinationRealm,
	}
}

func (p *diameterProbe) request(req *diameter.Message) (*diameter.Message, error) {
	p.next++
	req.HopByHopID = p.next
	req.EndToEndID = p.next

	if err := p.write(req); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(diameterProbeTimeout)

	for {
		m, err := p.read(deadline)
		if err != nil {
			return nil, err
		}

		if m.IsRequest() {
			if _, err := p.answerPeerRequest(m); err != nil {
				return nil, err
			}

			continue
		}

		if m.HopByHopID == req.HopByHopID {
			return m, nil
		}
	}
}

func (p *diameterProbe) awaitDisconnect(timeout time.Duration) (uint32, error) {
	deadline := time.Now().Add(timeout)

	for {
		m, err := p.read(deadline)
		if err != nil {
			return 0, err
		}

		if !m.IsRequest() {
			continue
		}

		cause, err := p.answerPeerRequest(m)
		if err != nil {
			return 0, err
		}

		if m.CommandCode == diameter.CommandDisconnectPeer {
			return cause, nil
		}
	}
}

func (p *diameterProbe) answerPeerRequest(m *diameter.Message) (uint32, error) {
	var cause uint32

	switch m.CommandCode {
	case diameter.CommandDeviceWatchdog:
	case diameter.CommandDisconnectPeer:
		a, ok := m.Find(diameter.AVPDisconnectCause, 0)
		if !ok {
			return 0, errors.New("DPR without Disconnect-Cause")
		}

		v, err := a.Unsigned32()
		if err != nil {
			return 0, err
		}

		cause = v
	default:
		return 0, fmt.Errorf("unexpected request %d from Ella Core", m.CommandCode)
	}

	ans := &diameter.Message{
		CommandCode: m.CommandCode,
		HopByHopID:  m.HopByHopID,
		EndToEndID:  m.EndToEndID,
		AVPs: []diameter.AVP{
			diameter.Unsigned32(diameter.AVPResultCode, diameter.AVPFlagMandatory, 0, diameter.ResultSuccess),
			diameter.UTF8String(diameter.AVPOriginHost, diameter.AVPFlagMandatory, 0, p.host),
			diameter.UTF8String(diameter.AVPOriginRealm, diameter.AVPFlagMandatory, 0, p.realm),
		},
	}

	return cause, p.write(ans)
}

func (p *diameterProbe) write(m *diameter.Message) error {
	b, err := m.Marshal()
	if err != nil {
		return err
	}

	if err := p.conn.SetWriteDeadline(time.Now().Add(diameterProbeTimeout)); err != nil {
		return err
	}

	_, err = p.conn.Write(b)

	return err
}

func (p *diameterProbe) read(deadline time.Time) (*diameter.Message, error) {
	if err := p.conn.SetReadDeadline(deadline); err != nil {
		return nil, err
	}

	header := make([]byte, 20)
	if _, err := io.ReadFull(p.conn, header); err != nil {
		return nil, err
	}

	length := int(header[1])<<16 | int(header[2])<<8 | int(header[3])
	if length < len(header) {
		return nil, fmt.Errorf("Diameter message of %d octets", length)
	}

	b := make([]byte, length)
	copy(b, header)

	if _, err := io.ReadFull(p.conn, b[len(header):]); err != nil {
		return nil, err
	}

	return diameter.Unmarshal(b)
}

func ellaIMSDiameterHost(ctx context.Context, t *testing.T) string {
	t.Helper()

	var identity struct {
		Host  string `json:"host"`
		Realm string `json:"realm"`
	}

	deadline := time.Now().Add(time.Minute)

	for {
		err := callIMS(ctx, http.MethodGet, "/api/v1/diameter", nil, &identity)
		if err == nil && identity.Host != "" {
			return identity.Host
		}

		if time.Now().After(deadline) {
			t.Fatalf("get the Diameter identity of Ella IMS: %v (%+v)", err, identity)
		}

		time.Sleep(time.Second)
	}
}

func createIMSPeer(ctx context.Context, t *testing.T, cl *client.Client, opts *client.CreateAllowedIMSPeerOptions) {
	t.Helper()

	if _, err := cl.CreateAllowedIMSPeer(ctx, opts); err != nil {
		t.Fatalf("create IMS peer %s: %v", opts.Name, err)
	}

	t.Cleanup(func() {
		if err := cl.DeleteAllowedIMSPeer(ctx, opts.Name); err != nil && !strings.Contains(strings.ToLower(err.Error()), "not found") {
			t.Logf("cleanup: delete IMS peer %s: %v", opts.Name, err)
		}
	})
}

func requireRestrictPeers(ctx context.Context, t *testing.T, cl *client.Client, want bool) {
	t.Helper()

	settings, err := cl.GetIMSSettings(ctx)
	if err != nil {
		t.Fatalf("get IMS settings: %v", err)
	}

	if settings.RestrictPeers != want {
		t.Fatalf("restrictPeers = %t, want %t", settings.RestrictPeers, want)
	}
}

func setRestrictPeers(ctx context.Context, t *testing.T, cl *client.Client, restrict bool) {
	t.Helper()

	settings, err := cl.GetIMSSettings(ctx)
	if err != nil {
		t.Fatalf("get IMS settings: %v", err)
	}

	opts := &client.UpdateIMSSettingsOptions{PCSCFAddresses: settings.PCSCFAddresses, HomeDomain: settings.HomeDomain, AdvertiseVoice: settings.AdvertiseVoice, RestrictPeers: restrict}
	if err := cl.UpdateIMSSettings(ctx, opts); err != nil {
		t.Fatalf("set restrictPeers %t: %v", restrict, err)
	}
}

func requireOpenIMSPeerPolicy(ctx context.Context, t *testing.T, cl *client.Client) {
	t.Helper()

	peers, err := cl.ListAllowedIMSPeers(ctx)
	if err != nil {
		t.Fatalf("list IMS peers: %v", err)
	}

	if len(peers.Items) != 0 {
		t.Fatalf("IMS peers %+v configured before the test, want none", peers.Items)
	}

	requireRestrictPeers(ctx, t, cl, false)

	host := "open." + probeIMSPeerRealm

	probe, err := dialDiameterProbeUntil(ctx, host, probeIMSPeerRealm, diameter.ResultSuccess)
	if err != nil {
		t.Fatalf("with no IMS peer configured: %v", err)
	}

	defer probe.close()

	peer, err := waitForInboundPeer(ctx, cl, host)
	if err != nil {
		t.Fatal(err)
	}

	if peer.Role != "unknown" || peer.Owner != nil {
		t.Fatalf("inbound peer %s with no IMS peer configured: role %q owner %+v, want role \"unknown\" and no owner", host, peer.Role, peer.Owner)
	}
}

func allowEllaIMS(ctx context.Context, t *testing.T, cl *client.Client) string {
	t.Helper()

	host := ellaIMSDiameterHost(ctx, t)

	createIMSPeer(ctx, t, cl, &client.CreateAllowedIMSPeerOptions{Name: ellaIMSPeerName, OriginHost: host, Addresses: anySourceAddress})
	requireRestrictPeers(ctx, t, cl, false)

	setRestrictPeers(ctx, t, cl, true)
	t.Cleanup(func() { setRestrictPeers(ctx, t, cl, false) })

	return host
}

func waitForInboundPeer(ctx context.Context, cl *client.Client, host string) (client.DiameterPeer, error) {
	deadline := time.Now().Add(2 * time.Minute)

	for {
		status, err := cl.GetDiameterStatus(ctx)
		if err == nil {
			for _, p := range status.Peers {
				if p.Direction == "inbound" && p.State == "open" && strings.EqualFold(p.OriginHost, host) {
					return p, nil
				}
			}
		}

		if time.Now().After(deadline) {
			return client.DiameterPeer{}, fmt.Errorf("no open inbound Diameter peer %s (status %+v, error %v)", host, status, err)
		}

		time.Sleep(time.Second)
	}
}

func waitForIMSPeerLink(ctx context.Context, cl *client.Client, host, name string) error {
	peer, err := waitForInboundPeer(ctx, cl, host)
	if err != nil {
		return err
	}

	if peer.Role != "ims" {
		return fmt.Errorf("inbound peer %s: role %q, want \"ims\"", host, peer.Role)
	}

	if peer.Owner == nil || peer.Owner.Kind != "imsPeer" || peer.Owner.Name != name {
		return fmt.Errorf("inbound peer %s: owner %+v, want IMS peer %s", host, peer.Owner, name)
	}

	imsPeer, err := cl.GetAllowedIMSPeer(ctx, name)
	if err != nil {
		return fmt.Errorf("get IMS peer %s: %w", name, err)
	}

	for _, c := range imsPeer.Connections {
		if c.State == "open" && strings.EqualFold(c.OriginHost, host) {
			return nil
		}
	}

	return fmt.Errorf("IMS peer %s: connections %+v, want an open connection from %s", name, imsPeer.Connections, host)
}

func requireUnknownPeerRejected(ctx context.Context, t *testing.T, cl *client.Client) {
	t.Helper()

	if _, err := dialDiameterProbeUntil(ctx, rogueDiameterHost, rogueDiameterRealm, diameter.ResultUnknownPeer); err != nil {
		t.Fatalf("not an IMS peer: %v", err)
	}

	name := probeIMSPeerName + "-elsewhere"

	createIMSPeer(ctx, t, cl, &client.CreateAllowedIMSPeerOptions{Name: name, Realm: probeIMSPeerRealm, Addresses: []string{"192.0.2.1/32"}})

	if _, err := dialDiameterProbeUntil(ctx, "elsewhere."+probeIMSPeerRealm, probeIMSPeerRealm, diameter.ResultUnknownPeer); err != nil {
		t.Fatalf("an allowed realm from a source address outside the entry: %v", err)
	}

	if err := cl.DeleteAllowedIMSPeer(ctx, name); err != nil {
		t.Fatalf("delete IMS peer %s: %v", name, err)
	}
}

func requireIMSPeerRemovalDisconnects(ctx context.Context, t *testing.T, cl *client.Client, imsHost string) {
	t.Helper()

	name := probeIMSPeerName + "-removal"
	host := "removal." + probeIMSPeerRealm

	createIMSPeer(ctx, t, cl, &client.CreateAllowedIMSPeerOptions{Name: name, Realm: probeIMSPeerRealm, Addresses: anySourceAddress})

	probe, err := dialDiameterProbeUntil(ctx, host, probeIMSPeerRealm, diameter.ResultSuccess)
	if err != nil {
		t.Fatalf("allowed by realm: %v", err)
	}

	defer probe.close()

	if err := waitForIMSPeerLink(ctx, cl, host, name); err != nil {
		t.Fatal(err)
	}

	if err := cl.DeleteAllowedIMSPeer(ctx, name); err != nil {
		t.Fatalf("delete IMS peer %s: %v", name, err)
	}

	cause, err := probe.awaitDisconnect(time.Minute)
	if err != nil {
		t.Fatalf("wait for the DPR of Ella Core after deleting IMS peer %s: %v", name, err)
	}

	if cause != diameter.DisconnectCauseDoNotWantToTalkToYou {
		t.Fatalf("Disconnect-Cause %d, want DO_NOT_WANT_TO_TALK_TO_YOU (%d)", cause, diameter.DisconnectCauseDoNotWantToTalkToYou)
	}

	if err := waitForIMSPeerLink(ctx, cl, imsHost, ellaIMSPeerName); err != nil {
		t.Fatalf("Ella IMS after deleting IMS peer %s: %v", name, err)
	}
}
