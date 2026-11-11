// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package formation

import (
	"encoding/base64"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap"
)

const (
	proofFailureWindow = time.Minute
	proofFailureLimit  = 10
)

var errNoSecret = errors.New("no cluster secret is configured")

type Auth struct {
	path string
	now  func() time.Time

	mu       sync.Mutex
	last     [][]byte
	failures []time.Time
	reported bool
}

func NewAuth(path string) *Auth {
	return &Auth{path: path, now: time.Now}
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

func (a *Auth) limited() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return len(a.recent()) >= proofFailureLimit
}

func (a *Auth) recent() []time.Time {
	cutoff := a.now().Add(-proofFailureWindow)

	a.failures = slices.DeleteFunc(a.failures, func(t time.Time) bool { return !t.After(cutoff) })

	return a.failures
}

func (a *Auth) fail() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.recent()) < proofFailureLimit {
		a.failures = append(a.failures, a.now())
	}
}
