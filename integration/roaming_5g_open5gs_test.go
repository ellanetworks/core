//go:build open5gs

// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/fixture"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const (
	r5gUDM       = "10.150.0.34"
	r5gHPLMNSEPP = "sepp1.5gc.mnc001.mcc001.3gppnetwork.org"
	r5gN32Port   = 7443
	r5gNoSDSlice = "no-sd"
)

var roaming5G = roamingRAT{
	name:        "5g",
	octet:       150,
	scenario:    "gnb/roaming/register",
	marker:      "ROAMER_REGISTERED",
	milestone:   "registration",
	logServices: []string{"sepp", "udm", "ausf"},
	interfaces:  "  n32:\n    address: %q\n    port: " + fmt.Sprint(r5gN32Port) + "\n",
	bringUp:     30 * time.Minute,
}

type r5gStack struct {
	*roamingStack
	home tls.Certificate
}

func r5gRoamer(i int) string { return fmt.Sprintf("00101000000020%d", i) }

func r5gNoSDRoamer(i int) string { return fmt.Sprintf("00101000000021%d", i) }

func r5gEllaSEPP(n int) string { return fmt.Sprintf("sepp%d.5gc.mnc070.mcc999.3gppnetwork.org", n) }

func r5gIssue(template, parent *x509.Certificate, pub any, signer *ecdsa.PrivateKey) (*x509.Certificate, error) {
	der, err := x509.CreateCertificate(rand.Reader, template, parent, pub, signer)
	if err != nil {
		return nil, err
	}

	return x509.ParseCertificate(der)
}

