// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/tester/scenarios"
)

type sliceDataNetworkRef struct {
	slice, dataNetwork string
	ims                bool
}

func createServedPolicy(ctx context.Context, cl *client.Client, opts *client.CreatePolicyOptions) error {
	if _, err := cl.GetSliceDataNetwork(ctx, opts.SliceName, opts.DataNetworkName); err != nil {
		if err := cl.CreateSliceDataNetwork(ctx, opts.SliceName, &client.CreateSliceDataNetworkOptions{DataNetwork: opts.DataNetworkName}); err != nil {
			return fmt.Errorf("serve data network %s on slice %s: %w", opts.DataNetworkName, opts.SliceName, err)
		}
	}

	return cl.CreatePolicy(ctx, opts)
}

func servedDataNetworks(ctx context.Context, cl *client.Client) ([]sliceDataNetworkRef, error) {
	slices, err := cl.ListSlices(ctx, &client.ListParams{Page: 1, PerPage: 100})
	if err != nil {
		return nil, fmt.Errorf("list slices: %w", err)
	}

	dataNetworks, err := cl.ListDataNetworks(ctx, &client.ListParams{Page: 1, PerPage: 100})
	if err != nil {
		return nil, fmt.Errorf("list data networks: %w", err)
	}

	ims := make(map[string]bool, len(dataNetworks.Items))
	for _, dn := range dataNetworks.Items {
		ims[dn.Name] = dn.IMS
	}

	var refs []sliceDataNetworkRef

	for _, slice := range slices.Items {
		served, err := cl.ListSliceDataNetworks(ctx, slice.Name)
		if err != nil {
			return nil, fmt.Errorf("list the data networks of slice %s: %w", slice.Name, err)
		}

		for _, sdn := range served.Items {
			refs = append(refs, sliceDataNetworkRef{slice: slice.Name, dataNetwork: sdn.DataNetwork, ims: ims[sdn.DataNetwork]})
		}
	}

	return refs, nil
}

func updateServedDataNetworks(ctx context.Context, cl *client.Client, opts client.UpdateSliceDataNetworkOptions) error {
	refs, err := servedDataNetworks(ctx, cl)
	if err != nil {
		return err
	}

	for _, ref := range refs {
		if ref.ims {
			continue
		}

		if err := cl.UpdateSliceDataNetwork(ctx, ref.slice, ref.dataNetwork, &opts); err != nil {
			return fmt.Errorf("update data network %s on slice %s: %w", ref.dataNetwork, ref.slice, err)
		}
	}

	return nil
}

func setNATOnAllDataNetworks(ctx context.Context, cl *client.Client, enabled bool) error {
	return updateServedDataNetworks(ctx, cl, client.UpdateSliceDataNetworkOptions{NAT: &enabled})
}

func setLocalSwitchOnAllDataNetworks(ctx context.Context, cl *client.Client, enabled bool) error {
	return updateServedDataNetworks(ctx, cl, client.UpdateSliceDataNetworkOptions{LocalSwitch: &enabled})
}

func defaultDataNetworkForwarding(ctx context.Context, cl *client.Client) (nat, localSwitch bool, err error) {
	sdn, err := cl.GetSliceDataNetwork(ctx, scenarios.DefaultSliceName, scenarios.DefaultDNN)
	if err != nil {
		return false, false, fmt.Errorf("get the default data network on the default slice: %w", err)
	}

	return sdn.NAT, sdn.LocalSwitch, nil
}

func updateDefaultDataNetworkForwarding(ctx context.Context, cl *client.Client, nat, localSwitch *bool) error {
	return cl.UpdateSliceDataNetwork(ctx, scenarios.DefaultSliceName, scenarios.DefaultDNN, &client.UpdateSliceDataNetworkOptions{NAT: nat, LocalSwitch: localSwitch})
}
