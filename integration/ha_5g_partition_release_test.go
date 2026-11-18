// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const (
	partitionReleaseHold     = 60 * time.Second
	partitionReleaseDeadline = 2 * time.Minute
)

func TestIntegration5GHAPartitionReleasesLease(t *testing.T) {
	suites.Require(t, suites.HA3GPP5G)

	runHA3GPPScenario(t, "ha/partition_release_5g", func(ctx context.Context, ha *client.Client, nodeID client.NodeID) error {
		dc, err := NewDockerClient()
		if err != nil {
			return fmt.Errorf("docker client: %w", err)
		}

		defer func() { _ = dc.Close() }()

		service, err := ha3GPPServiceOf(ctx, nodeID)
		if err != nil {
			return err
		}

		container, err := dc.ResolveComposeContainer(ctx, "ha-3gpp", service)
		if err != nil {
			return fmt.Errorf("resolve %s container: %w", service, err)
		}

		if err := partitionClusterPort(ctx, dc, container); err != nil {
			return fmt.Errorf("partition %s: %w", service, err)
		}

		healed := false

		defer func() {
			if !healed {
				_ = healClusterPort(context.Background(), dc, container)
			}
		}()

		HALogf(t, "partitioned %s (node %s); the UE deregisters on it", service, nodeID)

		select {
		case <-time.After(partitionReleaseHold):
		case <-ctx.Done():
			return ctx.Err()
		}

		held, err := ipv4AllocationHeld(ctx, ha, scenarios.DefaultDNN, scenarios.DefaultIMSI)
		if err != nil {
			return err
		}

		if !held {
			return fmt.Errorf("the lease of %s was released during the partition, so the partition did not make the release fail", scenarios.DefaultIMSI)
		}

		if err := healClusterPort(ctx, dc, container); err != nil {
			return fmt.Errorf("heal %s: %w", service, err)
		}

		healed = true

		HALogf(t, "healed %s; waiting for the lease the failed release left to be released", service)

		deadline := time.Now().Add(partitionReleaseDeadline)

		for {
			held, err := ipv4AllocationHeld(ctx, ha, scenarios.DefaultDNN, scenarios.DefaultIMSI)
			if err == nil && !held {
				HALogf(t, "lease of %s released after the heal", scenarios.DefaultIMSI)
				return nil
			}

			if time.Now().After(deadline) {
				return fmt.Errorf("lease of %s still held %v after the heal (last error: %v)", scenarios.DefaultIMSI, partitionReleaseDeadline, err)
			}

			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})
}

func ha3GPPServiceOf(ctx context.Context, nodeID client.NodeID) (string, error) {
	for i, url := range []string{"http://10.100.0.11:5002", "http://10.100.0.12:5002", "http://10.100.0.13:5002"} {
		c, err := client.New(&client.Config{BaseURLs: []string{url}})
		if err != nil {
			return "", fmt.Errorf("client for %s: %w", url, err)
		}

		id, err := nodeIDOf(ctx, c)
		if err != nil {
			continue
		}

		if id == nodeID {
			return fmt.Sprintf("ella-core-%d", i+1), nil
		}
	}

	return "", fmt.Errorf("no node reports node id %s", nodeID)
}

func ipv4AllocationHeld(ctx context.Context, c *client.Client, dnn, imsi string) (bool, error) {
	resp, err := c.ListIPv4Allocations(ctx, &client.ListIPAllocationsOptions{DataNetworkName: dnn}, &client.ListParams{Page: 1, PerPage: 100})
	if err != nil {
		return false, fmt.Errorf("list IPv4 allocations: %w", err)
	}

	for _, a := range resp.Items {
		if a.IMSI == imsi && a.SessionID != nil {
			return true, nil
		}
	}

	return false, nil
}
