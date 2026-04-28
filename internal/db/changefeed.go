// Copyright 2026 Ella Networks

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
	TopicRoutes                 Topic = "routes"
	TopicNATSettings            Topic = "nat_settings"
	TopicFlowAccountingSettings Topic = "flow_accounting_settings"
	TopicN3Settings             Topic = "n3_settings"
	TopicPolicies               Topic = "policies"
	TopicNetworkRules           Topic = "network_rules"
	TopicBGPSettings            Topic = "bgp_settings"
	TopicBGPPeers               Topic = "bgp_peers"
	TopicDataNetworks           Topic = "data_networks"
	TopicIPLeases               Topic = "ip_leases"
)

// Event is published once per (topic, applied-index) and carries no
// row data: subscribers always re-read the relevant table on receipt.
// This keeps the broker zero-copy on the FSM hot path and removes any
// staleness window between event-time and read-time state.
type Event struct {
	Topic Topic
	Index uint64
}

// subscriptionBufferSize bounds per-subscriber memory and matches the
// drop-and-resync contract: when full, the publisher signals
// Subscription.Dropped and the reconciler reconciles from current DB
// state on the next wake.
const subscriptionBufferSize = 128

// Subscription is the consumer-side handle. Events arrive in commit
// order per topic. Dropped fires (closes once) when the publisher
// could not enqueue because the subscriber hasn't drained Events;
// after a Dropped wake-up the subscriber must reconcile from current
// state because individual missed events are unrecoverable.
type Subscription struct {
	Events  <-chan Event
	Dropped <-chan struct{}

	feed    *Changefeed
	topics  map[Topic]struct{}
	events  chan Event
	dropped chan struct{}

	closeMu sync.Mutex
	closed  bool
}

// Close unregisters the subscription from the broker. Idempotent.
func (s *Subscription) Close() {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return
	}

	s.closed = true
	s.feed.unsubscribe(s)
}

// Changefeed is an in-process publish/subscribe broker. One per
// *Database. Publish is non-blocking; the FSM hot path must never
// wait on a slow subscriber.
type Changefeed struct {
	mu   sync.RWMutex
	subs map[*Subscription]struct{}
}

func NewChangefeed() *Changefeed {
	return &Changefeed{subs: make(map[*Subscription]struct{})}
}

// Subscribe registers interest in the given topics. The returned
// subscription must be closed when the consumer is done.
func (c *Changefeed) Subscribe(topics ...Topic) *Subscription {
	events := make(chan Event, subscriptionBufferSize)
	dropped := make(chan struct{}, 1)

	topicSet := make(map[Topic]struct{}, len(topics))
	for _, t := range topics {
		topicSet[t] = struct{}{}
	}

	sub := &Subscription{
		Events:  events,
		Dropped: dropped,
		feed:    c,
		topics:  topicSet,
		events:  events,
		dropped: dropped,
	}

	c.mu.Lock()
	c.subs[sub] = struct{}{}
	c.mu.Unlock()

	return sub
}

// Publish delivers an event to every subscription that watches the
// given topic. Non-blocking: if a subscriber's buffer is full, it
// receives a Dropped signal instead and is expected to resync from
// current state. The publisher never waits.
func (c *Changefeed) Publish(topic Topic, index uint64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ev := Event{Topic: topic, Index: index}

	for sub := range c.subs {
		if _, ok := sub.topics[topic]; !ok {
			continue
		}

		select {
		case sub.events <- ev:
		default:
			select {
			case sub.dropped <- struct{}{}:
			default:
			}
		}
	}
}

func (c *Changefeed) unsubscribe(sub *Subscription) {
	c.mu.Lock()
	delete(c.subs, sub)
	c.mu.Unlock()
}

// Wakeup is a convenience for reconcilers that don't care which event
// fired or whether the buffer dropped — they just want to be told
// "something changed, go reconcile." Returns a coalesced channel and
// a stop function that closes the underlying subscription.
//
// Multiple events between reads coalesce to a single wakeup; the
// receiver will run Reconcile once and observe the latest state.
func (c *Changefeed) Wakeup(topics ...Topic) (<-chan struct{}, func()) {
	sub := c.Subscribe(topics...)
	wakeup := make(chan struct{}, 1)
	stop := make(chan struct{})

	go func() {
		for {
			select {
			case <-stop:
				return
			case <-sub.Events:
			case <-sub.Dropped:
			}

			select {
			case wakeup <- struct{}{}:
			default:
			}
		}
	}()

	cancel := func() {
		close(stop)
		sub.Close()
	}

	return wakeup, cancel
}
