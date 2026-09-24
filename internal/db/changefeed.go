// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"sync"
)

// Topic identifies a stream of change events. One topic per replicated
// SQL table that has a reconciler watching it.
type Topic string

const (
	// Topics are co-located here so authors can see, at a glance,
	// the full set of replicated tables that drive runtime state.
	// Adding a new reconciler is two lines: declare the topic, mark
	// the ops that touch it via AffectsTopic in operations_register.go.
	TopicNATSettings            Topic = "nat_settings"
	TopicFlowAccountingSettings Topic = "flow_accounting_settings"
	TopicLocalSwitchSettings    Topic = "local_switch_settings"
	TopicN3Settings             Topic = "n3_settings"
	TopicPolicies               Topic = "policies"
	TopicNetworkRules           Topic = "network_rules"
	TopicBGPSettings            Topic = "bgp_settings"
	TopicBGPPeers               Topic = "bgp_peers"
	TopicDataNetworks           Topic = "data_networks"
	TopicIPLeases               Topic = "ip_leases"
	TopicClusterNodeCerts       Topic = "cluster_node_certs"
	TopicSessionReconcile       Topic = "session_reconcile"
	TopicClusterMembers         Topic = "cluster_members"
	TopicFramedRoutes           Topic = "subscriber_framed_routes"
)

// Changefeed is an in-process publish/subscribe broker. One per
// *Database. Publish is non-blocking; the FSM hot path must never
// wait on a slow subscriber.
type Changefeed struct {
	mu   sync.Mutex
	subs map[*subscription]struct{}
}

type subscription struct {
	topics map[Topic]struct{}
	wakeup chan struct{}
}

func NewChangefeed() *Changefeed {
	return &Changefeed{subs: make(map[*subscription]struct{})}
}

func (c *Changefeed) Publish(topic Topic) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for sub := range c.subs {
		if _, ok := sub.topics[topic]; !ok {
			continue
		}

		select {
		case sub.wakeup <- struct{}{}:
		default:
		}
	}
}

// Wakeup tells reconcilers "something changed, go reconcile." Returns a
// coalesced channel and a stop function that closes the underlying
// subscription.
//
// Multiple events between reads coalesce to a single wakeup; the
// receiver will run Reconcile once and observe the latest state.
func (c *Changefeed) Wakeup(topics ...Topic) (<-chan struct{}, func()) {
	sub := &subscription{
		topics: make(map[Topic]struct{}, len(topics)),
		wakeup: make(chan struct{}, 1),
	}

	for _, t := range topics {
		sub.topics[t] = struct{}{}
	}

	c.mu.Lock()
	c.subs[sub] = struct{}{}
	c.mu.Unlock()

	stop := func() {
		c.mu.Lock()
		delete(c.subs, sub)
		c.mu.Unlock()
	}

	return sub.wakeup, stop
}
