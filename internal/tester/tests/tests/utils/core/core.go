package core

import (
	"context"
	"crypto/ecdh"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/ellanetworks/core/client"
)

type SubscriberConfig struct {
	Imsi           string
	Key            string
	SequenceNumber string
	OPc            string
	ProfileName    string
}

type ProfileConfig struct {
	Name           string
	UeAmbrUplink   string
	UeAmbrDownlink string
}

type SliceConfig struct {
	Name string
	SST  int32
	SD   string
}

type PolicyConfig struct {
	Name                string
	ProfileName         string
	SliceName           string
	SessionAmbrUplink   string
	SessionAmbrDownlink string
	Var5qi              int32
	Arp                 int32
	DataNetworkName     string
}

type DataNetworkConfig struct {
	Name   string
	IPPool string
	DNS    string
	Mtu    int32
}

type OperatorID struct {
	MCC string
	MNC string
}

type OperatorTracking struct {
	SupportedTACs []string
}

type OperatorHomeNetwork struct {
	KeyIdentifier int
	Scheme        string
	PrivateKey    string
}

type OperatorConfig struct {
	ID          OperatorID
	Tracking    OperatorTracking
	HomeNetwork OperatorHomeNetwork
}

type EllaCoreConfig struct {
	Operator     OperatorConfig
	Profiles     []ProfileConfig
	Slices       []SliceConfig
	DataNetworks []DataNetworkConfig
	Policies     []PolicyConfig
	Subscribers  []SubscriberConfig
}

type EllaCoreEnv struct {
	Client              *client.Client
	Config              EllaCoreConfig
	createdDataNetworks []string
	createdProfiles     []string
	createdPolicies     []string
	createdSlices       []string
}

func NewEllaCoreEnv(client *client.Client, config EllaCoreConfig) *EllaCoreEnv {
	return &EllaCoreEnv{
		Client: client,
		Config: config,
	}
}

func (c *EllaCoreEnv) Create(ctx context.Context) error {
	err := c.Delete(ctx)
	if err != nil {
		return fmt.Errorf("could not clean up existing EllaCore environment: %v", err)
	}

	err = c.updateOperator(ctx)
	if err != nil {
		return fmt.Errorf("could not update operator: %v", err)
	}

	err = c.createProfiles(ctx)
	if err != nil {
		return fmt.Errorf("could not create profiles: %v", err)
	}

	err = c.createSlices(ctx)
	if err != nil {
		return fmt.Errorf("could not create slices: %v", err)
	}

	err = c.createDataNetworks(ctx)
	if err != nil {
		return fmt.Errorf("could not create data networks: %v", err)
	}

	err = c.createPolicies(ctx)
	if err != nil {
		return fmt.Errorf("could not create policies: %v", err)
	}

	err = c.createSubs(ctx)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("could not create subscribers: %v", err)
	}

	return nil
}

// Delete all subscribers, policies, data networks, slices, and profiles created during the test.
func (c *EllaCoreEnv) Delete(ctx context.Context) error {
	err := c.deleteSubscribers(ctx)
	if err != nil {
		return fmt.Errorf("could not delete subscribers: %v", err)
	}

	err = c.deletePolicies(ctx)
	if err != nil {
		return fmt.Errorf("could not delete policies: %v", err)
	}

	err = c.deleteDataNetworks(ctx)
	if err != nil {
		return fmt.Errorf("could not delete data networks: %v", err)
	}

	err = c.deleteSlices(ctx)
	if err != nil {
		return fmt.Errorf("could not delete slices: %v", err)
	}

	err = c.deleteProfiles(ctx)
	if err != nil {
		return fmt.Errorf("could not delete profiles: %v", err)
	}

	if c.Config.Operator.HomeNetwork.PrivateKey != "" {
		err = c.deleteHomeNetworkKey(ctx, c.Config.Operator.HomeNetwork.KeyIdentifier)
		if err != nil {
			return fmt.Errorf("could not cleanup home network key: %v", err)
		}
	}

	return nil
}

func (c *EllaCoreEnv) deleteHomeNetworkKey(ctx context.Context, keyId int) error {
	opConfig, err := c.Client.GetOperator(ctx)
	if err != nil {
		return fmt.Errorf("failed to get operator: %v", err)
	}

	for _, k := range opConfig.HomeNetworkKeys {
		if k.KeyIdentifier == keyId {
			err = c.Client.DeleteHomeNetworkKey(ctx, k.ID)
			if err != nil && !strings.Contains(err.Error(), "not found") {
				return fmt.Errorf("could not delete home network key: %v", err)
			}
		}
	}

	return nil
}

