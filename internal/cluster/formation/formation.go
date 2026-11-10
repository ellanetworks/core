// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package formation

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/cluster/pkiagent"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	StateForming = "forming"
	StateJoining = "joining"
	StateMember  = "member"

	discoverInterval = 2 * time.Second
)

type Hello struct {
	NodeID    string `json:"nodeId"`
	Address   string `json:"address"`
	State     string `json:"state"`
	ClusterID string `json:"clusterId,omitempty"`
	Expect    int    `json:"bootstrapExpect,omitempty"`
	Candidate string `json:"candidate,omitempty"`
	Schema    int    `json:"schemaVersion"`
	Leader    bool   `json:"leader,omitempty"`
}

type RegisterRequest struct {
	NodeID  string `json:"nodeId"`
	CertPEM string `json:"certPEM"`
}

type RegisterReply struct {
	ClusterID string               `json:"clusterId"`
	Pins      []pkiagent.PinRecord `json:"pins"`
}

type LocalState struct {
	State     string
	ClusterID string
	Leader    bool
}

type Config struct {
	NodeID      string
	Advertise   string
	BindAddress string
	Join        []string
	Expect      int
	Schema      int
	Auth        *Auth

	Local        func(ctx context.Context) LocalState
	Register     func(ctx context.Context, nodeID string, certPEM []byte) (RegisterReply, error)
	HaveLeaf     func() bool
	PrepareJoin  func(clusterID string) ([]byte, error)
	CompleteJoin func(clusterID string, pins []pkiagent.PinRecord) error

	Dial     func(ctx context.Context, addr string) (*tls.Conn, error)
	LookupIP func(ctx context.Context, host string) ([]netip.Addr, error)
}

type Outcome struct {
	Bootstrap bool
	Seeds     []string
}

type Node struct {
	cfg Config

	mu          sync.Mutex
	candidate   string
	logged      string
	wrongSecret int
}

func New(cfg Config) *Node {
	return &Node{cfg: cfg}
}

func (n *Node) selfHello(ctx context.Context) Hello {
	h := Hello{NodeID: n.cfg.NodeID, Address: n.cfg.Advertise, Expect: n.cfg.Expect, Schema: n.cfg.Schema}

	local := LocalState{State: StateForming}
	if n.cfg.Local != nil {
		local = n.cfg.Local(ctx)
	}

	switch local.State {
	case StateMember:
		h.State, h.ClusterID, h.Leader = StateMember, local.ClusterID, local.Leader
	case StateJoining:
		h.State = StateJoining
	default:
		n.mu.Lock()
		h.State, h.Candidate = StateForming, n.candidate
		n.mu.Unlock()
	}

	return h
}

type action int

const (
	actionWait action = iota
	actionJoin
	actionBootstrap
)

type decision struct {
	action    action
	members   []string
	clusterID string
	found     int
	candidate string
	detail    string
	blocked   bool
}

