// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package formation

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/pkiagent"
	"github.com/ellanetworks/core/internal/pki"
)

func TestDecide(t *testing.T) {
	forming := func(id string, expect int, candidate string) Hello {
		return Hello{NodeID: id, State: StateForming, Expect: expect, Candidate: candidate, Schema: 7}
	}

	cases := []struct {
		name    string
		self    Hello
		peers   []Hello
		action  action
		blocked bool
		detail  string
	}{
		{
			name:   "waits until enough nodes answer",
			self:   forming("a", 3, ""),
			peers:  []Hello{forming("b", 3, "")},
			detail: "found 2 of 3 nodes",
		},
		{
			name:   "lowest node waits for every peer to name it",
			self:   forming("a", 3, ""),
			peers:  []Hello{forming("b", 3, "a"), forming("c", 3, "")},
			detail: "waiting for every node to agree",
		},
		{
			name:   "lowest node bootstraps once every peer names it",
			self:   forming("a", 3, "a"),
			peers:  []Hello{forming("b", 3, "a"), forming("c", 3, "a")},
			action: actionBootstrap,
		},
		{
			name:   "other nodes wait for the candidate",
			self:   forming("b", 3, "a"),
			peers:  []Hello{forming("a", 3, "a"), forming("c", 3, "a")},
			detail: "waiting for node a",
		},
		{
			name:    "different bootstrap-expect blocks",
			self:    forming("a", 3, ""),
			peers:   []Hello{forming("b", 5, ""), forming("c", 3, "")},
			blocked: true,
			detail:  "set the same cluster.bootstrap-expect",
		},
		{
			name: "different schema blocks",
			self: forming("a", 3, ""),
			peers: []Hello{
				{NodeID: "b", State: StateForming, Expect: 3, Schema: 8}, forming("c", 3, ""),
			},
			blocked: true,
			detail:  "runs schema 8",
		},
		{
			name: "a member wins over forming",
			self: forming("a", 3, "a"),
			peers: []Hello{
				forming("b", 3, "a"),
				{NodeID: "z", State: StateMember, ClusterID: "c1", Address: "10.0.0.26:7000"},
			},
			action: actionJoin,
		},
		{
			name: "members of two clusters block",
			self: forming("a", 0, ""),
			peers: []Hello{
				{NodeID: "y", State: StateMember, ClusterID: "c1"},
				{NodeID: "z", State: StateMember, ClusterID: "c2"},
			},
			blocked: true,
			detail:  "2 different clusters",
		},
		{
			name:   "without bootstrap-expect a node only joins",
			self:   forming("a", 0, ""),
			peers:  []Hello{forming("b", 0, "")},
			detail: "waiting for a cluster member",
		},
		{
			name: "joining peers are not counted",
			self: forming("a", 2, "a"),
			peers: []Hello{
				{NodeID: "b", State: StateJoining, Expect: 2, Schema: 7},
			},
			detail: "found 1 of 2 nodes",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := decide(tc.self, tc.peers)

			if d.blocked != tc.blocked {
				t.Fatalf("blocked = %v, want %v (%s)", d.blocked, tc.blocked, d.detail)
			}

			if !tc.blocked && d.action != tc.action {
				t.Fatalf("action = %v, want %v (%s)", d.action, tc.action, d.detail)
			}

			if tc.detail != "" && !strings.Contains(d.detail, tc.detail) {
				t.Fatalf("detail = %q, want it to contain %q", d.detail, tc.detail)
			}
		})
	}
}

func TestDecideJoinPrefersLeader(t *testing.T) {
	d := decide(Hello{NodeID: "a", State: StateForming}, []Hello{
		{NodeID: "b", State: StateMember, ClusterID: "c1", Address: "b:7000"},
		{NodeID: "c", State: StateMember, ClusterID: "c1", Address: "c:7000", Leader: true},
	})

	if d.action != actionJoin || d.clusterID != "c1" || strings.Join(d.members, ",") != "c:7000,b:7000" {
		t.Fatalf("decision = %+v, want a join of c1 trying the leader first", d)
	}
}

func writeSecretFile(t *testing.T, secrets ...string) string {
	t.Helper()

	p := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(p, []byte(strings.Join(secrets, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	return p
}

const (
	secretA = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	secretB = "ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA="
)

func selfSigned(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

type bufferedWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferedWriter) Header() http.Header { return b.header }

func (b *bufferedWriter) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}

func (b *bufferedWriter) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}

	return b.body.Write(p)
}

