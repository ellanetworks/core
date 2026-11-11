//go:build open5gs

// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const (
	r4gHSSIdentity = "hss.epc.mnc001.mcc001.3gppnetwork.org"
	r4gHSSRealm    = "epc.mnc001.mcc001.3gppnetwork.org"
	r4gHSSAddress  = "10.130.0.30"
	r4gCorporate   = "corporate"
)

var roaming4G = roamingRAT{
	name:        "4g",
	octet:       130,
	scenario:    "s1enb/roaming/attach",
	marker:      "ROAMER_ATTACHED",
	milestone:   "attach",
	logServices: []string{"hss"},
	interfaces:  "  diameter:\n    address: %q\n",
	bringUp:     20 * time.Minute,
}

type r4gStack struct {
	*roamingStack
	hosts    []string
	moverRun *roamingTesterRun
}

func r4gRoamer(i int) string { return fmt.Sprintf("0010100000001%02d", i) }

func r4gMSISDN(i int) string { return fmt.Sprintf("+155501000%02d", i) }

func r4gWriteHSSConfig(hosts []string) error {
	var b strings.Builder

	fmt.Fprintf(&b, `Identity = %q;
Realm = %q;
ListenOn = %q;
TLS_Cred = "/cfg/hss.crt", "/cfg/hss.key";
TLS_CA = "/cfg/hss.crt";
NoRelay;
LoadExtension = "/usr/lib/x86_64-linux-gnu/freeDiameter/dbg_msg_dumps.fdx" : "0x8888";
LoadExtension = "/usr/lib/x86_64-linux-gnu/freeDiameter/dict_rfc5777.fdx";
LoadExtension = "/usr/lib/x86_64-linux-gnu/freeDiameter/dict_mip6i.fdx";
LoadExtension = "/usr/lib/x86_64-linux-gnu/freeDiameter/dict_nasreq.fdx";
LoadExtension = "/usr/lib/x86_64-linux-gnu/freeDiameter/dict_nas_mipv6.fdx";
LoadExtension = "/usr/lib/x86_64-linux-gnu/freeDiameter/dict_dcca.fdx";
LoadExtension = "/usr/lib/x86_64-linux-gnu/freeDiameter/dict_dcca_3gpp.fdx";
`, r4gHSSIdentity, r4gHSSRealm, r4gHSSAddress)

	for i, h := range hosts {
		fmt.Fprintf(&b, "ConnectPeer = %q { ConnectTo = %q; No_TLS; };\n", h, roaming4G.nodeIP(i+1))
	}

	certPEM, keyPEM, err := r4gSelfSigned(r4gHSSIdentity)
	if err != nil {
		return err
	}

	if err := roaming4G.writeFile("cfg/hss/hss.crt", certPEM); err != nil {
		return err
	}

	if err := roaming4G.writeFile("cfg/hss/hss.key", keyPEM); err != nil {
		return err
	}

	return roaming4G.writeFile("cfg/hss/hss.conf", b.String())
}

