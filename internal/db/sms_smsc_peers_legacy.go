// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
)

type legacySMSCPeer struct {
	ID               string   `json:"id"`
	DiameterIdentity string   `json:"diameter_identity"`
	Address          string   `json:"address"`
	Port             int      `json:"port"`
	ServiceCentres   []string `json:"service_centres"`
}

var errSMSCPeersRetired = fmt.Errorf("%w: SMSC peers are now SMSC nodes", ErrRetiredOperation)

func (db *Database) applyCreateSMSCPeer(context.Context, *legacySMSCPeer) (any, error) {
	return nil, errSMSCPeersRetired
}

func (db *Database) applyUpdateSMSCPeer(context.Context, *legacySMSCPeer) (any, error) {
	return nil, errSMSCPeersRetired
}

func (db *Database) applyDeleteSMSCPeer(context.Context, *stringPayload) (any, error) {
	return nil, errSMSCPeersRetired
}

func legacySMSCName(identity, address string, taken []string) string {
	base := cmp.Or(identity, address)
	name := base

	for i := 2; slices.ContainsFunc(taken, func(t string) bool { return strings.EqualFold(t, name) }); i++ {
		name = fmt.Sprintf("%s (%d)", base, i)
	}

	return name
}
