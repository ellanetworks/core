package core

import (
	"context"
	"crypto/ecdh"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ellanetworks/core/client"
)

type SubscriberConfig struct {
	Imsi           string
	Key            string
	SequenceNumber string
	OPc            string
	PolicyName     string
}

type PolicyConfig struct {
	Name            string
	BitrateUplink   string
	BitrateDownlink string
	Var5qi          int32
	Arp             int32
	DataNetworkName string
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

type OperatorSlice struct {
	SST int32
	SD  string
}

type OperatorTracking struct {
	SupportedTACs []string
}

type OperatorHomeNetwork struct {
	PrivateKey string
}

type OperatorConfig struct {
	ID          OperatorID
	Slice       OperatorSlice
	Tracking    OperatorTracking
	HomeNetwork OperatorHomeNetwork
}

type EllaCoreConfig struct {
	Operator     OperatorConfig
	DataNetworks []DataNetworkConfig
	Policies     []PolicyConfig
	Subscribers  []SubscriberConfig
}

type EllaCoreEnv struct {
	Client *client.Client
	Config EllaCoreConfig
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

	err = c.createDataNetworks(ctx)
	if err != nil {
		return fmt.Errorf("could not create data networks: %v", err)
	}

	err = c.createPolicies(ctx)
	if err != nil {
		return fmt.Errorf("could not create policies: %v", err)
	}

	err = c.createSubs(ctx)
	if err != nil {
		return fmt.Errorf("could not create subscribers: %v", err)
	}

	return nil
}

// Delete all subscribers, policies, and data networks created during the test.
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

	if opConfig.Slice.Sst != int(c.Config.Operator.Slice.SST) || opConfig.Slice.Sd != c.Config.Operator.Slice.SD {
		err := c.Client.UpdateOperatorSlice(ctx, &client.UpdateOperatorSliceOptions{
			Sst: int(c.Config.Operator.Slice.SST),
			Sd:  c.Config.Operator.Slice.SD,
		})
		if err != nil {
			return fmt.Errorf("failed to update operator slice: %v", err)
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

	pubKeyBytes := privateKey.PublicKey().Bytes()

	if opConfig.HomeNetwork.PublicKey == hex.EncodeToString(pubKeyBytes) {
		return nil
	}

	err = c.Client.UpdateOperatorHomeNetwork(ctx, &client.UpdateOperatorHomeNetworkOptions{
		PrivateKey: hex.EncodeToString(privateKey.Bytes()),
	})
	if err != nil {
		return fmt.Errorf("failed to update operator home network: %v", err)
	}

	return nil
}

func (c *EllaCoreEnv) createDataNetworks(ctx context.Context) error {
	for _, dnn := range c.Config.DataNetworks {
		err := c.Client.CreateDataNetwork(ctx, &client.CreateDataNetworkOptions{
			Name:   dnn.Name,
			IPPool: dnn.IPPool,
			DNS:    dnn.DNS,
			Mtu:    dnn.Mtu,
		})
		if err != nil {
			return fmt.Errorf("failed to create data network: %v", err)
		}
	}

	return nil
}

func (c *EllaCoreEnv) createPolicies(ctx context.Context) error {
	for _, policy := range c.Config.Policies {
		err := c.Client.CreatePolicy(ctx, &client.CreatePolicyOptions{
			Name:            policy.Name,
			BitrateUplink:   policy.BitrateUplink,
			BitrateDownlink: policy.BitrateDownlink,
			Var5qi:          policy.Var5qi,
			Arp:             policy.Arp,
			DataNetworkName: policy.DataNetworkName,
		})
		if err != nil {
			return fmt.Errorf("failed to create policy: %v", err)
		}
	}

	return nil
}

func (c *EllaCoreEnv) createSubs(ctx context.Context) error {
	for _, sub := range c.Config.Subscribers {
		err := c.Client.CreateSubscriber(ctx, &client.CreateSubscriberOptions{
			Imsi:           sub.Imsi,
			Key:            sub.Key,
			SequenceNumber: sub.SequenceNumber,
			PolicyName:     sub.PolicyName,
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
		subs, err := c.Client.ListSubscribers(ctx, &client.ListParams{
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
	pols, err := c.Client.ListPolicies(ctx, &client.ListParams{
		Page:    1,
		PerPage: 100,
	})
	if err != nil {
		return fmt.Errorf("failed to list policies: %v", err)
	}

	for _, policy := range pols.Items {
		err := c.Client.DeletePolicy(ctx, &client.DeletePolicyOptions{
			Name: policy.Name,
		})
		if err != nil {
			return fmt.Errorf("failed to delete policy %s: %v", policy.Name, err)
		}
	}

	return nil
}

func (c *EllaCoreEnv) deleteDataNetworks(ctx context.Context) error {
	dnns, err := c.Client.ListDataNetworks(ctx, &client.ListParams{
		Page:    1,
		PerPage: 100,
	})
	if err != nil {
		return fmt.Errorf("failed to list data networks: %v", err)
	}

	for _, dnn := range dnns.Items {
		err := c.Client.DeleteDataNetwork(ctx, &client.DeleteDataNetworkOptions{
			Name: dnn.Name,
		})
		if err != nil {
			return fmt.Errorf("failed to delete data network %s: %v", dnn.Name, err)
		}
	}

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
