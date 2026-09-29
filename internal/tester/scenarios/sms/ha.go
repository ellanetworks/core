// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sms

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const (
	haMarker           = "PHASE1_DONE"
	haNodes            = 3
	nodeFailureTimeout = 60 * time.Second
	haDeliveryTimeout  = 90 * time.Second
)

func init() {
	register("ha_sms/cross_node_routing", pair("4g", 1100), runCrossNodeRouting)
	register("ha_sms/ue_moves", pair("4g", 1200)[:1], runUEMoves)
	register("ha_sms/node_failure", pair("4g", 1300)[:1], runNodeFailure)
}

type cluster struct {
	nodes []*lte
	hosts []string
	smsc  *smscClient
}

func startCluster(env scenarios.Env, p *params) (*cluster, error) {
	if len(env.CoreN2Addresses) != haNodes || len(p.NodeHosts) != haNodes {
		return nil, fmt.Errorf("HA SMS scenarios need %d cores and %d --node-host values, got %d and %d",
			haNodes, haNodes, len(env.CoreN2Addresses), len(p.NodeHosts))
	}

	smsc, err := newSMSC(p)
	if err != nil {
		return nil, err
	}

	c := &cluster{hosts: p.NodeHosts, smsc: smsc}

	for i, core := range env.CoreN2Addresses {
		l, err := startLTEAt(env, core, i)
		if err != nil {
			c.Close()
			return nil, fmt.Errorf("eNB on node %d: %w", i+1, err)
		}

		c.nodes = append(c.nodes, l)
	}

	return c, nil
}

func (c *cluster) Close() {
	for _, l := range c.nodes {
		l.Close()
	}
}

func (c *cluster) deliveredBy(ctx context.Context, id int64, node int) error {
	fetch := func(ctx context.Context) (smscMessage, error) { return c.smsc.message(ctx, id) }

	return c.smsc.waitFor(ctx, haDeliveryTimeout, fmt.Sprintf("message %d to be delivered by node %d (%s)", id, node+1, c.hosts[node]), fetch,
		func(m smscMessage) bool { return m.deliveredByHost(c.hosts[node]) })
}

func (c *cluster) inject(ctx context.Context, to phone, text string, node int) error {
	id, err := c.smsc.submit(ctx, to.Number(), text)
	if err != nil {
		return err
	}

	if err := expectText(ctx, to, injectedFrom, text); err != nil {
		return err
	}

	return c.deliveredBy(ctx, id, node)
}

func runCrossNodeRouting(ctx context.Context, env scenarios.Env, p *params) error {
	c, err := startCluster(env, p)
	if err != nil {
		return err
	}

	defer c.Close()

	subs := pair("4g", 1100)

	a, err := c.nodes[0].Attach(subs[0].IMSI, subs[0].MSISDN)
	if err != nil {
		return fmt.Errorf("attach the sender on node 1: %w", err)
	}

	defer a.Close()

	b, err := c.nodes[1].Attach(subs[1].IMSI, subs[1].MSISDN)
	if err != nil {
		return fmt.Errorf("attach the recipient on node 2: %w", err)
	}

	defer b.Close()

	routedBy := map[string]bool{}

	for i := range haNodes {
		text := fmt.Sprintf("Across the cluster %d", i)

		if err := submit(ctx, p, a, b.Number(), text); err != nil {
			return err
		}

		if err := expectText(ctx, b, a.Number(), text); err != nil {
			return err
		}

		m, err := c.smsc.latestTo(ctx, b.Number())
		if err != nil {
			return err
		}

		if err := c.deliveredBy(ctx, m.ID, 1); err != nil {
			return err
		}

		m, err = c.smsc.message(ctx, m.ID)
		if err != nil {
			return err
		}

		for _, host := range m.routedBy() {
			routedBy[strings.ToLower(host)] = true
		}
	}

	for host := range routedBy {
		if host != strings.ToLower(c.hosts[1]) {
			return nil
		}
	}

	return fmt.Errorf("every routing query was answered by the serving node (%v); cross-node routing was not exercised", routedBy)
}

func runUEMoves(ctx context.Context, env scenarios.Env, p *params) error {
	c, err := startCluster(env, p)
	if err != nil {
		return err
	}

	defer c.Close()

	sub := pair("4g", 1200)[0]

	b, err := c.nodes[1].Attach(sub.IMSI, sub.MSISDN)
	if err != nil {
		return fmt.Errorf("attach on node 2: %w", err)
	}

	if err := c.inject(ctx, b, "Before moving", 1); err != nil {
		b.Close()
		return err
	}

	if err := b.SwitchOff(); err != nil {
		b.Close()
		return fmt.Errorf("switch off on node 2: %w", err)
	}

	b.Close()

	moved, err := c.nodes[2].Attach(sub.IMSI, sub.MSISDN)
	if err != nil {
		return fmt.Errorf("attach on node 3: %w", err)
	}

	defer moved.Close()

	return c.inject(ctx, moved, "After moving", 2)
}

func runNodeFailure(ctx context.Context, env scenarios.Env, p *params) error {
	c, err := startCluster(env, p)
	if err != nil {
		return err
	}

	defer c.Close()

	sub := pair("4g", 1300)[0]

	b, err := c.nodes[1].Attach(sub.IMSI, sub.MSISDN)
	if err != nil {
		return fmt.Errorf("attach on node 2: %w", err)
	}

	defer b.Close()

	if err := c.inject(ctx, b, "Before the failure", 1); err != nil {
		return err
	}

	fmt.Println(haMarker)

	_ = os.Stdout.Sync()

	waitCtx, cancel := context.WithTimeout(ctx, nodeFailureTimeout)
	peer, err := c.nodes[1].enb.WaitForActivePeerChange(waitCtx)

	cancel()

	if err != nil {
		return fmt.Errorf("node 2 was not lost: %w", err)
	}

	if peer != "" {
		return fmt.Errorf("the eNB of node 2 moved to %s instead of losing its MME", peer)
	}

	moved, err := c.nodes[2].Attach(sub.IMSI, sub.MSISDN)
	if err != nil {
		return fmt.Errorf("attach on node 3 after node 2 failed: %w", err)
	}

	defer moved.Close()

	return c.inject(ctx, moved, "After the failure", 2)
}