func serve(t *testing.T, n *Node) string {
	t.Helper()

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{selfSigned(t)},
		NextProtos:   []string{listener.ALPNPKIBootstrap},
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}

			go func() {
				defer func() { _ = conn.Close() }()

				if err := conn.(*tls.Conn).HandshakeContext(context.Background()); err != nil {
					return
				}

				req, err := http.ReadRequest(bufio.NewReader(conn))
				if err != nil {
					return
				}

				w := &bufferedWriter{header: http.Header{}}
				n.ServeSecret(conn, w, req)

				resp := &http.Response{
					StatusCode: w.status, ProtoMajor: 1, ProtoMinor: 1, Header: w.header,
					Body: io.NopCloser(&w.body), ContentLength: int64(w.body.Len()),
				}

				_ = resp.Write(conn)
			}()
		}
	}()

	return ln.Addr().String()
}

type fakeNode struct {
	mu        sync.Mutex
	state     string
	clusterID string
	leaf      bool
	pending   string
	pins      []pkiagent.PinRecord
	admitted  []string
}

func (f *fakeNode) local(context.Context) LocalState {
	f.mu.Lock()
	defer f.mu.Unlock()

	return LocalState{State: f.state, ClusterID: f.clusterID, Leader: f.state == StateMember}
}

func newTestNode(t *testing.T, id, secretPath string, expect int, f *fakeNode, dial func(context.Context, string) (*tls.Conn, error)) (*Node, string) {
	t.Helper()

	n := New(Config{
		NodeID: id, Expect: expect, Schema: 7, Auth: NewAuth(secretPath),
		Local: f.local,
		Register: func(_ context.Context, nodeID string, certPEM []byte) (RegisterReply, error) {
			f.mu.Lock()
			defer f.mu.Unlock()

			f.admitted = append(f.admitted, nodeID+":"+string(certPEM))

			return RegisterReply{ClusterID: f.clusterID, Pins: []pkiagent.PinRecord{{NodeID: pki.NodeID(nodeID), Fingerprint: "fp-" + nodeID}}}, nil
		},
		HaveLeaf: func() bool {
			f.mu.Lock()
			defer f.mu.Unlock()

			return f.leaf
		},
		PrepareJoin: func(clusterID string) ([]byte, error) {
			f.mu.Lock()
			defer f.mu.Unlock()

			f.pending = clusterID

			return []byte("cert-of-" + id), nil
		},
		CompleteJoin: func(clusterID string, pins []pkiagent.PinRecord) error {
			f.mu.Lock()
			defer f.mu.Unlock()

			if clusterID != f.pending {
				return fmt.Errorf("completed %s, prepared %s", clusterID, f.pending)
			}

			f.leaf, f.state, f.pins = true, StateJoining, pins

			return nil
		},
		Dial: dial,
	})

	addr := serve(t, n)
	n.cfg.Advertise = addr

	return n, addr
}

func clientDial(ctx context.Context, addr string) (*tls.Conn, error) {
	var d tls.Dialer

	d.Config = &tls.Config{
		MinVersion: tls.VersionTLS13, NextProtos: []string{listener.ALPNPKIBootstrap},
		InsecureSkipVerify: true, // #nosec G402 -- test peer, authenticated by the secret proof
	}

	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	return conn.(*tls.Conn), nil
}