func (c *EllaCoreEnv) updateOperator(ctx context.Context) error {
	opConfig, err := c.Client.GetOperator(ctx)
	if err != nil {
		return fmt.Errorf("failed to get operator: %v", err)
	}

	err = c.updateOperatorHomeNetworkPublicKey(ctx, opConfig)
	if err != nil {
		return fmt.Errorf("could not update operator home network public key: %v", err)
	}

	if opConfig.ID.Mcc != c.Config.Operator.ID.MCC || opConfig.ID.Mnc != c.Config.Operator.ID.MNC {
		err := c.Client.UpdateOperatorID(ctx, &client.UpdateOperatorIDOptions{
			Mcc: c.Config.Operator.ID.MCC,
			Mnc: c.Config.Operator.ID.MNC,
		})
		if err != nil {
			return fmt.Errorf("failed to update operator ID: %v", err)
		}
	}

	currentTACsMap := make(map[string]bool)
	for _, tac := range opConfig.Tracking.SupportedTacs {
		currentTACsMap[tac] = true
	}

	needUpdate := false

	for _, tac := range c.Config.Operator.Tracking.SupportedTACs {
		if !currentTACsMap[tac] {
			needUpdate = true
			break
		}
	}

	if needUpdate {
		err := c.Client.UpdateOperatorTracking(ctx, &client.UpdateOperatorTrackingOptions{
			SupportedTacs: c.Config.Operator.Tracking.SupportedTACs,
		})
		if err != nil {
			return fmt.Errorf("failed to update operator tracking: %v", err)
		}
	}

	return nil
}

func (c *EllaCoreEnv) updateOperatorHomeNetworkPublicKey(ctx context.Context, opConfig *client.Operator) error {
	if c.Config.Operator.HomeNetwork.PrivateKey == "" {
		return nil
	}

	privKeyBytes, err := hex.DecodeString(c.Config.Operator.HomeNetwork.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to decode private key: %v", err)
	}

	privateKey, err := ecdh.X25519().NewPrivateKey(privKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to generate ECDH key: %v", err)
	}

	pubKeyHex := hex.EncodeToString(privateKey.PublicKey().Bytes())

	for _, key := range opConfig.HomeNetworkKeys {
		if key.PublicKey == pubKeyHex {
			return nil
		}
	}

	err = c.Client.CreateHomeNetworkKey(ctx, &client.CreateHomeNetworkKeyOptions{
		KeyIdentifier: c.Config.Operator.HomeNetwork.KeyIdentifier,
		Scheme:        c.Config.Operator.HomeNetwork.Scheme,
		PrivateKey:    hex.EncodeToString(privateKey.Bytes()),
	})
	if err != nil {
		return fmt.Errorf("failed to create home network key: %v", err)
	}

	return nil
}

func (c *EllaCoreEnv) createProfiles(ctx context.Context) error {
	if len(c.Config.Profiles) == 0 {
		return nil
	}

	existingProfiles, err := c.Client.ListProfiles(ctx, &client.ListParams{
		Page:    1,
		PerPage: 100,
	})
	if err != nil {
		return fmt.Errorf("failed to list profiles: %v", err)
	}

	if len(existingProfiles.Items) == 0 {
		return fmt.Errorf("expected at least 1 existing profile")
	}

	// Update the existing profile with the first config entry.
	err = c.Client.UpdateProfile(ctx, existingProfiles.Items[0].Name, &client.UpdateProfileOptions{
		UeAmbrUplink:   c.Config.Profiles[0].UeAmbrUplink,
		UeAmbrDownlink: c.Config.Profiles[0].UeAmbrDownlink,
	})
	if err != nil {
		return fmt.Errorf("failed to update profile: %v", err)
	}

	// Create additional profiles.
	for _, profile := range c.Config.Profiles[1:] {
		err := c.Client.CreateProfile(ctx, &client.CreateProfileOptions{
			Name:           profile.Name,
			UeAmbrUplink:   profile.UeAmbrUplink,
			UeAmbrDownlink: profile.UeAmbrDownlink,
		})
		if err != nil {
			return fmt.Errorf("failed to create profile: %v", err)
		}

		c.createdProfiles = append(c.createdProfiles, profile.Name)
	}

	return nil
}