func decide(self Hello, peers []Hello) decision {
	var forming, members []Hello

	clusters := map[string]bool{}

	for _, p := range peers {
		if p.NodeID == self.NodeID {
			continue
		}

		switch p.State {
		case StateMember:
			members = append(members, p)
			clusters[p.ClusterID] = true
		case StateForming:
			forming = append(forming, p)
		}
	}

	if len(clusters) > 1 {
		return decision{blocked: true, detail: fmt.Sprintf("the peers belong to %d different clusters", len(clusters))}
	}

	if len(members) > 0 {
		slices.SortFunc(members, func(a, b Hello) int {
			switch {
			case a.Leader && !b.Leader:
				return -1
			case b.Leader && !a.Leader:
				return 1
			}

			return strings.Compare(a.NodeID, b.NodeID)
		})

		d := decision{action: actionJoin, clusterID: members[0].ClusterID, detail: "joining cluster " + members[0].ClusterID}

		for _, m := range members {
			d.members = append(d.members, m.Address)
		}

		return d
	}

	if self.Expect == 0 {
		return decision{detail: "waiting for a cluster member to answer"}
	}

	group := append([]Hello{self}, forming...)

	for _, p := range group {
		switch {
		case p.Expect != self.Expect:
			return decision{blocked: true, detail: fmt.Sprintf("node %s expects %d nodes, this node %d: set the same "+
				"cluster.bootstrap-expect on every node", p.NodeID, p.Expect, self.Expect)}
		case p.Schema != self.Schema:
			return decision{blocked: true, detail: fmt.Sprintf("node %s runs schema %d, this node %d: run the same version on every node",
				p.NodeID, p.Schema, self.Schema)}
		}
	}

	d := decision{found: len(group)}

	if len(group) < self.Expect {
		d.detail = fmt.Sprintf("found %d of %d nodes", len(group), self.Expect)
		return d
	}

	d.candidate = self.NodeID

	for _, p := range forming {
		d.candidate = min(d.candidate, p.NodeID)
	}

	if d.candidate != self.NodeID {
		d.detail = "waiting for node " + d.candidate + " to form the cluster"
		return d
	}

	for _, p := range forming {
		if p.Candidate != self.NodeID {
			d.detail = "waiting for every node to agree that this node forms the cluster"
			return d
		}
	}

	d.action, d.detail = actionBootstrap, "forming the cluster"

	return d
}