func r4gSelfSigned(cn string) (string, string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		DNSNames:              []string{cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	return string(certPEM), string(keyPEM), nil
}

func r4gBringUp(t *testing.T, nodes int) *r4gStack {
	t.Helper()

	s := &r4gStack{roamingStack: roamingBringUp(t, roaming4G, nodes)}

	fx := fixture.New(t, s.ctx, s.api)
	fx.DataNetwork(fixture.DataNetworkSpec{Name: r4gCorporate, IPv4Pool: "10.46.0.0/16", DNS: scenarios.DefaultDNS, MTU: scenarios.DefaultMTU})
	fx.Policy(fixture.PolicySpec{
		Name: "roamers-corporate", ProfileName: roamingProfile, SliceName: scenarios.DefaultSliceName,
		DataNetworkName: r4gCorporate, SessionAmbrUplink: "200 Mbps", SessionAmbrDownlink: "200 Mbps",
		Var5qi: 9, Arp: 15,
	})

	s.startTester()
	s.createPartner()
	s.startHSS()

	return s
}

func (s *r4gStack) createPartner() {
	t := s.t
	t.Helper()

	peer, err := s.api.CreateRoamingDiameterPeer(s.ctx, &client.RoamingDiameterPeerOptions{
		OriginHost: r4gHSSIdentity, Address: r4gHSSAddress, Port: 3868,
	})
	if err != nil {
		t.Fatalf("create roaming Diameter peer: %v", err)
	}

	partner, err := s.api.CreateRoamingPartner(s.ctx, &client.RoamingPartnerOptions{
		Name: roamingPartner, PLMNs: []client.RoamingPLMN{{Mcc: "001", Mnc: "01"}}, ProfileName: roamingProfile, DiameterPeerID: peer.ID,
	})
	if err != nil {
		t.Fatalf("create roaming partner: %v", err)
	}

	s.partner = partner.ID
}

func (s *r4gStack) startHSS() {
	t := s.t
	t.Helper()

	s.hosts = make([]string, len(s.nodes))

	for i, c := range s.nodes {
		err := roamingPoll(s.ctx, time.Minute, func() error {
			st, err := c.GetDiameterStatus(s.ctx)
			if err != nil {
				return err
			}

			if st.LocalOriginHost() == "" {
				return errors.New("no Diameter identity yet")
			}

			s.hosts[i] = st.LocalOriginHost()

			return nil
		})
		if err != nil {
			t.Fatalf("Diameter identity of node %d: %v", i+1, err)
		}

		t.Logf("node %d Origin-Host %s", i+1, s.hosts[i])
	}

	if err := r4gWriteHSSConfig(s.hosts); err != nil {
		t.Fatalf("write HSS config: %v", err)
	}

	if err := s.rat.compose(s.ctx, "up", "-d", "hss"); err != nil {
		t.Fatal(err)
	}

	for i := range s.nodes {
		s.waitHSSPeer(i + 1)
	}
}

func (s *r4gStack) waitHSSPeer(n int) {
	s.t.Helper()

	err := roamingPoll(s.ctx, 2*time.Minute, func() error {
		st, err := s.nodes[n-1].GetDiameterStatus(s.ctx)
		if err != nil {
			return err
		}

		for _, p := range st.Peers {
			if p.OriginHost == r4gHSSIdentity || p.Address == r4gHSSAddress {
				if p.State == "open" {
					return nil
				}

				return fmt.Errorf("HSS peer is %s (%s)", p.State, p.Error)
			}
		}

		return errors.New("no HSS peer")
	})
	if err != nil {
		s.t.Fatalf("node %d HSS peer: %v", n, err)
	}
}

type r4gHSSRecord struct {
	MMEHost   string `json:"mme_host"`
	MMERealm  string `json:"mme_realm"`
	PurgeFlag bool   `json:"purge_flag"`
	SQN       int64  `json:"sqn"`
}

func (s *r4gStack) mongo(js string) string {
	s.t.Helper()

	container, err := s.dc.ResolveComposeContainer(s.ctx, s.rat.project(), "mongodb")
	if err != nil {
		s.t.Fatalf("resolve mongodb container: %v", err)
	}

	out, err := s.dc.Exec(s.ctx, container, []string{"mongosh", "--quiet", "mongodb://localhost/open5gs?replicaSet=rs0", "--eval", js}, false, 30*time.Second, nil)
	if err != nil {
		s.t.Fatalf("mongosh: %v", err)
	}

	return strings.TrimSpace(out)
}

func (s *r4gStack) hssRecord(imsi string) r4gHSSRecord {
	s.t.Helper()

	out := s.mongo(fmt.Sprintf(`const d = db.subscribers.findOne({imsi: %q}); print(JSON.stringify({mme_host: typeof d.mme_host === "string" ? d.mme_host : "", mme_realm: typeof d.mme_realm === "string" ? d.mme_realm : "", purge_flag: d.purge_flag === true, sqn: Number(d.security.sqn)}))`, imsi))

	var rec r4gHSSRecord
	if err := json.Unmarshal([]byte(out[strings.LastIndex(out, "{"):]), &rec); err != nil {
		s.t.Fatalf("parse HSS record %q: %v", out, err)
	}

	return rec
}

func (s *r4gStack) waitHSSRecord(imsi, what string, ok func(r4gHSSRecord) bool) r4gHSSRecord {
	s.t.Helper()

	var rec r4gHSSRecord

	err := roamingPoll(s.ctx, 30*time.Second, func() error {
		rec = s.hssRecord(imsi)
		if ok(rec) {
			return nil
		}

		return fmt.Errorf("HSS record %+v", rec)
	})
	if err != nil {
		s.t.Fatalf("%s: %v", what, err)
	}

	return rec
}

func TestRoaming4GOpen5GS(t *testing.T) {
	s := r4gBringUp(t, 1)

	t.Run("unknown PLMN is rejected with #11", func(t *testing.T) {
		s.tester(1, "001020000000001", "--expect-reject", "11").wait(t, "unknown PLMN")
	})

	t.Run("attach, LBO data and detach with purge", func(t *testing.T) {
		imsi := r4gRoamer(1)

		run := s.tester(1, imsi, "--expect-apn", scenarios.DefaultDNN, "--hold", "10s")
		run.reached(t, "roamer attach")
		s.waitRoamerMSISDN(imsi, r4gMSISDN(1))
		run.wait(t, "roamer attach")

		rec := s.waitHSSRecord(imsi, "PUR after the detach", func(r r4gHSSRecord) bool { return r.PurgeFlag })
		if rec.MMEHost != s.hosts[0] {
			t.Fatalf("HSS registered MME %q, want Ella's %q", rec.MMEHost, s.hosts[0])
		}

		roaming, notRoaming := true, false

		err := roamingPoll(s.ctx, 75*time.Second, func() error {
			usage, err := s.api.ListUsagePerSubscriber(s.ctx, &client.ListUsageParams{Subscriber: imsi, Roaming: &roaming})
			if err != nil {
				return err
			}

			for _, u := range usage {
				if u.HomePLMN != nil && *u.HomePLMN == (client.RoamingPLMN{Mcc: "001", Mnc: "01"}) && u.RoamingPartner == roamingPartner && u.UplinkBytes > 0 && u.DownlinkBytes > 0 {
					return nil
				}
			}

			return fmt.Errorf("roaming usage %+v", usage)
		})
		if err != nil {
			t.Fatalf("roaming usage of the roamer: %v", err)
		}

		home, err := s.api.ListUsagePerSubscriber(s.ctx, &client.ListUsageParams{Subscriber: imsi, Roaming: &notRoaming})
		if err != nil {
			t.Fatalf("subscriber usage: %v", err)
		}

		for _, u := range home {
			if u.TotalBytes > 0 {
				t.Fatalf("visitor usage recorded as subscriber usage: %+v", home)
			}
		}
	})

	t.Run("authentication resynchronisation with the home HSS", func(t *testing.T) {
		s.tester(1, r4gRoamer(2), "--sqn", "000000100000", "--expect-resyncs", "1", "--ping=false").wait(t, "resync attach")
	})

	t.Run("HSS-initiated IDR updates the UE-AMBR", func(t *testing.T) {
		imsi := r4gRoamer(3)
		run := s.tester(1, imsi, "--ping=false", "--await-ue-ambr-dl", "100", "--await-ue-ambr-ul", "50")
		run.reached(t, "IDR roamer")

		s.mongo(fmt.Sprintf(`db.subscribers.updateOne({imsi: %q}, {$set: {ambr: {downlink: {value: NumberInt(100), unit: NumberInt(2)}, uplink: {value: NumberInt(50), unit: NumberInt(2)}}}})`, imsi))

		run.wait(t, "IDR roamer")
	})

	t.Run("HSS-initiated IDR barring E-UTRAN detaches the roamer", func(t *testing.T) {
		imsi := r4gRoamer(8)
		run := s.tester(1, imsi, "--ping=false", "--await-network-detach", "--expect-detach-cause", "15")
		run.reached(t, "ARD roamer")

		s.mongo(fmt.Sprintf(`db.subscribers.updateOne({imsi: %q}, {$set: {access_restriction_data: NumberInt(48)}})`, imsi))

		run.wait(t, "ARD roamer")
		s.waitHSSRecord(imsi, "PUR after the network detach", func(r r4gHSSRecord) bool { return r.PurgeFlag })
	})

	t.Run("re-attach while registered keeps the HSS registration", func(t *testing.T) {
		imsi := r4gRoamer(4)

		s.tester(1, imsi, "--ping=false", "--detach=false").wait(t, "first attach")
		s.tester(1, imsi, "--ping=false").wait(t, "second attach")

		s.waitHSSRecord(imsi, "PUR after the final detach", func(r r4gHSSRecord) bool { return r.PurgeFlag })
	})

	t.Run("combined attach with deferred ESM information is answered EPS only with #18", func(t *testing.T) {
		imsi := r4gRoamer(10)

		s.tester(1, imsi, "--combined", "--defer-esm-info", "--apn", scenarios.DefaultDNN, "--expect-apn", scenarios.DefaultDNN).wait(t, "combined attach")
		s.waitHSSRecord(imsi, "PUR after the detach", func(r r4gHSSRecord) bool { return r.PurgeFlag })
	})

	t.Run("integrity-protected GUTI re-attach skips authentication, then a switch-off detach by GUTI", func(t *testing.T) {
		imsi := r4gRoamer(11)
		sqn := s.hssRecord(imsi).SQN
		requests := s.s6aSuccesses()

		s.tester(1, imsi, "--defer-esm-info", "--apn", scenarios.DefaultDNN, "--expect-apn", scenarios.DefaultDNN,
			"--reattach-with-guti", "--switch-off").wait(t, "GUTI re-attach")

		rec := s.waitHSSRecord(imsi, "PUR after the switch-off detach", func(r r4gHSSRecord) bool { return r.PurgeFlag })
		if rec.SQN != sqn+32 {
			t.Fatalf("HSS SQN went from %d to %d: the attach and the GUTI re-attach took %d AIRs, want 1", sqn, rec.SQN, (rec.SQN-sqn)/32)
		}

		if got := s.s6aSuccesses() - requests; got != 4 {
			t.Fatalf("%v S6a requests, want 4: AIR and ULR for the attach, ULR for the re-attach, PUR for the detach", got)
		}
	})

	t.Run("IPv4v6 request on an IPv4-only data network is answered IPv4 with #50", func(t *testing.T) {
		s.tester(1, r4gRoamer(13), "--ping=false", "--apn", r4gCorporate, "--expect-apn", r4gCorporate,
			"--pdn-type", "ipv4v6", "--expect-pdn-type", "ipv4", "--expect-esm-cause", "50").wait(t, "IPv4v6 attach")
	})

	t.Run("IPv4v6 request with an IPv4_OR_IPv6 home subscription is answered IPv4 with #52", func(t *testing.T) {
		s.tester(1, r4gRoamer(12), "--expect-apn", scenarios.DefaultDNN,
			"--pdn-type", "ipv4v6", "--expect-pdn-type", "ipv4", "--expect-esm-cause", "52").wait(t, "IPv4v6 attach")
	})

	t.Run("roaming metrics", func(t *testing.T) {
		m, err := s.api.GetMetrics(s.ctx)
		if err != nil {
			t.Fatalf("get metrics: %v", err)
		}

		if v := m[`app_connected_roaming_partners{interface="s6a"}`]; v != 1 {
			t.Errorf("connected s6a roaming partners = %v, want 1", v)
		}

		for _, want := range []struct {
			name   string
			labels []string
		}{
			{"app_peer_requests_total", []string{`interface="s6a"`, `result="success"`}},
			{"app_peer_request_duration_seconds_count", []string{`interface="s6a"`}},
			{"app_registration_attempts_total", []string{`rat="4g"`, `result="accept"`, `roaming="true"`}},
			{"app_registration_attempts_total", []string{`rat="4g"`, `result="reject"`, `roaming="false"`}},
			{"app_session_establishment_attempts_total", []string{`rat="4g"`, `result="accept"`, `roaming="true"`}},
		} {
			if v := r4gMetricSum(m, want.name, want.labels...); v == 0 {
				t.Errorf("%s%v = 0, want > 0", want.name, want.labels)
			}
		}
	})
}

func (s *r4gStack) s6aSuccesses() float64 {
	s.t.Helper()

	m, err := s.api.GetMetrics(s.ctx)
	if err != nil {
		s.t.Fatalf("get metrics: %v", err)
	}

	return r4gMetricSum(m, "app_peer_requests_total", `interface="s6a"`, `result="success"`)
}

func r4gMetricSum(m map[string]float64, name string, labels ...string) float64 {
	var sum float64

	for series, v := range m {
		if !strings.HasPrefix(series, name+"{") {
			continue
		}

		matches := true

		for _, l := range labels {
			if !strings.Contains(series, l) {
				matches = false
				break
			}
		}

		if matches {
			sum += v
		}
	}

	return sum
}

func TestRoaming4GOpen5GSHA(t *testing.T) {
	s := r4gBringUp(t, 3)

	mover := r4gRoamer(5)

	t.Run("a roamer moving to another node is cancelled on the old one", func(t *testing.T) {
		s.tester(1, mover, "--detach=false").wait(t, "attach on node 1")
		s.waitRegistered(1, 1, "roamer registered on node 1")

		run := s.tester(2, mover, "--await-network-detach", "--expect-detach-cause", "11")
		run.reached(t, "attach on node 2")

		rec := s.waitHSSRecord(mover, "the HSS registers node 2", func(r r4gHSSRecord) bool { return r.MMEHost == s.hosts[1] && !r.PurgeFlag })
		t.Logf("HSS record after the move: %+v", rec)

		s.waitRegistered(1, 0, "CLR MME_UPDATE drops the context on node 1")

		time.Sleep(15 * time.Second)

		if got := s.registered(2); got != 1 {
			t.Fatalf("node 2 has %v registered EPS subscribers after the CLR on node 1, want 1", got)
		}

		if rec := s.hssRecord(mover); rec.PurgeFlag || rec.MMEHost != s.hosts[1] {
			t.Fatalf("node 1 purged the moved roamer at the HSS: %+v", rec)
		}

		s.moverRun = run
	})

	crashed := r4gRoamer(6)

	t.Run("a restarted node drops its crash leftovers", func(t *testing.T) {
		s.tester(3, crashed, "--ping=false", "--detach=false").wait(t, "attach on node 3")

		if err := composeKill(s.ctx, s.rat.dir(), roamingFile, "ella-core-3"); err != nil {
			t.Fatal(err)
		}

		if err := s.rat.compose(s.ctx, "up", "-d", "ella-core-3"); err != nil {
			t.Fatal(err)
		}

		if err := waitForAllNodesReady(s.ctx, s.nodes); err != nil {
			t.Fatalf("nodes not ready after the restart: %v", err)
		}

		s.waitHSSPeer(3)
		s.waitRegistered(3, 0, "node 3 holds no context after the restart")

		if rec := s.hssRecord(crashed); rec.PurgeFlag {
			t.Fatalf("the restarted node sent a PUR for its crash leftover: %+v", rec)
		}
	})

	t.Run("deleting the partner detaches its roamers on every node", func(t *testing.T) {
		if s.moverRun == nil {
			t.Skip("no roamer on node 2")
		}

		other := r4gRoamer(7)
		run := s.tester(3, other, "--ping=false", "--enb-index", "1", "--await-network-detach", "--expect-detach-cause", "11")
		run.reached(t, "attach on node 3")

		start := time.Now()

		if err := s.api.DeleteRoamingPartner(s.ctx, s.partner); err != nil {
			t.Fatalf("delete the partner with roamers on nodes 2 and 3: %v", err)
		}

		t.Logf("partner deleted after %s", time.Since(start))

		s.moverRun.wait(t, "roamer on node 2")
		run.wait(t, "roamer on node 3")

		for _, imsi := range []string{mover, other} {
			s.waitHSSRecord(imsi, "PUR of "+imsi, func(r r4gHSSRecord) bool { return r.PurgeFlag })
		}

		for n := 1; n <= 3; n++ {
			s.waitRegistered(n, 0, fmt.Sprintf("node %d holds no roamer", n))
		}
	})
}
