// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/cluster/formation"
	"github.com/ellanetworks/core/internal/cluster/pkiagent"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	ellapki "github.com/ellanetworks/core/internal/pki"
)

func newFormation(cfg config.Config, raftID string, p *pkiState, dbInstance *db.Database) *formation.Node {
	return formation.New(formation.Config{
		NodeID:      raftID,
		Advertise:   cfg.Cluster.AdvertiseAddress,
		BindAddress: cfg.Cluster.BindAddress,
		Join:        cfg.Cluster.Join,
		Expect:      cfg.Cluster.BootstrapExpect,
		Schema:      db.SchemaVersion(),
		Auth:        formation.NewAuth(cfg.Cluster.SecretFile),
		Local: func(context.Context) formation.LocalState {
			switch {
			case p.agent.Leaf() == nil && !p.agent.HaveLeafOnDisk():
				return formation.LocalState{State: formation.StateForming}
			case p.agent.Leaf() == nil || dbInstance.DiscoveryPending():
				return formation.LocalState{State: formation.StateJoining}
			}

			return formation.LocalState{State: formation.StateMember, ClusterID: p.agent.ClusterID, Leader: dbInstance.IsLeader()}
		},
		Register: func(ctx context.Context, nodeID string, certPEM []byte) (formation.RegisterReply, error) {
			if normalized, err := ellapki.NormalizeNodeID(nodeID); err != nil || normalized != nodeID {
				return formation.RegisterReply{}, fmt.Errorf("node id %q is not a canonical node id", nodeID)
			}

			issuer := p.Issuer()
			if issuer == nil {
				return formation.RegisterReply{}, errors.New("cluster PKI is not ready")
			}

			_, pins, err := issuer.RegisterJoiningCert(ctx, nodeID, certPEM)
			if err != nil {
				return formation.RegisterReply{}, err
			}

			records := make([]pkiagent.PinRecord, 0, len(pins))
			for _, pin := range pins {
				records = append(records, pkiagent.PinRecord{NodeID: ellapki.NodeID(pin.NodeID), Fingerprint: pin.Fingerprint})
			}

			return formation.RegisterReply{ClusterID: p.agent.ClusterID, Pins: records}, nil
		},
		LeafClusterID: p.agent.LeafClusterIDOnDisk,
		PrepareJoin:   p.agent.PrepareSecretJoin,
		CompleteJoin:  p.agent.CompleteSecretJoin,
	})
}

func formCluster(ctx context.Context, former *formation.Node, p *pkiState, dbInstance *db.Database) error {
	out, err := former.Run(ctx)
	if err != nil {
		return err
	}

	if out.Bootstrap {
		dbInstance.SetBootstrap()

		return nil
	}

	if err := p.agent.Load(); err != nil {
		return fmt.Errorf("load leaf: %w", err)
	}

	p.SeedPinsFromAgentDisk()
	dbInstance.SetJoinSeeds(out.Seeds, "nonvoter")

	return nil
}