func (n *Node) lookup(ctx context.Context, host string) ([]netip.Addr, error) {
	if n.cfg.LookupIP != nil {
		return n.cfg.LookupIP(ctx, host)
	}

	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

func (n *Node) resolveJoin(ctx context.Context) []string {
	var out []string

	seen := map[string]bool{}

	for _, target := range n.cfg.Join {
		host, port, err := net.SplitHostPort(target)
		if err != nil {
			continue
		}

		var addrs []netip.Addr

		if ip, err := netip.ParseAddr(host); err == nil {
			addrs = []netip.Addr{ip}
		} else {
			addrs, err = n.lookup(ctx, host)
			if err != nil {
				logger.EllaLog.Debug("failed to resolve a cluster.join name", zap.String("name", host), zap.Error(err))
				continue
			}
		}

		for _, a := range addrs {
			addr := net.JoinHostPort(a.Unmap().String(), port)
			if !seen[addr] {
				seen[addr] = true
				out = append(out, addr)
			}
		}
	}

	return out
}

type round struct {
	peers       []Hello
	wrongSecret int
}

func (n *Node) helloRound(ctx context.Context) round {
	self := n.selfHello(ctx)

	var (
		mu  sync.Mutex
		out round
		wg  sync.WaitGroup
	)

	byID := map[string]bool{}

	for _, addr := range n.resolveJoin(ctx) {
		wg.Go(func() {
			r, err := n.call(ctx, addr, PathHello, self)

			mu.Lock()
			defer mu.Unlock()

			switch {
			case errors.Is(err, ErrWrongSecret), errors.Is(err, errBadProof):
				out.wrongSecret++
				return
			case err != nil:
				logger.EllaLog.Debug("cluster hello failed", zap.String("address", addr), zap.Error(err))
				return
			case r.status != http.StatusOK:
				return
			}

			var h Hello
			if err := json.Unmarshal(r.body, &h); err != nil || h.NodeID != r.nodeID || h.NodeID == n.cfg.NodeID || byID[h.NodeID] {
				return
			}

			byID[h.NodeID] = true
			out.peers = append(out.peers, h)
		})
	}

	wg.Wait()

	slices.SortFunc(out.peers, func(a, b Hello) int { return strings.Compare(a.NodeID, b.NodeID) })

	return out
}

func (n *Node) logOnce(key string, log func()) {
	n.mu.Lock()
	changed := n.logged != key
	n.logged = key
	n.mu.Unlock()

	if changed {
		log()
	}
}

func (n *Node) evaluate(ctx context.Context) decision {
	rd := n.helloRound(ctx)

	n.mu.Lock()
	changed := rd.wrongSecret != n.wrongSecret
	n.wrongSecret = rd.wrongSecret
	n.mu.Unlock()

	if changed && rd.wrongSecret > 0 {
		logger.EllaLog.Warn("peers refused this node's cluster secret, or answered without proving it: "+
			"check that every node has the same cluster secret", zap.Int("peers", rd.wrongSecret))
	}

	d := decide(n.selfHello(ctx), rd.peers)

	n.mu.Lock()
	n.candidate = d.candidate
	n.mu.Unlock()

	return d
}

func (n *Node) Run(ctx context.Context) (Outcome, error) {
	if !n.cfg.Auth.Enabled() {
		return Outcome{}, errors.New("cluster.join is set, but no cluster secret is configured: set cluster.secret-file")
	}

	logger.EllaLog.Info("Forming or joining a cluster from cluster.join",
		zap.Strings("join", n.cfg.Join), zap.Int("bootstrap_expect", n.cfg.Expect))

	for {
		wait := discoverInterval/2 + rand.N(discoverInterval)

		select {
		case <-ctx.Done():
			return Outcome{}, ctx.Err()
		case <-time.After(wait):
		}

		d := n.evaluate(ctx)

		switch {
		case d.blocked:
			n.logOnce("blocked:"+d.detail, func() {
				logger.EllaLog.Error("not forming the cluster: "+d.detail, zap.Int("expected", n.cfg.Expect))
			})
		case d.action == actionWait:
			n.logOnce(fmt.Sprintf("wait:%d:%s", d.found, d.detail), func() {
				logger.EllaLog.Info("waiting for the cluster to form", zap.Int("found", d.found),
					zap.Int("expected", n.cfg.Expect), zap.String("detail", d.detail))
			})
		case d.action == actionJoin:
			out, err := n.join(ctx, d)
			if err == nil {
				logger.EllaLog.Info("registered with the cluster, joining it", zap.String("cluster", d.clusterID),
					zap.Strings("members", d.members))

				return out, nil
			}

			n.logOnce("join:"+err.Error(), func() {
				logger.EllaLog.Warn("failed to join the cluster, retrying", zap.String("cluster", d.clusterID), zap.Error(err))
			})
		case d.action == actionBootstrap:
			if again := n.evaluate(ctx); again.action != actionBootstrap {
				continue
			}

			logger.EllaLog.Info("forming the cluster: the other nodes join it", zap.Int("nodes", n.cfg.Expect))

			return Outcome{Bootstrap: true}, nil
		}
	}
}

func (n *Node) join(ctx context.Context, d decision) (Outcome, error) {
	if d.clusterID == "" {
		return Outcome{}, errors.New("the cluster member reported no cluster id")
	}

	if n.cfg.HaveLeaf() {
		return Outcome{Seeds: d.members}, nil
	}

	certPEM, err := n.cfg.PrepareJoin(d.clusterID)
	if err != nil {
		return Outcome{}, err
	}

	last := errors.New("no member to join")

	for _, addr := range d.members {
		r, err := n.call(ctx, addr, PathRegister, RegisterRequest{NodeID: n.cfg.NodeID, CertPEM: string(certPEM)})
		if err != nil {
			last = err
			continue
		}

		if r.status != http.StatusCreated {
			var e rpcError

			_ = json.Unmarshal(r.body, &e)
			last = fmt.Errorf("member %s at %s refused the registration (status %d): %s", r.nodeID, addr, r.status, e.Error)

			continue
		}

		var out RegisterReply
		if err := json.Unmarshal(r.body, &out); err != nil {
			last = err
			continue
		}

		if out.ClusterID != d.clusterID {
			last = fmt.Errorf("member %s at %s registered this node in cluster %s, expected %s", r.nodeID, addr, out.ClusterID, d.clusterID)
			continue
		}

		if err := n.cfg.CompleteJoin(d.clusterID, out.Pins); err != nil {
			return Outcome{}, err
		}

		return Outcome{Seeds: d.members}, nil
	}

	return Outcome{}, last
}