func TestThreeNodesFormOneCluster(t *testing.T) {
	secret := writeSecretFile(t, secretA)
	ids := []string{"0192a000-0000-7000-8000-00000000000c", "0192a000-0000-7000-8000-00000000000a", "0192a000-0000-7000-8000-00000000000b"}

	fakes := make([]*fakeNode, len(ids))
	nodes := make([]*Node, len(ids))
	addrs := make([]string, len(ids))

	for i, id := range ids {
		fakes[i] = &fakeNode{state: StateForming}
		nodes[i], addrs[i] = newTestNode(t, id, secret, 3, fakes[i], clientDial)
	}

	for _, n := range nodes {
		n.cfg.Join = addrs
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	type result struct {
		i   int
		out Outcome
		err error
	}

	results := make(chan result, len(nodes))

	for i, n := range nodes {
		go func() {
			out, err := n.Run(ctx)
			if err == nil && out.Bootstrap {
				fakes[i].mu.Lock()
				fakes[i].state, fakes[i].clusterID, fakes[i].leaf = StateMember, "cluster-1", true
				fakes[i].mu.Unlock()
			}

			results <- result{i, out, err}
		}()
	}

	bootstrapped := -1

	for range nodes {
		r := <-results
		if r.err != nil {
			t.Fatalf("node %d: %v", r.i, r.err)
		}

		if r.out.Bootstrap {
			if bootstrapped != -1 {
				t.Fatalf("nodes %d and %d both formed a cluster", bootstrapped, r.i)
			}

			bootstrapped = r.i
		}
	}

	if bootstrapped != 1 {
		t.Fatalf("node %d formed the cluster, want node 1, which has the lowest node id", bootstrapped)
	}

	for i, f := range fakes {
		if i == bootstrapped {
			if len(f.admitted) != 2 {
				t.Errorf("the founder admitted %v, want the other two nodes", f.admitted)
			}

			continue
		}

		if f.pending != "cluster-1" || !f.leaf || len(f.pins) != 1 {
			t.Errorf("node %d did not register with cluster-1: %+v", i, f)
		}
	}
}

func TestWrongSecretIsRefused(t *testing.T) {
	member := &fakeNode{state: StateMember, clusterID: "cluster-1", leaf: true}
	_, memberAddr := newTestNode(t, "0192a000-0000-7000-8000-00000000000a", writeSecretFile(t, secretA), 0, member, clientDial)

	joiner := &fakeNode{state: StateForming}
	j, _ := newTestNode(t, "0192a000-0000-7000-8000-00000000000b", writeSecretFile(t, secretB), 0, joiner, clientDial)

	_, err := j.call(context.Background(), memberAddr, PathHello, j.selfHello(context.Background()))
	if !errors.Is(err, ErrWrongSecret) {
		t.Fatalf("err = %v, want ErrWrongSecret", err)
	}

	_, err = j.call(context.Background(), memberAddr, PathRegister, RegisterRequest{NodeID: j.cfg.NodeID, CertPEM: "x"})
	if !errors.Is(err, ErrWrongSecret) {
		t.Fatalf("register err = %v, want ErrWrongSecret", err)
	}

	if len(member.admitted) != 0 {
		t.Fatalf("a node with the wrong secret was admitted: %v", member.admitted)
	}
}

func TestRotatedSecretIsAccepted(t *testing.T) {
	member := &fakeNode{state: StateMember, clusterID: "cluster-1", leaf: true}
	_, memberAddr := newTestNode(t, "0192a000-0000-7000-8000-00000000000a", writeSecretFile(t, secretA, secretB), 0, member, clientDial)

	joiner := &fakeNode{state: StateForming}
	j, _ := newTestNode(t, "0192a000-0000-7000-8000-00000000000b", writeSecretFile(t, secretB, secretA), 0, joiner, clientDial)

	r, err := j.call(context.Background(), memberAddr, PathHello, j.selfHello(context.Background()))
	if err != nil || r.status != http.StatusOK {
		t.Fatalf("status = %d, err = %v: nodes signing with different secrets must accept each other while both files list both", r.status, err)
	}
}

func TestRegisterRefusedBeforeMembership(t *testing.T) {
	secret := writeSecretFile(t, secretA)

	forming := &fakeNode{state: StateForming}
	_, addr := newTestNode(t, "0192a000-0000-7000-8000-00000000000a", secret, 3, forming, clientDial)

	joiner := &fakeNode{state: StateForming}
	j, _ := newTestNode(t, "0192a000-0000-7000-8000-00000000000b", secret, 3, joiner, clientDial)

	r, err := j.call(context.Background(), addr, PathRegister, RegisterRequest{NodeID: j.cfg.NodeID, CertPEM: "x"})
	if err != nil {
		t.Fatal(err)
	}

	if r.status != http.StatusServiceUnavailable || len(forming.admitted) != 0 {
		t.Fatalf("status = %d, admitted = %v: a forming node must not register joiners", r.status, forming.admitted)
	}
}

func TestRegisterMustBeForTheCaller(t *testing.T) {
	secret := writeSecretFile(t, secretA)

	member := &fakeNode{state: StateMember, clusterID: "cluster-1", leaf: true}
	_, addr := newTestNode(t, "0192a000-0000-7000-8000-00000000000a", secret, 0, member, clientDial)

	joiner := &fakeNode{state: StateForming}
	j, _ := newTestNode(t, "0192a000-0000-7000-8000-00000000000b", secret, 0, joiner, clientDial)

	r, err := j.call(context.Background(), addr, PathRegister, RegisterRequest{NodeID: "0192a000-0000-7000-8000-00000000000c", CertPEM: "x"})
	if err != nil {
		t.Fatal(err)
	}

	if r.status != http.StatusBadRequest || len(member.admitted) != 0 {
		t.Fatalf("status = %d, admitted = %v: a node must not register another node id", r.status, member.admitted)
	}
}