func (c *EllaCoreEnv) deleteProfiles(ctx context.Context) error {
	profiles, err := c.Client.ListProfiles(ctx, &client.ListParams{
		Page:    1,
		PerPage: 100,
	})
	if err != nil {
		return fmt.Errorf("failed to list profiles: %v", err)
	}

	// Keep the first profile (the default one), delete the rest.
	for _, profile := range profiles.Items[1:] {
		err := c.Client.DeleteProfile(ctx, &client.DeleteProfileOptions{
			Name: profile.Name,
		})
		if err != nil {
			return fmt.Errorf("failed to delete profile %s: %v", profile.Name, err)
		}
	}

	c.createdProfiles = nil

	return nil
}

func (c *EllaCoreEnv) createSlices(ctx context.Context) error {
	if len(c.Config.Slices) == 0 {
		return nil
	}

	existingSlices, err := c.Client.ListSlices(ctx, &client.ListParams{
		Page:    1,
		PerPage: 100,
	})
	if err != nil {
		return fmt.Errorf("failed to list slices: %v", err)
	}

	if len(existingSlices.Items) == 0 {
		return fmt.Errorf("expected at least 1 existing slice")
	}

	// Update the existing (default) slice with the first config entry.
	first := c.Config.Slices[0]

	err = c.Client.UpdateSlice(ctx, existingSlices.Items[0].Name, &client.UpdateSliceOptions{
		Sst: int(first.SST),
		Sd:  first.SD,
	})
	if err != nil {
		return fmt.Errorf("failed to update slice: %v", err)
	}

	// Create additional slices.
	for _, slice := range c.Config.Slices[1:] {
		err := c.Client.CreateSlice(ctx, &client.CreateSliceOptions{
			Name: slice.Name,
			Sst:  int(slice.SST),
			Sd:   slice.SD,
		})
		if err != nil {
			return fmt.Errorf("failed to create slice %s: %v", slice.Name, err)
		}

		c.createdSlices = append(c.createdSlices, slice.Name)
	}

	return nil
}

func (c *EllaCoreEnv) deleteSlices(ctx context.Context) error {
	for _, name := range c.createdSlices {
		err := c.Client.DeleteSlice(ctx, &client.DeleteSliceOptions{
			Name: name,
		})
		if err != nil && !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("failed to delete slice %s: %v", name, err)
		}
	}

	c.createdSlices = nil

	return nil
}

func (c *EllaCoreEnv) createDataNetworks(ctx context.Context) error {
	for _, dnn := range c.Config.DataNetworks {
		_, err := c.Client.GetDataNetwork(ctx, &client.GetDataNetworkOptions{
			Name: dnn.Name,
		})
		if err == nil {
			continue
		}

		err = c.Client.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
			Name:   dnn.Name,
			IPPool: dnn.IPPool,
			DNS:    dnn.DNS,
			Mtu:    dnn.Mtu,
		})
		if err != nil {
			return fmt.Errorf("failed to create data network: %v", err)
		}

		c.createdDataNetworks = append(c.createdDataNetworks, dnn.Name)
	}

	return nil
}

func (c *EllaCoreEnv) createPolicies(ctx context.Context) error {
	if len(c.Config.Policies) == 0 {
		return nil
	}

	existingPolicies, err := c.Client.ListPolicies(ctx, &client.ListParams{
		Page:    1,
		PerPage: 100,
	})
	if err != nil {
		return fmt.Errorf("failed to list policies: %v", err)
	}

	if len(existingPolicies.Items) == 0 {
		return fmt.Errorf("expected at least 1 existing policy")
	}

	// Update the existing policy with the first config entry.
	first := c.Config.Policies[0]

	err = c.Client.UpdatePolicy(ctx, existingPolicies.Items[0].Name, &client.UpdatePolicyOptions{
		ProfileName:         first.ProfileName,
		SliceName:           first.SliceName,
		SessionAmbrUplink:   first.SessionAmbrUplink,
		SessionAmbrDownlink: first.SessionAmbrDownlink,
		Var5qi:              first.Var5qi,
		Arp:                 first.Arp,
		DataNetworkName:     first.DataNetworkName,
	})
	if err != nil {
		return fmt.Errorf("failed to update policy: %v", err)
	}

	// Create additional policies.
	for _, policy := range c.Config.Policies[1:] {
		err := c.Client.CreatePolicy(ctx, &client.CreatePolicyOptions{
			Name:                policy.Name,
			ProfileName:         policy.ProfileName,
			SliceName:           policy.SliceName,
			SessionAmbrUplink:   policy.SessionAmbrUplink,
			SessionAmbrDownlink: policy.SessionAmbrDownlink,
			Var5qi:              policy.Var5qi,
			Arp:                 policy.Arp,
			DataNetworkName:     policy.DataNetworkName,
		})
		if err != nil {
			return fmt.Errorf("failed to create policy: %v", err)
		}

		c.createdPolicies = append(c.createdPolicies, policy.Name)
	}

	return nil
}