func (s *r5gStack) writeHPLMNSEPP(ellaCA string, nodes int) {
	t := s.t
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{Organization: []string{"Open5GS"}, CommonName: "Open5GS Root CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	ca, err := r5gIssue(caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	leaf, err := r5gIssue(&x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano() + 1),
		Subject:      pkix.Name{Organization: []string{"Open5GS"}, CommonName: r5gHPLMNSEPP},
		DNSNames:     []string{r5gHPLMNSEPP},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw})

	s.home = tls.Certificate{Certificate: [][]byte{leaf.Raw, ca.Raw}, PrivateKey: key, Leaf: leaf}

	var clients strings.Builder

	for n := 1; n <= nodes; n++ {
		fmt.Fprintf(&clients, `        - receiver: %[1]s
          uri: https://%[1]s:%[2]d
          target_plmn_id:
            mcc: 999
            mnc: 70
          cacert: /sepp/ella-ca.crt
          client_cert: /sepp/open5gs-sepp.crt
          client_private_key: /sepp/open5gs-sepp.key
`, r5gEllaSEPP(n), r5gN32Port)
	}

	config := fmt.Sprintf(`logger:
  level: debug
global:
  max:
    ue: 64
sepp:
  serving:
    - plmn_id:
        mcc: 001
        mnc: 01
  sbi:
    server:
      - address: %[1]s
        port: 80
    client:
      nrf:
        - uri: http://nrf.5gc.mnc001.mcc001.3gppnetwork.org
  n32:
    server:
      - sender: %[1]s
        scheme: https
        address: 0.0.0.0
        port: 7777
        private_key: /sepp/open5gs-sepp.key
        cert: /sepp/open5gs-sepp.crt
        verify_client: true
        verify_client_cacert: /sepp/ella-ca.crt
    client:
      sepp:
%[2]s`, r5gHPLMNSEPP, clients.String())

	for name, body := range map[string]string{
		"cfg/sepp/open5gs-sepp.crt": string(certPEM),
		"cfg/sepp/open5gs-sepp.key": string(keyPEM),
		"cfg/sepp/open5gs-ca.crt":   string(caPEM),
		"cfg/sepp/ella-ca.crt":      ellaCA,
		"cfg/sepp/sepp.yaml":        config,
	} {
		if err := roaming5G.writeFile(name, body); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func r5gBringUp(t *testing.T, nodes int) *r5gStack {
	t.Helper()

	s := &r5gStack{roamingStack: roamingBringUp(t, roaming5G, nodes)}

	fx := fixture.New(t, s.ctx, s.api)
	fx.Slice(fixture.SliceSpec{Name: r5gNoSDSlice, SST: scenarios.DefaultSST})
	fx.Policy(fixture.PolicySpec{
		Name: "roamers-internet-no-sd", ProfileName: roamingProfile, SliceName: r5gNoSDSlice,
		DataNetworkName: scenarios.DefaultDNN, SessionAmbrUplink: "200 Mbps", SessionAmbrDownlink: "200 Mbps",
		Var5qi: 9, Arp: 15,
	})

	s.writeHPLMNSEPP(s.ellaTrustAnchor(nodes), nodes)

	if err := roaming5G.compose(s.ctx, "up", "-d", "mongodb", "provision", "nrf", "udr", "udm", "ausf", "sepp"); err != nil {
		t.Fatal(err)
	}

	s.createPartner()

	for n := 1; n <= nodes; n++ {
		s.waitN32(n)
	}

	s.startTester()

	return s
}

func (s *r5gStack) ellaTrustAnchor(nodes int) string {
	t := s.t
	t.Helper()

	var anchor string

	err := roamingPoll(s.ctx, 2*time.Minute, func() error {
		for i, c := range s.nodes[:nodes] {
			st, err := c.GetSEPPStatus(s.ctx)
			if err != nil {
				return err
			}

			own := slices.ContainsFunc(st.Certificates, func(c client.SEPPCertificate) bool { return c.SEPPID == st.Listener.SEPPID && c.Fingerprint != "" })
			if len(st.TrustAnchors) != 1 || !st.Listener.Listening || !own {
				return fmt.Errorf("SEPP status of node %d: %d trust anchors, listening %t, certificate %t", i+1, len(st.TrustAnchors), st.Listener.Listening, own)
			}

			anchor = st.TrustAnchors[0].PEM
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Ella SEPP identity: %v", err)
	}

	return anchor
}

func (s *r5gStack) createPartner() {
	t := s.t
	t.Helper()

	caPEM, err := os.ReadFile(filepath.Join(roaming5G.dir(), "cfg/sepp/open5gs-ca.crt"))
	if err != nil {
		t.Fatal(err)
	}

	partner, err := s.api.CreateRoamingPartner(s.ctx, &client.RoamingPartnerOptions{
		Name: roamingPartner, PLMNs: []client.RoamingPLMN{{Mcc: "001", Mnc: "01", NRFAPIRoot: "http://nrf.5gc.mnc001.mcc001.3gppnetwork.org"}}, ProfileName: roamingProfile,
		SEPP: &client.RoamingPartnerSEPP{
			FQDN:           r5gHPLMNSEPP,
			N32cURI:        "https://" + r5gHPLMNSEPP + ":7777",
			TrustAnchorPEM: string(caPEM),
		},
	})
	if err != nil {
		t.Fatalf("create roaming partner: %v", err)
	}

	s.partner = partner.ID
}

func (s *r5gStack) waitN32(node int) {
	s.t.Helper()

	err := roamingPoll(s.ctx, 3*time.Minute, func() error {
		p, err := s.nodes[node-1].GetRoamingPartner(s.ctx, s.partner)
		if err != nil {
			return err
		}

		if p.N32 != nil && p.N32.State == "established" {
			return nil
		}

		return fmt.Errorf("N32 context %+v", p.N32)
	})
	if err != nil {
		s.t.Fatalf("node %d N32 context: %v", node, err)
	}
}

type r5gUDMRegistration struct {
	AmfInstanceID    string `json:"amfInstanceId"`
	DeregCallbackURI string `json:"deregCallbackUri"`
	PurgeFlag        bool   `json:"purgeFlag"`
	Guami            struct {
		AmfID string `json:"amfId"`
	} `json:"guami"`
}

func r5gH2C() *http.Client {
	tr := &http.Transport{}
	tr.Protocols = new(http.Protocols)
	tr.Protocols.SetUnencryptedHTTP2(true)

	return &http.Client{Transport: tr, Timeout: 10 * time.Second}
}

func (s *r5gStack) udmRegistration(imsi string) (r5gUDMRegistration, int, error) {
	req, err := http.NewRequestWithContext(s.ctx, http.MethodGet, fmt.Sprintf("http://%s/nudm-uecm/v1/imsi-%s/registrations/amf-3gpp-access", r5gUDM, imsi), nil)
	if err != nil {
		return r5gUDMRegistration{}, 0, err
	}

	req.Host = "udm.5gc.mnc001.mcc001.3gppnetwork.org"

	resp, err := r5gH2C().Do(req)
	if err != nil {
		return r5gUDMRegistration{}, 0, err
	}

	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	var reg r5gUDMRegistration

	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(body, &reg); err != nil {
			return reg, resp.StatusCode, fmt.Errorf("parse %s: %w", body, err)
		}
	}

	return reg, resp.StatusCode, nil
}

func (s *r5gStack) waitUDM(imsi, what string, ok func(r5gUDMRegistration, int) bool) r5gUDMRegistration {
	s.t.Helper()

	var reg r5gUDMRegistration

	err := roamingPoll(s.ctx, 30*time.Second, func() error {
		r, status, err := s.udmRegistration(imsi)
		if err != nil {
			return err
		}

		reg = r

		if ok(r, status) {
			return nil
		}

		return fmt.Errorf("UDM registration %+v (HTTP %d)", r, status)
	})
	if err != nil {
		s.t.Fatalf("%s: %v", what, err)
	}

	return reg
}

func (s *r5gStack) notify(ownerNode int, imsi, kind string, body any) (int, string) {
	const node = 1

	s.t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		s.t.Fatal(err)
	}

	url := fmt.Sprintf("https://%s:%d/roaming-callbacks/v1/imsi-%s/%s", roaming5G.nodeIP(node), r5gN32Port, imsi, kind)

	req, err := http.NewRequestWithContext(s.ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		s.t.Fatal(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("3gpp-Sbi-Target-apiRoot", fmt.Sprintf("https://amf%d.5gc.mnc070.mcc999.3gppnetwork.org", ownerNode))

	c := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{ForceAttemptHTTP2: true, TLSClientConfig: &tls.Config{
		MinVersion:         tls.VersionTLS12,
		NextProtos:         []string{"h2"},
		Certificates:       []tls.Certificate{s.home},
		ServerName:         r5gEllaSEPP(node),
		InsecureSkipVerify: true, //nolint:gosec
	}}}

	resp, err := c.Do(req)
	if err != nil {
		s.t.Fatalf("N32-f callback to node %d: %v", node, err)
	}

	defer func() { _ = resp.Body.Close() }()

	out, _ := io.ReadAll(resp.Body)

	return resp.StatusCode, string(out)
}

func (s *r5gStack) udmProfileAKey() string {
	s.t.Helper()

	container, err := s.dc.ResolveComposeContainer(s.ctx, s.rat.project(), "udm")
	if err != nil {
		s.t.Fatalf("resolve UDM container: %v", err)
	}

	out, err := s.dc.Exec(s.ctx, container, []string{"openssl", "pkey", "-in", "/etc/open5gs/hnet/curve25519-1.key", "-pubout"}, false, 30*time.Second, nil)
	if err != nil {
		s.t.Fatalf("read the home network public key: %v", err)
	}

	block, _ := pem.Decode([]byte(out[max(strings.Index(out, "-----BEGIN"), 0):]))
	if block == nil {
		s.t.Fatalf("no PEM public key in %q", out)
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		s.t.Fatalf("parse the home network public key: %v", err)
	}

	key, ok := pub.(*ecdh.PublicKey)
	if !ok {
		s.t.Fatalf("home network key %T is not X25519", pub)
	}

	return hex.EncodeToString(key.Bytes())
}

func (s *r5gStack) sessionFlags() []string {
	if os.Getenv("ROAMING5G_NO_SESSION") != "" {
		return []string{"--session=false", "--ping=false"}
	}

	return nil
}

func TestRoaming5GOpen5GS(t *testing.T) {
	s := r5gBringUp(t, 1)

	t.Run("unknown PLMN is rejected with #11", func(t *testing.T) {
		s.tester(1, "001020000000001", "--home-mnc", "02", "--expect-reject", "11").wait(t, "unknown PLMN")
	})

	t.Run("registration through the home AUSF and UDM, LBO data, deregistration with purge", func(t *testing.T) {
		imsi := r5gRoamer(1)
		run := s.tester(1, imsi, s.sessionFlags()...)
		run.reached(t, "roamer registration")

		reg := s.waitUDM(imsi, "UECM registration at the home UDM", func(r r5gUDMRegistration, status int) bool {
			return status == http.StatusOK && !r.PurgeFlag
		})

		if !strings.HasPrefix(reg.DeregCallbackURI, "https://amf1.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/imsi-"+imsi) || reg.AmfInstanceID == "" {
			t.Fatalf("UECM registration %+v", reg)
		}

		run.wait(t, "roamer registration")

		s.waitUDM(imsi, "purge at the home UDM after the deregistration", func(r r5gUDMRegistration, status int) bool {
			return status == http.StatusOK && r.PurgeFlag
		})
	})

	t.Run("authentication resynchronisation with the home AUSF", func(t *testing.T) {
		s.tester(1, r5gRoamer(2), append([]string{"--sqn", "000000100000"}, s.sessionFlags()...)...).wait(t, "resync registration")
	})

	t.Run("UDM-initiated deregistration (SUBSCRIPTION_WITHDRAWN) of a connected roamer over N32", func(t *testing.T) {
		imsi := r5gRoamer(3)
		run := s.tester(1, imsi, "--ping=false", "--await-network-deregistration")
		run.reached(t, "roamer registration")

		s.waitUDM(imsi, "UECM registration", func(r r5gUDMRegistration, status int) bool { return status == http.StatusOK && !r.PurgeFlag })

		_, err := s.api.GetLocation(s.ctx, &client.GetLocationOptions{SUPI: "imsi-" + imsi, Method: "cell_id"})
		if err == nil || !strings.Contains(err.Error(), "server error 403") {
			t.Fatalf("locating the roamer: %v, want 403: roamers cannot be located", err)
		}

		status, body := s.notify(1, imsi, "uecm-dereg", map[string]string{"deregReason": "SUBSCRIPTION_WITHDRAWN", "accessType": "3GPP_ACCESS"})
		if status != http.StatusNoContent {
			t.Fatalf("deregistration notification: %d %s", status, body)
		}

		run.wait(t, "network-initiated deregistration")
		s.waitRegistered(1, 0, "the withdrawn roamer is gone")

		if reg, _, err := s.udmRegistration(imsi); err != nil || reg.PurgeFlag {
			t.Fatalf("the AMF purged a registration the home UDM had withdrawn: %+v %v", reg, err)
		}
	})

	t.Run("UDM-initiated deregistration of an idle roamer is implicit", func(t *testing.T) {
		imsi := r5gRoamer(6)
		s.tester(1, imsi, "--session=false", "--ping=false", "--deregister=false").wait(t, "roamer registration")
		s.waitRegistered(1, 1, "the idle roamer stays registered")

		status, body := s.notify(1, imsi, "uecm-dereg", map[string]string{"deregReason": "SUBSCRIPTION_WITHDRAWN"})
		if status != http.StatusNoContent {
			t.Fatalf("deregistration notification: %d %s", status, body)
		}

		s.waitRegistered(1, 0, "the idle roamer is removed")

		if status, _ := s.notify(1, imsi, "uecm-dereg", map[string]string{"deregReason": "SUBSCRIPTION_WITHDRAWN"}); status != http.StatusNotFound {
			t.Fatalf("notification for a removed roamer: %d, want 404", status)
		}

		change := map[string]any{"notifyItems": []map[string]any{{"resourceId": "imsi-" + imsi + "/am-data", "changes": []map[string]string{{"op": "REPLACE", "path": "/subscribedUeAmbr"}}}}}
		if status, body := s.notify(1, imsi, "sdm-change", change); status != http.StatusNotFound || !strings.Contains(body, "SUBSCRIPTION_NOT_FOUND") {
			t.Fatalf("data change for a removed roamer: %d %s, want 404 SUBSCRIPTION_NOT_FOUND", status, body)
		}

		if reg, _, err := s.udmRegistration(imsi); err != nil || reg.PurgeFlag {
			t.Fatalf("the AMF purged a registration the home UDM had withdrawn: %+v %v", reg, err)
		}
	})

	t.Run("SST 1 without SD on both sides, with the GPSI from the home UDM as MSISDN", func(t *testing.T) {
		imsi := r5gNoSDRoamer(1)
		run := s.tester(1, imsi, append([]string{"--sd=", "--hold", "10s"}, s.sessionFlags()...)...)
		run.reached(t, "roamer registration")
		s.waitRoamerMSISDN(imsi, "+15550200011")
		run.wait(t, "roamer registration")
	})

	t.Run("IPv4v6 PDU session request with an IPv4-only home subscription is answered IPv4 with #50", func(t *testing.T) {
		s.tester(1, r5gNoSDRoamer(2), "--sd=", "--pdu-session-type", "ipv4v6", "--expect-pdu-session-type", "ipv4", "--expect-gsm-cause", "50").wait(t, "IPv4v6 session")
	})

	t.Run("Profile A SUCI de-concealed by the home UDM over N32", func(t *testing.T) {
		s.tester(1, r5gRoamer(7), append([]string{"--suci-public-key", s.udmProfileAKey(), "--suci-public-key-id", "1"}, s.sessionFlags()...)...).wait(t, "Profile A registration")
	})

	t.Run("UE without S1 mode registers with a single Security Mode Command", func(t *testing.T) {
		s.tester(1, r5gRoamer(8), append([]string{"--no-s1-mode", "--expect-security-mode-commands", "1"}, s.sessionFlags()...)...).wait(t, "registration without S1 mode")
	})

	t.Run("deleting the partner deregisters its roamers with #11", func(t *testing.T) {
		imsi := r5gRoamer(4)
		run := s.tester(1, imsi, "--ping=false", "--await-network-deregistration", "--expect-dereg-cause", "11")
		run.reached(t, "roamer registration")

		if err := s.api.DeleteRoamingPartner(s.ctx, s.partner); err != nil {
			t.Fatalf("delete the partner with a roamer: %v", err)
		}

		run.wait(t, "network-initiated deregistration")

		s.waitUDM(imsi, "purge at the home UDM after the withdrawal", func(r r5gUDMRegistration, status int) bool {
			return status == http.StatusOK && r.PurgeFlag
		})

		s.waitRegistered(1, 0, "no roamer left")
	})
}

func TestRoaming5GOpen5GSHA(t *testing.T) {
	s := r5gBringUp(t, 2)

	imsi := r5gRoamer(5)

	t.Run("a callback for another node's AMF is refused", func(t *testing.T) {
		run := s.tester(2, imsi, "--session=false", "--ping=false", "--deregister=false")
		run.wait(t, "roamer registration on node 2")

		reg := s.waitUDM(imsi, "UECM registration from node 2", func(r r5gUDMRegistration, status int) bool {
			return status == http.StatusOK && !r.PurgeFlag
		})

		if !strings.HasPrefix(reg.DeregCallbackURI, "https://amf2.") {
			t.Fatalf("callback URI %q, want node 2's AMF", reg.DeregCallbackURI)
		}

		status, body := s.notify(2, imsi, "uecm-dereg", map[string]string{"deregReason": "SUBSCRIPTION_WITHDRAWN"})
		if status != http.StatusNotFound || !strings.Contains(body, "CONTEXT_NOT_FOUND") {
			t.Fatalf("callback for node 2 sent to node 1: %d %s", status, body)
		}
	})

	t.Run("the roamer moves to another node", func(t *testing.T) {
		s.tester(1, imsi, "--session=false", "--ping=false", "--deregister=false", "--gnb-index", "1").wait(t, "roamer registration on node 1")

		reg := s.waitUDM(imsi, "UECM registration from node 1", func(r r5gUDMRegistration, status int) bool {
			return status == http.StatusOK && strings.HasPrefix(r.DeregCallbackURI, "https://amf1.")
		})

		if reg.PurgeFlag {
			t.Fatalf("UDM registration purged after the move: %+v", reg)
		}

		s.waitRegistered(2, 0, "node 2 releases the superseded roamer")

		time.Sleep(15 * time.Second)

		if reg, _, err := s.udmRegistration(imsi); err != nil || reg.PurgeFlag || !strings.HasPrefix(reg.DeregCallbackURI, "https://amf1.") {
			t.Fatalf("node 2 purged the moved roamer at the home UDM: %+v %v", reg, err)
		}
	})
}
