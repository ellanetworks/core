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
	TopicFlowAccountingSettings Topic = "flow_accounting_settings"
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
	TopicUERegistrations        Topic = "ue_registrations"
	TopicSMSSettings            Topic = "sms_settings"
	TopicIMSSettings            Topic = "ims_settings"
	TopicOperatorIdentity       Topic = "operator_identity"
	TopicJWTSecret              Topic = "jwt_secret"
	TopicRoaming                Topic = "roaming"
	TopicSEPPIdentity           Topic = "sepp_identity"
	TopicOIDC                   Topic = "oidc"
	TopicRadioEventSettings     Topic = "radio_event_settings"
)

// Changefeed is an in-process publish/subscribe broker. One per
// *Database. Publish is non-blocking; the FSM hot path must never
// wait on a slow subscriber.
type Changefeed struct {
	mu        sync.Mutex
	subs      map[*subscription]struct{}
	published map[Topic]uint64
	all       uint64
	latest    uint64
	trackers  map[*Tracker]struct{}
	armed     bool
	realized  uint64
}

type subscription struct {
	topics map[Topic]struct{}
	wakeup chan struct{}
}

func NewChangefeed() *Changefeed {
	return &Changefeed{
		subs:      make(map[*subscription]struct{}),
		published: make(map[Topic]uint64),
		trackers:  make(map[*Tracker]struct{}),
	}
}

func (c *Changefeed) Publish(topic Topic, index uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.published[topic] = max(c.published[topic], index)
	c.latest = max(c.latest, index)

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

func (c *Changefeed) PublishAll(index uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.all = max(c.all, index)
	c.latest = max(c.latest, index)

	for sub := range c.subs {
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
	sub := c.subscribe(topics)

	return sub.wakeup, func() { c.unsubscribe(sub) }
}

func (c *Changefeed) subscribe(topics []Topic) *subscription {
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

	return sub
}

func (c *Changefeed) unsubscribe(sub *subscription) {
	c.mu.Lock()
	delete(c.subs, sub)
	c.mu.Unlock()
}

type Tracker struct {
	feed   *Changefeed
	sub    *subscription
	passed bool
	synced uint64
}

func (c *Changefeed) Track(topics ...Topic) *Tracker {
	t := &Tracker{feed: c, sub: c.subscribe(topics)}

	c.mu.Lock()
	c.trackers[t] = struct{}{}
	c.mu.Unlock()

	return t
}

func (t *Tracker) C() <-chan struct{} {
	return t.sub.wakeup
}

func (t *Tracker) Begin() uint64 {
	t.feed.mu.Lock()
	defer t.feed.mu.Unlock()

	return t.feed.latest
}

func (t *Tracker) Done(index uint64) {
	t.feed.mu.Lock()
	defer t.feed.mu.Unlock()

	t.passed = true
	t.synced = max(t.synced, index)
}

func (t *Tracker) Stop() {
	t.feed.mu.Lock()
	delete(t.feed.trackers, t)
	t.feed.armed = false
	t.feed.mu.Unlock()

	t.feed.unsubscribe(t.sub)
}

func (c *Changefeed) targetLocked(t *Tracker) uint64 {
	target := c.all

	for topic := range t.sub.topics {
		target = max(target, c.published[topic])
	}

	return target
}

func (c *Changefeed) Arm() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.armed = true
}

func (c *Changefeed) Realized(applied uint64) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.armed {
		return 0
	}

	realized := applied

	for t := range c.trackers {
		if !t.passed || t.synced < c.targetLocked(t) {
			realized = min(realized, t.synced)
		}
	}

	c.realized = max(c.realized, realized)

	return c.realized
}