func (c *EllaCoreEnv) createSubs(ctx context.Context) error {
	for _, sub := range c.Config.Subscribers {
		err := c.Client.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
			Imsi:           sub.Imsi,
			Key:            sub.Key,
			SequenceNumber: sub.SequenceNumber,
			ProfileName:    sub.ProfileName,
			OPc:            sub.OPc,
		})
		if err != nil {
			return fmt.Errorf("failed to create subscriber: %v", err)
		}
	}

	return nil
}

func (c *EllaCoreEnv) deleteSubscribers(ctx context.Context) error {
	perPage := 100

	for {
		subs, err := c.Client.ListSubscribers(ctx, &client.ListSubscribersParams{
			Page:    1,
			PerPage: perPage,
		})
		if err != nil {
			return fmt.Errorf("failed to list subscribers: %v", err)
		}

		for _, sub := range subs.Items {
			err := c.Client.DeleteSubscriber(ctx, &client.DeleteSubscriberOptions{
				ID: sub.Imsi,
			})
			if err != nil {
				return fmt.Errorf("failed to delete subscriber %s: %v", sub.Imsi, err)
			}
		}

		if len(subs.Items) < perPage {
			break
		}
	}

	return nil
}

func (c *EllaCoreEnv) deletePolicies(ctx context.Context) error {
	policies, err := c.Client.ListPolicies(ctx, &client.ListParams{
		Page:    1,
		PerPage: 100,
	})
	if err != nil {
		return fmt.Errorf("failed to list policies: %v", err)
	}

	// Keep the first policy (the default one), delete the rest.
	for _, policy := range policies.Items[1:] {
		err := c.Client.DeletePolicy(ctx, &client.DeletePolicyOptions{
			Name: policy.Name,
		})
		if err != nil {
			return fmt.Errorf("failed to delete policy %s: %v", policy.Name, err)
		}
	}

	c.createdPolicies = nil

	return nil
}

func (c *EllaCoreEnv) deleteDataNetworks(ctx context.Context) error {
	policies, err := c.Client.ListPolicies(ctx, &client.ListParams{
		Page:    1,
		PerPage: 100,
	})
	if err != nil {
		return fmt.Errorf("failed to list policies: %v", err)
	}

	referencedDNs := make(map[string]bool)
	for _, policy := range policies.Items {
		referencedDNs[policy.DataNetworkName] = true
	}

	dataNetworks, err := c.Client.ListDataNetworks(ctx, &client.ListParams{
		Page:    1,
		PerPage: 100,
	})
	if err != nil {
		return fmt.Errorf("failed to list data networks: %v", err)
	}

	// Keep the first data network (the default one) and any referenced by policies.
	for _, dn := range dataNetworks.Items[1:] {
		if referencedDNs[dn.Name] {
			continue
		}

		err := c.Client.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{
			Name: dn.Name,
		})
		if err != nil {
			return fmt.Errorf("failed to delete data network %s: %v", dn.Name, err)
		}
	}

	c.createdDataNetworks = nil

	return nil
}

func WaitForUsage(cl *client.Client, imsi string, timeout time.Duration) (uint64, uint64, error) {
	deadline := time.Now().Add(timeout)

	for {
		uplinkBytes, downlinkBytes, err := getEllaCoreUsage(cl, imsi)
		if err != nil {
			return 0, 0, fmt.Errorf("could not get EllaCore usage: %v", err)
		}

		if uplinkBytes > 0 && downlinkBytes > 0 {
			return uplinkBytes, downlinkBytes, nil
		}

		if time.Now().After(deadline) {
			return 0, 0, fmt.Errorf("timeout waiting for usage for IMSI %s", imsi)
		}

		time.Sleep(1 * time.Second)
	}
}

func getEllaCoreUsage(cl *client.Client, imsi string) (uint64, uint64, error) {
	usage, err := cl.ListUsage(context.Background(), &client.ListUsageParams{
		GroupBy:    "day",
		Subscriber: imsi,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("could not list usage: %v", err)
	}

	today := time.Now().Format("2006-01-02")

	var totalUplinkBytes uint64

	var totalDownlinkBytes uint64

	for _, subscriberUsage := range *usage {
		ku, ok := subscriberUsage[today]
		if !ok {
			continue
		}

		totalUplinkBytes += uint64(ku.UplinkBytes)
		totalDownlinkBytes += uint64(ku.DownlinkBytes)
	}

	return totalUplinkBytes, totalDownlinkBytes, nil
}
