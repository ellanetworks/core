// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package formation

import (
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap"
)

const (
	proofFailureWindow = time.Minute
	proofFailureLimit  = 10
	maxTrackedIPs      = 4096
)

var errNoSecret = errors.New("no cluster secret is configured")

type Auth struct {
	path string
	now  func() time.Time

	mu       sync.Mutex
	last     [][]byte
	failures map[string][]time.Time
	reported bool
}

func NewAuth(path string) *Auth {
	return &Auth{path: path, now: time.Now, failures: map[string][]time.Time{}}
}

func (a *Auth) Enabled() bool {
	return a != nil && a.path != ""
}

func (a *Auth) keys() ([][]byte, error) {
	if !a.Enabled() {
		return nil, errNoSecret
	}

	keys, err := pki.ReadSecrets(a.path)

	a.mu.Lock()
	defer a.mu.Unlock()

	if err != nil {
		if a.last == nil {
			return nil, err
		}

		if !a.reported {
			a.reported = true

			logger.EllaLog.Error("failed to read the cluster secret, using the last one read", zap.Error(err))
		}

		return a.last, nil
	}

	a.last, a.reported = keys, false

	return keys, nil
}

func (a *Auth) prove(in pki.ProofInput) (string, error) {
	keys, err := a.keys()
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(pki.Prove(keys[0], in)), nil
}

func (a *Auth) verify(in pki.ProofInput, proof string) bool {
	keys, err := a.keys()
	if err != nil {
		return false
	}

	raw, err := base64.StdEncoding.DecodeString(proof)
	if err != nil {
		return false
	}

	return pki.VerifyProof(keys, in, raw)
}

func (a *Auth) limited(ip string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return len(a.recent(ip)) >= proofFailureLimit
}

func (a *Auth) recent(ip string) []time.Time {
	cutoff := a.now().Add(-proofFailureWindow)
	kept := a.failures[ip][:0]

	for _, t := range a.failures[ip] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) == 0 {
		delete(a.failures, ip)
		return nil
	}

	a.failures[ip] = kept

	return kept
}

func (a *Auth) fail(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.failures) >= maxTrackedIPs {
		for other := range a.failures {
			a.recent(other)
		}
	}

	if len(a.failures) < maxTrackedIPs || a.failures[ip] != nil {
		a.failures[ip] = append(a.recent(ip), a.now())
	}
}
