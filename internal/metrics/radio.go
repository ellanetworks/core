// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package metrics

import (
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

const radioIDLabel = "radio_id"

type radioSeries struct {
	mu      sync.RWMutex
	ids     sync.Map
	pruning atomic.Bool
	known   func() []string

	signaling     *prometheus.CounterVec
	registrations *prometheus.CounterVec
}

var radios *radioSeries

func newRadioSeries(known func() []string) *radioSeries {
	return &radioSeries{
		known: known,
		signaling: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "app_radio_signaling_messages_total",
			Help: "Radio signaling messages (NGAP for 5G, S1AP for 4G) exchanged with a radio that holds a global RAN node ID, by radio, direction, and type. Series of a radio this node no longer knows are removed. Join with app_radio_info on radio_id for the radio's name.",
		}, []string{radioIDLabel, "direction", "type"}),
		registrations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "app_radio_registration_attempts_total",
			Help: "UE registration (5G) and attach/tracking-area-update (4G) attempts, by the radio serving the UE, type, and result. Series of a radio this node no longer knows are removed. Join with app_radio_info on radio_id for the radio's name.",
		}, []string{radioIDLabel, "type", "result"}),
	}
}

func RegisterRadioSeries(known func() []string) {
	radios = newRadioSeries(known)

	prometheus.MustRegister(radios)
}

func RadioSignalingMessage(radioID, direction, msgType string) {
	radios.signalingMessage(radioID, direction, msgType)
}

func RadioRegistrationAttempt(radioID, regType, result string) {
	radios.registrationAttempt(radioID, regType, result)
}

func (s *radioSeries) signalingMessage(radioID, direction, msgType string) {
	if s == nil {
		return
	}

	s.inc(s.signaling, radioID, direction, msgType)
}

func (s *radioSeries) registrationAttempt(radioID, regType, result string) {
	if s == nil {
		return
	}

	s.inc(s.registrations, radioID, regType, result)
}

func (s *radioSeries) inc(vec *prometheus.CounterVec, radioID string, lvs ...string) {
	if radioID == "" {
		return
	}

	s.mu.RLock()

	_, seen := s.ids.LoadOrStore(radioID, struct{}{})

	vec.WithLabelValues(append([]string{radioID}, lvs...)...).Inc()

	s.mu.RUnlock()

	if !seen {
		s.schedulePrune()
	}
}

func (s *radioSeries) schedulePrune() {
	if !s.pruning.CompareAndSwap(false, true) {
		return
	}

	go func() {
		defer s.pruning.Store(false)

		s.prune()
	}()
}

func (s *radioSeries) prune() {
	if s == nil || s.known == nil {
		return
	}

	var tracked []string

	s.ids.Range(func(key, _ any) bool {
		tracked = append(tracked, key.(string))

		return true
	})

	if len(tracked) == 0 {
		return
	}

	known := make(map[string]struct{})
	for _, id := range s.known() {
		known[id] = struct{}{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range tracked {
		if _, ok := known[id]; ok {
			continue
		}

		match := prometheus.Labels{radioIDLabel: id}

		s.signaling.DeletePartialMatch(match)
		s.registrations.DeletePartialMatch(match)
		s.ids.Delete(id)
	}
}

func (s *radioSeries) Describe(ch chan<- *prometheus.Desc) {
	s.signaling.Describe(ch)
	s.registrations.Describe(ch)
}

func (s *radioSeries) Collect(ch chan<- prometheus.Metric) {
	s.prune()

	s.signaling.Collect(ch)
	s.registrations.Collect(ch)
}
